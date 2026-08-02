package authn

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/bryanster/purpleops/internal/authn/password"
	"github.com/bryanster/purpleops/internal/authn/session"
	"github.com/bryanster/purpleops/internal/httpapi/apierr"
	"github.com/bryanster/purpleops/internal/store/identity"
)

// Users and Memberships are the parts of the identity store this package needs.
// [*identity.Users] and [*identity.Memberships] satisfy them.
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
)

// Service is the local login path: signing in, signing out, reporting who the
// caller is, and changing one's own password.
//
// It holds the rules that decide those things, so that the HTTP layer above it
// does no more than translate. SSO (M1-009, M1-010) and service tokens (M1-011)
// add their own ways of arriving at a [Subject]; the session, the cookie and the
// rotation rules stay here and are shared.
type Service struct {
	users       Users
	memberships Memberships
	sessions    *session.Manager
	log         *slog.Logger
}

// NewService returns a Service over the given repositories and session manager.
// A nil logger means slog.Default().
func NewService(users Users, memberships Memberships, sessions *session.Manager, log *slog.Logger) (*Service, error) {
	switch {
	case users == nil:
		return nil, errors.New("authn: no user repository")
	case memberships == nil:
		return nil, errors.New("authn: no membership repository")
	case sessions == nil:
		return nil, errors.New("authn: no session manager")
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{users: users, memberships: memberships, sessions: sessions, log: log}, nil
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

	if user.MFAEnforced {
		// Fail closed. An administrator has required a second factor of this
		// person and this server cannot yet check one, so there is nothing to
		// issue a session on the strength of. M1-006 through M1-008 turn this
		// into a challenge the caller can answer; until then the credentials
		// were right and that is all the caller is told.
		s.log.InfoContext(ctx, "login requires a second factor",
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
	return subjectOf(user, found), nil
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

	// MFASatisfied is a fact about the session the request arrived on, not about
	// the user, which is why it is not read off User.
	MFASatisfied bool
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
	return Profile{User: user, Memberships: memberships, MFASatisfied: subject.MFASatisfied}, nil
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
