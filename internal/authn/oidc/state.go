package oidc

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bryanster/blacklight/internal/authn/returnto"
)

// The pending state of one sign-in: what [Provider.Start] mints and
// [Provider.Complete] has to be given back.
//
// It lives in a cookie rather than in a table. The three things it holds are all
// short-lived, all belong to one browser, and none of them is of the slightest
// interest to another request — so a table would be a table that grows,
// needs sweeping, and can be queried by a request that should not be able to see
// anybody else's row. Sealed with AEAD under a key derived for this purpose
// alone (internal/authn/secrets), it cannot be read or forged by the browser
// holding it, and a value from any other context will not open here.
//
// Its three jobs, and the attack each refuses:
//
//   - `state` ties the callback to a sign-in this deployment started. A callback
//     with no cookie, a cookie that will not open, or a state that does not match
//     is refused — which is login CSRF, an attacker completing *their* sign-in in
//     your browser so that what you do next is done in their account.
//   - `nonce` ties the ID token to this same exchange. It is checked against the
//     token's claim, so an ID token obtained elsewhere and replayed here is
//     refused.
//   - The PKCE verifier ties the authorization code to this browser. An
//     intercepted code — from a log, a referrer header, a shared machine — is
//     worthless without it.

// CookieName is where the sealed state travels. As with every other cookie this
// application sets, the "bl_" prefix says whose it is.
const CookieName = "bl_oidc"

// stateBytes is 32: 256 bits, the same as a session token. `state` is not a
// credential, but a guessable one is a login-CSRF token an attacker can predict.
const stateBytes = 32

var stateEncoding = base64.RawURLEncoding

// pending is the sealed payload. The JSON keys are short because this is
// serialized into a cookie on every sign-in, and long ones would buy nothing:
// nothing but this file ever reads them.
type pending struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
	ReturnTo string `json:"r,omitempty"`

	// ExpiresAt is checked by the server on the way back in. The cookie carries
	// a matching Max-Age so a browser drops it at about the same moment, but that
	// is a courtesy to the browser and never the check.
	ExpiresAt time.Time `json:"e"`
}

// seal encrypts the pending state into the value that goes in the cookie.
func (p *Provider) seal(state pending) (string, error) {
	encoded, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("oidc: encode the pending sign-in: %w", err)
	}
	sealed, err := p.cipher.Seal(encoded)
	if err != nil {
		return "", fmt.Errorf("oidc: seal the pending sign-in: %w", err)
	}
	return sealed, nil
}

// open reverses seal and checks what only the server can check: that this state
// was issued here, has not expired, and is the one the callback names.
//
// Every failure is [ErrNoPendingSignIn]. A caller cannot act differently on the
// difference between "no cookie", "a cookie from a previous deployment key",
// "expired" and "the state does not match" — and being able to tell them apart
// is how an attacker finds out which half of a forged callback was wrong.
func (p *Provider) open(sealed, state string) (pending, error) {
	if sealed == "" {
		return pending{}, fmt.Errorf("%w: the request carries no state cookie", ErrNoPendingSignIn)
	}
	if state == "" {
		return pending{}, fmt.Errorf("%w: the callback carries no state parameter", ErrNoPendingSignIn)
	}

	raw, err := p.cipher.Open(sealed)
	if err != nil {
		return pending{}, fmt.Errorf("%w: the state cookie is not one this deployment sealed: %w",
			ErrNoPendingSignIn, err)
	}
	var found pending
	if err := json.Unmarshal(raw, &found); err != nil {
		return pending{}, fmt.Errorf("%w: the state cookie does not decode: %w", ErrNoPendingSignIn, err)
	}

	// Constant time, like every other comparison of a value somebody could be
	// probing for.
	if subtle.ConstantTimeCompare([]byte(found.State), []byte(state)) != 1 {
		return pending{}, fmt.Errorf(
			"%w: the state in the callback is not the state this browser was issued", ErrNoPendingSignIn)
	}
	if !p.now().Before(found.ExpiresAt) {
		return pending{}, fmt.Errorf("%w: the sign-in was started at %s and has expired",
			ErrNoPendingSignIn, found.ExpiresAt.Add(-p.stateTTL).Format(time.RFC3339))
	}
	return found, nil
}

// Cookie returns the Set-Cookie carrying a sealed pending sign-in.
//
// SameSite=Lax, and this is the one cookie in the application that is not
// Strict. The callback is a top-level navigation *from the provider*, and a
// browser does not attach a Strict cookie to a cross-site navigation — so Strict
// here would mean the cookie is never sent back and no sign-in ever completes.
// Lax attaches it to exactly this case: a top-level GET the person themselves
// was navigated into. It is HttpOnly, scoped to the two OIDC endpoints, and
// worthless to script either way.
func (p *Provider) Cookie(sealed string) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // Secure mirrors p.secure (http dev hosts); SameSite=Lax is the browser-facing design
		Name:     CookieName,
		Value:    sealed,
		Path:     p.cookiePath,
		Expires:  p.now().Add(p.stateTTL).UTC(),
		MaxAge:   int(p.stateTTL.Seconds()),
		HttpOnly: true,
		Secure:   p.secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// ClearCookie returns the Set-Cookie that removes the state cookie. It is what
// makes a state single-use: the callback that spends one takes it out of the
// browser, so the same state arriving again finds nothing to match against.
//
// The attributes have to be the ones it was set with or the browser keeps its
// copy, which is why this is not written out at the call site.
func (p *Provider) ClearCookie() *http.Cookie {
	return &http.Cookie{ //nolint:gosec // mirrors Cookie()'s attributes; a clear has to match the set
		Name:     CookieName,
		Value:    "",
		Path:     p.cookiePath,
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   p.secure,
		SameSite: http.SameSiteLaxMode,
	}
}

// SealedFrom returns the sealed state in the request's cookie, or "" when there
// is none.
func SealedFrom(r *http.Request) string {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

// ErrUnsafeReturnTo reports a `return_to` that is not a path within this
// application.
var ErrUnsafeReturnTo = returnto.ErrUnsafe

// SafeReturnTo returns the path a completed sign-in should land on, or
// [ErrUnsafeReturnTo].
//
// The rule itself moved to internal/authn/returnto when SAML (M1-010) became
// its second caller — an open-redirect check that exists twice is one that gets
// fixed once. This is the name the OIDC path calls it by.
func SafeReturnTo(raw string) (string, error) { return returnto.Safe(raw) }

// newState mints an unguessable `state`.
func newState() (string, error) {
	raw := make([]byte, stateBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("oidc: read %d random bytes: %w", stateBytes, err)
	}
	return stateEncoding.EncodeToString(raw), nil
}
