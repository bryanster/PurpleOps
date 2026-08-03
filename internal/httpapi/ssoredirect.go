package httpapi

import (
	"net/http"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

// ssoRedirect is the 302 every single sign-on path answers with, and the one
// response in this package that is not one of the generated types.
//
// It exists because a completed sign-in sets more than one cookie — the session,
// the CSRF companion, and the pending state being cleared — and the generated
// response carries a single Set-Cookie string, which http.Header.Set would use
// to overwrite the others. The specification still describes the response (302,
// Location, Set-Cookie); this only changes how those headers are written, and
// adds one the specification does not talk about: see [noStore].
//
// It arrived with OIDC (M1-009) and serves SAML too (M1-010) — the four
// operations differ only in the generated interface they satisfy.
type ssoRedirect struct {
	location string
	cookies  []*http.Cookie
}

func redirectTo(location string, cookies []*http.Cookie) ssoRedirect {
	return ssoRedirect{location: location, cookies: cookies}
}

// It satisfies all four operations' response interfaces, which is why one type
// serves both starts and both callbacks.
var (
	_ gen.StartOidcSignInResponseObject    = ssoRedirect{}
	_ gen.CompleteOidcSignInResponseObject = ssoRedirect{}
	_ gen.StartSamlSignInResponseObject    = ssoRedirect{}
	_ gen.CompleteSamlSignInResponseObject = ssoRedirect{}
)

func (r ssoRedirect) VisitStartOidcSignInResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r ssoRedirect) VisitCompleteOidcSignInResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r ssoRedirect) VisitStartSamlSignInResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r ssoRedirect) VisitCompleteSamlSignInResponse(w http.ResponseWriter) error {
	return r.write(w)
}

func (r ssoRedirect) write(w http.ResponseWriter) error {
	for _, cookie := range r.cookies {
		// http.SetCookie adds a header rather than replacing one, which is the
		// whole reason this type exists.
		http.SetCookie(w, cookie)
	}
	w.Header().Set("Cache-Control", noStore)
	w.Header().Set("Location", r.location)
	w.WriteHeader(http.StatusFound)
	return nil
}

// noStore keeps these redirects out of every cache between here and the
// browser. The two GETs are the only cacheable *method* in this application that
// carries a Set-Cookie, and a shared cache that stored one would hand somebody
// else's session, or somebody else's pending sign-in, to the next person through
// it. The assertion consumer is a POST and so is not cacheable anyway; it says
// so as well, because a response that sets a session cookie should not depend on
// a cache getting that right.
const noStore = "no-store"

// unavailable is what a caller is told when a configured identity provider
// cannot be reached: the same 404 as one that was never configured.
//
// The two really are the same thing to whoever is trying to sign in — single
// sign-on is not available here — and the difference between them is an
// operational fact about somebody else's server, which the log records and a
// response has no business carrying.
func unavailable(cause error) error {
	return apierr.NotFound("reachable single sign-on provider", cause.Error())
}

// value reads an optional query parameter, which the generated types hand over
// as a pointer.
func value(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
