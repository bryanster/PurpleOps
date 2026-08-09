package httpapi

import (
	"context"
	"crypto/subtle"
	"log/slog"
	"net/http"
	"slices"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authn/saml"
	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

// CSRFHeader is where a state-changing request echoes the value of the
// bl_csrf cookie. It is declared as a parameter on the operations that need
// it in api/openapi.yaml, and TestTheCSRFHeaderMatchesTheSpecification checks
// that the two agree.
//
// It is exported because the frontend's copy of the name (web/src/api/client.ts)
// and this one have to match, and a constant something can be tested against is
// how that stays true.
const CSRFHeader = "X-CSRF-Token"

// csrfSafeMethods are the ones that must not change anything, and so have
// nothing to protect. A handler that mutates on GET is a bug in that handler —
// this list is not the place to compensate for one.
var csrfSafeMethods = []string{http.MethodGet, http.MethodHead, http.MethodOptions}

// csrfExemptRoutes are the state-changing routes that are deliberately not
// behind the double-submit check, keyed by "METHOD path" and valued with the
// reason. There are no patterns and no prefixes: an exemption is one route,
// written out, with an argument next to it.
//
// TestEveryMutatingRouteIsCoveredByCSRF walks the router and fails when a
// mutating route is neither enforced nor listed here — so a new endpoint cannot
// join this list without somebody typing its reason.
var csrfExemptRoutes = map[string]string{
	// There is no session to protect yet: the caller is presenting a password,
	// not spending authority a browser attached for them. A request that
	// happens to carry a stale session cookie is exempt too, because the check
	// would then depend on whether the browser had one — and a login form that
	// works or 403s depending on that is worse than the login CSRF it would
	// prevent, which costs the attacker their own credentials to mount.
	"POST " + BasePath + "/auth/login": "no session exists yet; the credential in the body is the proof",

	// The second half of the same sign-in, exempt for the same reason and with
	// the same caveat: the caller normally has no session cookie, so the check
	// would not apply anyway — and for the browser that happens to be holding a
	// stale one it would turn "finish signing in" into a 403 that depends on
	// what was in the cookie jar. The credential is the pending cookie plus a
	// code from a device, which an attacker forging a cross-site request has
	// neither of.
	"POST " + BasePath + mfaPathPrefix + "/totp/verify": "no session exists yet; the pending cookie and the code are the proof",

	// The same half of the same sign-in, reached with a printed code instead of
	// an authenticator (M1-007). Exempt for exactly the reasons above and for no
	// additional one: it is the same exchange, with the same pending cookie, and
	// a caller who has not started a sign-in has nothing to present.
	"POST " + BasePath + mfaPathPrefix + "/recovery/verify": "no session exists yet; the pending cookie and the code are the proof",

	// The SAML assertion consumer (M1-010). It is a cross-site POST from the
	// identity provider *by design* — that is what the HTTP-POST binding is —
	// so the double-submit check could never pass here: there is no way for the
	// identity provider's form to carry a header, and no session to protect
	// anyway.
	//
	// What stands in its place is stronger than a CSRF token rather than weaker.
	// The body is an XML document signed by the identity provider, audience- and
	// recipient-restricted to this deployment, valid for a few minutes, refused
	// if it has been seen before — and, for a sign-in that started here, bound
	// to *this browser* by a request ID in a sealed cookie that the assertion
	// has to name. An attacker who could forge a cross-site POST to this route
	// still has to produce a signed assertion, which is the whole point.
	//
	// The OIDC callback is not in this list because it is a GET, and safe
	// methods are exempt above.
	"POST " + BasePath + samlACSPath: "the signed, replay-checked assertion is the proof, and the identity provider's cross-site POST could not carry a header",

	// Share claim and password routes (M6-012). Public-ish: authorization is
	// by share grant, not session. The share token in the URL is the proof of
	// the right to claim. Guest registration has no session at all — the
	// caller is creating their first credential.
	"POST " + BasePath + "/report-views/{token}/claim":    "no session required; the share token in the URL authorizes the claim",
	"POST " + BasePath + "/report-views/{token}/password": "no session required; the share token in the URL authorizes the password check",
	"POST " + BasePath + "/auth/guest-register":           "no session exists yet; the caller is creating their first credential",
}

// samlACSPath is the assertion consumer's path, from the package that owns it,
// so the exemption above cannot drift from the route it exempts.
const samlACSPath = saml.ACSPath

// requireCSRF is the double-submit check, and step 10 of the chain described in
// internal/httpapi/server.go.
//
// PLAN.md §4 records that v1 had CSRF protection, removed it, and kept the
// header plumbing — which is the worst of the three states, because the code
// still looks protected. So this middleware is on the router rather than in the
// handlers, it refuses before the handler is entered, and the route-enumeration
// test is what stops it decaying the same way.
//
// What it enforces, for a state-changing request that authenticated by cookie:
// the X-CSRF-Token header must equal the bl_csrf cookie (the double-submit),
// *and* both must equal the value derived from this session's token
// ([session.Manager.CSRFToken]). The second comparison is what a cookie an
// attacker planted cannot satisfy.
//
// What it exempts, and why the exemption cannot be claimed by a caller:
//
//   - Safe methods, which change nothing.
//   - Requests that did not authenticate by cookie. That is read from
//     [authn.Subject.Method], which the authentication step sets from what it
//     actually resolved — so sending an Authorization header does not buy an
//     exemption, it buys whatever that header authenticates to, and an invalid
//     one authenticates to nothing at all.
//   - The routes in csrfExemptRoutes, one at a time, with reasons.
//
// It also keeps the browser's CSRF cookie correct on the way out; see
// [csrfWriter].
func requireCSRF(sessions *session.Manager, responder *apierr.Responder,
	log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subject, _ := authn.SubjectFrom(r.Context())

			// Derived from the cookie on this request rather than looked up, so
			// it costs an HMAC and no query. Empty for a request that did not
			// arrive on a session cookie, which is also the value that can never
			// match anything below.
			var expected string
			if subject.Method == authz.MethodCookie {
				expected = sessions.CSRFToken(session.FromRequest(r))
				r = r.WithContext(withCSRFToken(r.Context(), expected))
			}

			writer := &csrfWriter{
				ResponseWriter: w,
				sessions:       sessions,
				expected:       expected,
				presented:      session.CSRFFromRequest(r),
			}

			if csrfExempt(r, subject) {
				next.ServeHTTP(writer, r)
				return
			}

			header := r.Header.Get(CSRFHeader)
			if !csrfMatches(header, writer.presented, expected) {
				// Warn, not debug: in a working deployment this does not happen.
				// Either somebody is attempting a cross-site write, or a client
				// has stopped sending the header — and both are things an
				// operator should find in the log rather than in a support
				// ticket. Nothing here records a token value, matching or not.
				log.WarnContext(r.Context(), "refused a request with no valid CSRF token",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("user_id", subject.UserID),
					slog.Bool("header_present", header != ""),
					slog.Bool("cookie_present", writer.presented != ""))
				responder.Write(writer, r, apierr.Forbidden("a state-changing request with no valid CSRF token"))
				return
			}

			next.ServeHTTP(writer, r)
		})
	}
}

