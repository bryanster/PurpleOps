package authn

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bryanster/blacklight/internal/authn/challenge"
	"github.com/bryanster/blacklight/internal/authn/password"
	"github.com/bryanster/blacklight/internal/authn/recovery"
	"github.com/bryanster/blacklight/internal/authn/secrets"
	"github.com/bryanster/blacklight/internal/authn/servicetoken"
	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Users, Memberships and TOTPs are the parts of the identity store this package
// needs. [*identity.Users], [*identity.Memberships] and [*identity.TOTPs]
// satisfy them.
type (
	Users interface {
		ByID(ctx context.Context, id string) (identity.User, error)
		ByEmail(ctx context.Context, email string) (identity.User, error)
		Page(ctx context.Context, f identity.PageFilter) ([]identity.User, string, error)
		Create(ctx context.Context, u identity.NewUser, after ...identity.After) (identity.User, error)
		Update(ctx context.Context, u identity.User, after ...identity.After) (identity.User, error)
		SetLastLoginAt(ctx context.Context, id string, at time.Time) error
	}

	Memberships interface {
		ListByUser(ctx context.Context, userID string) ([]identity.Membership, error)
	}

	TOTPs interface {
		ByUserID(ctx context.Context, userID string) (identity.TOTP, error)
		Enroll(ctx context.Context, in identity.NewTOTP) (identity.TOTP, error)
		Accept(ctx context.Context, userID string, step int64, at time.Time) (bool, error)
		Delete(ctx context.Context, userID string) error
	}

	RecoveryCodes interface {
		Replace(ctx context.Context, userID string, hashes []string) ([]identity.RecoveryCode, error)
		Unused(ctx context.Context, userID string) ([]identity.RecoveryCode, error)
		CountUnused(ctx context.Context, userID string) (int, error)
		Use(ctx context.Context, id string, at time.Time) (bool, error)
		DeleteForUser(ctx context.Context, userID string) error
	}
)

// Deps is everything a [Service] is built from. It is a struct rather than a
// list of arguments because M1 keeps adding to it — SSO (M1-009, M1-010) and
// service tokens (M1-011) both arrive here — and a positional constructor with
// nine parameters is one a caller gets wrong silently.
type Deps struct {
	Users         Users
	Memberships   Memberships
	TOTP          TOTPs
	RecoveryCodes RecoveryCodes

	// Identities is the login methods attached to an account. It is what the
	// federated sign-in path (M1-009) looks an account up by, and it is required
	// whether or not this deployment has an identity provider configured — a
	// Service that could not answer "whose subject is this?" would answer it
	// wrongly rather than not at all.
	Identities Identities

	// Settings holds the platform MFA policy (M1-008), which is read on every
	// sign-in and on every request made with a session that has not satisfied
	// MFA.
	Settings Settings

	Sessions   *session.Manager
	Challenges *challenge.Manager

	// Tokens is the service-token manager (M1-011). It is required whether or
	// not anybody in this deployment holds a token: a Service that could not
	// answer "whose token is this?" would leave the authentication middleware
	// with a credential it had no way to check, and the safe thing to do with
	// one of those is not to serve.
	Tokens *servicetoken.Manager

	// Secrets encrypts what this server holds on somebody else's behalf: today
	// the TOTP shared secrets, which is the only thing that reads it.
	Secrets *secrets.Cipher

	// Recovery hashes the codes that stand in for an authenticator (M1-007).
	// It is a separate thing from Secrets because it does a different job —
	// authenticating a value rather than storing one — under a key derived
	// separately from the same configured material.
	Recovery *recovery.Hasher

	// Activity is the append-only security event log (M1-015). Nil skips
	// durable rows; production always sets it.
	Activity *events.Log

	// Issuer is what an authenticator app shows as the name of the entry. It
	// names the deployment, so that somebody with an account on two of these can
	// tell their two entries apart.
	Issuer string

	// Log receives what a response cannot carry. Nil means slog.Default().
	Log *slog.Logger

	// Now reads the clock. Nil means time.Now; a test supplies its own so that a
	// TOTP window can be reached without waiting for one.
	Now func() time.Time
}

