package session

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Store is the part of the identity store this package needs.
// [*identity.Sessions] satisfies it.
//
// It is declared here, in the package that consumes it, so that this package's
// dependency is the six methods it calls rather than everything a database
// happens to offer — and so that a test can substitute one that fails on demand.
type Store interface {
	Create(ctx context.Context, in identity.NewSession, after ...identity.After) (identity.Session, error)
	ByTokenHash(ctx context.Context, hash string) (identity.Session, error)
	Rotate(ctx context.Context, id, tokenHash string, at time.Time) (identity.Session, error)
	SetLastSeenAt(ctx context.Context, id string, at time.Time) error
	SetMFASatisfied(ctx context.Context, id string) error
	Revoke(ctx context.Context, id string, at time.Time, after ...identity.After) error
	RevokeOthersForUser(ctx context.Context, userID, keepID string, at time.Time) (int64, error)
}

// ErrNoSession reports that a request carries no usable session: no cookie, a
// token that resolves to nothing, or a session that has expired, timed out or
// been revoked.
//
// The four are one error on purpose. Nothing above this package acts
// differently on the difference — the answer is "sign in" in every case — and
// the specific reason travels in the wrapped text for the log.
var ErrNoSession = errors.New("session: no usable session")

// touchInterval is how stale last_seen_at is allowed to get before a request
// writes it back.
//
// Recording every single request would put a database write in front of every
// authenticated read, and writes are serialized (PLAN.md §1) — one lock, held by
// every request, for a column whose only consumer is a timeout measured in
// hours. A minute of slack costs an idle session at most a minute of its
// timeout and costs a busy one a write a minute.
const touchInterval = time.Minute

// Options configures a [Manager]. Build one from the process configuration with
// [OptionsFrom]; construct it by hand in a test that needs a clock or a
// two-second lifetime.
type Options struct {
	// Secret keys the hash a token is stored under. Required.
	Secret []byte

	// Lifetime is the absolute expiry: how long a session may live at most,
	// counted from when it was issued. Nothing extends it.
	Lifetime time.Duration

	// IdleTimeout is how long a session may go unused before it ends. It must
	// not exceed Lifetime, which would leave it with nothing to do.
	IdleTimeout time.Duration

	// Secure sets the Secure attribute on the cookie. False only for a
	// development deployment on plain http — see [Manager.Cookie].
	Secure bool

	// Now reads the clock. Nil means time.Now; a test supplies its own so that
	// expiry and idleness can be reached without sleeping.
	Now func() time.Time

	// Activity records session.login / session.logout in the same transaction
	// as the session change (M1-015). Nil skips the durable row; tests that do
	// not care about the feed leave it unset.
	Activity *events.Log
}

// OptionsFrom derives the session policy from the process configuration.
//
// The one security decision it makes is Secure: on for every deployment posture
// except development. That relaxation exists so a developer can sign in over
// plain http on a laptop, and config refuses a production deployment whose base
// URL would need it (internal/config/validate.go).
func OptionsFrom(cfg config.Config) Options {
	return Options{
		Secret:      cfg.Session.Secret.Reveal(),
		Lifetime:    cfg.Session.Lifetime,
		IdleTimeout: cfg.Session.IdleTimeout,
		Secure:      !cfg.Env.IsDevelopment(),
	}
}

// Manager issues, resolves, rotates and revokes sessions. Construct it with
// [New]; the zero value has no secret and would hash every token to the same
// thing.
type Manager struct {
	store    Store
	secret   []byte
	activity *events.Log

	lifetime    time.Duration
	idleTimeout time.Duration
	secure      bool

	now func() time.Time
}

