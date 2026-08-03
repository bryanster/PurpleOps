package identity_test

import (
	"errors"
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

func TestListReturnsEveryUserInEmailOrder(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	// Created out of order, and with casing that would sort differently if the
	// display column were the one being ordered by.
	for _, email := range []string{"Zoe@example.com", "alice@example.com", "Bob@example.com"} {
		mustCreateUser(t, r, email)
	}

	users, err := r.users.List(t.Context())
	if err != nil {
		t.Fatalf("List() = %v, want nil", err)
	}

	var got []string
	for _, u := range users {
		got = append(got, u.Email)
	}
	want := []string{"alice@example.com", "Bob@example.com", "Zoe@example.com"}
	if len(got) != len(want) {
		t.Fatalf("List() returned %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("List() returned %v, want %v", got, want)
		}
	}
}

func TestListOnAnEmptyDatabaseIsEmptyAndNotAnError(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	users, err := r.users.List(t.Context())
	if err != nil {
		t.Fatalf("List() = %v, want nil", err)
	}
	if len(users) != 0 {
		t.Errorf("List() = %v on a fresh database, want nothing", users)
	}
}
