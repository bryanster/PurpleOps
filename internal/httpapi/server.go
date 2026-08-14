package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"

	"github.com/bryanster/blacklight/api"
	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authn/challenge"
	"github.com/bryanster/blacklight/internal/authn/oidc"
	"github.com/bryanster/blacklight/internal/authn/recovery"
	"github.com/bryanster/blacklight/internal/authn/saml"
	"github.com/bryanster/blacklight/internal/authn/secrets"
	"github.com/bryanster/blacklight/internal/authn/servicetoken"
	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/authn/throttle"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/content/atomic"

	"github.com/bryanster/blacklight/internal/analytics"

	"github.com/bryanster/blacklight/internal/content/attack"
	"github.com/bryanster/blacklight/internal/content/attackpin"
	"github.com/bryanster/blacklight/internal/content/ctid"
	"github.com/bryanster/blacklight/internal/content/sigma"
	engagement "github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/events/presence"
	"github.com/bryanster/blacklight/internal/evidence"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/report"
	"github.com/bryanster/blacklight/internal/report/blocks"
	pdfreport "github.com/bryanster/blacklight/internal/report/pdf"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/identity"
	storereport "github.com/bryanster/blacklight/internal/store/report"
	"github.com/bryanster/blacklight/internal/store/settings"
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

	// Ownership loads the facts an engagement-scoped authorization decision
	// needs (M1-013). Nil defaults to the store-backed loader that walks a
	// resource to its owning engagement (M7-011); tests may substitute a fake.
	Ownership Ownership

	// ContentAdapters registers kind→adapter implementations on the content
	// job runner (M2-003). Production leaves this nil until concrete adapters
	// land (M2-006+); tests inject a fixture adapter to exercise the pipeline
	// end to end over HTTP.
	ContentAdapters map[storecontent.Kind]content.Adapter
	// ContentLookupIP resolves a content source hostname during URL validation
	// (M7-014). Nil uses net.DefaultResolver. Tests inject a stub so content
	// URL validation never touches the network.
	ContentLookupIP func(ctx context.Context, host string) ([]net.IP, error)

	// DisableContentRunner skips boot/start of the content job worker. Tests
	// that substitute a non-functional store (panickyStore) set this so
	// construction does not touch the database.
	DisableContentRunner bool

	// Presence is the in-memory presence registry (M4-006). Nil for tests
	// that don't need presence.
	Presence *presence.Registry
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
	if deps.Ownership == nil {
		// The store-backed loader: Facts walks the named row to its owning
		// engagement, so blind mode and child-id mismatch are answered the same
		// way as a missing row.
		deps.Ownership = NewOwnership(deps.Store)
	}
	responder := apierr.NewResponder(log)

	validate, err := requestValidator(doc, responder)
	if err != nil {
		return nil, err
	}

	// Before anything else is built, because this is the one that fails on a
	// specification with a gap in it — and a server that would serve an
	// unprotected endpoint should not get as far as opening a socket.
	authorized, err := authorize(doc, deps.Ownership, responder, log)
	if err != nil {
		return nil, err
	}

	activityLog := events.New(activity.New(deps.Store))

	// M4-002: set up the post-commit fan-out queue. DB.Write flushes it
	// after every successful commit. The log's hub is wired below.
	store.PostCommitFanout.Store(new(store.FanoutQueue))

	auth, sessions, challenges, err := newAuthn(deps, activityLog, log)
	if err != nil {
		return nil, err
	}

	provider, err := newOIDC(deps, log)
	if err != nil {
		return nil, err
	}

	federation, err := newSAML(deps, log)
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
	// 12. the MFA enrolment gate (M1-008): a session belonging to somebody who
	//     is required to hold a second factor and holds none may reach the
	//     enrolment endpoints and nothing else. After authentication, because
	//     the state it acts on is decided there.
	// 13. authorization (M1-013): the one place that decides what a caller may
	//     do, immediately in front of the handlers and on the same router — so
	//     there is no route that can quietly avoid it. Last, because it is the
	//     step that needs everything the ones above establish, and because a
	//     request refused here has already been counted, logged and identified.
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
		responder.Write(w, r, apierr.NotFound("endpoint", apierr.RedactPath(r.URL.Path)))
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
		throttleCredentials(limiter, credentialAccounts(auth), activityLog, responder, log),
		authenticate(auth, responder, log),
		requireCSRF(sessions, responder, log),
		clearSpentChallenge(challenges),
		requireMFAEnrolment(responder, log),
		authorized,
	)
	gen.HandlerWithOptions(strictHandler(deps, auth, sessions, challenges, provider, federation, activityLog, log, responder, apiRouter),
		gen.ChiServerOptions{
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
func newAuthn(deps Deps, activityLog *events.Log, log *slog.Logger) (*authn.Service, *session.Manager, *challenge.Manager, error) {
	sessionOpts := session.OptionsFrom(deps.Config)
	sessionOpts.Activity = activityLog
	sessions, err := session.New(identity.NewSessions(deps.Store), sessionOpts)
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
	// The same configured material as the cipher, and a different derived key:
	// one encrypts what this server holds, the other authenticates what somebody
	// presents (M1-007). internal/authn/recovery says why they are separate
	// derivations rather than one shared key.
	hasher, err := recovery.NewHasher(deps.Config.Encryption.Key.Reveal())
	if err != nil {
		return nil, nil, nil, err
	}
	// A third derivation from the same material, for the third thing that has
	// to be authenticated rather than stored (M1-011). It is keyed off the
	// encryption key and not the session secret deliberately: rotating the
	// session secret is the documented way to sign every browser out, and it
	// must not also break every integration in the deployment.
	tokenOptions, err := servicetoken.OptionsFrom(deps.Config)
	if err != nil {
		return nil, nil, nil, err
	}
	tokenOptions.Log = log
	tokenOptions.Activity = activityLog
	tokens, err := servicetoken.New(identity.NewServiceTokens(deps.Store), tokenOptions)
	if err != nil {
		return nil, nil, nil, err
	}

	auth, err := authn.NewService(authn.Deps{
		Users:         identity.NewUsers(deps.Store),
		Memberships:   identity.NewMemberships(deps.Store),
		TOTP:          identity.NewTOTPs(deps.Store),
		RecoveryCodes: identity.NewRecoveryCodes(deps.Store),
		Identities:    identity.NewIdentities(deps.Store),
		Settings:      settings.New(deps.Store),
		Sessions:      sessions,
		Challenges:    challenges,
		Tokens:        tokens,
		Secrets:       cipher,
		Recovery:      hasher,
		Activity:      activityLog,
		Issuer:        totpIssuer(deps.Config),
		Log:           log,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return auth, sessions, challenges, nil
}

// newOIDC builds the single sign-on provider, or returns nil when this
// deployment has none configured (M1-009).
//
// Nil is a supported state and the default one: every endpoint that needs a
// provider answers 404 without it, and `GET /auth/providers` offers a password
// and nothing else. Only a configuration this server cannot make sense of is an
// error here — a provider that cannot be *reached* is not, which is the whole
// point of the next paragraph.
//
// Discovery is kicked off in the background and its result is ignored. A server
// that waited for it would start slowly on a good day and not at all on a bad
// one, and the page it would have served — the one with local sign-in on it — is
// exactly what somebody needs when the identity provider is down. The provider
// retries on its own; nothing has to be restarted when it comes back.
func newOIDC(deps Deps, log *slog.Logger) (*oidc.Provider, error) {
	if !deps.Config.OIDC.Enabled() {
		return nil, nil
	}

	opts := oidc.OptionsFrom(deps.Config, deps.Config.Server.BaseURL.String(), BasePath)
	// The server's logger, not the process default: everything this provider
	// reports about a sign-in belongs in the same log as the request that caused
	// it.
	opts.Log = log

	provider, err := oidc.New(opts)
	if err != nil {
		return nil, err
	}
	log.Info("single sign-on is configured",
		slog.String("issuer", provider.Issuer()),
		slog.Bool("auto_provision", provider.AutoProvision()))

	// Detached from any request, and given a deadline of its own: this outlives
	// the call that started it and must not outlive the process's patience. The
	// error is the provider's to log — it does so, with the issuer and the reason
	// — and a goroutine has nowhere to return one to.
	go discoverInBackground(provider, log)

	return provider, nil
}

// discoverInBackground performs the startup discovery attempt.
//
// being built, and the attempt deliberately outlives that call.
//
//nolint:contextcheck // There is no request context here: this is the server
func discoverInBackground(provider *oidc.Provider, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), oidcStartupDiscovery)
	defer cancel()

	if err := provider.Refresh(ctx); err != nil {
		// The provider has already logged this at warn, with the issuer and the
		// reason. All that is left to say is that nothing is broken by it.
		log.DebugContext(ctx, "single sign-on will be offered once the provider answers",
			slog.String("issuer", provider.Issuer()))
	}
}

// oidcStartupDiscovery bounds the discovery attempt made at startup. It is
// generous: nothing waits for it.
const oidcStartupDiscovery = 30 * time.Second

// newSAML builds the SAML service provider, or returns nil when this deployment
// has none configured (M1-010).
//
// Everything [newOIDC] says applies here, including the part that matters most:
// a provider whose metadata cannot be *read* is not a startup failure. The
// difference is that a deployment configured with a metadata file has already
// read it — [saml.OptionsFrom] does, so an unreadable or malformed file is a
// configuration error with the variable's name in it, which is right, because
// nothing about a local file is going to improve while the server runs.
func newSAML(deps Deps, log *slog.Logger) (*saml.Provider, error) {
	if !deps.Config.SAML.Enabled() {
		return nil, nil
	}

	opts, err := saml.OptionsFrom(deps.Config, deps.Config.Server.BaseURL.String(), BasePath)
	if err != nil {
		return nil, err
	}
	opts.Log = log
	// The replay cache, which is the one thing in this package that SAML needs
	// and OIDC does not: an assertion has no nonce, so nothing but a record of
	// what has been accepted stops it being accepted twice.
	opts.Assertions = identity.NewSAMLAssertions(deps.Store)

	provider, err := saml.New(opts)
	if err != nil {
		return nil, err
	}
	log.Info("SAML single sign-on is configured",
		slog.String("entity_id", provider.EntityID()),
		slog.Bool("auto_provision", provider.AutoProvision()))

	go readMetadataInBackground(provider, log)

	return provider, nil
}

// readMetadataInBackground performs the startup metadata read.
//
// being built, and the attempt deliberately outlives that call.
//
//nolint:contextcheck // There is no request context here: this is the server
func readMetadataInBackground(provider *saml.Provider, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), oidcStartupDiscovery)
	defer cancel()

	if err := provider.Refresh(ctx); err != nil {
		// The provider has already logged this at warn, with the URL and the
		// reason. All that is left to say is that nothing is broken by it.
		log.DebugContext(ctx, "SAML sign-in will be offered once the identity provider answers",
			slog.String("entity_id", provider.EntityID()))
	}
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

// signInLandingPath is the SPA route that shows the sign-in page. It is the one
// place in the Go tree that knows it; web/src/app/routes/app-routes.tsx is the
// other end.
const signInLandingPath = "/login"

// signInURL is the absolute address of this deployment's sign-in page, which is
// what `POST /users` hands back as the invite link (M1-016).
//
// From the configured base URL and never from a request: a link built out of a
// Host header is a link whoever sent the request chose, and this one is about to
// be emailed to somebody by an administrator who will not check it.
func signInURL(cfg config.Config) string {
	return strings.TrimSuffix(cfg.Server.BaseURL.String(), "/") + signInLandingPath
}

// strictHandler wraps the handlers in the generated strict-mode adapter, with
// both of its error hooks pointed at the one responder. An error returned by a
// handler, and a response that will not serialize, then produce the same shape
// as everything else (M0B-007).
//
//nolint:contextcheck // boots the process-scoped content runner with Background
func strictHandler(deps Deps, auth *authn.Service, sessions *session.Manager,
	challenges *challenge.Manager, provider *oidc.Provider, federation *saml.Provider,
	activityLog *events.Log, log *slog.Logger, responder *apierr.Responder, apiRouter chi.Router) gen.ServerInterface {
	paths := storecontent.NewPaths(deps.Config.Content.Dir)
	sources := storecontent.NewSources(deps.Store)
	versions := storecontent.NewVersions(deps.Store, paths)
	jobs := storecontent.NewJobs(deps.Store)
	objects := storecontent.NewObjects(deps.Store)
	procedures := storecontent.NewProcedures(deps.Store)
	detections := storecontent.NewDetections(deps.Store)
	emulationPlans := storecontent.NewEmulationPlans(deps.Store)
	notes := storecontent.NewNotes(deps.Store)

	contentPolicy := content.URLPolicy{
		AllowHTTP: deps.Config.Env.IsDevelopment(),
		LookupIP:  deps.ContentLookupIP,
	}

	registry, err := content.New(content.Deps{
		Sources:  sources,
		Versions: versions,
		Jobs:     jobs,
		Activity: activityLog,
		Policy:   contentPolicy,
	})
	if err != nil {
		// Construction only fails when a repository is nil, which is a
		// programming error at this call site rather than a runtime condition.
		// Panicking keeps NewServer's signature and forces the bug loud.
		panic("httpapi: content registry: " + err.Error())
	}
	scenarios := storengagement.NewScenarios(deps.Store)
	steps := storengagement.NewSteps(deps.Store)
	executions := storengagement.NewExecutions(deps.Store)
	engagements := storengagement.NewEngagements(deps.Store)
	refs := storengagement.NewReferences(engagements)
	pin, err := attackpin.New(attackpin.Deps{
		Sources:  sources,
		Versions: versions,
		Objects:  objects,
		Paths:    paths,
		Activity: activityLog,
		Refs:     refs,
	})
	if err != nil {
		panic("httpapi: attackpin: " + err.Error())
	}
	memberships := identity.NewMemberships(deps.Store)
	users := identity.NewUsers(deps.Store)
	comments := storengagement.NewComments(deps.Store)
	findingsRepo := storengagement.NewFindings(deps.Store)
	engSvc, err := engagement.New(engagement.Deps{
		Engagements: engagements,
		AttackPin:   pin,
		Activity:    activityLog,
		Memberships: memberships,
		Scenarios:   scenarios,
		Steps:       steps,
		Executions:  executions,
		Comments:    comments,
		Findings:    findingsRepo,
		Users:       users,
	})

	if err != nil {
		panic("httpapi: engagement: " + err.Error())
	}

	reportRegistry := report.NewRegistry()

	// Narrative blocks with full definitions and renderers (M6-006).
	reportRegistry.Register(blocks.CoverDef)
	reportRegistry.SetRenderer(report.IDCover, blocks.CoverRenderer{})

	reportRegistry.Register(blocks.SummaryDef)
	reportRegistry.SetRenderer(report.IDExecutiveSummary, blocks.SummaryRenderer{})

	reportRegistry.Register(blocks.ScopeDef)
	reportRegistry.SetRenderer(report.IDScopeRoE, blocks.ScopeRenderer{})

	reportRegistry.Register(blocks.RichTextDef)
	reportRegistry.SetRenderer(report.IDRichText, blocks.RichTextRenderer{})

	reportRegistry.Register(blocks.PageBreakDef)
	reportRegistry.SetRenderer(report.IDPageBreak, blocks.PageBreakRenderer{})

	// Analytics blocks with full definitions and renderers (M6-007).
	reportRegistry.Register(blocks.HeatmapDef)
	reportRegistry.SetRenderer(report.IDCoverageHeatmap, blocks.HeatmapRenderer{})

	reportRegistry.Register(blocks.ScorecardDef)
	reportRegistry.SetRenderer(report.IDTacticScorecard, blocks.ScorecardRenderer{})

	reportRegistry.Register(blocks.DistributionDef)
	reportRegistry.SetRenderer(report.IDDetectionDistribution, blocks.DistributionRenderer{})

	reportRegistry.Register(blocks.GapsDef)
	reportRegistry.SetRenderer(report.IDDetectionGaps, blocks.GapsRenderer{})

	reportRegistry.Register(blocks.MTTDDef)
	reportRegistry.SetRenderer(report.IDMTTD, blocks.MTTDRenderer{})

	reportRegistry.Register(blocks.CompareDef)
	reportRegistry.SetRenderer(report.IDEngagementCompare, blocks.CompareRenderer{})

	reportRegistry.Register(blocks.WalkthroughDef)
	reportRegistry.SetRenderer(report.IDScenarioWalkthrough, blocks.WalkthroughRenderer{})

	reportRegistry.Register(blocks.FindingsDef)
	reportRegistry.SetRenderer(report.IDFindingsBacklog, blocks.FindingsRenderer{})

	reportRegistry.Register(blocks.EvidenceDef)
	reportRegistry.SetRenderer(report.IDEvidenceAppendix, blocks.EvidenceRenderer{})

	// PDF printer (M6-010). Nil when Chrome is not configured — the server
	// starts without it, and the endpoint returns a clear error.
	var pdfPrinter *pdfreport.Printer
	if deps.Config.Report.ChromePath != "" {
		var err error
		pdfPrinter, err = pdfreport.New(deps.Config.Report.ChromePath, 0) // default timeout
		if err != nil {
			log.Warn("PDF printer unavailable; /preview.pdf will return 503",
				"chrome_path", deps.Config.Report.ChromePath,
				"err", err,
			)
			pdfPrinter = nil
		}
	}
	docRenderer := report.NewDocumentRenderer(reportRegistry)

	reportRepo := storereport.NewReports(deps.Store)
	reportSvc, err := report.New(report.Deps{
		Reports:  reportRepo,
		Registry: reportRegistry,
		Activity: activityLog,
	})
	if err != nil {
		panic("httpapi: report: " + err.Error())
	}

	// Report templates (M6-003).
	templateRepo := storereport.NewTemplates(deps.Store)

	templateSvc, err := report.NewTemplateService(report.TemplateDeps{
		Templates: templateRepo,
		Reports:   reportRepo,
		Registry:  reportRegistry,
		Activity:  activityLog,
	})
	if err != nil {
		panic("httpapi: report templates: " + err.Error())
	}

	// Report branding (M6-004).
	settingsStore := settings.New(deps.Store)
	brandingSvc, err := report.NewBrandingSettingsService(settingsStore, deps.Config.Report.BrandingDir)
	if err != nil {
		panic("httpapi: report branding: " + err.Error())
	}

	// Branding resolver + publish service (M6-011).
	brandingResolver := report.NewBrandingResolver(brandingSvc)
	versionRepo := storereport.NewVersions(deps.Store)
	publishSvc, err := report.NewPublishService(report.PublishDeps{
		Reports:  reportRepo,
		Versions: versionRepo,
		Renderer: docRenderer,
		Resolver: brandingResolver,
		Activity: activityLog,
	})
	if err != nil {
		panic("httpapi: report publish: " + err.Error())
	}

	// Report share links and grants (M6-012).
	shareRepo := storereport.NewShares(deps.Store)
	grantRepo := storereport.NewGrants(deps.Store)
	shareTokenHasher, err := report.NewShareTokenHasher(deps.Config.Encryption.Key.Reveal())
	if err != nil {
		panic("httpapi: report share hasher: " + err.Error())
	}
	shareSvc, err := report.NewShareService(report.ShareServiceDeps{
		Shares:      shareRepo,
		Grants:      grantRepo,
		Versions:    versionRepo,
		TokenHasher: shareTokenHasher,
		Activity:    activityLog,
		BaseURL:     strings.TrimSuffix(deps.Config.Server.BaseURL.String(), "/"),
		SessionSec:  deps.Config.Session.Secret.Reveal(),
	})
	if err != nil {
		panic("httpapi: report share: " + err.Error())
	}

	customSvc, err := content.NewCustom(content.CustomDeps{
		Sources:      sources,
		Procedures:   procedures,
		Detections:   detections,
		Notes:        notes,
		Activity:     activityLog,
		NoteMaxBytes: int(deps.Config.Content.NoteMaxBytes.Int64()),
	})
	if err != nil {
		panic("httpapi: content custom: " + err.Error())
	}

	adapters := deps.ContentAdapters
	if adapters == nil {
		adapters = map[storecontent.Kind]content.Adapter{}
	}
	// Production adapters. Tests may pre-populate ContentAdapters (including
	// replacing a kind with a fixture); only fill kinds that are still empty.
	if _, ok := adapters[storecontent.KindAttack]; !ok {
		adapters[storecontent.KindAttack] = attack.New()
	}
	if _, ok := adapters[storecontent.KindAtomic]; !ok {
		adapters[storecontent.KindAtomic] = atomic.New()
	}
	if _, ok := adapters[storecontent.KindSigma]; !ok {
		adapters[storecontent.KindSigma] = sigma.New()
	}
	if _, ok := adapters[storecontent.KindCTID]; !ok {
		adapters[storecontent.KindCTID] = ctid.New()
	}

	runner, err := content.NewRunner(content.RunnerDeps{
		DB:         deps.Store,
		Sources:    sources,
		Versions:   versions,
		Jobs:       jobs,
		Paths:      paths,
		Activity:   activityLog,
		Custom:     customSvc,
		Adapters:   adapters,
		MaxBytes:   deps.Config.Content.MaxBytes.Int64(),
		JobTimeout: deps.Config.Content.JobTimeout,
		WriteBatch: deps.Config.Content.WriteBatch,
		Log:        log,
		Policy:     contentPolicy,
	})
	if err != nil {
		panic("httpapi: content runner: " + err.Error())
	}

	hub := events.NewHub(events.Options{
		MaxSubscribers: deps.Config.Events.MaxSubscribers,
		Buffer:         deps.Config.Events.Buffer,
	})

	// M4-002: enable SSE fan-out from the activity log through the hub.
	activityLog.SetHub(hub)
	if !deps.DisableContentRunner {
		// Boot/Start are process-lifetime, not request-scoped. There is no
		// request context at server construction.
		//
		//nolint:contextcheck // process boot, not a request
		if err := runner.Boot(context.Background()); err != nil {
			panic("httpapi: content runner boot: " + err.Error())
		}
		//nolint:contextcheck // process worker, cancelled via Runner.Stop
		runner.Start(context.Background())

		// Bridge runner progress → hub for the life of the process. The
		// channel closes when nothing holds the subscription; we never unsub
		// deliberately so every job tick reaches SSE clients.
		progCh, progUnsub := runner.Subscribe(deps.Config.Events.Buffer)
		go bridgeContentProgress(hub, progCh, progUnsub, log)
	}

	evidenceMIMEAllowlist := parseMIMEAllowlist(deps.Config.Evidence.MIMEAllowlist)

	evidenceRepo := storengagement.NewEvidenceRepo(deps.Store)

	queries := analytics.NewQueries(deps.Store)

	blobRepo := storengagement.NewEvidenceBlobRepo(deps.Store)
	evidenceStore := evidence.NewStore(deps.Config.Evidence.Dir, deps.Config.Evidence, blobRepo)
	activityEntries := activity.New(deps.Store)

	h := &handlers{
		store:           deps.Store,
		auth:            auth,
		ownership:       deps.Ownership,
		evidenceStore:   evidenceStore,
		evidenceRepo:    evidenceRepo,
		blobRepo:        blobRepo,
		scenarios:       scenarios,
		steps:           steps,
		executions:      executions,
		comments:        comments,
		findings:        findingsRepo,
		activityEntries: activityEntries,
		sessions:        sessions,
		challenges:      challenges,
		oidc:            provider,
		templates:       templateSvc,
		saml:            federation,
		activity:        activityLog,
		content:         registry,
		runner:          runner,
		objects:         objects,
		pdfPrinter:      pdfPrinter,
		procedures:      procedures,
		detections:      detections,
		emulationPlans:  emulationPlans,
		custom:          customSvc,

		attackpin:             pin,
		evidenceMIMEAllowlist: evidenceMIMEAllowlist,
		shareSvc:              shareSvc,
		reports:               reportSvc,

		publishSvc: publishSvc,
		versions:   versionRepo,

		users:       users,
		engagements: engSvc,
		hub:         hub,

		brandingSettings: brandingSvc,
		docRenderer:      docRenderer,
		analytics:        queries,
		presence:         deps.Presence,
		eventsMaxReplay:  deps.Config.Events.MaxReplayEvents,
		eventsHeartbeat:  deps.Config.Events.Heartbeat,
		signInURL:        signInURL(deps.Config),
		log:              log,
	}

	// M4-004: wire the RevealLookup for step-scoped event data.
	activityLog.SetRevealLookup(h)

	handler := gen.NewStrictHandlerWithOptions(
		h,
		nil, // No strict middleware: the chain is chi's, so there is one of them.
		gen.StrictHTTPServerOptions{
			RequestErrorHandlerFunc:  responder.Write,
			ResponseErrorHandlerFunc: responder.Write,
		},
	)

	return handler
}

// parseMIMEAllowlist splits and trims the comma-separated MIME allowlist
// from config. Returns nil when the string is empty — callers treat nil as
// "no restriction" for backward compatibility with tests that never set it.
func parseMIMEAllowlist(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	list := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			list = append(list, p)
		}
	}
	if len(list) == 0 {
		return nil
	}
	return list
}
