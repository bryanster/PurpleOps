package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/bryanster/purpleops/internal/authn"
	"github.com/bryanster/purpleops/internal/authn/challenge"
	"github.com/bryanster/purpleops/internal/authn/session"
	"github.com/bryanster/purpleops/internal/httpapi/apierr"
)

// authenticate resolves the session cookie and puts the caller in the request
// context. It is the "authentication" step in the chain described in
// internal/httpapi/server.go.
//
// It decides nothing about access. A request with no cookie, an expired session
// or a revoked one goes through exactly as it arrived, with no subject in its
// context — refusing is authorization's job, and it happens in one place
// (M1-013) rather than in a middleware that would have to know which endpoints
// are public. Until that lands, the handlers that need a caller say so
// themselves, in authhandlers.go and mfahandlers.go.
//
// A database failure is different, and is answered here: "the store did not
// answer" is not the same as "you are not signed in", and reporting it as the
// latter would sign everybody out whenever the database hiccupped.
func authenticate(svc *authn.Service, responder *apierr.Responder, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The pending token travels alongside the origin, and for the same
			// reason: a strict handler is handed a context, not a request. It
			// is *not* resolved here — a challenge is not a session, and
			// nothing outside the verification endpoint may turn one into a
			// caller (M1-006).
			ctx := withPendingToken(withOrigin(r.Context(), r), challenge.FromRequest(r))

			subject, err := svc.Authenticate(ctx, session.FromRequest(r))
			switch {
			case errors.Is(err, session.ErrNoSession):
				// Debug, not warn: an anonymous request is the normal state of
				// the login page, and a line per unauthenticated request would
				// bury the ones worth reading. The wrapped reason says whether
				// the cookie was absent, expired, idle or revoked.
				log.DebugContext(ctx, "request is not authenticated",
					slog.String("reason", err.Error()))
				next.ServeHTTP(w, r.WithContext(ctx))

			case err != nil:
				responder.Write(w, r, err)

			default:
				next.ServeHTTP(w, r.WithContext(authn.WithSubject(ctx, subject)))
			}
		})
	}
}

// originKey is this file's context key. It is its own type, so it cannot
// collide with the one in clientip.go or with anything a library puts in a
// context.
type originKey struct{}

// withOrigin records where a request came from, so that a strict handler — which
// is handed a context and not a request — can put it on the session it creates.
//
// The address is the one [realIP] resolved rather than RemoteAddr: a deployment
// behind a trusted proxy should record the client, and one that is not should
// record the peer and never a header.
func withOrigin(ctx context.Context, r *http.Request) context.Context {
	return context.WithValue(ctx, originKey{}, session.Request{
		IP:        ClientIP(ctx),
		UserAgent: r.UserAgent(),
	})
}

// originFrom returns what was recorded by [withOrigin], or the zero value for a
// context that never went through the middleware. Both fields are allowed to be
// empty: the session columns are "" for "we did not record it", which is the
// same fact as "it was absent" to whoever reads them later.
func originFrom(ctx context.Context) session.Request {
	origin, ok := ctx.Value(originKey{}).(session.Request)
	if !ok {
		return session.Request{}
	}
	return origin
}

// subjectFrom returns the caller, or [apierr.Unauthenticated] for a request that
// arrived without a usable session.
//
// It is the one place a handler in this package turns "nobody" into a 401, so
// that the handlers which need a caller cannot each phrase it differently.
// M1-013 moves the decision out of handlers entirely; this is what it replaces.
func subjectFrom(ctx context.Context) (authn.Subject, error) {
	subject, ok := authn.SubjectFrom(ctx)
	if !ok {
		return authn.Subject{}, apierr.Unauthenticated("the request carries no usable session")
	}
	return subject, nil
}
