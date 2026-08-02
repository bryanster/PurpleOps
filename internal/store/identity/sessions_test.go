package identity_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/purpleops/internal/httpapi/apierr"
	"github.com/bryanster/purpleops/internal/store/identity"
)

func TestSessionRoundTrips(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")
	expires := time.Now().Add(12 * time.Hour)

	created, err := r.sessions.Create(t.Context(), identity.NewSession{
		UserID:       user.ID,
		TokenHash:    "hash-round-trip",
		ExpiresAt:    expires,
		IP:           "203.0.113.7",
		UserAgent:    "Mozilla/5.0",
		MFASatisfied: true,
	})
	if err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}

	if created.ID == "" {
		t.Error("Create() returned a session with no identifier")
	}
	if created.CreatedAt != created.LastSeenAt {
		t.Errorf("CreatedAt = %s, LastSeenAt = %s; a new session has just been seen",
			created.CreatedAt, created.LastSeenAt)
	}
	if !created.RevokedAt.IsZero() {
		t.Errorf("RevokedAt = %s on a new session, want the zero time", created.RevokedAt)
	}
	if !created.MFASatisfied {
		t.Error("MFASatisfied = false, want true")
	}

	found, err := r.sessions.ByTokenHash(t.Context(), "hash-round-trip")
	if err != nil {
		t.Fatalf("ByTokenHash() = %v, want the session", err)
	}
	if found != created {
		t.Errorf("ByTokenHash() = %+v, want %+v", found, created)
	}
}

// TestASessionWithoutARequestFingerprintIsStored: ip and user_agent are empty
// rather than null when the request did not carry them, so no caller needs a
// null check to read an audit trail.
func TestASessionWithoutARequestFingerprintIsStored(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	created, err := r.sessions.Create(t.Context(), identity.NewSession{
		UserID: user.ID, TokenHash: "hash-bare", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}
	if created.IP != "" || created.UserAgent != "" {
		t.Errorf("IP = %q, UserAgent = %q; want both empty", created.IP, created.UserAgent)
	}
}

func TestByTokenHashReportsNotFoundWithoutEchoingTheToken(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	_, err := r.sessions.ByTokenHash(t.Context(), "hash-that-was-never-issued")
	if !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("ByTokenHash() = %v, want not found", err)
	}
	// The hash is a credential. It belongs in neither the response nor the log,
	// and apierr puts an identifier in the log.
	if got := err.Error(); strings.Contains(got, "hash-that-was-never-issued") {
		t.Errorf("the error repeats the token: %s", got)
	}
}

func TestSessionTokenHashesAreUnique(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	alice := mustCreateUser(t, r, "alice@example.com")
	bob := mustCreateUser(t, r, "bob@example.com")

	if _, err := r.sessions.Create(t.Context(), identity.NewSession{
		UserID: alice.ID, TokenHash: "shared", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}

	_, err := r.sessions.Create(t.Context(), identity.NewSession{
		UserID: bob.ID, TokenHash: "shared", ExpiresAt: time.Now().Add(time.Hour),
	})
	if !errors.Is(err, apierr.ErrConflict) {
		t.Errorf("Create() with a reused token hash = %v, want a conflict", err)
	}
}

func TestSetLastSeenAtRecordsUse(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")
	created, err := r.sessions.Create(t.Context(), identity.NewSession{
		UserID: user.ID, TokenHash: "hash-seen", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}

	seen := created.CreatedAt.Add(5 * time.Minute)
	if err := r.sessions.SetLastSeenAt(t.Context(), created.ID, seen); err != nil {
		t.Fatalf("SetLastSeenAt() = %v, want nil", err)
	}

	found, err := r.sessions.ByTokenHash(t.Context(), "hash-seen")
	if err != nil {
		t.Fatal(err)
	}
	if !found.LastSeenAt.Equal(seen) {
		t.Errorf("LastSeenAt = %s, want %s", found.LastSeenAt, seen)
	}
	if !found.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("CreatedAt moved to %s", found.CreatedAt)
	}

	if err := r.sessions.SetLastSeenAt(t.Context(), "no-such-session", seen); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("SetLastSeenAt() on a missing session = %v, want not found", err)
	}
}

// TestRevokeKeepsTheFirstRevocation: the moment access actually stopped is the
// first one, and a second call must not move it forward.
func TestRevokeKeepsTheFirstRevocation(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")
	created, err := r.sessions.Create(t.Context(), identity.NewSession{
		UserID: user.ID, TokenHash: "hash-revoke", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}

	first := created.CreatedAt.Add(time.Minute)
	if err := r.sessions.Revoke(t.Context(), created.ID, first); err != nil {
		t.Fatalf("Revoke() = %v, want nil", err)
	}
	// Revoking twice is not a failure — the caller's intent is already met.
	if err := r.sessions.Revoke(t.Context(), created.ID, first.Add(time.Hour)); err != nil {
		t.Fatalf("second Revoke() = %v, want nil", err)
	}

	found, err := r.sessions.ByTokenHash(t.Context(), "hash-revoke")
	if err != nil {
		t.Fatal(err)
	}
	if !found.RevokedAt.Equal(first) {
		t.Errorf("RevokedAt = %s, want the first revocation at %s", found.RevokedAt, first)
	}

	if err := r.sessions.Revoke(t.Context(), "no-such-session", first); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("Revoke() on a missing session = %v, want not found", err)
	}
}

