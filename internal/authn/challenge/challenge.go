// Package challenge is the pending state between a correct password and a
// presented second factor (M1-006).
//
// It looks like internal/authn/session and is deliberately not it. A session
// says "this person is signed in"; a challenge says "this person's password was
// right and that is all". Nothing resolves a challenge into a caller: the only
// thing that can be done with one is present a code to the verification
// endpoint, and the only thing that produces is a session. Making it a separate
// package with a separate cookie and a separate table is what makes that true by
// construction rather than by every reader of authn.Subject remembering it.
//
// The three properties that matter, and where each is enforced:
//
//   - It expires. The row carries expires_at and [Manager.Resolve] refuses a
//     row past it, so a browser left on the code entry screen does not stay one
//     guess away from a session all afternoon.
//   - It is spent by use. [Manager.Consume] guards on consumed_at in the
//     statement, so one correct code buys exactly one session even if two
//     requests arrive together.
//   - It is superseded. Opening a challenge deletes whatever the same person had
//     pending (identity.MFAChallenges.Open), so an abandoned attempt is not still
//     answerable.
package challenge

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bryanster/purpleops/internal/config"
	"github.com/bryanster/purpleops/internal/httpapi/apierr"
	"github.com/bryanster/purpleops/internal/store/identity"
)

// CookieName is the cookie the pending token travels in. The "pops_" prefix is
// this application's, for the same reason the session cookie has one.
//
// It is a different name from the session cookie on purpose: a token that could
// arrive in `pops_session` would be a token the authentication middleware tried
// to resolve, and the answer to "is a pending token a session" has to be no in
// the plumbing and not only in the logic.
const CookieName = "pops_mfa"

// tokenBytes is 32 — 256 bits, the same as a session token. The challenge is
// short-lived, which is not a reason for it to be guessable: guessing one is
// guessing past somebody's password.
const tokenBytes = 32

// tokenEncoding is base64url without padding, safe in a cookie value unquoted.
var tokenEncoding = base64.RawURLEncoding

var tokenLength = tokenEncoding.EncodedLen(tokenBytes)

// hashDomain separates this HMAC from the session token's, which is keyed with
// the same secret. Without it a token that was somehow accepted by both lookups
// would hash to the same stored value in both tables.
const hashDomain = "purpleops/mfa-challenge\x00"

const redacted = "[redacted]"

// ErrNoChallenge reports that there is no usable pending state: no cookie, a
// token that resolves to nothing, or a challenge that has expired or been
// spent.
//
// The reasons are one error for the same reason [session.ErrNoSession]'s are:
// nothing above this package acts differently on the difference, and a client
// that could tell them apart could tell whether a password was right.
var ErrNoChallenge = errors.New("challenge: no usable MFA challenge")

// Token is a pending-state token: the value in the cookie, and a credential as
// much as a password is. Every ordinary way of rendering one produces
// [redacted]; reading it takes [Token.Reveal].
type Token string

// Reveal returns the token itself. Call it where the value is used — hashing it,
// or putting it in a Set-Cookie header — and do not store what it returns.
func (t Token) Reveal() string { return string(t) }

func (Token) String() string   { return redacted }
func (Token) GoString() string { return redacted }

// Format implements fmt.Formatter, which is what makes the redaction total: fmt
// consults it for every verb rather than only the ones Stringer covers.
func (Token) Format(f fmt.State, verb rune) {
	switch verb {
	case 'q':
		fmt.Fprintf(f, "%q", redacted)
	default:
		fmt.Fprint(f, redacted)
	}
}

// LogValue implements slog.LogValuer, so a token logged as an attribute — or as
// a field of a struct being logged — records the placeholder.
func (Token) LogValue() slog.Value { return slog.StringValue(redacted) }

// MarshalJSON and MarshalText cover the encoders. A pending token has one
// destination, the Set-Cookie header, so anything serializing one is sending it
// somewhere it does not belong.
func (Token) MarshalJSON() ([]byte, error) { return json.Marshal(redacted) }
func (Token) MarshalText() ([]byte, error) { return []byte(redacted), nil }

// Store is the part of the identity store this package needs.
// [*identity.MFAChallenges] satisfies it.
type Store interface {
	Open(ctx context.Context, in identity.NewMFAChallenge) (identity.MFAChallenge, error)
	ByTokenHash(ctx context.Context, hash string) (identity.MFAChallenge, error)
	Consume(ctx context.Context, id string, at time.Time) (bool, error)
	DeleteForUser(ctx context.Context, userID string) error
}

// Options configures a [Manager].
type Options struct {
	// Secret keys the hash a token is stored under. Required.
	Secret []byte

	// TTL is how long a challenge lasts. Required, and short: this is the gap
	// in which a password alone is nearly enough.
	TTL time.Duration

	// Secure sets the Secure attribute on the cookie, on for every deployment
	// posture except development — the same relaxation, for the same reason, as
	// the session cookie.
	Secure bool

	// Path scopes the cookie. Unlike the session cookie this is deliberately not
	// "/": the pending token is only ever presented to the MFA endpoints, so a
	// browser has no reason to attach it to anything else and every reason not
	// to. Required — an empty Path would make the browser scope it to the
	// directory of whatever URL happened to set it.
	Path string

	// Now reads the clock. Nil means time.Now.
	Now func() time.Time
}

// OptionsFrom derives the policy from the process configuration. path is the
// prefix the MFA endpoints are served under, which this package cannot know:
// the routes belong to internal/httpapi.
func OptionsFrom(cfg config.Config, path string) Options {
	return Options{
		Secret: cfg.Session.Secret.Reveal(),
		TTL:    cfg.MFA.PendingTTL,
		Secure: !cfg.Env.IsDevelopment(),
		Path:   path,
	}
}

