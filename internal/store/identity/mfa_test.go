package identity_test

import (
	"errors"
	"testing"
	"time"

	"github.com/bryanster/purpleops/internal/httpapi/apierr"
	"github.com/bryanster/purpleops/internal/store/identity"
)

// The two MFA tables (M1-006). What is being tested here is what the schema and
// the guarded statements promise: that a new enrolment starts unconfirmed and
// unspent, that a step cannot be spent twice, and that a challenge cannot be
// consumed twice.

func TestTOTPStartsUnconfirmedAndUnspent(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	created, err := r.totp.Enroll(t.Context(), identity.NewTOTP{
		UserID:          user.ID,
		SecretEncrypted: "sealed-1",
	})
	if err != nil {
		t.Fatalf("Enroll() = %v, want nil", err)
	}
	switch {
	case created.Confirmed():
		t.Error("a new enrolment is confirmed; a half-finished one must gate nothing")
	case created.LastUsedStep != 0:
		t.Errorf("LastUsedStep = %d on a new enrolment, want 0", created.LastUsedStep)
	case created.SecretEncrypted != "sealed-1":
		t.Errorf("SecretEncrypted = %q, want the ciphertext as given", created.SecretEncrypted)
	}

	found, err := r.totp.ByUserID(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("ByUserID() = %v, want the enrolment", err)
	}
	if found != created {
		t.Errorf("ByUserID() = %+v, want %+v", found, created)
	}
}

func TestNoTOTPIsNotFound(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	if _, err := r.totp.ByUserID(t.Context(), user.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("ByUserID() for a user with no enrolment = %v, want not-found", err)
	}
}

// TestReEnrollingResetsEverything: a second enrolment is a fresh secret, so it
// must not inherit the previous one's confirmation or its spent step — a new
// secret that started at the old step would refuse its own first code.
func TestReEnrollingResetsEverything(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	if _, err := r.totp.Enroll(t.Context(), identity.NewTOTP{
		UserID: user.ID, SecretEncrypted: "sealed-1",
	}); err != nil {
		t.Fatalf("Enroll() = %v", err)
	}
	if _, err := r.totp.Accept(t.Context(), user.ID, 5_000_000, time.Now()); err != nil {
		t.Fatalf("Accept() = %v", err)
	}

	replaced, err := r.totp.Enroll(t.Context(), identity.NewTOTP{
		UserID: user.ID, SecretEncrypted: "sealed-2",
	})
	if err != nil {
		t.Fatalf("the second Enroll() = %v", err)
	}
	switch {
	case replaced.SecretEncrypted != "sealed-2":
		t.Errorf("SecretEncrypted = %q, want the new ciphertext", replaced.SecretEncrypted)
	case replaced.Confirmed():
		t.Error("the replacement enrolment is already confirmed")
	case replaced.LastUsedStep != 0:
		t.Errorf("LastUsedStep = %d on the replacement, want 0", replaced.LastUsedStep)
	}
}

// TestAcceptConfirmsOnceAndOnlyMovesForward is the replay window as the database
// enforces it.
func TestAcceptConfirmsOnceAndOnlyMovesForward(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")
	if _, err := r.totp.Enroll(t.Context(), identity.NewTOTP{
		UserID: user.ID, SecretEncrypted: "sealed",
	}); err != nil {
		t.Fatalf("Enroll() = %v", err)
	}

	firstAt := time.Now().Add(-time.Hour).UTC()
	accepted, err := r.totp.Accept(t.Context(), user.ID, 100, firstAt)
	if err != nil || !accepted {
		t.Fatalf("Accept(100) = (%t, %v), want accepted", accepted, err)
	}

	confirmed, err := r.totp.ByUserID(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("ByUserID() = %v", err)
	}
	if !confirmed.Confirmed() {
		t.Fatal("the first accepted code did not confirm the enrolment")
	}
	if confirmed.LastUsedStep != 100 {
		t.Errorf("LastUsedStep = %d, want 100", confirmed.LastUsedStep)
	}

	// The same step again, and an earlier one: both are replays.
	for _, step := range []int64{100, 99} {
		accepted, err := r.totp.Accept(t.Context(), user.ID, step, time.Now())
		if err != nil {
			t.Fatalf("Accept(%d) = %v", step, err)
		}
		if accepted {
			t.Errorf("Accept(%d) succeeded after step 100 was spent", step)
		}
	}

	// A later one is not, and it must not move confirmed_at: the enrolment was
	// confirmed once, and when is a fact about that moment.
	accepted, err = r.totp.Accept(t.Context(), user.ID, 101, time.Now())
	if err != nil || !accepted {
		t.Fatalf("Accept(101) = (%t, %v), want accepted", accepted, err)
	}
	after, err := r.totp.ByUserID(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("ByUserID() = %v", err)
	}
	if !after.ConfirmedAt.Equal(confirmed.ConfirmedAt) {
		t.Errorf("ConfirmedAt moved from %s to %s on a later code",
			confirmed.ConfirmedAt, after.ConfirmedAt)
	}
}

