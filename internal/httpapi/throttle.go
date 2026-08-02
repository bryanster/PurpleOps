package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/bryanster/purpleops/internal/authn/throttle"
	"github.com/bryanster/purpleops/internal/httpapi/apierr"
)

// credentialRoutes are the endpoints where a caller presents a credential, and
// the field of the request body naming the account they are presenting it for.
//
// A middleware rather than a check inside each handler, because the point of
// M1-004 is that v1 had login rate limiting, lost it, and nothing noticed:
// deleting this from the chain fails five tests in throttle_test.go. It is a
// table rather than a route registration because the generated router owns the
// routes (M0B-005): a path is added here, next to the others, instead of
// somewhere a reader would have to know to look.
//
// M1-006 adds the MFA verification endpoint, and M1-011 the service-token one —
// whose credential is a header rather than a body field, and which therefore
// needs an extractor here rather than a field name.
var credentialRoutes = map[string]string{
	BasePath + "/auth/login": "email",
}

// maxCredentialBody is how much of a request body this middleware will read to
// find the account being attempted. The validator has already held the body to
// api/openapi.yaml by the time we get here, so anything approaching this is
// impossible rather than merely unusual; the limit is here so that a change to
// the specification cannot turn into an unbounded read on a public endpoint.
const maxCredentialBody = 64 << 10

// throttleCredentials rations failed sign-in attempts. It is step 8 of the
// chain described in internal/httpapi/server.go.
//
// It sits before authentication rather than after it, so that a locked-out
// source does not reach the session lookup, and so that M1-011 — whose
// credential *is* checked by the authentication step — is throttled by this
// middleware rather than by a second one.
//
// The outcome is read from the status the handler produced: a 401 is a failed
// attempt, a 2xx is a successful one, and anything else — a malformed body, the
// database failing — is neither, because neither says anything about whether the
// caller knows the password. That is what keeps the rule in one place instead of
// asking every handler to report itself.
func throttleCredentials(limiter *throttle.Limiter, responder *apierr.Responder,
	log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			field, guarded := credentialRoutes[routePath(r)]
			if !guarded || r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			attempt := throttle.Attempt{
				Account: accountField(r, field),
				Source:  ClientIP(ctx),
			}

			if err := limiter.Check(attempt); err != nil {
				// Before the handler, so the right password during a lockout is
				// refused too — a lockout that the right password ends is not a
				// lockout — and so that a locked-out caller costs no Argon2id
				// derivation, which is the other half of what throttling is for.
				log.InfoContext(ctx, "refused a throttled sign-in attempt",
					slog.String("client_ip", attempt.Source),
					slog.String("path", r.URL.Path))
				responder.Write(w, r, err)
				return
			}

			recorder := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(recorder, r)

			switch status := statusOf(recorder); {
			case status == http.StatusUnauthorized:
				for _, lockout := range limiter.Failed(attempt) {
					// Warn, not info: this is the line an operator is looking for
					// when somebody reports they cannot sign in, and the line that
					// says an attack is in progress. M1-015 gives it a durable
					// home in the activity log — this is the record until then,
					// and the reason the account is named.
					log.WarnContext(ctx, "locked out after repeated failed sign-in attempts",
						slog.String("scope", string(lockout.Scope)),
						slog.String("key", lockout.Key),
						slog.String("client_ip", attempt.Source),
						slog.Duration("retry_after", lockout.RetryAfter))
				}
			case status >= 200 && status < 300:
				limiter.Succeeded(attempt)
			}
		})
	}
}

// routePath is the request path with a trailing slash removed, which is the form
// credentialRoutes is keyed by.
//
// "/auth/login/" is not a path this API serves — the validator answers it 404
// before this middleware runs — but the throttle deciding that for itself is a
// cheaper thing to be sure of than the two routers agreeing forever.
func routePath(r *http.Request) string {
	path := r.URL.Path
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	return path
}

// accountField reads one string field out of a JSON request body and puts the
// body back for the handler.
//
// It returns "" for anything it cannot read, and that is not a hole: an
// unreadable body is one the handler will refuse as well, and the source
// limiter — which needs nothing from the body — is still counting either way.
func accountField(r *http.Request, field string) string {
	if r.Body == nil {
		return ""
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, maxCredentialBody))
	if err != nil {
		return ""
	}
	// Put it back whatever happened below: this middleware is not the thing that
	// decides whether the request is valid, so it must not be the thing that
	// leaves the handler with an empty body.
	r.Body = io.NopCloser(bytes.NewReader(raw))

	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil {
		return ""
	}
	var value string
	if err := json.Unmarshal(body[field], &value); err != nil {
		return ""
	}
	return value
}