// New returns a Manager over store, or an error describing an option that
// cannot produce a usable session.
//
// The checks are startup checks: a deployment configured with no secret or a
// zero lifetime must not get as far as issuing a session nobody can use.
func New(store Store, opts Options) (*Manager, error) {
	switch {
	case store == nil:
		return nil, errors.New("session: no store; there is nowhere to keep a session")
	case len(opts.Secret) == 0:
		return nil, errors.New("session: no secret; every token would hash to the same value")
	case opts.Lifetime <= 0:
		return nil, errors.New("session: the lifetime must be positive")
	case opts.IdleTimeout <= 0:
		return nil, errors.New("session: the idle timeout must be positive")
	case opts.IdleTimeout > opts.Lifetime:
		return nil, fmt.Errorf("session: the idle timeout (%s) is longer than the lifetime (%s), "+
			"so it can never end a session", opts.IdleTimeout, opts.Lifetime)
	}

	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Manager{
		store:       store,
		secret:      opts.Secret,
		activity:    opts.Activity,
		lifetime:    opts.Lifetime,
		idleTimeout: opts.IdleTimeout,
		secure:      opts.Secure,
		now:         now,
	}, nil
}

// Issued is a new or newly rotated session: the row, and the token that reaches
// the browser. The token is returned exactly once, here — it is never stored and
// cannot be recovered afterwards.
type Issued struct {
	Session identity.Session
	Token   Token
}

// Request is what a session records about where it was created: the client
// address and the user agent, both as the request presented them.
type Request struct {
	IP        string
	UserAgent string
}

// Issue creates a session for a user and returns it with its token.
//
// mfaSatisfied says whether a second factor was presented in the exchange that
// led here. Local password login passes false; M1-006 through M1-008 are what
// ever pass true.
func (m *Manager) Issue(ctx context.Context, userID string, req Request, mfaSatisfied bool) (Issued, error) {
	token, err := newToken()
	if err != nil {
		return Issued{}, err
	}

	created, err := m.store.Create(ctx, identity.NewSession{
		UserID:       userID,
		TokenHash:    m.hash(token),
		ExpiresAt:    m.now().Add(m.lifetime),
		IP:           req.IP,
		UserAgent:    req.UserAgent,
		MFASatisfied: mfaSatisfied,
	}, m.loginAfter(userID, req, mfaSatisfied))
	if err != nil {
		return Issued{}, err
	}
	return Issued{Session: created, Token: token}, nil
}

// loginAfter records session.login on the Create transaction. The session id
// is read from the context the store sets before running the hook.
func (m *Manager) loginAfter(userID string, req Request, mfaSatisfied bool) identity.After {
	if m.activity == nil {
		return nil
	}
	return func(ctx context.Context, tx *sql.Tx) error {
		return m.activity.Record(ctx, tx, events.Entry{
			ActorID:    userID,
			Verb:       events.VerbSessionLogin,
			ObjectType: events.ObjectSession,
			ObjectID:   identity.AfterEntityID(ctx),
			Delta: events.Delta(map[string]any{
				"ip":            req.IP,
				"user_agent":    req.UserAgent,
				"mfa_satisfied": mfaSatisfied,
			}),
		})
	}
}

// Resolve returns the session a token stands for, and records that it was used.
//
// Anything that means "not signed in" — an empty or malformed token, a token
// nobody holds, a revoked, expired or idle-timed-out session — is [ErrNoSession]
// with the specific reason wrapped for the log. Any other error is the database
// failing, which is not the caller's fault and must not be reported to them as a
// failure to authenticate.
func (m *Manager) Resolve(ctx context.Context, token Token) (identity.Session, error) {
	if len(token) != tokenLength {
		// Covers the empty token as well. No token this server issued has any
		// other length, so there is nothing to look up — and refusing here keeps
		// a request that sends a megabyte of cookie from costing a query.
		return identity.Session{}, fmt.Errorf("%w: the cookie is absent or not a token", ErrNoSession)
	}

	found, err := m.store.ByTokenHash(ctx, m.hash(token))
	if errors.Is(err, apierr.ErrNotFound) {
		// Also the answer for a rotated session's old token: rotation replaces
		// the stored hash, so the previous one matches nothing.
		return identity.Session{}, fmt.Errorf("%w: no session for that token", ErrNoSession)
	}
	if err != nil {
		return identity.Session{}, err
	}

	if err := m.usable(found); err != nil {
		return identity.Session{}, err
	}

	now := m.now()
	if now.Sub(found.LastSeenAt) >= touchInterval {
		if err := m.store.SetLastSeenAt(ctx, found.ID, now); err != nil {
			return identity.Session{}, err
		}
		found.LastSeenAt = now.UTC()
	}
	return found, nil
}

