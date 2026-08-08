package identity_test

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

func TestCreateStoresAUserAndAssignsItsIdentity(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	before := time.Now().UTC().Truncate(time.Microsecond)

	created, err := r.users.Create(t.Context(), identity.NewUser{
		Email:        "Alice@Example.com",
		DisplayName:  "Alice",
		PasswordHash: "argon2id$hash",
		PlatformRole: authz.PlatformRoleAdmin,
		Status:       identity.StatusInvited,
		MFAEnforced:  true,
	})
	if err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}

	if created.ID == "" {
		t.Error("Create() returned a user with no identifier")
	}
	// The address is kept as typed: it is how its owner writes it, and how it
	// belongs in a report.
	if created.Email != "Alice@Example.com" {
		t.Errorf("Email = %q, want the address as it was given", created.Email)
	}
	if created.PlatformRole != authz.PlatformRoleAdmin {
		t.Errorf("PlatformRole = %q, want %q", created.PlatformRole, authz.PlatformRoleAdmin)
	}
	if created.Status != identity.StatusInvited {
		t.Errorf("Status = %q, want %q", created.Status, identity.StatusInvited)
	}
	if !created.MFAEnforced {
		t.Error("MFAEnforced = false, want true")
	}
	if created.PasswordHash != "argon2id$hash" {
		t.Errorf("PasswordHash = %q, want what was stored", created.PasswordHash)
	}
	if created.CreatedAt.Before(before) || created.CreatedAt != created.UpdatedAt {
		t.Errorf("CreatedAt = %s and UpdatedAt = %s, want one moment at or after %s",
			created.CreatedAt, created.UpdatedAt, before)
	}
	if !created.LastLoginAt.IsZero() {
		t.Errorf("LastLoginAt = %s on a new account, want the zero time", created.LastLoginAt)
	}

	// What Create returned is what was stored, not a copy of what went in.
	read, err := r.users.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() = %v, want nil", err)
	}
	if read != created {
		t.Errorf("ByID() = %+v, want the created user %+v", read, created)
	}
}

// TestAnSSOOnlyUserHasNoPassword covers the nullable column: an empty hash is
// stored as NULL and read back as empty, so a caller never sees "" as if it
// were a hash somebody could match against.
func TestAnSSOOnlyUserHasNoPassword(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	created, err := r.users.Create(t.Context(), identity.NewUser{
		Email:        "sso@example.com",
		DisplayName:  "SSO",
		PlatformRole: authz.PlatformRoleMember,
		Status:       identity.StatusActive,
	})
	if err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}
	if created.PasswordHash != "" {
		t.Errorf("PasswordHash = %q on an SSO-only account, want empty", created.PasswordHash)
	}

	var isNull bool
	if err := r.db.Read().QueryRowContext(t.Context(),
		`SELECT password_hash IS NULL FROM app."user" WHERE id = ?`, created.ID).Scan(&isNull); err != nil {
		t.Fatalf("reading the stored hash: %v", err)
	}
	if !isNull {
		t.Error("an absent password is stored as something other than NULL")
	}
}

// TestByEmailIgnoresCaseAndSurroundingSpace is the case-insensitivity
// requirement from the lookup side. The uniqueness side is in schema_test.go.
func TestByEmailIgnoresCaseAndSurroundingSpace(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	created := mustCreateUser(t, r, "Alice@Example.com")

	for _, spelling := range []string{
		"Alice@Example.com",
		"alice@example.com",
		"ALICE@EXAMPLE.COM",
		"aLiCe@ExAmPlE.cOm",
		"  alice@example.com  ",
	} {
		found, err := r.users.ByEmail(t.Context(), spelling)
		if err != nil {
			t.Errorf("ByEmail(%q) = %v, want the account", spelling, err)
			continue
		}
		if found.ID != created.ID {
			t.Errorf("ByEmail(%q) found %q, want %q", spelling, found.ID, created.ID)
		}
	}
}

// TestCreateTrimsTheStoredAddress: the display column is trimmed too, so it
// cannot disagree with the normalized one — the CHECK in 0002_identity.sql
// would reject the row if it did.
func TestCreateTrimsTheStoredAddress(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	created, err := r.users.Create(t.Context(), member("  Spaced@Example.com  "))
	if err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}
	if created.Email != "Spaced@Example.com" {
		t.Errorf("Email = %q, want it trimmed", created.Email)
	}
}