func TestAcceptingForSomebodyWithNoEnrolmentIsNotFound(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")

	if _, err := r.totp.Accept(t.Context(), user.ID, 1, time.Now()); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("Accept() with no enrolment = %v, want not-found", err)
	}
}

func TestDeletingATOTPIsIdempotent(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")
	if _, err := r.totp.Enroll(t.Context(), identity.NewTOTP{
		UserID: user.ID, SecretEncrypted: "sealed",
	}); err != nil {
		t.Fatalf("Enroll() = %v", err)
	}

	for range 2 {
		if err := r.totp.Delete(t.Context(), user.ID); err != nil {
			t.Fatalf("Delete() = %v, want nil — the caller wanted them to have none", err)
		}
	}
	if _, err := r.totp.ByUserID(t.Context(), user.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("the enrolment survived Delete(): %v", err)
	}
}

// --- Challenges ---------------------------------------------------------------

func TestChallengeRoundTrips(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")
	expires := time.Now().Add(5 * time.Minute)

	opened, err := r.challenges.Open(t.Context(), identity.NewMFAChallenge{
		UserID:    user.ID,
		TokenHash: "challenge-hash",
		ExpiresAt: expires,
	})
	if err != nil {
		t.Fatalf("Open() = %v, want nil", err)
	}
	if opened.ID == "" {
		t.Error("Open() returned a challenge with no identifier")
	}
	if !opened.ConsumedAt.IsZero() {
		t.Errorf("ConsumedAt = %s on a new challenge, want the zero time", opened.ConsumedAt)
	}

	found, err := r.challenges.ByTokenHash(t.Context(), "challenge-hash")
	if err != nil {
		t.Fatalf("ByTokenHash() = %v, want the challenge", err)
	}
	if found != opened {
		t.Errorf("ByTokenHash() = %+v, want %+v", found, opened)
	}
}

// TestOpeningAChallengeSupersedesTheLast: an abandoned sign-in must not leave a
// challenge that is still answerable while its owner starts another.
func TestOpeningAChallengeSupersedesTheLast(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")
	expires := time.Now().Add(5 * time.Minute)

	first, err := r.challenges.Open(t.Context(), identity.NewMFAChallenge{
		UserID: user.ID, TokenHash: "first", ExpiresAt: expires,
	})
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}
	if _, err := r.challenges.Open(t.Context(), identity.NewMFAChallenge{
		UserID: user.ID, TokenHash: "second", ExpiresAt: expires,
	}); err != nil {
		t.Fatalf("the second Open() = %v", err)
	}

	if _, err := r.challenges.ByTokenHash(t.Context(), "first"); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("the first challenge (%s) is still resolvable: %v", first.ID, err)
	}
	if _, err := r.challenges.ByTokenHash(t.Context(), "second"); err != nil {
		t.Errorf("the second challenge is not resolvable: %v", err)
	}
}

