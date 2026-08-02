package authn

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/bryanster/blacklight/internal/authn/challenge"
	"github.com/bryanster/blacklight/internal/authn/password"
	"github.com/bryanster/blacklight/internal/authn/recovery"
	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Recovery codes (M1-007): the way past a lost authenticator, and the only one
// this deployment has — there is no mail transport to send a link through and,
// in a self-hosted tool whose only administrator may be the person who is
// locked out, no help desk to ring.
//
// The rules that make them worth having, and where each lives:
//
//   - They are shown exactly once, by the response that mints them. Nothing
//     reads a code back; the database holds hashes and this package holds no
//     plaintext past the end of the call that generated it.
//   - One code, one use. [identity.RecoveryCodes.Use] guards on used_at in the
//     statement, so two requests arriving with the same code cannot both win.
//   - Presenting one signs you *all the way* in. The session it produces is
//     marked mfa_satisfied, because a person who holds a printed code has
//     satisfied a second factor — a half-session would only mean asking them
//     for the authenticator they have just told us they no longer have.
//   - They live and die with the enrolment. Confirming one mints them,
//     replacing an enrolment replaces them, and removing one deletes them.

// RecoveryCodeSet is a freshly minted set of codes, as the person who has to
// write them down needs to see them. It is returned by exactly two calls —
// [Service.ConfirmTOTP] and [Service.RegenerateRecoveryCodes] — and by nothing
// that reads.
type RecoveryCodeSet struct {
	// Codes are the plaintext codes. This is the only place they exist outside
	// the client: what is stored is [recovery.Hasher.Hash] of each.
	Codes []recovery.Code

	// GeneratedAt is when the set was written, read back off the stored rows
	// rather than taken from the clock here — so the timestamp shown to a person
	// is the one the database holds, not one a second either side of it.
	GeneratedAt time.Time
}

// issueRecoveryCodes mints a fresh set for a user, replacing whatever they had.
//
// The hashes go to the store and the codes go back up. Nothing in between holds
// both, and nothing logs either.
func (s *Service) issueRecoveryCodes(ctx context.Context, userID string) (RecoveryCodeSet, error) {
	codes, err := recovery.Generate()
	if err != nil {
		return RecoveryCodeSet{}, err
	}
	stored, err := s.recoveryCodes.Replace(ctx, userID, s.recovery.HashAll(codes))
	if err != nil {
		return RecoveryCodeSet{}, err
	}
	if len(stored) != len(codes) {
		// The set that came back is not the set that went in, which means
		// something wrote to this user's codes inside the same transaction —
		// impossible with the serialized writer (PLAN.md §1), and worth
		// refusing rather than handing somebody a list that does not match what
		// will be checked.
		return RecoveryCodeSet{}, fmt.Errorf(
			"authn: stored %d recovery codes for user %q, want %d", len(stored), userID, len(codes))
	}

	// The count is safe to log and is the thing an operator wants to see; the
	// codes are not and never appear. M1-015 gives this a durable home in the
	// activity log — minting a set of credentials that bypass a second factor
	// belongs in an audit trail, and this line is the record until then.
	s.log.InfoContext(ctx, "recovery codes issued",
		slog.String("user_id", userID),
		slog.Int("count", len(codes)))

	return RecoveryCodeSet{Codes: codes, GeneratedAt: stored[0].CreatedAt}, nil
}

// VerifyRecoveryCode turns a pending state and a printed code into a session.
//
// It is [Service.VerifyTOTP] with a different second factor, deliberately down
// to the shape of its failures: every way of not getting in is
// [apierr.BadSecondFactor] with one detail, so that "that was a real code, but
// you have already used it" is not a sentence this endpoint can be made to say.
//
// A code is not consulted against the enrolment at all — a person reaching for
// one has typically lost the device the enrolment describes. What it is
// consulted against is the set of unused codes belonging to the account behind
// the pending state, compared in constant time, and the ordering below is what
// makes one code buy exactly one session: the code is spent first, and only a
// caller who won that race goes on to spend the challenge and be issued
// anything.
func (s *Service) VerifyRecoveryCode(ctx context.Context, token challenge.Token, presented string,
	req session.Request) (VerifyResult, error) {
	pending, err := s.challenges.Resolve(ctx, token)
	if errors.Is(err, challenge.ErrNoChallenge) {
		return VerifyResult{}, apierr.BadSecondFactor(err.Error())
	}
	if err != nil {
		return VerifyResult{}, err
	}

	user, err := s.users.ByID(ctx, pending.UserID)
	if errors.Is(err, apierr.ErrNotFound) {
		return VerifyResult{}, apierr.BadSecondFactor(
			"challenge " + pending.ID + " belongs to user " + pending.UserID + ", which is gone")
	}
	if err != nil {
		return VerifyResult{}, err
	}
	if user.Status != identity.StatusActive {
		// Disabling somebody between their password and their code must stop
		// them here, not when the challenge happens to expire.
		return VerifyResult{}, apierr.BadSecondFactor(
			"user " + user.ID + " is " + string(user.Status))
	}

	code, err := recovery.Parse(presented)
	if err != nil {
		// Malformed rather than wrong, and answered identically. It reaches
		// here at all only because api/openapi.yaml is deliberately looser than
		// [recovery.Parse]: the shape of a code is defined in one place, and it
		// is that function.
		return VerifyResult{}, apierr.BadSecondFactor(
			"malformed recovery code for user " + user.ID + ": " + err.Error())
	}

	// Only the unused ones. A code with no confirmed enrolment behind it cannot
	// exist — minting them is what confirming an enrolment does, and removing
	// one deletes them — so there is no separate "are they enrolled" check to
	// disagree with this list.
	held, err := s.recoveryCodes.Unused(ctx, user.ID)
	if err != nil {
		return VerifyResult{}, err
	}
	matched, found := matchRecoveryCode(s.recovery.Hash(code), held)
	if !found {
		return VerifyResult{}, apierr.BadSecondFactor("no unused recovery code matches for user " + user.ID)
	}

	spent, err := s.recoveryCodes.Use(ctx, matched.ID, s.now())
	if err != nil {
		return VerifyResult{}, err
	}
	if !spent {
		// Another request got there first, or a regeneration removed the row
		// between the read and the write. Either way this caller is presenting
		// something that is no longer live.
		return VerifyResult{}, apierr.BadSecondFactor(
			"recovery code " + matched.ID + " was already spent for user " + user.ID)
	}

	consumed, err := s.challenges.Spend(ctx, pending.ID)
	if err != nil {
		return VerifyResult{}, err
	}
	if !consumed {
		return VerifyResult{}, apierr.BadSecondFactor("challenge " + pending.ID + " was already spent")
	}

	// mfa_satisfied, and genuinely so: this is a full session and not a
	// diminished one. A person who holds a printed code has presented a second
	// factor, and the acceptance criterion says so in as many words.
	issued, err := s.sessions.Issue(ctx, user.ID, req, true)
	if err != nil {
		return VerifyResult{}, err
	}
	if err := s.users.SetLastLoginAt(ctx, user.ID, issued.Session.CreatedAt); err != nil {
		return VerifyResult{}, err
	}

	remaining, err := s.recoveryCodes.CountUnused(ctx, user.ID)
	if err != nil {
		return VerifyResult{}, err
	}
	// Warn, not info. Somebody signing in with a recovery code has either lost
	// their authenticator or is using a code that was not theirs to hold, and
	// both are worth standing out from the sign-ins around them. This is the
	// security-relevant event M1-007 wants written to M1-015's activity log;
	// until that exists this line is the record, which is why it carries what an
	// entry would.
	s.log.WarnContext(ctx, "login completed with a recovery code",
		slog.String("user_id", user.ID),
		slog.String("session_id", issued.Session.ID),
		slog.String("code_id", matched.ID),
		slog.Int("codes_remaining", remaining))

	return VerifyResult{Subject: subjectOf(user, issued.Session), Issued: issued}, nil
}