func TestCreateRefusesAnEmailAlreadyInUse(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	mustCreateUser(t, r, "alice@example.com")

	// Different casing: the whole point of the normalized column.
	_, err := r.users.Create(t.Context(), member("Alice@Example.com"))
	if err == nil {
		t.Fatal("Create() = nil, want a failure: that address is taken")
	}
	if !errors.Is(err, apierr.ErrConflict) {
		t.Errorf("Create() = %v, want a conflict — this is the caller's to fix, not a server fault", err)
	}
}

func TestByIDAndByEmailReportNotFound(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	mustCreateUser(t, r, "alice@example.com")

	if _, err := r.users.ByID(t.Context(), "no-such-id"); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("ByID() = %v, want not found", err)
	}
	if _, err := r.users.ByEmail(t.Context(), "nobody@example.com"); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("ByEmail() = %v, want not found", err)
	}
}

func TestUpdateWritesTheMutableFields(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	created := mustCreateUser(t, r, "alice@example.com")

	edited := created
	edited.Email = "Alice.New@Example.com"
	edited.DisplayName = "Alice Renamed"
	edited.PasswordHash = "argon2id$rotated"
	edited.PlatformRole = authz.PlatformRoleAdmin
	edited.Status = identity.StatusDisabled
	edited.MFAEnforced = true

	updated, err := r.users.Update(t.Context(), edited)
	if err != nil {
		t.Fatalf("Update() = %v, want nil", err)
	}

	if updated.Email != "Alice.New@Example.com" || updated.DisplayName != "Alice Renamed" {
		t.Errorf("Update() left %q/%q", updated.Email, updated.DisplayName)
	}
	if updated.PlatformRole != authz.PlatformRoleAdmin || updated.Status != identity.StatusDisabled {
		t.Errorf("Update() left role %q status %q", updated.PlatformRole, updated.Status)
	}
	if updated.PasswordHash != "argon2id$rotated" || !updated.MFAEnforced {
		t.Errorf("Update() left hash %q mfaEnforced %v", updated.PasswordHash, updated.MFAEnforced)
	}
	if updated.CreatedAt != created.CreatedAt {
		t.Errorf("CreatedAt moved from %s to %s", created.CreatedAt, updated.CreatedAt)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) && updated.UpdatedAt != created.UpdatedAt {
		t.Errorf("UpdatedAt = %s, want it at or after %s", updated.UpdatedAt, created.UpdatedAt)
	}

	// The lookup key moved with the address.
	found, err := r.users.ByEmail(t.Context(), "alice.new@example.com")
	if err != nil || found.ID != created.ID {
		t.Errorf("ByEmail(new address) = %+v, %v; want the same account", found, err)
	}
	if _, err := r.users.ByEmail(t.Context(), "alice@example.com"); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("the old address still resolves: %v", err)
	}
}

// TestUpdateDoesNotDisturbTheLoginTimestamp: last_login_at belongs to logging
// in, so a read-modify-write of a stale User cannot roll back a login that
// happened while somebody was editing the account.
func TestUpdateDoesNotDisturbTheLoginTimestamp(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	created := mustCreateUser(t, r, "alice@example.com")

	loginAt := time.Now().UTC().Truncate(time.Microsecond)
	if err := r.users.SetLastLoginAt(t.Context(), created.ID, loginAt); err != nil {
		t.Fatalf("SetLastLoginAt() = %v, want nil", err)
	}

	// created still carries the zero LastLoginAt from before the login.
	stale := created
	stale.DisplayName = "Renamed"
	updated, err := r.users.Update(t.Context(), stale)
	if err != nil {
		t.Fatalf("Update() = %v, want nil", err)
	}
	if !updated.LastLoginAt.Equal(loginAt) {
		t.Errorf("LastLoginAt = %s after an unrelated update, want %s", updated.LastLoginAt, loginAt)
	}
}

func TestUpdateRefusesAnEmailAnotherUserHolds(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	mustCreateUser(t, r, "alice@example.com")
	bob := mustCreateUser(t, r, "bob@example.com")

	bob.Email = "ALICE@example.com"
	if _, err := r.users.Update(t.Context(), bob); !errors.Is(err, apierr.ErrConflict) {
		t.Errorf("Update() = %v, want a conflict", err)
	}
}

func TestUpdateAndSetLastLoginAtReportAMissingUser(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	ghost := identity.User{
		ID: "no-such-id", Email: "ghost@example.com", DisplayName: "Ghost",
		PlatformRole: authz.PlatformRoleMember, Status: identity.StatusActive,
	}

	if _, err := r.users.Update(t.Context(), ghost); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("Update() = %v, want not found", err)
	}
	if err := r.users.SetLastLoginAt(t.Context(), ghost.ID, time.Now()); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("SetLastLoginAt() = %v, want not found", err)
	}
}