// TestAChallengeIsConsumedOnce: one correct code buys exactly one session, and
// the guard that says so is in the statement rather than in a caller.
func TestAChallengeIsConsumedOnce(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")
	opened, err := r.challenges.Open(t.Context(), identity.NewMFAChallenge{
		UserID: user.ID, TokenHash: "hash", ExpiresAt: time.Now().Add(5 * time.Minute),
	})
	if err != nil {
		t.Fatalf("Open() = %v", err)
	}

	at := time.Now().UTC()
	consumed, err := r.challenges.Consume(t.Context(), opened.ID, at)
	if err != nil || !consumed {
		t.Fatalf("Consume() = (%t, %v), want consumed", consumed, err)
	}

	again, err := r.challenges.Consume(t.Context(), opened.ID, time.Now())
	if err != nil {
		t.Fatalf("the second Consume() = %v", err)
	}
	if again {
		t.Error("a challenge was consumed twice")
	}

	spent, err := r.challenges.ByTokenHash(t.Context(), "hash")
	if err != nil {
		t.Fatalf("ByTokenHash() = %v", err)
	}
	if spent.ConsumedAt.IsZero() {
		t.Error("consumed_at was not recorded")
	}
}

func TestConsumingAChallengeThatIsGoneIsNotFound(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	if _, err := r.challenges.Consume(t.Context(), "018f-nope", time.Now()); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("Consume() of a missing challenge = %v, want not-found", err)
	}
}

func TestDeletingAUsersChallengesRemovesThemAll(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	alice := mustCreateUser(t, r, "alice@example.com")
	bob := mustCreateUser(t, r, "bob@example.com")
	expires := time.Now().Add(5 * time.Minute)

	for _, in := range []identity.NewMFAChallenge{
		{UserID: alice.ID, TokenHash: "alice", ExpiresAt: expires},
		{UserID: bob.ID, TokenHash: "bob", ExpiresAt: expires},
	} {
		if _, err := r.challenges.Open(t.Context(), in); err != nil {
			t.Fatalf("Open() = %v", err)
		}
	}

	if err := r.challenges.DeleteForUser(t.Context(), alice.ID); err != nil {
		t.Fatalf("DeleteForUser() = %v", err)
	}
	if _, err := r.challenges.ByTokenHash(t.Context(), "alice"); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("alice's challenge survived: %v", err)
	}
	if _, err := r.challenges.ByTokenHash(t.Context(), "bob"); err != nil {
		t.Errorf("bob's challenge was deleted with alice's: %v", err)
	}
}

// TestMFARowsNeedARealUser: the foreign keys 0002 declared had to go (0003), so
// the rule they enforced is enforced in Go — and this is what says it still is.
func TestMFARowsNeedARealUser(t *testing.T) {
	t.Parallel()

	r := newRepos(t)

	if _, err := r.totp.Enroll(t.Context(), identity.NewTOTP{
		UserID: "018f-nobody", SecretEncrypted: "sealed",
	}); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("Enroll() for a user who does not exist = %v, want not-found", err)
	}
	if _, err := r.challenges.Open(t.Context(), identity.NewMFAChallenge{
		UserID: "018f-nobody", TokenHash: "hash", ExpiresAt: time.Now().Add(time.Minute),
	}); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("Open() for a user who does not exist = %v, want not-found", err)
	}
}

// TestSetMFASatisfiedMarksTheSession is the other half of M1-006's session
// change: confirming a factor marks the session that did it, and a revoked
// session cannot be marked at all.
func TestSetMFASatisfiedMarksTheSession(t *testing.T) {
	t.Parallel()

	r := newRepos(t)
	user := mustCreateUser(t, r, "alice@example.com")
	created, err := r.sessions.Create(t.Context(), identity.NewSession{
		UserID:    user.ID,
		TokenHash: "session-hash",
		ExpiresAt: time.Now().Add(12 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}
	if created.MFASatisfied {
		t.Fatal("a password-only session starts with mfa_satisfied set")
	}

	if err := r.sessions.SetMFASatisfied(t.Context(), created.ID); err != nil {
		t.Fatalf("SetMFASatisfied() = %v", err)
	}
	found, err := r.sessions.ByTokenHash(t.Context(), "session-hash")
	if err != nil {
		t.Fatalf("ByTokenHash() = %v", err)
	}
	if !found.MFASatisfied {
		t.Error("mfa_satisfied was not recorded")
	}

	if err := r.sessions.Revoke(t.Context(), created.ID, time.Now()); err != nil {
		t.Fatalf("Revoke() = %v", err)
	}
	if err := r.sessions.SetMFASatisfied(t.Context(), created.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("SetMFASatisfied() on a revoked session = %v, want not-found: the caller is "+
			"about to hand its token out as an authenticated one", err)
	}
}