// RegenerateRecoveryCodes replaces the caller's set and returns the new codes.
//
// It asks for two things, and the second is the interesting one. The current
// password, for the reason every self-service change here asks for it: a
// session left open on a shared machine must not be enough to mint credentials
// that walk past a second factor. And a session that has *already* satisfied
// MFA — rather than a fresh code from the authenticator, which M1-007 would
// also allow — because signing in with a recovery code produces exactly such a
// session, and requiring a code would lock the person whose phone is gone out of
// replacing the codes they are spending. That is the case these exist for.
//
// Every previous code is invalidated, unused ones included. Somebody
// regenerating because a printout went missing is telling us that the missing
// printout must stop working.
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, subject Subject,
	current password.Plaintext) (RecoveryCodeSet, error) {
	user, err := s.users.ByID(ctx, subject.UserID)
	if err != nil {
		return RecoveryCodeSet{}, err
	}

	if !subject.MFASatisfied {
		return RecoveryCodeSet{}, apierr.Forbidden("mint recovery codes for user " + user.ID +
			" from a session that has not satisfied MFA")
	}
	if _, enrolled, err := s.confirmedTOTP(ctx, user.ID); err != nil {
		return RecoveryCodeSet{}, err
	} else if !enrolled {
		// Codes stand in for an authenticator, so there has to be one to stand
		// in for. Without this a session that satisfied MFA and then removed the
		// factor could mint a set that outlived what they replace.
		return RecoveryCodeSet{}, apierr.Conflict(
			"there is no authenticator enrolled; recovery codes stand in for one, " +
				"and are issued when an enrolment is confirmed")
	}
	if user.PasswordHash == "" {
		return RecoveryCodeSet{}, apierr.Validation(apierr.Field("currentPassword",
			"this account signs in through an identity provider and has no password here"))
	}

	ok, _, err := password.Verify(current, user.PasswordHash)
	if err != nil {
		return RecoveryCodeSet{}, fmt.Errorf("authn: verify the password of user %q: %w", user.ID, err)
	}
	if !ok {
		// A field error rather than a 401: the caller is signed in and knows
		// perfectly well whether this account exists, so there is nothing to
		// give away, and a form can put the message next to the input.
		return RecoveryCodeSet{}, apierr.Validation(apierr.Field("currentPassword", "is not correct"))
	}

	set, err := s.issueRecoveryCodes(ctx, user.ID)
	if err != nil {
		return RecoveryCodeSet{}, err
	}

	s.log.WarnContext(ctx, "recovery codes regenerated; every previous code is now invalid",
		slog.String("user_id", user.ID),
		slog.String("session_id", subject.SessionID))

	return set, nil
}

// matchRecoveryCode finds the row whose stored hash is hash, comparing every
// candidate in constant time.
//
// It does not stop at the first match, and that is the point: returning early
// would make the time this takes depend on *which* code was presented, and the
// loop is ten iterations of a byte comparison either way. The index is carried
// rather than the row so that the branch inside the loop does no work worth
// measuring.
func matchRecoveryCode(hash string, held []identity.RecoveryCode) (identity.RecoveryCode, bool) {
	found := -1
	for i, candidate := range held {
		if recovery.Equal(hash, candidate.CodeHash) {
			found = i
		}
	}
	if found < 0 {
		return identity.RecoveryCode{}, false
	}
	return held[found], true
}