func TestPageReturnsEveryUserInCreationOrder(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	// Created in an order that is not alphabetical, so a page that came back
	// sorted by email would fail rather than accidentally agree.
	want := []string{"Zoe@example.com", "alice@example.com", "Bob@example.com"}
	for _, email := range want {
		mustCreateUser(t, r, email)
	}

	users, next, err := r.users.Page(t.Context(), identity.PageFilter{})
	if err != nil {
		t.Fatalf("Page() = %v, want nil", err)
	}
	if next != "" {
		t.Errorf("Page() returned the cursor %q for a single page", next)
	}
	if got := emailsOf(users); !slices.Equal(got, want) {
		t.Errorf("Page() returned %v, want %v", got, want)
	}
}

func TestPageOnAnEmptyDatabaseIsEmptyAndNotAnError(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	users, next, err := r.users.Page(t.Context(), identity.PageFilter{})
	if err != nil {
		t.Fatalf("Page() = %v, want nil", err)
	}
	if len(users) != 0 || next != "" {
		t.Errorf("Page() = %v, %q on a fresh database, want nothing", emailsOf(users), next)
	}
}

// TestPageWalksEveryUserExactlyOnce is the pagination criterion of M1-016, at
// the layer that decides it. A thousand rows is what the ticket names; the
// property is that following the cursor visits every account once and stops.
func TestPageWalksEveryUserExactlyOnce(t *testing.T) {
	t.Parallel()

	const total = 1000
	r := newRepos(t)
	for i := range total {
		mustCreateUser(t, r, fmt.Sprintf("person-%04d@example.com", i))
	}

	seen := map[string]bool{}
	pages, cursor := 0, ""
	for {
		users, next, err := r.users.Page(t.Context(), identity.PageFilter{Cursor: cursor})
		if err != nil {
			t.Fatalf("Page(cursor=%q) = %v, want nil", cursor, err)
		}
		pages++
		if pages > total {
			t.Fatal("Page() never reported the last page; the cursor is not advancing")
		}
		for _, u := range users {
			if seen[u.ID] {
				t.Fatalf("Page() returned %s twice, on page %d", u.Email, pages)
			}
			seen[u.ID] = true
		}
		if next == "" {
			break
		}
		cursor = next
	}

	if len(seen) != total {
		t.Errorf("walking the pages saw %d accounts, want %d", len(seen), total)
	}
	// Exactly 1000/50 pages and no trailing empty one: Page reads one row
	// beyond the limit to decide whether there is a next page, so a last page
	// that is exactly full still reports no cursor.
	if want := total / 50; pages != want {
		t.Errorf("the walk took %d pages, want %d", pages, want)
	}
}

func TestPageClampsTheLimit(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	for i := range 3 {
		mustCreateUser(t, r, fmt.Sprintf("clamp-%d@example.com", i))
	}

	for name, limit := range map[string]int{"zero": 0, "negative": -1, "above the maximum": 10_000} {
		users, _, err := r.users.Page(t.Context(), identity.PageFilter{Limit: limit})
		if err != nil {
			t.Fatalf("Page(limit=%d) = %v, want nil", limit, err)
		}
		if len(users) != 3 {
			t.Errorf("Page() with a %s limit returned %d accounts, want 3", name, len(users))
		}
	}
}

