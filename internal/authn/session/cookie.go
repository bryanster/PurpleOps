package session

import (
	"net/http"
	"time"
)

// CookieName is the cookie the session token travels in. It is also declared as
// the `cookieSession` security scheme in api/openapi.yaml; the two must agree,
// and TestTheCookieNameMatchesTheSpecification checks that they do.
//
// The "bl_" prefix is this application's, so that a deployment sharing a
// hostname with something else does not have two cookies called "session".
const CookieName = "bl_session"

// Cookie returns the Set-Cookie for a live session.
//
// The attributes are the whole point of this function, so they are stated in one
// place and never assembled by a caller:
//
//   - HttpOnly, so that script cannot read the token. This is what keeps a
//     cross-site scripting bug from being an immediate session theft.
//   - Secure, unless BLACKLIGHT_ENV=development. A browser will not send a Secure
//     cookie over plain http, which is why config rejects a production
//     deployment on a non-loopback http base URL.
//   - SameSite=Strict, so the browser does not attach it to a request another
//     site caused. It is most of a CSRF defence on its own; csrf.go adds the
//     double-submit token for the rest (M1-005).
//   - Path=/ and no Domain. No Domain means the cookie is scoped to exactly this
//     host — setting one would widen it to every subdomain, which on a shared
//     domain hands the session to whoever runs the neighbours.
//
// Expires matches the session's absolute expiry, so a browser drops it at the
// same moment the server stops honouring it. The server never trusts that: an
// expired session is refused by [Manager.Resolve] whatever the browser did with
// its copy.
func (m *Manager) Cookie(token Token, expires time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    token.Reveal(),
		Path:     "/",
		Expires:  expires.UTC(),
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
	}
}

// ClearCookie returns the Set-Cookie that removes the session cookie: the same
// attributes, an empty value and an expiry in the past.
//
// The attributes have to match the ones the cookie was set with, or the browser
// treats it as a different cookie and keeps the original. That is the whole
// reason this is not written out at the call site.
func (m *Manager) ClearCookie() *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteStrictMode,
	}
}

// FromRequest returns the token in the request's session cookie, or the empty
// token when there is no such cookie. A malformed Cookie header is the same
// answer as no cookie: there is nothing the caller can do differently.
func FromRequest(r *http.Request) Token {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return Token(cookie.Value)
}
