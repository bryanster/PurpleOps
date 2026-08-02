package session

import (
	"crypto/hmac"
	"crypto/sha256"
	"net/http"
)

// The CSRF half of a cookie session (M1-005). It lives here rather than in the
// HTTP layer because it is derived from the session token and keyed with the
// same secret, and because "the cookie the browser gets" is one decision made
// in one file — cookie.go for the session, this file for its companion.

// CSRFCookieName is the cookie the double-submit token travels in. Unlike the
// session cookie it is deliberately *not* HttpOnly: the SPA has to read it to
// put it in the X-CSRF-Token header, which is the whole mechanism.
//
// The "pops_" prefix is this application's, for the same reason
// [CookieName] has one.
const CSRFCookieName = "pops_csrf"

// csrfDomain separates the CSRF derivation from the token hash in token.go.
//
// Without it, HMAC(secret, token) would be *both* the value stored in
// session.token_hash and the value handed to script in a readable cookie — so
// any XSS, or anyone who saw a request, would hold the key that looks a session
// up in the database. The two are different functions of the same input, and
// this prefix is what makes them different.
const csrfDomain = "purpleops/csrf\x00"

// CSRFToken returns the double-submit token that belongs to a session token, or
// "" when there is no session token to derive one from.
//
// It is derived rather than stored, which buys three things: no column and no
// migration; it rotates with the session for free, because rotation replaces
// the token it is derived from (M1-003); and the server can *recompute* what the
// header should be instead of only comparing the header with the cookie. That
// last one is the difference between this and naive double-submit — an attacker
// who can write a cookie for this host (a neighbouring subdomain, an active
// network position on a sibling http origin) can make the header and the cookie
// agree with each other, but not with a value keyed by a secret they do not
// have. It is the case docs/tickets M1-005 names and the reason the token is not
// simply a second random string.
//
// It is not a credential: a session cookie is what authenticates: this only says
// "whoever sent this request could read a response from this origin".
func (m *Manager) CSRFToken(token Token) string {
	if len(token) != tokenLength {
		return ""
	}
	mac := hmac.New(sha256.New, m.secret)
	// hash.Hash never returns an error, as its own documentation states.
	mac.Write([]byte(csrfDomain))
	mac.Write([]byte(token.Reveal()))
	return tokenEncoding.EncodeToString(mac.Sum(nil))
}

// CSRFCookie returns the Set-Cookie carrying a CSRF token.
//
// The attributes are the session cookie's with one deliberate difference:
//
//   - HttpOnly is false, and this is the entire point. Script must read it.
//     Getting this backwards on the *session* cookie is the classic version of
//     this mistake, which is why TestTheCookieFlagsAreNotSwappedOver asserts
//     both cookies rather than only this one.
//   - Secure and SameSite=Strict, exactly as the session cookie: this value
//     should not travel over plain http or be attached to a cross-site request
//     either.
//   - No Expires and no MaxAge, so a browser keeps it for as long as it is open.
//     The session cookie's expiry is a security boundary and this one's is not:
//     the value is checked against a derivation from the live session token, so a
//     stale copy cannot authorize anything — it fails, and the middleware
//     replaces it on the way out.
func (m *Manager) CSRFCookie(csrfToken string) *http.Cookie {
	return &http.Cookie{
		Name:     CSRFCookieName,
		Value:    csrfToken,
		Path:     "/",
		HttpOnly: false,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
	}
}

// ClearCSRFCookie returns the Set-Cookie that removes the CSRF cookie. It is
// sent wherever [Manager.ClearCookie] is: the pair is issued together and
// dropped together, so a signed-out browser is not left holding half of it.
func (m *Manager) ClearCSRFCookie() *http.Cookie {
	cookie := m.CSRFCookie("")
	cookie.MaxAge = -1
	return cookie
}

// CSRFFromRequest returns the token in the request's CSRF cookie, or "" when
// there is no such cookie.
func CSRFFromRequest(r *http.Request) string {
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