func TestPageFiltersByStatusAndRole(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	mustCreateUser(t, r, "plain-member@example.com") // member, active
	if _, err := r.users.Create(t.Context(), identity.NewUser{
		Email: "boss@example.com", DisplayName: "Boss",
		PlatformRole: authz.PlatformRoleAdmin, Status: identity.StatusActive,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.users.Create(t.Context(), identity.NewUser{
		Email: "retired@example.com", DisplayName: "Retired",
		PlatformRole: authz.PlatformRoleMember, Status: identity.StatusDisabled,
	}); err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		filter identity.PageFilter
		want   []string
	}{
		"by role": {
			filter: identity.PageFilter{Role: authz.PlatformRoleAdmin},
			want:   []string{"boss@example.com"},
		},
		"by status": {
			filter: identity.PageFilter{Status: identity.StatusDisabled},
			want:   []string{"retired@example.com"},
		},
		"both, and they are an AND": {
			filter: identity.PageFilter{Role: authz.PlatformRoleAdmin, Status: identity.StatusDisabled},
			want:   nil,
		},
		"neither": {
			filter: identity.PageFilter{},
			want:   []string{"plain-member@example.com", "boss@example.com", "retired@example.com"},
		},
	} {
		users, _, err := r.users.Page(t.Context(), tc.filter)
		if err != nil {
			t.Fatalf("%s: Page() = %v, want nil", name, err)
		}
		if got := emailsOf(users); !slices.Equal(got, tc.want) {
			t.Errorf("%s: Page() returned %v, want %v", name, got, tc.want)
		}
	}
}

func TestPageSearchesNameAndEmailWithoutRegardToCase(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	if _, err := r.users.Create(t.Context(), identity.NewUser{
		Email: "aisha.khan@example.com", DisplayName: "Aisha Khan",
		PlatformRole: authz.PlatformRoleMember, Status: identity.StatusActive,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.users.Create(t.Context(), identity.NewUser{
		Email: "b@contractor.example", DisplayName: "Bo Nilsson",
		PlatformRole: authz.PlatformRoleMember, Status: identity.StatusActive,
	}); err != nil {
		t.Fatal(err)
	}

	for name, tc := range map[string]struct {
		search string
		want   []string
	}{
		"a fragment of the display name":   {"isha", []string{"aisha.khan@example.com"}},
		"the display name in another case": {"AISHA KHAN", []string{"aisha.khan@example.com"}},
		"a fragment of the email domain":   {"contractor", []string{"b@contractor.example"}},
		"surrounding whitespace":           {"  nilsson  ", []string{"b@contractor.example"}},
		"nothing that matches":             {"nobody", nil},
		// The two LIKE metacharacters, typed by somebody who meant them
		// literally. Unescaped, "%" matches every account and "_" matches every
		// account with a character in it.
		"a literal percent":    {"%", nil},
		"a literal underscore": {"_", nil},
	} {
		users, _, err := r.users.Page(t.Context(), identity.PageFilter{Search: tc.search})
		if err != nil {
			t.Fatalf("%s: Page() = %v, want nil", name, err)
		}
		if got := emailsOf(users); !slices.Equal(got, tc.want) {
			t.Errorf("%s: Page(search=%q) returned %v, want %v", name, tc.search, got, tc.want)
		}
	}
}

func TestPageRefusesACursorItDidNotIssue(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	if _, _, err := r.users.Page(t.Context(), identity.PageFilter{Cursor: "not base64!!"}); err == nil {
		t.Error("Page() accepted a malformed cursor, want a validation failure")
	} else if !errors.Is(err, apierr.ErrValidation) {
		t.Errorf("Page() = %v, want a validation failure", err)
	}
}

// TestCountActiveAdminsCountsOnlyTheAdministratorsWhoCanSignIn is the fact the
// last-administrator guard in internal/authn is built on. A disabled
// administrator is not one who can put the installation back.
func TestCountActiveAdminsCountsOnlyTheAdministratorsWhoCanSignIn(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	for _, u := range []identity.NewUser{
		{Email: "a@example.com", DisplayName: "A", PlatformRole: authz.PlatformRoleAdmin, Status: identity.StatusActive},
		{Email: "b@example.com", DisplayName: "B", PlatformRole: authz.PlatformRoleAdmin, Status: identity.StatusDisabled},
		{Email: "c@example.com", DisplayName: "C", PlatformRole: authz.PlatformRoleAdmin, Status: identity.StatusInvited},
		{Email: "d@example.com", DisplayName: "D", PlatformRole: authz.PlatformRoleMember, Status: identity.StatusActive},
	} {
		if _, err := r.users.Create(t.Context(), u); err != nil {
			t.Fatal(err)
		}
	}

	var count int
	if err := r.db.Write(t.Context(), func(tx *sql.Tx) error {
		var err error
		count, err = identity.CountActiveAdmins(t.Context(), tx)
		return err
	}); err != nil {
		t.Fatalf("CountActiveAdmins() = %v, want nil", err)
	}
	if count != 1 {
		t.Errorf("CountActiveAdmins() = %d, want 1", count)
	}
}

// emailsOf renders a page for a failure message: the addresses, in the order
// they came back, and nil for an empty page so that slices.Equal treats "no
// results" and "no results" alike.
func emailsOf(users []identity.User) []string {
	if len(users) == 0 {
		return nil
	}
	emails := make([]string, len(users))
	for i, u := range users {
		emails[i] = u.Email
	}
	return emails
}
