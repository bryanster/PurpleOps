package identity_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// newToken is a plausible token for a given owner, so that a test changing one
// field is not also silently changing five others.
func newToken(owner identity.User, prefix string) identity.NewServiceToken {
	return identity.NewServiceToken{
		Name:        "nightly export",
		Prefix:      prefix,
		TokenHash:   "hash-" + prefix,
		OwnerUserID: owner.ID,
		CreatedBy:   owner.ID,
		Scopes: []authz.TokenScope{
			authz.TokenScopeEngagementsRead,
			authz.TokenScopeReportsRead,
		},
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}
}

func TestAServiceTokenRoundTrips(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	owner := mustCreateUser(t, r, "owner@example.com")

	created, err := r.tokens.Create(t.Context(), newToken(owner, "prefix-round-trip"))
	if err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}

	switch {
	case created.ID == "":
		t.Error("Create() returned a token with no identifier")
	case !created.LastUsedAt.IsZero():
		t.Errorf("LastUsedAt = %s on a token nobody has used, want the zero time", created.LastUsedAt)
	case !created.RevokedAt.IsZero():
		t.Errorf("RevokedAt = %s on a new token, want the zero time", created.RevokedAt)
	case created.EngagementID != "":
		t.Errorf("EngagementID = %q on an unbound token, want empty", created.EngagementID)
	}

	found, err := r.tokens.ByPrefix(t.Context(), "prefix-round-trip")
	if err != nil {
		t.Fatalf("ByPrefix() = %v, want the token", err)
	}
	if !slices.Equal(found.Scopes, created.Scopes) {
		t.Errorf("ByPrefix().Scopes = %v, want %v", found.Scopes, created.Scopes)
	}
	if found.ID != created.ID || found.TokenHash != created.TokenHash {
		t.Errorf("ByPrefix() = %+v, want %+v", found, created)
	}
}

// TestTheScopeListSurvivesTheColumn is the one thing the space-separated column
// could plausibly get wrong: order, and a single-element list.
func TestTheScopeListSurvivesTheColumn(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	owner := mustCreateUser(t, r, "scopes@example.com")

	for name, want := range map[string][]authz.TokenScope{
		"one":   {authz.TokenScopeContentRead},
		"all":   authz.TokenScopes(),
		"order": {authz.TokenScopeReportsRead, authz.TokenScopeAdminRead, authz.TokenScopeContentSync},
	} {
		in := newToken(owner, "prefix-scopes-"+name)
		in.Scopes = want

		created, err := r.tokens.Create(t.Context(), in)
		if err != nil {
			t.Fatalf("Create(%s) = %v", name, err)
		}
		if !slices.Equal(created.Scopes, want) {
			t.Errorf("Create(%s).Scopes = %v, want %v", name, created.Scopes, want)
		}
	}
}

// TestATokenBoundToAnEngagementKeepsItsBinding: the column is nullable, and the
// two states have to come back as the two states rather than both as "".
func TestATokenBoundToAnEngagementKeepsItsBinding(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	owner := mustCreateUser(t, r, "bound@example.com")

	in := newToken(owner, "prefix-bound")
	in.EngagementID = "0192f1a0-0000-7000-8000-000000000001"

	created, err := r.tokens.Create(t.Context(), in)
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}
	if created.EngagementID != in.EngagementID {
		t.Errorf("EngagementID = %q, want %q", created.EngagementID, in.EngagementID)
	}
}

// TestATokenNeedsAnAccountThatExists: the foreign keys had to go
// (0003_user_updatable), so the rule they enforced is enforced in the write
// transaction instead — and a test is the only thing that keeps it there.
func TestATokenNeedsAnAccountThatExists(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	owner := mustCreateUser(t, r, "real@example.com")

	missing := newToken(owner, "prefix-missing-owner")
	missing.OwnerUserID = "0192f1a0-0000-7000-8000-00000000dead"

	if _, err := r.tokens.Create(t.Context(), missing); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("Create() for an owner who does not exist = %v, want not-found", err)
	}

	issuer := newToken(owner, "prefix-missing-issuer")
	issuer.CreatedBy = "0192f1a0-0000-7000-8000-00000000beef"

	if _, err := r.tokens.Create(t.Context(), issuer); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("Create() for an issuer who does not exist = %v, want not-found", err)
	}
}

