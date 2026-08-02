package httpapi

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"

	"github.com/bryanster/blacklight/api"
	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authn/challenge"
	"github.com/bryanster/blacklight/internal/authn/secrets"
	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/authn/throttle"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// BasePath is where the generated routes are mounted. It is the one server
// declared in api/openapi.yaml, so the spec, the router and the SPA's fetch
// calls all agree by construction rather than by three people remembering.
const BasePath = "/api/v1"

// Deps is everything the server needs from the process around it. It is a
// struct rather than a list of arguments so that adding a dependency in M1 is
// an additive change at every call site — and it is explicit rather than a
// package-level global, which is what PLAN.md §6 requires.
type Deps struct {
	// Config is the whole configuration. Only the Server section is read today;
	// the rest is here so that a handler added later — one that needs the
	// evidence directory, or the session secret — does not change this
	// signature or every call site with it.
	Config config.Config

	// Store is the database. Required — a server that cannot answer /healthz
	// truthfully has nothing to offer an orchestrator.
	Store store.Store

	// Logger receives the request log and everything the middleware reports. A
	// nil logger means slog.Default(), for a test that does not care.
	Logger *slog.Logger

	// UI is the built single-page app: index.html and the assets it loads,
	// served on every path the API does not own (M0B-010). cmd/blacklight passes
	// web.Dist(), which is either the embedded frontend or a placeholder page.
	//
	// Nil serves no UI at all — every unknown path is then the JSON 404 it was
	// before this existed, which is what the tests in this package that are
	// about the API want.
	UI fs.FS
}

// NewServer builds the HTTP handler: the middleware chain, the routes
// generated from api/openapi.yaml, and the problem responses for everything
// that does not reach a handler.
//
// It returns an error rather than panicking, because the specification it
// loads is a file that can be wrong; a server that cannot be built is a startup
// failure with a message, not a stack trace.
func NewServer(deps Deps) (http.Handler, error) {
	doc, err := api.Load()
	if err != nil {
		return nil, fmt.Errorf("httpapi: load the API specification: %w", err)
	}
	return newServer(deps, doc, nil)
}

// newServer is the body of [NewServer] with two things passed in that the
// process never varies.
//
// doc is the specification the request validator enforces. extraRoutes
// registers handlers on the API router, behind that validator, alongside the
// generated ones. Both exist for the tests: api/openapi.yaml describes no
// request body until M1, so proving that a bad body is rejected before a
// handler runs needs an operation that takes one — and a handler that records
// whether it ran. Production passes nil for both.
func newServer(deps Deps, doc *openapi3.T, extraRoutes func(chi.Router)) (http.Handler, error) {
	if deps.Store == nil {
		return nil, errors.New("httpapi: no store; the health check has nothing to report on")
	}
	log := deps.Logger
	if log == nil {
		log = slog.Default()
	}
	responder := apierr.NewResponder(log)

	validate, err := requestValidator(doc, responder)
	if err != nil {
		return nil, err
	}

	auth, sessions, challenges, err := newAuthn(deps, log)
	if err != nil {
		return nil, err
	}

	policy := throttle.PolicyFrom(deps.Config)
	policy.Log = log
	limiter, err := throttle.New(policy)
	if err != nil {
		return nil, err
	}

	router := chi.NewRouter()

	// The order is the design, outermost first — the order a request meets them:
	//
	//  1. requestID     — every line below it, and every response, carries one.
	//  2. realIP        — so the log and (M1-004) the throttler agree on who
	//                     this is, before anything records it.
	//  3. requestLogger — outside the recoverer, so the line it writes reports
	//                     the 500 the recoverer produced rather than the status
	//                     of a request that never finished. See its comment;
	//                     this is the one place the chain deviates from the
	//                     order M0B-006 suggested, and there is a test for it.
	//  4. recoverer     — a panic becomes a problem document and a logged stack.
	//  5. timeout       — the per-request deadline, inside recovery so that a
	//                     handler panicking on a cancelled context is caught.
	//  6. security headers.
	//  7. request validation, mounted on the API router below: it needs the
	//     path to be one the specification describes, which /healthz under any
	//     other prefix is not.
	//  8. credential throttling, also on the API router: it rations failed
	//     sign-in attempts (M1-004). After validation, so a request that does
	//     not match the specification is not counted as a guess; before
	//     authentication, so a locked-out caller costs nothing at all.
	//  9. authentication: it resolves the session cookie into a subject and
	//     decides nothing else (M1-003).
	// 10. CSRF, also on the API router: a state-changing request that
	//     authenticated by cookie must echo the CSRF token (M1-005). After
	//     authentication, because whether it applies depends on *how* the
	//     request authenticated and nothing else.
	// 11. clearing the spent MFA challenge cookie (M1-006), which is a response
	//     wrapper and decides nothing about the request.
	//
	// M1-013 inserts authorization between 11 and the handlers, on the same
	// router — one chain, so there is no route that can quietly avoid it.
	router.Use(
		requestID,
		realIP(deps.Config.Server.TrustedProxies),
		requestLogger(log),
		recoverer(log, responder),
		timeout(deps.Config.Server.RequestTimeout),
		securityHeaders(deps.Config.Server.BaseURL),
	)

	// chi answers an unrouted request with plain text by default. Everything
	// this server says about a failure is a problem document, including the
	// failures that never reach a handler.
	notFound := func(w http.ResponseWriter, r *http.Request) {
		responder.Write(w, r, apierr.NotFound("endpoint", r.URL.Path))
	}
	methodNotAllowed := func(w http.ResponseWriter, r *http.Request) {
		responder.Write(w, r, apierr.MethodNotAllowed(r.Method))
	}
	router.NotFound(notFound)
	router.MethodNotAllowed(methodNotAllowed)

	apiRouter := chi.NewRouter()
	// Set explicitly rather than inherited. chi.Mount gives a sub-router the
	// parent's handlers only when it has none of its own and the parent already
	// has them — a rule that depends on the order of two calls in this function.
	// Saying it here means an unknown path under /api/v1 answers with a problem
	// document because this line says so, not because of where a later edit
	// puts the SPA's catch-all.
	apiRouter.NotFound(notFound)
	apiRouter.MethodNotAllowed(methodNotAllowed)
	apiRouter.Use(
		validate,
		throttleCredentials(limiter, credentialAccounts(auth), responder, log),
		authenticate(auth, responder, log),
		requireCSRF(sessions, responder, log),
		clearSpentChallenge(challenges),
	)
	gen.HandlerWithOptions(strictHandler(deps, auth, sessions, challenges, log, responder), gen.ChiServerOptions{
		BaseRouter: apiRouter,
		// Reached when the generated wrapper cannot bind a parameter. The
		// validator rejects those first, so this is a belt-and-braces path —
		// but its default writes http.Error, which would be the one response in
		// the application that is not a problem document.
		ErrorHandlerFunc: responder.Write,
	})
	if extraRoutes != nil {
		extraRoutes(apiRouter)
	}
	router.Mount(BasePath, apiRouter)

	// After the mount: chi matches the more specific pattern, so /api/v1/… goes
	// to the router above and never to the catch-all, but registering the
	// catch-all first would make Mount's own conflict check the thing standing
	// between the two — and it is checking for a different mistake.
	if deps.UI != nil {
		ui, err := newSPA(deps.UI, responder)
		if err != nil {
			return nil, err
		}
		// GET and HEAD only. A POST to a path the SPA owns is a client bug or a
		// probe, and chi answers a registered path with an unregistered method
		// with the 405 problem — rather than 200 and a page of HTML, which is
		// what a plain http.FileServer under a wildcard would do.
		router.Method(http.MethodGet, "/*", ui)
		router.Method(http.MethodHead, "/*", ui)
	}

	return router, nil
}