// TestRevokeAllForUserLeavesOtherPeopleAlone is the storage half of "rotation
// on privilege change" (PLAN.md §4). Signing everybody out would be a much
// worse bug than signing nobody out, so the blast radius is the assertion.
func TestRevokeAllForUserLeavesOtherPeopleAlone(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	alice := mustCreateUser(t, r, "alice@example.com")
	bob := mustCreateUser(t, r, "bob@example.com")

	for _, hash := range []string{"alice-1", "alice-2"} {
		if _, err := r.sessions.Create(t.Context(), identity.NewSession{
			UserID: alice.ID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.sessions.Create(t.Context(), identity.NewSession{
		UserID: bob.ID, TokenHash: "bob-1", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	at := time.Now()
	revoked, err := r.sessions.RevokeAllForUser(t.Context(), alice.ID, at)
	if err != nil {
		t.Fatalf("RevokeAllForUser() = %v, want nil", err)
	}
	if revoked != 2 {
		t.Errorf("RevokeAllForUser() revoked %d, want 2", revoked)
	}

	for _, hash := range []string{"alice-1", "alice-2"} {
		s, err := r.sessions.ByTokenHash(t.Context(), hash)
		if err != nil {
			t.Fatal(err)
		}
		if s.RevokedAt.IsZero() {
			t.Errorf("%s is still live", hash)
		}
	}
	bobs, err := r.sessions.ByTokenHash(t.Context(), "bob-1")
	if err != nil {
		t.Fatal(err)
	}
	if !bobs.RevokedAt.IsZero() {
		t.Error("Bob was signed out by a change to Alice's account")
	}

	// A second sweep finds nothing left to revoke, and says so rather than
	// re-reporting the same sessions.
	again, err := r.sessions.RevokeAllForUser(t.Context(), alice.ID, at)
	if err != nil {
		t.Fatalf("second RevokeAllForUser() = %v, want nil", err)
	}
	if again != 0 {
		t.Errorf("second RevokeAllForUser() revoked %d, want 0", again)
	}
}

func TestListByUserReturnsEveryStateOfSession(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	alice := mustCreateUser(t, r, "alice@example.com")
	bob := mustCreateUser(t, r, "bob@example.com")

	live, err := r.sessions.Create(t.Context(), identity.NewSession{
		UserID: alice.ID, TokenHash: "live", ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	dead, err := r.sessions.Create(t.Context(), identity.NewSession{
		UserID: alice.ID, TokenHash: "dead", ExpiresAt: time.Now().Add(-time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.sessions.Revoke(t.Context(), dead.ID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := r.sessions.Create(t.Context(), identity.NewSession{
		UserID: bob.ID, TokenHash: "bobs", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}

	sessions, err := r.sessions.ListByUser(t.Context(), alice.ID)
	if err != nil {
		t.Fatalf("ListByUser() = %v, want nil", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("ListByUser() returned %d sessions, want 2 — expired and revoked ones included",
			len(sessions))
	}
	// Newest first: identifiers are UUIDv7, so descending by id is descending
	// by creation.
	if sessions[0].ID != dead.ID || sessions[1].ID != live.ID {
		t.Errorf("ListByUser() returned %q then %q, want newest first (%q, %q)",
			sessions[0].ID, sessions[1].ID, dead.ID, live.ID)
	}
}

func TestDeleteExpiredRemovesOnlyWhatIsPastTheCutoff(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")
	now := time.Now()

	for hash, expires := range map[string]time.Time{
		"long-gone": now.Add(-90 * 24 * time.Hour),
		"recent":    now.Add(-time.Hour),
		"live":      now.Add(time.Hour),
	} {
		if _, err := r.sessions.Create(t.Context(), identity.NewSession{
			UserID: user.ID, TokenHash: hash, ExpiresAt: expires,
		}); err != nil {
			t.Fatal(err)
		}
	}

	// A retention window rather than "now": the rows are the record of who was
	// signed in, and deleting them the instant they expire throws that away.
	deleted, err := r.sessions.DeleteExpired(t.Context(), now.Add(-30*24*time.Hour))
	if err != nil {
		t.Fatalf("DeleteExpired() = %v, want nil", err)
	}
	if deleted != 1 {
		t.Errorf("DeleteExpired() removed %d, want 1", deleted)
	}

	remaining, err := r.sessions.ListByUser(t.Context(), user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != 2 {
		t.Errorf("%d sessions remain, want 2", len(remaining))
	}
	if _, err := r.sessions.ByTokenHash(t.Context(), "long-gone"); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("the long-expired session is still there: %v", err)
	}
}