// usable reports why a session may not be used, or nil.
//
// The three reasons are kept apart in the message and joined in the error: an
// operator reading a log wants to know which it was, and a client must not be
// able to tell.
func (m *Manager) usable(s identity.Session) error {
	now := m.now()
	switch {
	case !s.RevokedAt.IsZero():
		return fmt.Errorf("%w: session %s was revoked at %s",
			ErrNoSession, s.ID, s.RevokedAt.Format(time.RFC3339))
	case !now.Before(s.ExpiresAt):
		return fmt.Errorf("%w: session %s expired at %s",
			ErrNoSession, s.ID, s.ExpiresAt.Format(time.RFC3339))
	case now.Sub(s.LastSeenAt) >= m.idleTimeout:
		return fmt.Errorf("%w: session %s was last used at %s, more than %s ago",
			ErrNoSession, s.ID, s.LastSeenAt.Format(time.RFC3339), m.idleTimeout)
	}
	return nil
}

// Rotate issues a new token for an existing session and returns both.
//
// It is called on every privilege change: sign-in completing, MFA being
// satisfied, a password being changed, a platform role changing. The session
// keeps its identifier and its absolute expiry — rotation is not a way to stay
// signed in forever — and the token it had stops resolving to anything.
func (m *Manager) Rotate(ctx context.Context, sessionID string) (Issued, error) {
	token, err := newToken()
	if err != nil {
		return Issued{}, err
	}

	rotated, err := m.store.Rotate(ctx, sessionID, m.hash(token), m.now())
	if err != nil {
		return Issued{}, err
	}
	return Issued{Session: rotated, Token: token}, nil
}

// SatisfyMFA records that a second factor was presented for a session and
// rotates it onto a new token, returning both.
//
// Both halves, in that order, because satisfying MFA is a privilege change: the
// caller is more powerful afterwards than they were before, and PLAN.md §4 wants
// the token they hold to change whenever that is true. The flag is written
// first, so a failure between the two leaves a session that is correctly marked
// and still on its old token rather than one that is on a new token and does not
// know why.
func (m *Manager) SatisfyMFA(ctx context.Context, sessionID string) (Issued, error) {
	if err := m.store.SetMFASatisfied(ctx, sessionID); err != nil {
		return Issued{}, err
	}
	return m.Rotate(ctx, sessionID)
}

// Revoke ends one session. Revoking one that has already ended is not an error:
// the caller wanted it gone, and it is.
func (m *Manager) Revoke(ctx context.Context, sessionID string) error {
	return m.RevokeAs(ctx, sessionID, "")
}

// RevokeAs ends one session and attributes the session.logout activity row to
// actorID when it is known (a voluntary logout). An empty actorID is fine for
// paths that only know the session identifier.
func (m *Manager) RevokeAs(ctx context.Context, sessionID, actorID string) error {
	return m.store.Revoke(ctx, sessionID, m.now(), m.logoutAfter(sessionID, actorID))
}

func (m *Manager) logoutAfter(sessionID, actorID string) identity.After {
	if m.activity == nil {
		return nil
	}
	return func(ctx context.Context, tx *sql.Tx) error {
		return m.activity.Record(ctx, tx, events.Entry{
			ActorID:    actorID,
			Verb:       events.VerbSessionLogout,
			ObjectType: events.ObjectSession,
			ObjectID:   sessionID,
		})
	}
}

// RevokeOthers ends every session a user has except one, and reports how many.
// It is what a password change calls, so that the browser making the change
// keeps working and every other one is signed out.
func (m *Manager) RevokeOthers(ctx context.Context, userID, keepSessionID string) (int64, error) {
	return m.store.RevokeOthersForUser(ctx, userID, keepSessionID, m.now())
}