// Service is the local login path: signing in, signing out, reporting who the
// caller is, changing one's own password, and the second factor that stands
// between the first two (M1-006).
//
// It holds the rules that decide those things, so that the HTTP layer above it
// does no more than translate. SSO (M1-009, M1-010) and service tokens (M1-011)
// add their own ways of arriving at a [Subject]; the session, the cookie and the
// rotation rules stay here and are shared.
type Service struct {
	users         Users
	memberships   Memberships
	totp          TOTPs
	recoveryCodes RecoveryCodes
	identities    Identities
	settings      Settings

	sessions   *session.Manager
	challenges *challenge.Manager
	tokens     *servicetoken.Manager
	secrets    *secrets.Cipher
	recovery   *recovery.Hasher
	activity   *events.Log

	issuer string
	log    *slog.Logger
	now    func() time.Time
}

// NewService returns a Service over deps, or an error naming the dependency
// that is missing. Everything is required: a Service with no cipher would enrol
// authenticators it could not read back, and one with no challenge manager
// would answer mfa_required with nothing to answer it.
func NewService(deps Deps) (*Service, error) {
	switch {
	case deps.Users == nil:
		return nil, errors.New("authn: no user repository")
	case deps.Memberships == nil:
		return nil, errors.New("authn: no membership repository")
	case deps.TOTP == nil:
		return nil, errors.New("authn: no authenticator repository")
	case deps.RecoveryCodes == nil:
		return nil, errors.New("authn: no recovery code repository")
	case deps.Identities == nil:
		return nil, errors.New("authn: no identity repository; a federated sign-in could not find an account")
	case deps.Settings == nil:
		return nil, errors.New("authn: no settings store; the MFA policy could not be read")
	case deps.Sessions == nil:
		return nil, errors.New("authn: no session manager")
	case deps.Challenges == nil:
		return nil, errors.New("authn: no MFA challenge manager")
	case deps.Tokens == nil:
		return nil, errors.New("authn: no service token manager; a presented token could not be checked")
	case deps.Secrets == nil:
		return nil, errors.New("authn: no cipher; enrolled secrets could not be stored")
	case deps.Recovery == nil:
		return nil, errors.New("authn: no recovery code hasher; codes could not be checked")
	case strings.TrimSpace(deps.Issuer) == "":
		return nil, errors.New("authn: no issuer; authenticator apps would show an unnamed entry")
	}

	log := deps.Log
	if log == nil {
		log = slog.Default()
	}
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		users:         deps.Users,
		memberships:   deps.Memberships,
		totp:          deps.TOTP,
		recoveryCodes: deps.RecoveryCodes,
		identities:    deps.Identities,
		settings:      deps.Settings,
		sessions:      deps.Sessions,
		challenges:    deps.Challenges,
		tokens:        deps.Tokens,
		secrets:       deps.Secrets,
		recovery:      deps.Recovery,
		activity:      deps.Activity,
		issuer:        deps.Issuer,
		log:           log,
		now:           now,
	}, nil
}

// LoginStatus is how far a sign-in got.
type LoginStatus string

const (
	// LoginAuthenticated means the credentials were right and a session was
	// issued.
	LoginAuthenticated LoginStatus = "authenticated"

	// LoginMFARequired means the credentials were right and are not enough: a
	// second factor is required of this person and was not presented. No session
	// exists yet.
	LoginMFARequired LoginStatus = "mfa_required"

	// LoginMFAEnrolmentRequired means the credentials were right, a second
	// factor is required of this person, and they hold none to present. A
	// session exists and may do exactly one thing: enrol one (M1-008).
	LoginMFAEnrolmentRequired LoginStatus = "mfa_enrolment_required"
)

// Login is a sign-in attempt.
type Login struct {
	Email    string
	Password password.Plaintext

	// Request is what to record about where this session was created.
	Request session.Request
}

// LoginResult is the outcome of a successful sign-in attempt. Successful is not
// the same as signed in: read Status.
type LoginResult struct {
	Status LoginStatus

	// Subject and Issued are the session, and are zero unless Status is
	// [LoginAuthenticated].
	Subject Subject
	Issued  session.Issued

	// Challenge is the pending state, and is zero unless Status is
	// [LoginMFARequired]. It is what the caller answers with a code; it
	// authorizes nothing else.
	Challenge challenge.Issued
}