// TestTwoTokensCannotShareAPrefix: the prefix is what every authenticated
// request looks a row up by, and two rows sharing one is a lookup with no
// single answer.
func TestTwoTokensCannotShareAPrefix(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	owner := mustCreateUser(t, r, "prefix@example.com")

	if _, err := r.tokens.Create(t.Context(), newToken(owner, "prefix-duplicate")); err != nil {
		t.Fatalf("the first Create() = %v", err)
	}
	_, err := r.tokens.Create(t.Context(), newToken(owner, "prefix-duplicate"))
	if !errors.Is(err, apierr.ErrConflict) {
		t.Errorf("the second Create() with the same prefix = %v, want a conflict", err)
	}
}

func TestAnUnknownPrefixIsNotFoundAndIsNotEchoed(t *testing.T) {
	t.Parallel()

	r := newRepos(t)

	_, err := r.tokens.ByPrefix(t.Context(), "prefix-nobody-holds")
	if !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("ByPrefix() = %v, want not-found", err)
	}
	// The prefix is the public half of a credential somebody presented. Echoing
	// it would put every guessed value into the log.
	if got := err.Error(); strings.Contains(got, "prefix-nobody-holds") {
		t.Errorf("the error repeats the presented prefix: %s", got)
	}
}

func TestListingIsScopedToTheOwnerAndNewestFirst(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	alice := mustCreateUser(t, r, "alice@example.com")
	bob := mustCreateUser(t, r, "bob@example.com")

	first, err := r.tokens.Create(t.Context(), newToken(alice, "prefix-alice-1"))
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}
	second, err := r.tokens.Create(t.Context(), newToken(alice, "prefix-alice-2"))
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}
	if _, err := r.tokens.Create(t.Context(), newToken(bob, "prefix-bob-1")); err != nil {
		t.Fatalf("Create() = %v", err)
	}

	listed, err := r.tokens.ListByOwner(t.Context(), alice.ID)
	if err != nil {
		t.Fatalf("ListByOwner() = %v", err)
	}
	if len(listed) != 2 {
		t.Fatalf("ListByOwner() returned %d tokens, want 2 — Bob's is not Alice's", len(listed))
	}
	// UUIDv7 sorts by creation, so ORDER BY id DESC is newest first.
	if listed[0].ID != second.ID || listed[1].ID != first.ID {
		t.Errorf("ListByOwner() returned %s then %s, want the newest first (%s, %s)",
			listed[0].ID, listed[1].ID, second.ID, first.ID)
	}
}

// TestRevokingSomebodyElsesTokenIsIndistinguishableFromOneThatIsNotThere is the
// property the WHERE clause exists for: an identifier that belongs to another
// account must not be answerable differently from an invented one, or the
// difference enumerates them.
func TestRevokingSomebodyElsesTokenIsIndistinguishableFromOneThatIsNotThere(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	alice := mustCreateUser(t, r, "alice2@example.com")
	bob := mustCreateUser(t, r, "bob2@example.com")

	hers, err := r.tokens.Create(t.Context(), newToken(alice, "prefix-hers"))
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}

	_, theirs := r.tokens.Revoke(t.Context(), hers.ID, bob.ID, bob.ID, time.Now())
	_, invented := r.tokens.Revoke(t.Context(),
		"0192f1a0-0000-7000-8000-00000000face", bob.ID, bob.ID, time.Now())

	if !errors.Is(theirs, apierr.ErrNotFound) {
		t.Errorf("revoking another owner's token = %v, want not-found", theirs)
	}
	if !errors.Is(invented, apierr.ErrNotFound) {
		t.Errorf("revoking a token that does not exist = %v, want not-found", invented)
	}

	// And it is still live, which is the half that would go unnoticed if the
	// statement had matched on the identifier alone.
	still, err := r.tokens.ByPrefix(t.Context(), "prefix-hers")
	if err != nil {
		t.Fatalf("ByPrefix() = %v", err)
	}
	if !still.RevokedAt.IsZero() {
		t.Errorf("the token was revoked by somebody who does not own it, at %s", still.RevokedAt)
	}
}

// TestRevokingTwiceKeepsTheFirstTimestampAndTheFirstRevoker: the first
// revocation is when access actually stopped and who stopped it, and overwriting
// either loses the fact an incident review wants. Whoever arrived second stopped
// nothing.
func TestRevokingTwiceKeepsTheFirstTimestampAndTheFirstRevoker(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	owner := mustCreateUser(t, r, "revoke@example.com")
	admin := mustCreateUser(t, r, "revoke-admin@example.com")

	created, err := r.tokens.Create(t.Context(), newToken(owner, "prefix-revoke-twice"))
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}

	first := time.Now()
	revoked, err := r.tokens.Revoke(t.Context(), created.ID, owner.ID, owner.ID, first)
	if err != nil {
		t.Fatalf("the first Revoke() = %v", err)
	}
	if revoked.RevokedAt.IsZero() {
		t.Fatal("Revoke() returned a token with no revocation time")
	}
	if revoked.RevokedBy != owner.ID {
		t.Errorf("the owner's own revocation recorded %q as the revoker, want %s", revoked.RevokedBy, owner.ID)
	}

	again, err := r.tokens.Revoke(t.Context(), created.ID, owner.ID, admin.ID, first.Add(time.Hour))
	if err != nil {
		t.Fatalf("the second Revoke() = %v, want nil — the caller's intent is satisfied", err)
	}
	if !again.RevokedAt.Equal(revoked.RevokedAt) {
		t.Errorf("the second revocation moved the timestamp from %s to %s",
			revoked.RevokedAt, again.RevokedAt)
	}
	if again.RevokedBy != owner.ID {
		t.Errorf("the second revocation rewrote the revoker to %s; %s is who actually ended it",
			again.RevokedBy, owner.ID)
	}
}

