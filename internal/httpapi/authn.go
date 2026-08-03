package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authn/challenge"
	"github.com/bryanster/blacklight/internal/authn/servicetoken"
	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

// authenticate resolves the caller's credential and puts them in the request
// context. It is the "authentication" step in the chain described in
// internal/httpapi/server.go.
//
// Two credentials arrive here and one subject leaves: a session cookie
// (M1-003) and an `Authorization: Bearer` service token (M1-011). They are
// resolved in the same middleware, into the same [authn.Subject], on purpose —
// PLAN.md §4's complaint about v1 is that its API keys "authenticate nothing",
// and the way a credential ends up checking nothing is by being checked
// somewhere the session path never goes. There is one path, so there is one
// thing to get right.
//
// It decides nothing about access. A request with no credential, an expired
// session or a revoked token goes through exactly as it arrived, with no
// subject in its context — refusing is authorization's job, and it happens in
// one place (authorize.go, M1-013) rather than in a middleware that would have
// to know which endpoints are public. Which ones are is a fact api/openapi.yaml
// carries, and only that middleware reads it.
//
// A database failure is different, and is answered here: "the store did not
// answer" is not the same as "you are not signed in", and reporting it as the
// latter would sign everybody out — and break every integration — whenever the
// database hiccupped.
func authenticate(svc *authn.Service, responder *apierr.Responder, log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The pending token travels alongside the origin, and for the same
			// reason: a strict handler is handed a context, not a request. It
			// is *not* resolved here — a challenge is not a session, and
			// nothing outside the verification endpoint may turn one into a
			// caller (M1-006). The sealed single sign-on state rides along for
			// the same mechanical reason and with the same caveat: they are
			// opaque values here, opened by internal/authn/oidc (M1-009) and
			// internal/authn/saml (M1-010).
			ctx := withSSOState(
				withPendingToken(withOrigin(r.Context(), r), challenge.FromRequest(r)), r)

			subject, err := resolveCaller(ctx, svc, r)
			switch {
			case errors.Is(err, session.ErrNoSession), errors.Is(err, servicetoken.ErrNoToken):
				// Debug, not warn: an anonymous request is the normal state of
				// the login page, and a line per unauthenticated request would
				// bury the ones worth reading. The wrapped reason says which of
				// the ways of not being authenticated this was; the response
				// says none of them.
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

// resolveCaller turns whichever credential a request carries into a subject.
//
// A bearer token is tried first and, when it resolves, is the whole answer: a
// caller who sent one said which credential they meant, and a request that
// authenticated as a token must not be silently upgraded to the session cookie
// a browser happened to be holding.
//
// A bearer token that does *not* resolve falls through to the cookie, which is
// what M1-005 requires and is the conservative half of this function. The
// exemption in [csrfExempt] turns on [authn.Subject.Method], so the one thing
// that must never happen is a request *claiming* the token method by sending a
// header: falling through means such a request is judged as the cookie request
// it actually is, CSRF check and all, rather than as an anonymous one that the
// exemption would let past for a different reason.
//
// The failed attempt is recorded either way, and recorded here rather than
// inferred downstream from a status code (M1-004). It is the only place that
// knows a token was presented and did not resolve — by the time a response has
// a status, a wrong token that fell back to a good cookie looks exactly like a
// request that never presented one.
func resolveCaller(ctx context.Context, svc *authn.Service, r *http.Request) (authn.Subject, error) {
	presented, sentToken := bearerToken(r)
	if !sentToken {
		return svc.Authenticate(ctx, session.FromRequest(r))
	}

	subject, err := svc.AuthenticateToken(ctx, presented)
	switch {
	case err == nil:
		markCredentialAccepted(ctx)
		return subject, nil
	case !errors.Is(err, servicetoken.ErrNoToken):
		// The store did not answer. Not a failed credential, and not something
		// to fall back from either — the cookie lookup would fail the same way.
		return authn.Subject{}, err
	}

	markCredentialRejected(ctx)
	return svc.Authenticate(ctx, session.FromRequest(r))
}

// bearerScheme is the authentication scheme a service token is sent under. It
// is compared case-insensitively because RFC 9110 §11.1 says the scheme is
// case-insensitive, and a client that sends "bearer" is not wrong.
const bearerScheme = "bearer"

// bearerToken returns the service token an Authorization header carries, and
// false when the request carries no bearer credential at all.
//
// False is not the same as an empty token. A request with no header, or with a
// header for some other scheme, has not attempted to authenticate this way and
// falls through to the cookie; a request with `Bearer` and nothing usable after
// it has, and is answered as the failed attempt it is — which is also what gets
// it counted by the throttle (M1-004).
func bearerToken(r *http.Request) (servicetoken.Token, bool) {
	header := r.Header.Get("Authorization")
	if len(header) < len(bearerScheme) || !strings.EqualFold(header[:len(bearerScheme)], bearerScheme) {
		return "", false
	}
	rest := header[len(bearerScheme):]
	if rest != "" && rest[0] != ' ' {
		// "Bearerish something": a different scheme that happens to share a
		// prefix, not a malformed bearer credential.
		return "", false
	}
	return servicetoken.Token(strings.TrimSpace(rest)), true
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
// It is how a handler takes the caller off the context, and not a decision: the
// authorization middleware has already refused an anonymous request to anything
// that is not declared public, so the error below is unreachable through the
// server's own chain. It stays because a handler that reads a subject has to do
// something when there is none, and 401 is the honest answer — a nil dereference
// is not.
func subjectFrom(ctx context.Context) (authn.Subject, error) {
	subject, ok := authn.SubjectFrom(ctx)
	if !ok {
		return authn.Subject{}, apierr.Unauthenticated("the request carries no usable session")
	}
	return subject, nil
}