// Login checks an email address and password and, if nothing else is required,
// issues a session.
//
// Every failure is [apierr.BadCredentials] — one code, one detail, one body, for
// an address nobody holds, a wrong password, an account that has been disabled
// and one that signs in through an identity provider instead. The work done is
// the same too: an unknown address is checked against a decoy hash, so that the
// time a login takes does not say whether the account exists.
func (s *Service) Login(ctx context.Context, in Login) (LoginResult, error) {
	user, err := s.users.ByEmail(ctx, in.Email)
	switch {
	case errors.Is(err, apierr.ErrNotFound):
		// The decoy is hashed with today's parameters, so this path costs what
		// the real one costs. Its result is discarded — there is no password it
		// could match.
		if _, _, decoyErr := password.Verify(in.Password, decoyHash()); decoyErr != nil {
			return LoginResult{}, fmt.Errorf("authn: verify against the decoy hash: %w", decoyErr)
		}
		s.recordLoginFailed(ctx, in, "unknown_email")
		return LoginResult{}, apierr.BadCredentials("no account holds that address")
	case err != nil:
		return LoginResult{}, err
	}

	stored := user.PasswordHash
	if stored == "" {
		// An account with no local password — every SSO-only one. It still costs
		// a hash, for the same reason the unknown address does.
		stored = decoyHash()
	}
	ok, needsRehash, err := password.Verify(in.Password, stored)
	if err != nil {
		// The stored hash could not be read. That is a damaged row and an
		// operational problem, not a failed login, and reporting it as one would
		// leave nobody looking at it.
		return LoginResult{}, fmt.Errorf("authn: verify the password of user %q: %w", user.ID, err)
	}
	// The order is about what is checked, not about what is answered — every one
	// of these is the same 401. An account with no local password is refused
	// before the comparison's result is consulted, because that result was
	// against a decoy and means nothing.
	switch {
	case user.PasswordHash == "":
		s.recordLoginFailed(ctx, in, "no_local_password")
		return LoginResult{}, apierr.BadCredentials("user " + user.ID + " has no local password")
	case !ok:
		s.recordLoginFailed(ctx, in, "password_mismatch")
		return LoginResult{}, apierr.BadCredentials("password mismatch for user " + user.ID)
	case user.Status != identity.StatusActive:
		s.recordLoginFailed(ctx, in, "account_"+string(user.Status))
		return LoginResult{}, apierr.BadCredentials(
			"user " + user.ID + " is " + string(user.Status))
	}

	if needsRehash {
		s.upgradeHash(ctx, user, in.Password)
	}

	return s.completeSignIn(ctx, user, in.Request)
}

// recordLoginFailed appends session.login_failed. Failures here are logged and
// swallowed: refusing a 401 because the audit row could not be written would
// turn bookkeeping into an outage, and the credentials were wrong either way.
func (s *Service) recordLoginFailed(ctx context.Context, in Login, reason string) {
	if s.activity == nil {
		return
	}
	if err := s.activity.RecordAlone(ctx, events.Entry{
		Verb:       events.VerbSessionLoginFailed,
		ObjectType: events.ObjectLogin,
		ObjectID:   "unknown",
		Delta: events.Delta(map[string]any{
			"email":  in.Email,
			"ip":     in.Request.IP,
			"reason": reason,
		}),
	}); err != nil {
		s.log.WarnContext(ctx, "could not record session.login_failed",
			slog.String("error", err.Error()))
	}
}

// recordAlone writes an activity row that has no sibling mutation. A failure is
// logged and swallowed so audit bookkeeping cannot break the auth flow that
// already succeeded.
func (s *Service) recordAlone(ctx context.Context, e events.Entry) {
	if s.activity == nil {
		return
	}
	if err := s.activity.RecordAlone(ctx, e); err != nil {
		s.log.WarnContext(ctx, "could not record activity",
			slog.String("verb", string(e.Verb)),
			slog.String("error", err.Error()))
	}
}