// csrfExempt reports whether this request is one of the three documented
// exemptions above.
func csrfExempt(r *http.Request, subject authn.Subject) bool {
	if slices.Contains(csrfSafeMethods, r.Method) {
		return true
	}
	if subject.Method != authz.MethodCookie {
		return true
	}
	_, listed := csrfExemptRoutes[r.Method+" "+routePath(r)]
	return listed
}

// csrfMatches is the comparison, in constant time and in both directions.
//
// header must equal the cookie — the double-submit, which is what an attacker
// who can make a browser send a request but cannot read this origin's cookies
// fails — and both must equal the value derived from the live session token,
// which is what an attacker who *can* write a cookie for this host still fails.
// Neither check is redundant: they refuse different attackers.
func csrfMatches(header, cookie, expected string) bool {
	if header == "" || cookie == "" || expected == "" {
		// subtle.ConstantTimeCompare would answer this as well; saying it here
		// means "absent" and "wrong" are the same answer by construction rather
		// than by the properties of a comparison function.
		return false
	}
	return constantTimeEqual(header, cookie) && constantTimeEqual(header, expected)
}

func constantTimeEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// csrfWriter keeps the browser's CSRF cookie matching its session cookie.
//
// It is a response wrapper rather than something the handlers do because a
// handler that forgets leaves a browser unable to make any state-changing
// request, and there will be more handlers that issue sessions — MFA completion
// (M1-006), the SSO callbacks (M1-009, M1-010). Doing it here means every one of
// them is right without knowing this exists. It also settles a mechanical
// problem: the generated response types carry a single Set-Cookie string, and
// two cookies cannot be folded into one header.
//
// At the moment the status is written, exactly one of these happens:
//
//   - The response sets or clears the session cookie: the CSRF cookie is set or
//     cleared to match, derived from the very token going into the response, so
//     rotation (M1-003) carries its CSRF token with it.
//   - Otherwise, if this request authenticated by cookie and its CSRF cookie was
//     absent or stale, the correct one is set. That is what recovers a browser
//     that lost the cookie, including the 403 above: the refusal itself carries
//     the value that makes the retry work.
type csrfWriter struct {
	http.ResponseWriter

	sessions *session.Manager
	// expected is the CSRF token this request's session should have, or "" when
	// the request did not authenticate by cookie.
	expected string
	// presented is the CSRF cookie the request arrived with.
	presented string

	done bool
}