// newAuthn builds the session manager, the MFA challenge manager and the login
// service the middleware and the auth handlers share.
//
// They are built here rather than passed in [Deps] because they are made of
// things already there — the store and the configuration — and a caller who had
// to assemble them could assemble two that disagreed about how long a session
// lasts.
func newAuthn(deps Deps, log *slog.Logger) (*authn.Service, *session.Manager, *challenge.Manager, error) {
	sessions, err := session.New(identity.NewSessions(deps.Store), session.OptionsFrom(deps.Config))
	if err != nil {
		return nil, nil, nil, err
	}
	// The pending cookie is scoped to the MFA endpoints and nothing else, so
	// the path it is told about is this router's, not "/".
	challenges, err := challenge.New(
		identity.NewMFAChallenges(deps.Store),
		challenge.OptionsFrom(deps.Config, BasePath+mfaPathPrefix))
	if err != nil {
		return nil, nil, nil, err
	}
	cipher, err := secrets.New(deps.Config.Encryption.Key.Reveal())
	if err != nil {
		return nil, nil, nil, err
	}

	auth, err := authn.NewService(authn.Deps{
		Users:       identity.NewUsers(deps.Store),
		Memberships: identity.NewMemberships(deps.Store),
		TOTP:        identity.NewTOTPs(deps.Store),
		Sessions:    sessions,
		Challenges:  challenges,
		Secrets:     cipher,
		Issuer:      totpIssuer(deps.Config),
		Log:         log,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return auth, sessions, challenges, nil
}

// totpIssuer is the name an authenticator app shows for this deployment.
//
// The host is in it because a person with an account on two Blacklight
// installations otherwise gets two entries called "Blacklight" and no way to tell
// which is which. A colon would move the boundary between the issuer and the
// account in the otpauth label, so a hostname carrying one — which no hostname
// does — is refused by internal/authn/totp rather than silently mangled.
func totpIssuer(cfg config.Config) string {
	host := cfg.Server.BaseURL.Hostname()
	if host == "" {
		return "Blacklight"
	}
	return "Blacklight (" + host + ")"
}

// strictHandler wraps the handlers in the generated strict-mode adapter, with
// both of its error hooks pointed at the one responder. An error returned by a
// handler, and a response that will not serialize, then produce the same shape
// as everything else (M0B-007).
func strictHandler(deps Deps, auth *authn.Service, sessions *session.Manager,
	challenges *challenge.Manager, log *slog.Logger, responder *apierr.Responder) gen.ServerInterface {
	return gen.NewStrictHandlerWithOptions(
		&handlers{
			store:      deps.Store,
			auth:       auth,
			sessions:   sessions,
			challenges: challenges,
			log:        log,
		},
		nil, // No strict middleware: the chain is chi's, so there is one of them.
		gen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  responder.Write,
			ResponseErrorHandlerFunc: responder.Write,
		},
	)
}
