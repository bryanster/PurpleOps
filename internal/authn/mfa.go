package authn

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bryanster/blacklight/internal/authn/challenge"
	"github.com/bryanster/blacklight/internal/authn/password"
	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/authn/totp"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// The second factor (M1-006): enrolling an authenticator, confirming it,
// presenting it at sign-in, and removing it.
//
// The mechanism is here and the *enforcement* is M1-008's. What this file
// establishes is that a confirmed factor is asked for, that presenting one is
// the only way from a pending state to a session, and that a code cannot be
// used twice — the three things v1 either did not have or had somewhere a
// handler could skip.

// EnrollTOTP mints a new, unconfirmed authenticator secret for the caller.
//
// Unconfirmed is the whole design: until [Service.ConfirmTOTP] succeeds this
// changes nothing about signing in, so a person who scans the QR code and then
// closes the tab has not locked themselves out. Calling it again replaces an
// unconfirmed secret, which is what makes a second attempt at the scan work.
//
// A *confirmed* enrolment is not replaced — that is [apierr.Conflict], and the
// way past it is [Service.DisableTOTP], which asks for the current password.
// Otherwise a borrowed session would be enough to swap somebody's second factor
// for one the borrower holds.
func (s *Service) EnrollTOTP(ctx context.Context, subject Subject) (totp.Enrolment, error) {
	user, err := s.users.ByID(ctx, subject.UserID)
	if err != nil {
		return totp.Enrolment{}, err
	}

	if _, enrolled, err := s.confirmedTOTP(ctx, user.ID); err != nil {
		return totp.Enrolment{}, err
	} else if enrolled {
		return totp.Enrolment{}, apierr.Conflict(
			"an authenticator is already enrolled; remove it before enrolling another")
	}

	enrolment, err := totp.Generate(s.issuer, user.Email)
	if err != nil {
		return totp.Enrolment{}, err
	}
	sealed, err := s.secrets.Seal([]byte(enrolment.Secret))
	if err != nil {
		return totp.Enrolment{}, err
	}
	if _, err := s.totp.Enroll(ctx, identity.NewTOTP{
		UserID:          user.ID,
		SecretEncrypted: sealed,
	}); err != nil {
		return totp.Enrolment{}, err
	}

	// The secret is not in this line and must never be: [totp.Enrolment] holds
	// it in three fields, and every one of them is the same credential.
	s.log.InfoContext(ctx, "authenticator enrolment started",
		slog.String("user_id", user.ID))

	return enrolment, nil
}

// ConfirmResult is a finished enrolment: the caller's rotated session, and the
// recovery codes minted alongside it.
//
// The codes are here and in no other result type, which is what makes "shown
// exactly once" (M1-007) structural rather than a rule somebody has to keep:
// there is no second endpoint that returns them, so a client that loses this
// response has nothing to retry — only [Service.RegenerateRecoveryCodes], which
// mints a different set.
type ConfirmResult struct {
	Issued   session.Issued
	Recovery RecoveryCodeSet
}

// ConfirmTOTP finishes an enrolment by checking a code from it, and returns the
// caller's rotated session together with a fresh set of recovery codes.
//
// Success does four things at once, and they are one event: the enrolment
// becomes confirmed, the code's step is spent so it cannot be replayed, ten
// recovery codes are minted (M1-007), and the session is marked as having
// satisfied MFA and rotated onto a new token.
//
// The codes are minted before the session is marked, so that a failure to store
// them is a failure of the whole exchange rather than an enrolment that
// silently has no way out behind it. The person is then still able to sign in
// with the authenticator they have just proved works, and can mint a set from
// there.
//
// A wrong code is a field error rather than a 401. The caller is signed in and
// is confirming their own secret, so there is nothing to give away and a form
// can put the message next to the input — and, for the same reason, this is not
// throttled: the only secret being guessed is the caller's own.
func (s *Service) ConfirmTOTP(ctx context.Context, subject Subject, code string) (ConfirmResult, error) {
	enrolment, err := s.totp.ByUserID(ctx, subject.UserID)
	if err != nil {
		// Not-found flows through as a 404: there is nothing to confirm, which
		// is a client that has lost track of the flow rather than a bad code.
		return ConfirmResult{}, err
	}

	step, err := s.checkCode(ctx, enrolment, code)
	switch {
	case errors.Is(err, totp.ErrNoMatch):
		return ConfirmResult{}, apierr.Validation(apierr.Field("code",
			"is not correct — check your authenticator and try the current code"))
	case err != nil:
		return ConfirmResult{}, err
	}

	accepted, err := s.totp.Accept(ctx, subject.UserID, step, s.now())
	if err != nil {
		return ConfirmResult{}, err
	}
	if !accepted {
		// The step was spent between the check and the write, which for a
		// confirmation means the same code was submitted twice. Answered
		// identically to a wrong code: it is wrong now.
		return ConfirmResult{}, apierr.Validation(apierr.Field("code",
			"has already been used — wait for the next one"))
	}

	set, err := s.issueRecoveryCodes(ctx, subject.UserID)
	if err != nil {
		return ConfirmResult{}, err
	}

	issued, err := s.sessions.SatisfyMFA(ctx, subject.SessionID)
	if err != nil {
		return ConfirmResult{}, err
	}

	s.log.InfoContext(ctx, "authenticator enrolment confirmed",
		slog.String("user_id", subject.UserID),
		slog.String("session_id", subject.SessionID))
	s.recordAlone(ctx, events.Entry{
		ActorID:    subject.UserID,
		Verb:       events.VerbMFAEnrolled,
		ObjectType: events.ObjectTOTP,
		ObjectID:   subject.UserID,
	})

	return ConfirmResult{Issued: issued, Recovery: set}, nil
}