// completeSignIn is everything a sign-in does once the credentials are settled:
// evaluate the MFA policy, and issue whatever that leaves — a session, a
// challenge, or a session confined to enrolling.
//
// It is shared by the two ways of arriving here, local (above) and federated
// (federated.go). That sharing is the point: the rules about second factors and
// what a session may do are M1-006's and M1-008's, and a second sign-in path
// with its own copy of them is how one of the two ends up exempt.
func (s *Service) completeSignIn(ctx context.Context, user identity.User, req session.Request) (LoginResult, error) {
	// Policy first, enrolment second. Asking them the other way round is the
	// defect M1-008 closes: enforcement that consults enrolment state is
	// enforcement that stops applying to whoever skipped enrolling.
	policy, err := s.mfaPolicy(ctx)
	if err != nil {
		return LoginResult{}, err
	}
	enrolment, enrolled, err := s.confirmedTOTP(ctx, user.ID)
	if err != nil {
		return LoginResult{}, err
	}

	switch decideLogin(policy.Requires(user), enrolled) {
	case outcomeChallenge:
		// The password was right and no session exists yet — what the caller
		// gets is a challenge, which authorizes nothing but the verification
		// endpoints.
		issued, err := s.challenges.Open(ctx, user.ID)
		if err != nil {
			return LoginResult{}, err
		}
		s.log.InfoContext(ctx, "login requires a second factor",
			slog.String("user_id", user.ID),
			slog.String("challenge_id", issued.Challenge.ID),
			slog.Time("enrolled_at", enrolment.ConfirmedAt))
		return LoginResult{Status: LoginMFARequired, Challenge: issued}, nil

	case outcomeEnrolment:
		// A second factor is required of this person and they hold none. Before
		// M1-008 this was a dead end — the credentials were right, no session
		// was issued and there was nothing the caller could do about it. Now
		// they get a session that can reach the enrolment endpoints and
		// nothing else (see [Service.Authenticate] and the gate in
		// internal/httpapi), so the requirement is something they can satisfy
		// rather than a door that will not open.
		issued, err := s.issueSession(ctx, user, req)
		if err != nil {
			return LoginResult{}, err
		}
		s.log.WarnContext(ctx, "login confined to enrolment: MFA is required and nothing is enrolled",
			slog.String("user_id", user.ID),
			slog.String("session_id", issued.Session.ID))

		subject := subjectOf(user, issued.Session)
		subject.MFAEnrolmentRequired = true
		return LoginResult{
			Status:  LoginMFAEnrolmentRequired,
			Subject: subject,
			Issued:  issued,
		}, nil

	default:
		issued, err := s.issueSession(ctx, user, req)
		if err != nil {
			return LoginResult{}, err
		}
		s.log.InfoContext(ctx, "login",
			slog.String("user_id", user.ID),
			slog.String("session_id", issued.Session.ID))

		return LoginResult{
			Status:  LoginAuthenticated,
			Subject: subjectOf(user, issued.Session),
			Issued:  issued,
		}, nil
	}
}

// issueSession creates the session a completed sign-in leaves with, and records
// the login against the account.
//
// A fresh session, not a reused one: whatever cookie the caller arrived with,
// the session they leave with is one this exchange created. That is rotation on
// sign-in, and it is what makes a session fixed by an attacker before login
// worthless afterwards. Any other session the person has stays live — signing in
// on a second machine does not sign out the first.
//
// mfa_satisfied is false on it in both of the cases that call this: an ordinary
// sign-in that was not asked for a factor, and one confined to enrolment. The
// flag means "a factor was presented for this session", and neither of them
// presented one. What separates the two is the policy, evaluated on every
// request against the flag — not a second meaning loaded onto the flag itself.
func (s *Service) issueSession(ctx context.Context, user identity.User, req session.Request) (session.Issued, error) {
	issued, err := s.sessions.Issue(ctx, user.ID, req, false)
	if err != nil {
		return session.Issued{}, err
	}
	if err := s.users.SetLastLoginAt(ctx, user.ID, issued.Session.CreatedAt); err != nil {
		return session.Issued{}, err
	}
	return issued, nil
}

