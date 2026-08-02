package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authn/challenge"
	"github.com/bryanster/blacklight/internal/authn/throttle"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

// accountOf names the account a request is presenting a credential for, or ""
// when it cannot tell. An empty answer is not a hole: the source limiter needs
// nothing from the request, and it is still counting.
type accountOf func(*http.Request) string

// credentialAccounts are the endpoints where a caller presents a credential, and
// how to find the account they are presenting it for.
//
// A middleware rather than a check inside each handler, because the point of
// M1-004 is that v1 had login rate limiting, lost it, and nothing noticed:
// deleting this from the chain fails seven tests across throttle_test.go and
// mfa_test.go. It is a table rather than a route registration because the
// generated router owns the routes (M0B-005): a path is added here, next to the
// others, instead of somewhere a reader would have to know to look.
//
// The two entries need the account from different places, which is why the value
// is a function rather than a field name. M1-011 adds the service-token
// endpoint, whose credential is a header.
func credentialAccounts(auth *authn.Service) map[string]accountOf {
	return map[string]accountOf{
		BasePath + "/auth/login": bodyField("email"),

		// The body here is six digits and names nobody, so the account comes
		// from the pending state the cookie stands for. It costs two reads on
		// this one route, and without it a code-guessing attack would be
		// counted only against the source limiter — which is far too generous,
		// because it is sized for spraying across every account rather than for
		// guessing one six-digit number.
		BasePath + mfaPathPrefix + "/totp/verify": func(r *http.Request) string {
			return auth.AccountForChallenge(r.Context(), challenge.FromRequest(r))
		},
	}
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
//
// The one exception is a handler that answers 2xx without the exchange being
// over: a login that returns mfa_required has checked a password and issued no
// session. Counting that as a success would clear the account's failure count,
// and an attacker who holds the password could then reset their code-guessing
// budget by signing in again between every guess. Such a handler says so through
// [markCredentialIncomplete], and the attempt counts as neither.
func throttleCredentials(limiter *throttle.Limiter, accounts map[string]accountOf,
	responder *apierr.Responder, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			account, guarded := accounts[routePath(r)]
			if !guarded || r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			attempt := throttle.Attempt{
				Account: account(r),
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

			outcome := &credentialOutcome{}
			recorder := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(recorder, r.WithContext(withCredentialOutcome(ctx, outcome)))

			if outcome.incomplete {
				// The credential was right and the exchange is not over. Neither
				// counted nor cleared: the failures already against this account
				// stay where they are until something actually completes.
				return
			}

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
// credentialAccounts is keyed by.
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

// bodyField reads one string field out of a JSON request body and puts the body
// back for the handler.
//
// It returns "" for anything it cannot read, and that is not a hole: an
// unreadable body is one the handler will refuse as well, and the source
// limiter — which needs nothing from the body — is still counting either way.
func bodyField(field string) accountOf {
	return func(r *http.Request) string {
		if r.Body == nil {
			return ""
		}

		raw, err := io.ReadAll(io.LimitReader(r.Body, maxCredentialBody))
		if err != nil {
			return ""
		}
		// Put it back whatever happened below: this middleware is not the thing
		// that decides whether the request is valid, so it must not be the thing
		// that leaves the handler with an empty body.
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
}

// credentialOutcome is how a handler tells this middleware that its 2xx does not
// mean the credential exchange finished. It is a pointer in the context rather
// than a return value because the handler is several frames below and behind a
// generated adapter that has no channel back.
//
// There is exactly one setter and one reader, both in this package, and the
// default — an unset flag — is the conservative one: a handler that says nothing
// is judged on its status, as every handler was before this existed.
type credentialOutcome struct {
	incomplete bool
}

type credentialOutcomeKey struct{}

func withCredentialOutcome(ctx context.Context, outcome *credentialOutcome) context.Context {
	return context.WithValue(ctx, credentialOutcomeKey{}, outcome)
}

// markCredentialIncomplete records that the credential presented on this request
// was accepted and did not finish the exchange — a password that was right where
// a second factor is still owed. It is a no-op on a request that did not come
// through the throttle, which is every request outside the credential routes.
func markCredentialIncomplete(ctx context.Context) {
	if outcome, ok := ctx.Value(credentialOutcomeKey{}).(*credentialOutcome); ok {
		outcome.incomplete = true
	}
}
