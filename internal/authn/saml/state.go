package saml

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// The pending state of one SAML sign-in: what [Provider.Start] mints and
// [Provider.Complete] has to be given back.
//
// It lives in a cookie rather than in a table, for the reasons
// internal/authn/oidc's equivalent does, and it does one job that the OIDC one
// also does and that SAML has no other way of doing: it ties the assertion to
// the browser that asked for it.
//
// Without it, a service-provider-initiated sign-in is only bound to *some*
// browser having started one. An attacker who signs in at the identity provider,
// captures the assertion minted for their own account, and delivers it into your
// browser signs you in as them — and then reads whatever you do next. That is
// login CSRF, it is a real attack against SAML deployments, and the request ID
// in here is what refuses it: the assertion's `InResponseTo` has to name a
// request *this* browser was handed.
//
// The identity-provider-initiated case has no such request and so cannot have
// the binding. That is inherent in the profile — see [Provider.pendingFor] and
// the `BLACKLIGHT_SAML_ALLOW_IDP_INITIATED` documentation, which says so out
// loud rather than leaving somebody to infer it.

// CookieName is where the sealed pending request travels. As with every other
// cookie this application sets, the "bl_" prefix says whose it is.
const CookieName = "bl_saml"

// pending is the sealed payload. Short JSON keys, because this is serialized
// into a cookie on every sign-in and nothing but this file ever reads them.
type pending struct {
	// RequestID is the ID of the AuthnRequest this browser was handed, and the
	// value the assertion's `InResponseTo` has to equal.
	RequestID string `json:"i"`

	ReturnTo string `json:"r,omitempty"`

	// ExpiresAt is checked by the server on the way back in. The cookie carries
	// a matching Max-Age so a browser drops it at about the same moment, but
	// that is a courtesy to the browser and never the check.
	ExpiresAt time.Time `json:"e"`
}

// seal encrypts the pending request into the value that goes in the cookie.
func (p *Provider) seal(state pending) (string, error) {
	encoded, err := json.Marshal(state)
	if err != nil {
		return "", fmt.Errorf("saml: encode the pending sign-in: %w", err)
	}
	sealed, err := p.cipher.Seal(encoded)
	if err != nil {
		return "", fmt.Errorf("saml: seal the pending sign-in: %w", err)
	}
	return sealed, nil
}

// open reverses seal and checks what only the server can check: that this
// request was issued here and has not expired.
//
// Every failure is [ErrNoPendingSignIn], and a caller cannot tell them apart —
// "no cookie", "a cookie sealed under a previous deployment key" and "expired"
// are one answer, for the reason the OIDC equivalent gives.
//
// It does *not* compare the request ID against anything: there is nothing in the
// form to compare it to, because RelayState is untrusted. The comparison is the
// library's, against the assertion's signed `InResponseTo`, which is the only
// copy of that value anybody has proved.
func (p *Provider) open(sealed string) (pending, error) {
	raw, err := p.cipher.Open(sealed)
	if err != nil {
		return pending{}, fmt.Errorf("%w: the pending-request cookie is not one this deployment "+
			"sealed: %w", ErrNoPendingSignIn, err)
	}
	var found pending
	if err := json.Unmarshal(raw, &found); err != nil {
		return pending{}, fmt.Errorf("%w: the pending-request cookie does not decode: %w",
			ErrNoPendingSignIn, err)
	}
	if found.RequestID == "" {
		return pending{}, fmt.Errorf("%w: the pending-request cookie names no request",
			ErrNoPendingSignIn)
	}
	if !p.now().Before(found.ExpiresAt) {
		return pending{}, fmt.Errorf("%w: the sign-in was started at %s and has expired",
			ErrNoPendingSignIn, found.ExpiresAt.Add(-p.stateTTL).Format(time.RFC3339))
	}
	return found, nil
}

// Cookie returns the Set-Cookie carrying a sealed pending request.
//
// `SameSite=None; Secure`, and this is the only cookie in the application that
// is neither Strict nor Lax. It is not a relaxation anybody chose: the assertion
// arrives as a cross-site **POST** from the identity provider, and a browser
// sends neither a Strict nor a Lax cookie on a cross-site POST — Lax covers
// top-level *GET* navigations only, which is what the OIDC callback is and this
// is not. Anything stricter here means the cookie never comes back and no
// sign-in ever completes.
//
// `Secure` is therefore unconditional, including in development, because a
// browser refuses `SameSite=None` without it. That works on http://localhost,
// which browsers treat as a secure context; it does not work on a development
// deployment served over plain http from another host, and that is the honest
// consequence of the paragraph above rather than something to paper over.
//
// It stays HttpOnly and scoped to the SAML endpoints. Nothing else has any
// reason to be sent it, and it is worthless to script either way.
func (p *Provider) Cookie(sealed string) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    sealed,
		Path:     p.cookiePath,
		Expires:  p.now().Add(p.stateTTL).UTC(),
		MaxAge:   int(p.stateTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	}
}

// ClearCookie returns the Set-Cookie that removes the pending-request cookie.
//
// It is the second half of what makes a sign-in single-use — the first is the
// replay cache, which is what actually enforces it. This one takes the request
// out of the browser so that the same assertion arriving again finds nothing to
// answer, and it runs whatever the outcome was: one request, one assertion.
//
// The attributes have to be the ones it was set with or the browser keeps its
// copy, which is why this is not written out at the call site.
func (p *Provider) ClearCookie() *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     p.cookiePath,
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
	}
}

// SealedFrom returns the sealed pending request in the request's cookie, or ""
// when there is none. An empty value is not an error here: it is what an
// identity-provider-initiated sign-in looks like.
func SealedFrom(r *http.Request) string {
	cookie, err := r.Cookie(CookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}