// VerifyResult is a completed sign-in: the caller, and the session they leave
// with.
type VerifyResult struct {
	Subject Subject
	Issued  session.Issued
}

// VerifyTOTP turns a pending state and a code into a session.
//
// Every way of failing is [apierr.BadSecondFactor] — no pending state, one that
// expired or was already spent, an account disabled since the password was
// checked, no enrolment, a wrong code, and a replayed one. They are one answer
// because a caller cannot act differently on the difference and an attacker
// could: "that code was right but stale" is a much smaller search space than
// "no".
//
// The order is what makes one correct code buy exactly one session. The code is
// checked first and its step spent atomically; only then is the challenge spent,
// also atomically; only then is a session issued. Two requests racing with the
// same code lose at the first of those, and two racing with different codes on
// one challenge lose at the second.
func (s *Service) VerifyTOTP(ctx context.Context, token challenge.Token, code string,
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

	enrolment, enrolled, err := s.confirmedTOTP(ctx, user.ID)
	if err != nil {
		return VerifyResult{}, err
	}
	if !enrolled {
		// Removed between the password and the code. There is nothing to check
		// against, so there is nothing to accept.
		return VerifyResult{}, apierr.BadSecondFactor("user " + user.ID + " has no confirmed authenticator")
	}

	step, err := s.checkCode(ctx, enrolment, code)
	switch {
	case errors.Is(err, totp.ErrNoMatch):
		return VerifyResult{}, apierr.BadSecondFactor("code mismatch for user " + user.ID)
	case err != nil:
		return VerifyResult{}, err
	}

	accepted, err := s.totp.Accept(ctx, user.ID, step, s.now())
	if err != nil {
		return VerifyResult{}, err
	}
	if !accepted {
		return VerifyResult{}, apierr.BadSecondFactor(
			fmt.Sprintf("step %d has already been used by user %s", step, user.ID))
	}

	spent, err := s.challenges.Spend(ctx, pending.ID)
	if err != nil {
		return VerifyResult{}, err
	}
	if !spent {
		return VerifyResult{}, apierr.BadSecondFactor("challenge " + pending.ID + " was already spent")
	}

	// A fresh session, exactly as password-only login issues one, and with
	// mfa_satisfied set — this is the one path that sets it at issue time
	// rather than by marking an existing session (see ConfirmTOTP).
	issued, err := s.sessions.Issue(ctx, user.ID, req, true)
	if err != nil {
		return VerifyResult{}, err
	}
	if err := s.users.SetLastLoginAt(ctx, user.ID, issued.Session.CreatedAt); err != nil {
		return VerifyResult{}, err
	}

	s.log.InfoContext(ctx, "login completed with a second factor",
		slog.String("user_id", user.ID),
		slog.String("session_id", issued.Session.ID))

	return VerifyResult{Subject: subjectOf(user, issued.Session), Issued: issued}, nil
}

