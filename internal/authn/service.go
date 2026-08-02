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
	"github.com/bryanster/blacklight/internal/authn/session"
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
		Update(ctx context.Context, u identity.User) (identity.User, error)
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

	Sessions   *session.Manager
	Challenges *challenge.Manager

	// Secrets encrypts what this server holds on somebody else's behalf: today
	// the TOTP shared secrets, which is the only thing that reads it.
	Secrets *secrets.Cipher

	// Recovery hashes the codes that stand in for an authenticator (M1-007).
	// It is a separate thing from Secrets because it does a different job —
	// authenticating a value rather than storing one — under a key derived
	// separately from the same configured material.
	Recovery *recovery.Hasher

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

	sessions   *session.Manager
	challenges *challenge.Manager
	secrets    *secrets.Cipher
	recovery   *recovery.Hasher

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
	case deps.Sessions == nil:
		return nil, errors.New("authn: no session manager")
	case deps.Challenges == nil:
		return nil, errors.New("authn: no MFA challenge manager")
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
		sessions:      deps.Sessions,
		challenges:    deps.Challenges,
		secrets:       deps.Secrets,
		recovery:      deps.Recovery,
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
	// [LoginMFARequired] *and* there is a factor the caller can actually
	// present. An account with MFA enforced but nothing enrolled produces
	// mfa_required with no challenge — the credentials were right and there is
	// no way forward, which is the dead end M1-008 exists to remove.
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
		return LoginResult{}, apierr.BadCredentials("user " + user.ID + " has no local password")
	case !ok:
		return LoginResult{}, apierr.BadCredentials("password mismatch for user " + user.ID)
	case user.Status != identity.StatusActive:
		return LoginResult{}, apierr.BadCredentials(
			"user " + user.ID + " is " + string(user.Status))
	}

	if needsRehash {
		s.upgradeHash(ctx, user, in.Password)
	}

	enrolment, enrolled, err := s.confirmedTOTP(ctx, user.ID)
	if err != nil {
		return LoginResult{}, err
	}
	switch {
	case enrolled:
		// A confirmed factor gates the sign-in whether or not an administrator
		// requires one: somebody who set up an authenticator meant it to be
		// asked for. The password was right and no session exists yet — what
		// the caller gets is a challenge, which authorizes nothing but the
		// verification endpoint.
		issued, err := s.challenges.Open(ctx, user.ID)
		if err != nil {
			return LoginResult{}, err
		}
		s.log.InfoContext(ctx, "login requires a second factor",
			slog.String("user_id", user.ID),
			slog.String("challenge_id", issued.Challenge.ID),
			slog.Time("enrolled_at", enrolment.ConfirmedAt))
		return LoginResult{Status: LoginMFARequired, Challenge: issued}, nil

	case user.MFAEnforced:
		// Fail closed. An administrator requires a second factor of this person
		// and they have not enrolled one, so there is nothing to issue a session
		// on the strength of and nothing to challenge them with either. M1-008
		// turns this into an enrolment the caller is walked through; until then
		// the credentials were right and that is all they are told.
		s.log.WarnContext(ctx, "login refused: MFA is enforced but nothing is enrolled",
			slog.String("user_id", user.ID))
		return LoginResult{Status: LoginMFARequired}, nil
	}

	// A fresh session, not a reused one: whatever cookie the caller arrived
	// with, the session they leave with is one this exchange created. That is
	// rotation on sign-in, and it is what makes a session fixed by an attacker
	// before login worthless afterwards. Any other session the person has stays
	// live — signing in on a second machine does not sign out the first.
	issued, err := s.sessions.Issue(ctx, user.ID, in.Request, false)
	if err != nil {
		return LoginResult{}, err
	}
	if err := s.users.SetLastLoginAt(ctx, user.ID, issued.Session.CreatedAt); err != nil {
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

	subject := subjectOf(user, found)
	// Set here, where the cookie was actually resolved, rather than in
	// subjectOf: this is the only function that knows a *cookie* is what proved
	// it, and M1-011's equivalent will say MethodServiceToken in its own.
	subject.Method = MethodCookie
	return subject, nil
}

// Logout revokes the caller's session. It is idempotent: a session that has
// already ended is not an error, because the caller's intent is satisfied.
func (s *Service) Logout(ctx context.Context, subject Subject) error {
	if subject.SessionID == "" {
		return nil
	}
	if err := s.sessions.Revoke(ctx, subject.SessionID); err != nil {
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
	return Profile{
		User:                   user,
		Memberships:            memberships,
		MFAEnrolled:            enrolled,
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
		MFAEnforced:  user.MFAEnforced,
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