// upgradeHash re-hashes a correct password under today's parameters, so that a
// cost raised in password.Default() reaches existing accounts as their owners
// sign in (M1-002).
//
// A failure here is logged and swallowed on purpose: the password was right, and
// refusing to sign somebody in because their hash could not be upgraded would
// turn a background improvement into an outage.
func (s *Service) upgradeHash(ctx context.Context, user identity.User, plaintext password.Plaintext) {
	hash, err := password.Hash(plaintext)
	if err == nil {
		user.PasswordHash = hash
		_, err = s.users.Update(ctx, user)
	}
	if err != nil {
		s.log.WarnContext(ctx, "could not upgrade a password hash on login",
			slog.String("user_id", user.ID),
			slog.String("error", err.Error()))
		return
	}
	s.log.InfoContext(ctx, "upgraded a password hash to the current cost",
		slog.String("user_id", user.ID))
}

// Authenticate resolves a session token to the caller it belongs to.
//
// It reports [session.ErrNoSession] for every way of not being signed in,
// including an account that has been disabled since the session was issued —
// disabling somebody must end their access now, not when their session happens
// to expire. Any other error is the database failing, and the caller must not
// report it as a failure to authenticate.
//
// It is also where the MFA policy reaches a session that already existed when
// the policy changed (M1-008). The requirement is evaluated here, on every
// request, rather than recorded on the session at sign-in — so turning it on
// applies to everybody who is already signed in at their next request, and there
// is no set of sessions quietly grandfathered under the old rules.
func (s *Service) Authenticate(ctx context.Context, token session.Token) (Subject, error) {
	found, err := s.sessions.Resolve(ctx, token)
	if err != nil {
		return Subject{}, err
	}

	user, err := s.users.ByID(ctx, found.UserID)
	if errors.Is(err, apierr.ErrNotFound) {
		return Subject{}, fmt.Errorf("%w: session %s belongs to user %s, which is gone",
			session.ErrNoSession, found.ID, found.UserID)
	}
	if err != nil {
		return Subject{}, err
	}
	if user.Status != identity.StatusActive {
		return Subject{}, fmt.Errorf("%w: user %s is %s",
			session.ErrNoSession, user.ID, user.Status)
	}

	confined, err := s.confineToEnrolment(ctx, user, found)
	if err != nil {
		return Subject{}, err
	}

	subject := subjectOf(user, found)
	subject.MFAEnrolmentRequired = confined
	// Set here, where the cookie was actually resolved, rather than in
	// subjectOf: this is the only function that knows a *cookie* is what proved
	// it, and M1-011's equivalent will say MethodServiceToken in its own.
	subject.Method = authz.MethodCookie
	return subject, nil
}

// confineToEnrolment decides what a live session may do under the policy as it
// stands right now: everything, nothing but enrolling, or nothing at all.
//
// The three answers, and why each is the one it is:
//
//   - The session presented a factor, or none is required of this person. It is
//     an ordinary session. This is the common case and it costs no queries — a
//     satisfied session is never confined, whatever the policy says, so the
//     policy is not read for one.
//   - A factor is required, this session did not present one, and there is
//     nothing enrolled. Confined to enrolling (true, nil). This is the sign-in
//     that happened before the policy was turned on, and the point of M1-008 is
//     that it neither keeps its access nor loses the ability to fix that.
//   - A factor is required, this session did not present one, and there *is* one
//     enrolled. [session.ErrNoSession]: the caller is signed out and must sign
//     in again, which is the flow that asks for the factor they hold. Confining
//     them to enrolment instead would be a dead end — the enrolment endpoint
//     refuses while a confirmed authenticator exists — and letting them through
//     would be the policy not applying to the people most able to satisfy it.
//     Reachable in one step: sign in on two browsers, enrol in one, and the
//     other is holding exactly this session when the policy goes on.
//
// The session row is left alone in all three cases. Nothing here revokes,
// because a policy turned on and then off again should leave the sessions it
// interrupted usable rather than having quietly ended them.
func (s *Service) confineToEnrolment(ctx context.Context, user identity.User, sess identity.Session) (bool, error) {
	if sess.MFASatisfied {
		return false, nil
	}
	policy, err := s.mfaPolicy(ctx)
	if err != nil {
		return false, err
	}
	if !policy.Requires(user) {
		return false, nil
	}

	_, enrolled, err := s.confirmedTOTP(ctx, user.ID)
	if err != nil {
		return false, err
	}
	if enrolled {
		return false, fmt.Errorf(
			"%w: session %s of user %s never presented the second factor now required of them",
			session.ErrNoSession, sess.ID, user.ID)
	}
	return true, nil
}