func (w *csrfWriter) WriteHeader(status int) {
	w.reconcile()
	w.ResponseWriter.WriteHeader(status)
}

func (w *csrfWriter) Write(b []byte) (int, error) {
	// A handler that writes without calling WriteHeader has still committed to
	// a 200, and the headers go out with it.
	w.reconcile()
	return w.ResponseWriter.Write(b)
}

// Flush and Unwrap keep the wrapper transparent: http.ResponseController finds
// everything else the underlying writer can do through Unwrap, and flushing
// commits the header, so it has to reconcile first.
func (w *csrfWriter) Flush() {
	w.reconcile()
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *csrfWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *csrfWriter) reconcile() {
	if w.done {
		return
	}
	w.done = true

	if cookie, ok := w.shadowSessionCookie(); ok {
		http.SetCookie(w.ResponseWriter, cookie)
		return
	}
	if w.expected != "" && w.presented != w.expected {
		http.SetCookie(w.ResponseWriter, w.sessions.CSRFCookie(w.expected))
	}
}

// shadowSessionCookie returns the CSRF cookie belonging to the session cookie
// this response is setting, and false when it is setting none.
//
// Reading the outgoing header is what makes this work for any handler that
// issues a session without that handler taking part. A Set-Cookie this server
// cannot parse is not one it wrote, so it is skipped rather than guessed at.
func (w *csrfWriter) shadowSessionCookie() (*http.Cookie, bool) {
	// Cloned because the loop body adds to the same header.
	for _, raw := range slices.Clone(w.Header().Values("Set-Cookie")) {
		set, err := http.ParseSetCookie(raw)
		if err != nil || set.Name != session.CookieName {
			continue
		}
		if set.Value == "" || set.MaxAge < 0 {
			return w.sessions.ClearCSRFCookie(), true
		}
		return w.sessions.CSRFCookie(w.sessions.CSRFToken(session.Token(set.Value))), true
	}
	return nil, false
}

// csrfKey is this file's context key, distinct from every other one in the
// package for the same reason theirs are.
type csrfKey struct{}

func withCSRFToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfKey{}, token)
}

// csrfTokenFrom returns the CSRF token belonging to this request's session, and
// "" for a request that did not authenticate by cookie.
//
// GET /auth/me is the only reader: the SPA takes the token from the cookie, and
// the response body is there for a client that cannot (M1-005). Handlers do not
// otherwise touch CSRF.
func csrfTokenFrom(ctx context.Context) string {
	token, ok := ctx.Value(csrfKey{}).(string)
	if !ok {
		return ""
	}
	return token
}
