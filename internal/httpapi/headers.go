package httpapi

import (
	"net/http"
	"strings"

	"github.com/bryanster/purpleops/internal/config"
)

// contentSecurityPolicy is the policy the embedded SPA (M0B-008, M0B-010) is
// built to satisfy. Everything loads from this origin and nothing else: there
// is no CDN, no analytics and no font service, because the deployment model is
// a single binary that may be air-gapped.
//
// The one relaxation is inline *styles*, which Radix — the primitives shadcn/ui
// is built on — writes as style attributes when it positions a popover or a
// dialog. Inline *scripts* stay forbidden, which is the half that matters for
// XSS: an injected <script> or an onclick attribute does not run under this
// policy. If M0B-008 needs more than this, the argument belongs in that ticket
// rather than in a quiet edit here.
const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; " +
	"font-src 'self'; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'none'; " +
	"form-action 'self'; " +
	"frame-ancestors 'none'"

// hstsMaxAge is one year, which is what a browser needs to see before the
// origin is worth remembering, and what preload lists require.
const hstsValue = "max-age=31536000; includeSubDomains"

// securityHeaders sets the response headers that constrain what a browser will
// do with what this server sends. They go on every response — including problem
// documents and /healthz — because "every response except that one" is how a
// header ends up missing from the response that mattered.
//
// HSTS is conditional on the deployment actually being reachable over https:
// sending it from an http deployment either does nothing or, once the base URL
// gains a scheme, locks clients out of a server that cannot serve TLS.
func securityHeaders(baseURL config.URL) func(http.Handler) http.Handler {
	https := !baseURL.IsZero() && strings.EqualFold(baseURL.Scheme, "https")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := w.Header()

			// Content sniffing turns an uploaded evidence file the server
			// labelled text/plain into whatever the browser thinks it looks
			// like — including HTML, on this origin (M3).
			header.Set("X-Content-Type-Options", "nosniff")

			// A report share link (M6) is a capability in a URL; a referrer
			// header is how it reaches whatever the user clicks through to.
			header.Set("Referrer-Policy", "no-referrer")

			// frame-ancestors in the CSP says the same thing to a modern
			// browser. This is for the ones that only understand the old
			// header, and it costs 24 bytes.
			header.Set("X-Frame-Options", "DENY")

			header.Set("Content-Security-Policy", contentSecurityPolicy)

			if https {
				header.Set("Strict-Transport-Security", hstsValue)
			}

			next.ServeHTTP(w, r)
		})
	}
}