// Logout revokes the caller's session. It is idempotent: a session that has
// already ended is not an error, because the caller's intent is satisfied.
func (s *Service) Logout(ctx context.Context, subject Subject) error {
	if subject.SessionID == "" {
		return nil
	}
	if err := s.sessions.RevokeAs(ctx, subject.SessionID, subject.UserID); err != nil {
		return err
	}
	s.log.InfoContext(ctx, "logout",
		slog.String("user_id", subject.UserID),
		slog.String("session_id", subject.SessionID))
	return nil
}

// Profile is everything GET /auth/me answers with.
type Profile struct {
	User        identity.User
	Memberships []identity.Membership

	// MFAEnrolled is whether this person has a *confirmed* authenticator. An
	// enrolment that was started and never confirmed is not one, because it
	// gates nothing — reporting it as enrolled would be a lie the interface
	// acted on.
	MFAEnrolled bool

	// MFARequired is whether this person must hold a second factor at all: the
	// platform policy, or their own [identity.User.MFAEnforced] flag, whichever
	// applies (M1-008).
	//
	// It is the effective answer, and the one an interface acts on. The flag on
	// its own is not: somebody can be required by the policy with the flag off,
	// and an interface reading the flag would let them past the screen that is
	// meant to stop them.
	MFARequired bool

	// MFASatisfied is a fact about the session the request arrived on, not about
	// the user, which is why it is not read off User.
	MFASatisfied bool

	// RecoveryCodesRemaining is how many unused recovery codes this person
	// holds (M1-007). It is a count and never the codes: those were shown once
	// and this server could not produce them again if asked.
	//
	// It is here so that an interface can warn somebody who is running out
	// before the last one is spent, which is the moment at which a lost
	// authenticator stops being an inconvenience.
	RecoveryCodesRemaining int
}

// Profile returns the caller, read fresh, with their engagement memberships.
//
// Fresh rather than from the [Subject]: this is what the interface builds itself
// from, and a display name or a role edited a moment ago should be the one it
// shows.
func (s *Service) Profile(ctx context.Context, subject Subject) (Profile, error) {
	user, err := s.users.ByID(ctx, subject.UserID)
	if err != nil {
		return Profile{}, err
	}
	memberships, err := s.memberships.ListByUser(ctx, subject.UserID)
	if err != nil {
		return Profile{}, err
	}
	_, enrolled, err := s.confirmedTOTP(ctx, subject.UserID)
	if err != nil {
		return Profile{}, err
	}
	remaining, err := s.recoveryCodes.CountUnused(ctx, subject.UserID)
	if err != nil {
		return Profile{}, err
	}
	// Read here rather than taken off the [Subject]: the subject carries the
	// state this *session* is in, which is false for a session that has already
	// satisfied MFA — and the interface still has to be able to say "a second
	// factor is required of you" on the account screen of somebody who has one.
	policy, err := s.mfaPolicy(ctx)
	if err != nil {
		return Profile{}, err
	}
	return Profile{
		User:                   user,
		Memberships:            memberships,
		MFAEnrolled:            enrolled,
		MFARequired:            policy.Requires(user),
		MFASatisfied:           subject.MFASatisfied,
		RecoveryCodesRemaining: remaining,
	}, nil
}