// DisableTOTP removes the caller's authenticator.
//
// It requires the current password for the same reason changing one does: a
// session left open on a shared machine must not be enough to take the second
// factor off an account.
//
// It is refused outright while a second factor is *required* of this person —
// by the platform policy or by their own flag, which are the same answer here
// (M1-008). Removing it would leave an account subject to a requirement it can
// no longer satisfy, and the account most likely to be doing this is the
// administrator who just turned the policy on: an endpoint that let them take
// their own factor off would let them lock the platform's last administrator
// into a state only the host's filesystem can undo.
func (s *Service) DisableTOTP(ctx context.Context, subject Subject, current password.Plaintext) error {
	user, err := s.users.ByID(ctx, subject.UserID)
	if err != nil {
		return err
	}

	policy, err := s.mfaPolicy(ctx)
	if err != nil {
		return err
	}
	if policy.Requires(user) {
		return apierr.Forbidden("remove the authenticator of user " + user.ID +
			", of whom a second factor is required")
	}
	if user.PasswordHash == "" {
		return apierr.Validation(apierr.Field("currentPassword",
			"this account signs in through an identity provider and has no password here"))
	}

	ok, _, err := password.Verify(current, user.PasswordHash)
	if err != nil {
		return fmt.Errorf("authn: verify the password of user %q: %w", user.ID, err)
	}
	if !ok {
		return apierr.Validation(apierr.Field("currentPassword", "is not correct"))
	}

	if err := s.totp.Delete(ctx, user.ID); err != nil {
		return err
	}
	// And the codes with it (M1-007). They stand in for the authenticator, so
	// leaving them behind would mean a second factor that was removed is still
	// presentable — and the next enrolment mints its own set anyway, so keeping
	// these could only ever mean two live sets.
	if err := s.recoveryCodes.DeleteForUser(ctx, user.ID); err != nil {
		return err
	}

	// Warn rather than info: an authenticator disappearing is the event somebody
	// reads back after an account is taken over, and it should stand out from
	// the successful sign-ins around it.
	s.log.WarnContext(ctx, "authenticator and recovery codes removed",
		slog.String("user_id", user.ID),
		slog.String("session_id", subject.SessionID))
	s.recordAlone(ctx, events.Entry{
		ActorID:    user.ID,
		Verb:       events.VerbMFADisabled,
		ObjectType: events.ObjectTOTP,
		ObjectID:   user.ID,
	})

	return nil
}

// AccountForChallenge returns the email address a pending token belongs to, or
// "" when the token stands for nothing usable.
//
// It exists for the throttle (M1-004), which keys on the account being
// attempted and cannot read one out of a body that contains only six digits.
// The answer never reaches a response — it reaches the limiter and the log —
// so this is not a way to turn a pending token into an address.
func (s *Service) AccountForChallenge(ctx context.Context, token challenge.Token) string {
	pending, err := s.challenges.Resolve(ctx, token)
	if err != nil {
		return ""
	}
	user, err := s.users.ByID(ctx, pending.UserID)
	if err != nil {
		return ""
	}
	return user.Email
}

// confirmedTOTP returns a user's enrolment and whether it is one that counts.
//
// "Counts" means confirmed. A started-but-unconfirmed enrolment is reported as
// not enrolled everywhere in this package, which is what makes the acceptance
// criterion — a half-finished enrolment cannot lock a user out — true in the
// login path, in the profile and in the verification path at once, rather than
// three times over.
func (s *Service) confirmedTOTP(ctx context.Context, userID string) (identity.TOTP, bool, error) {
	enrolment, err := s.totp.ByUserID(ctx, userID)
	if errors.Is(err, apierr.ErrNotFound) {
		return identity.TOTP{}, false, nil
	}
	if err != nil {
		return identity.TOTP{}, false, err
	}
	return enrolment, enrolment.Confirmed(), nil
}

// checkCode decrypts an enrolment's secret and reports which step the code
// belongs to, or [totp.ErrNoMatch].
//
// The plaintext secret exists only inside this function. It is never returned,
// never logged and never stored on the Service — the cipher is the only thing
// that holds it in any durable form.
func (s *Service) checkCode(ctx context.Context, enrolment identity.TOTP, code string) (int64, error) {
	secret, err := s.secrets.Open(enrolment.SecretEncrypted)
	if err != nil {
		// A row this key cannot open. That is an operational problem — a
		// restored backup, a rotated BLACKLIGHT_ENCRYPTION_KEY — and not a failed
		// code, and reporting it as one would leave nobody looking at it while
		// everybody's codes quietly stopped working.
		s.log.ErrorContext(ctx, "an enrolled authenticator secret could not be decrypted; "+
			"has BLACKLIGHT_ENCRYPTION_KEY changed?",
			slog.String("user_id", enrolment.UserID),
			slog.String("error", err.Error()))
		return 0, fmt.Errorf("authn: read the authenticator secret of user %q: %w",
			enrolment.UserID, err)
	}
	return totp.Validate(string(secret), code, s.now(), enrolment.LastUsedStep)
}