// Manager opens, resolves and spends challenges. Construct it with [New].
type Manager struct {
	store  Store
	secret []byte
	ttl    time.Duration
	secure bool
	path   string
	now    func() time.Time
}

// New returns a Manager over store, or an error describing an option that
// cannot produce a usable challenge. The checks are startup checks: a
// deployment configured with no secret or a zero window must not get as far as
// issuing something nobody can answer.
func New(store Store, opts Options) (*Manager, error) {
	switch {
	case store == nil:
		return nil, errors.New("challenge: no store; there is nowhere to keep a challenge")
	case len(opts.Secret) == 0:
		return nil, errors.New("challenge: no secret; every token would hash to the same value")
	case opts.TTL <= 0:
		return nil, errors.New("challenge: the pending window must be positive")
	case !strings.HasPrefix(opts.Path, "/"):
		return nil, fmt.Errorf("challenge: the cookie path is %q; it must be absolute, or a "+
			"browser scopes the cookie to whatever directory happened to set it", opts.Path)
	}

	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Manager{
		store:  store,
		secret: opts.Secret,
		ttl:    opts.TTL,
		secure: opts.Secure,
		path:   opts.Path,
		now:    now,
	}, nil
}

// Issued is a new challenge: the row, and the token that reaches the browser.
// The token is returned exactly once, here.
type Issued struct {
	Challenge identity.MFAChallenge
	Token     Token
}

// Open starts a challenge for a user, superseding any they already had.
func (m *Manager) Open(ctx context.Context, userID string) (Issued, error) {
	token, err := newToken()
	if err != nil {
		return Issued{}, err
	}

	opened, err := m.store.Open(ctx, identity.NewMFAChallenge{
		UserID:    userID,
		TokenHash: m.hash(token),
		ExpiresAt: m.now().Add(m.ttl),
	})
	if err != nil {
		return Issued{}, err
	}
	return Issued{Challenge: opened, Token: token}, nil
}

// Resolve returns the challenge a token stands for, without spending it.
//
// Everything that means "there is nothing pending here" is [ErrNoChallenge]
// with the specific reason wrapped for the log. Any other error is the database
// failing, which the caller must not report as a failed verification.
func (m *Manager) Resolve(ctx context.Context, token Token) (identity.MFAChallenge, error) {
	if len(token) != tokenLength {
		// Covers the empty token. Nothing this server issued has another
		// length, so there is nothing to look up.
		return identity.MFAChallenge{}, fmt.Errorf(
			"%w: the cookie is absent or not a token", ErrNoChallenge)
	}

	found, err := m.store.ByTokenHash(ctx, m.hash(token))
	if errors.Is(err, apierr.ErrNotFound) {
		return identity.MFAChallenge{}, fmt.Errorf("%w: no challenge for that token", ErrNoChallenge)
	}
	if err != nil {
		return identity.MFAChallenge{}, err
	}

	now := m.now()
	switch {
	case !found.ConsumedAt.IsZero():
		return identity.MFAChallenge{}, fmt.Errorf("%w: challenge %s was spent at %s",
			ErrNoChallenge, found.ID, found.ConsumedAt.Format(time.RFC3339))
	case !now.Before(found.ExpiresAt):
		return identity.MFAChallenge{}, fmt.Errorf("%w: challenge %s expired at %s",
			ErrNoChallenge, found.ID, found.ExpiresAt.Format(time.RFC3339))
	}
	return found, nil
}

// Spend marks a challenge used and reports whether this call was the one that
// used it. A false means another request got there first, and the caller must
// not issue anything on the strength of it.
func (m *Manager) Spend(ctx context.Context, challengeID string) (bool, error) {
	return m.store.Consume(ctx, challengeID, m.now())
}

// Discard drops every challenge a user has. It is what a completed sign-in and
// a password change call: a pending challenge that outlived the credentials
// which opened it would be a way back in.
func (m *Manager) Discard(ctx context.Context, userID string) error {
	return m.store.DeleteForUser(ctx, userID)
}

// Cookie returns the Set-Cookie carrying a pending token.
//
// The attributes are the session cookie's, with a much shorter life: MaxAge is
// the pending window, so a browser drops it at about the moment the server
// stops honouring it. The server never trusts that — [Manager.Resolve] checks
// the row's expiry whatever the browser did with its copy.
func (m *Manager) Cookie(token Token, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    token.Reveal(),
		Path:     m.path,
		Expires:  expires.UTC(),
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
	}
}

// ClearCookie returns the Set-Cookie that removes the pending cookie: the same
// attributes, an empty value and an expiry in the past. The attributes have to
// match the ones it was set with, or a browser keeps the original — which is
// the whole reason this is not written out at the call site.
func (m *Manager) ClearCookie() *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     m.path,
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
	}
}

// FromRequest returns the token in the request's pending cookie, or the empty
// token when there is no such cookie.
func FromRequest(r *http.Request) Token {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return Token(cookie.Value)
}

// newToken mints a token from crypto/rand. A failure here is not recoverable
// and must never be papered over with a weaker source.
func newToken() (Token, error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("challenge: read %d random bytes: %w", tokenBytes, err)
	}
	return Token(tokenEncoding.EncodeToString(raw)), nil
}

// hash returns what is stored for a token: HMAC-SHA256 under the deployment's
// session secret, base64url. Keyed rather than bare for the reasons
// session.Manager.hash gives — a stolen database is not enough to look a token
// up, and rotating the secret invalidates every one of these too.
func (m *Manager) hash(token Token) string {
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(hashDomain))
	mac.Write([]byte(token.Reveal()))
	return tokenEncoding.EncodeToString(mac.Sum(nil))
}