// TestAnAdministrativeRevocationRecordsWhoEndedIt is the column 0010 added, and
// the question it exists to answer: comparing it against owner_user_id says
// whether this was the owner's own rotation or somebody stepping in (M1-018).
func TestAnAdministrativeRevocationRecordsWhoEndedIt(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	owner := mustCreateUser(t, r, "held@example.com")
	admin := mustCreateUser(t, r, "stepped-in@example.com")

	created, err := r.tokens.Create(t.Context(), newToken(owner, "prefix-admin-revoked"))
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}
	if created.RevokedBy != "" {
		t.Errorf("a token nobody has revoked names %q as its revoker", created.RevokedBy)
	}

	revoked, err := r.tokens.Revoke(t.Context(), created.ID, owner.ID, admin.ID, time.Now())
	if err != nil {
		t.Fatalf("Revoke() = %v", err)
	}
	if revoked.RevokedBy != admin.ID {
		t.Errorf("the revoker is %q, want the administrator %s", revoked.RevokedBy, admin.ID)
	}
	if revoked.OwnerUserID != owner.ID {
		t.Errorf("revoking moved the owner to %s", revoked.OwnerUserID)
	}

	// And it reads back the same way, which is the half that would go unnoticed
	// if the column were written but not selected.
	stored, err := r.tokens.ByPrefix(t.Context(), "prefix-admin-revoked")
	if err != nil {
		t.Fatalf("ByPrefix() = %v", err)
	}
	if stored.RevokedBy != admin.ID {
		t.Errorf("the stored row names %q as the revoker, want %s", stored.RevokedBy, admin.ID)
	}
}

func TestTouchingATokenRecordsItsLastUse(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	owner := mustCreateUser(t, r, "touch@example.com")

	created, err := r.tokens.Create(t.Context(), newToken(owner, "prefix-touch"))
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}

	used := time.Now().Add(time.Minute)
	if err := r.tokens.SetLastUsedAt(t.Context(), created.ID, used); err != nil {
		t.Fatalf("SetLastUsedAt() = %v", err)
	}

	found, err := r.tokens.ByPrefix(t.Context(), "prefix-touch")
	if err != nil {
		t.Fatalf("ByPrefix() = %v", err)
	}
	switch {
	case found.LastUsedAt.IsZero():
		t.Error("LastUsedAt is still the zero time after a use was recorded")
	case found.LastUsedAt.Location() != time.UTC:
		t.Errorf("LastUsedAt is in %s, want UTC", found.LastUsedAt.Location())
	}

	if err := r.tokens.SetLastUsedAt(t.Context(), "0192f1a0-0000-7000-8000-0000000000ff", used); !errors.Is(
		err, apierr.ErrNotFound) {
		t.Errorf("SetLastUsedAt() on a token that does not exist = %v, want not-found", err)
	}
}

func TestExpiredTokensAreSweptAndLiveOnesAreNot(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	owner := mustCreateUser(t, r, "sweep@example.com")

	stale := newToken(owner, "prefix-stale")
	stale.ExpiresAt = time.Now().Add(-48 * time.Hour)
	if _, err := r.tokens.Create(t.Context(), stale); err != nil {
		t.Fatalf("Create() = %v", err)
	}
	if _, err := r.tokens.Create(t.Context(), newToken(owner, "prefix-live")); err != nil {
		t.Fatalf("Create() = %v", err)
	}

	deleted, err := r.tokens.DeleteExpired(t.Context(), time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("DeleteExpired() = %v", err)
	}
	if deleted != 1 {
		t.Errorf("DeleteExpired() removed %d tokens, want 1", deleted)
	}
	if _, err := r.tokens.ByPrefix(t.Context(), "prefix-live"); err != nil {
		t.Errorf("the live token was swept as well: %v", err)
	}
}