// ChangePassword replaces the caller's own password.
//
// It requires the current one, so that a session left open on a shared machine
// is not enough to take the account over. On success the caller's session is
// rotated onto a new token and every other session they have is revoked —
// changing a password signs out the places it was not changed from, which is the
// point of doing it after a scare.
//
// The returned [session.Issued] is the rotated session; its token goes into a
// fresh cookie, and the one the caller sent stops working.
func (s *Service) ChangePassword(ctx context.Context, subject Subject, current, next password.Plaintext) (session.Issued, error) {
	user, err := s.users.ByID(ctx, subject.UserID)
	if err != nil {
		return session.Issued{}, err
	}
	if user.PasswordHash == "" {
		return session.Issued{}, apierr.Validation(apierr.Field("currentPassword",
			"this account signs in through an identity provider and has no password here"))
	}

	ok, _, err := password.Verify(current, user.PasswordHash)
	if err != nil {
		return session.Issued{}, fmt.Errorf("authn: verify the password of user %q: %w", user.ID, err)
	}
	if !ok {
		// A field error rather than a 401: the caller is signed in and knows
		// perfectly well whether this account exists, so there is nothing to
		// give away, and a form can put the message next to the input.
		return session.Issued{}, apierr.Validation(apierr.Field("currentPassword", "is not correct"))
	}

	if err := password.Validate("newPassword", next); err != nil {
		return session.Issued{}, err
	}
	if same, _, err := password.Verify(next, user.PasswordHash); err == nil && same {
		return session.Issued{}, apierr.Validation(apierr.Field("newPassword",
			"must be different from your current password"))
	}

	hash, err := password.Hash(next)
	if err != nil {
		return session.Issued{}, fmt.Errorf("authn: hash the new password of user %q: %w", user.ID, err)
	}
	user.PasswordHash = hash
	if _, err := s.users.Update(ctx, user); err != nil {
		return session.Issued{}, err
	}

	// Others first, then this one. Revoking runs over every session the user
	// has except this one, so the order only decides which token this browser is
	// holding while it happens — and doing it this way means a failure between
	// the two leaves the other sessions gone rather than still live.
	revoked, err := s.sessions.RevokeOthers(ctx, user.ID, subject.SessionID)
	if err != nil {
		return session.Issued{}, err
	}
	// And any half-finished sign-in with it. A pending challenge was opened by
	// the password that has just been replaced; leaving it answerable would mean
	// changing a password after a scare left one door on the old one open.
	if err := s.challenges.Discard(ctx, user.ID); err != nil {
		return session.Issued{}, err
	}
	issued, err := s.sessions.Rotate(ctx, subject.SessionID)
	if err != nil {
		return session.Issued{}, err
	}

	s.log.InfoContext(ctx, "password changed",
		slog.String("user_id", user.ID),
		slog.String("session_id", subject.SessionID),
		slog.Int64("other_sessions_revoked", revoked))
	s.recordAlone(ctx, events.Entry{
		ActorID:    user.ID,
		Verb:       events.VerbUserPasswordChanged,
		ObjectType: events.ObjectUser,
		ObjectID:   user.ID,
	})

	return issued, nil
}

// subjectOf builds the caller from the two rows that describe them.
func subjectOf(user identity.User, sess identity.Session) Subject {
	return Subject{
		UserID:       user.ID,
		Email:        user.Email,
		DisplayName:  user.DisplayName,
		PlatformRole: user.PlatformRole,
		SessionID:    sess.ID,
		MFASatisfied: sess.MFASatisfied,
	}
}

// decoyHash returns a hash of a password nobody knows, for the sign-in paths
// where there is no stored hash to check against.
//
// Without it, an address nobody holds would be answered before an Argon2id
// derivation had run and one that does would be answered after — a difference of
// a hundred milliseconds or more, which is a reliable oracle for enumerating who
// has an account here.
//
// Built once, on the first login that needs it rather than at startup: it costs
// a full derivation, and a process that never sees a failed login should not pay
// for one. It is made with today's parameters, so it stays as expensive as the
// real thing when those are raised.
var decoyHash = sync.OnceValue(func() string {
	secret, err := randomPlaintext()
	if err == nil {
		var hash string
		if hash, err = password.Hash(secret); err == nil {
			return hash
		}
	}
	// Unreachable short of crypto/rand failing, which is not survivable anyway.
	// Panicking here would turn it into a crash inside a login attempt; instead
	// the empty string flows on to Verify, which reports a malformed hash, and
	// the login becomes a 500 that says so.
	slog.Error("could not build the decoy password hash; "+
		"failed logins will be answered faster than real ones",
		slog.String("error", err.Error()))
	return ""
})

// randomPlaintext returns a password nobody has: 32 bytes from crypto/rand,
// base64url.
func randomPlaintext() (password.Plaintext, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("authn: read random bytes: %w", err)
	}
	return password.Plaintext(base64.RawURLEncoding.EncodeToString(raw)), nil
}
