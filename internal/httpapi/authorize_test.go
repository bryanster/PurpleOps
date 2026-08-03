package httpapi

// The authorization middleware, tested through the chain the process builds
// rather than by calling it directly. A middleware asserted in isolation proves
// it works when it is reached, and "is it reached" is the whole of what M1-013
// is about — v1's checks were correct too, on the endpoints that had them.
//
// The fixture document below is the one thing not taken from api/openapi.yaml:
// it has the endpoints M1 does not have yet — something an engagement owns, and
// something nobody has mapped — because the two failures worth proving are about
// endpoints that do not exist here yet, and one of them must never exist at all.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"

	"github.com/bryanster/blacklight/api"
	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// engagementSpec describes one engagement-scoped read, mapped the way M3 will
// map its own: the action, the resource, and the path parameter the engagement
// comes out of.
const engagementSpec = `
openapi: 3.1.0
info: {title: authorization fixture, version: 1.0.0}
servers: [{url: /api/v1}]
paths:
  /engagements/{engagementId}/steps:
    parameters:
      - {name: engagementId, in: path, required: true, schema: {type: string}}
    get:
      operationId: listSteps
      summary: Read the steps of one engagement.
      x-authz-action: execution.read
      x-authz-resource: {type: execution, engagement: engagementId}
      responses: {"200": {description: ok}}
`

// TestADeniedRequestNeverEntersTheHandler is the criterion stated as a
// mechanism: the handler behind this route panics, so a test that gets a 403
// rather than a 500 has proved the handler did not run — not that it ran and
// changed nothing.
func TestADeniedRequestNeverEntersTheHandler(t *testing.T) {
	t.Parallel()

	server, _ := authorizingServer(t, ownedBy("engagement-1", false), panicHandler(t))

	recorder := do(server, asMember(t, "observer-1", authz.PlatformRoleMember,
		httptest.NewRequest(http.MethodGet, BasePath+"/engagements/engagement-2/steps", nil)))

	if got, want := recorder.Code, http.StatusNotFound; got != want {
		t.Fatalf("status = %d, want %d\nbody: %s", got, want, recorder.Body.String())
	}
}

// TestANonMemberIsNotToldTheEngagementExists is the 404 half, and the reason
// [authz.Decision.Conceal] exists. PLAN.md §4: "Non-members get nothing on an
// engagement, including its existence." A 403 here would confirm the engagement
// is real, and an identifier that answers 403 while its neighbours answer 404 is
// an engagement somebody has enumerated.
func TestANonMemberIsNotToldTheEngagementExists(t *testing.T) {
	t.Parallel()

	// The engagement is real, and this caller is not in it. That is the only
	// interesting case: an engagement that does not exist answers 404 whoever
	// asks, and the point is that the two are the same answer.
	server, logs := authorizingServer(t, ownedBy("engagement-1", false), panicHandler(t))

	recorder := do(server, asMember(t, "outsider-1", authz.PlatformRoleMember,
		httptest.NewRequest(http.MethodGet, BasePath+"/engagements/engagement-1/steps", nil)))

	if got, want := recorder.Code, http.StatusNotFound; got != want {
		t.Fatalf("status = %d, want %d\nbody: %s", got, want, recorder.Body.String())
	}
	if got, want := decodeProblem(t, recorder).Code, gen.ProblemCodeNotFound; got != want {
		t.Errorf("code = %q, want %q", got, want)
	}
	assertRefusalLeaksNothing(t, recorder.Body.String())

	// The reason is the operator's, and it is in the log — a denial nobody can
	// account for is the state v1 was in.
	if record := logs.find(t, "refused a request the caller may not make"); record["authorization"] == nil {
		t.Errorf("the refusal was logged without the decision it was made on:\n%s", logs.String())
	}
}

// TestAnOrdinaryRefusalIs403 is the other half, driven through the real
// specification rather than a fixture: `GET /settings/mfa` requires
// `settings.read`, which the member platform role does not hold.
//
// It is the regression case from PLAN.md §4 stated as a request. v1's
// /manage/access was reachable by anybody signed in; this is the same shape of
// endpoint, and the same caller, answered the way it should have been.
func TestAnOrdinaryRefusalIs403(t *testing.T) {
	t.Parallel()

	server, _ := newTestServerWith(t, testConfig(t), storetest.Migrated(t))

	recorder := do(server, asMember(t, "member-1", authz.PlatformRoleMember,
		httptest.NewRequest(http.MethodGet, BasePath+"/settings/mfa", nil)))

	if got, want := recorder.Code, http.StatusForbidden; got != want {
		t.Fatalf("status = %d, want %d — a member reached the platform settings\nbody: %s",
			got, want, recorder.Body.String())
	}
	if got, want := decodeProblem(t, recorder).Code, gen.ProblemCodeForbidden; got != want {
		t.Errorf("code = %q, want %q", got, want)
	}
	assertRefusalLeaksNothing(t, recorder.Body.String())
}

// TestAnAdministratorReachesTheSameEndpoint is what stops the test above from
// passing against a middleware that refuses everybody. It also proves the
// deletion in internal/authn was a move rather than a removal: the check that
// used to live in the service is now in front of the handler, and it still lets
// the right caller through.
func TestAnAdministratorReachesTheSameEndpoint(t *testing.T) {
	t.Parallel()

	server, _ := newTestServerWith(t, testConfig(t), storetest.Migrated(t))

	recorder := do(server, asMember(t, "admin-1", authz.PlatformRoleAdmin,
		httptest.NewRequest(http.MethodGet, BasePath+"/settings/mfa", nil)))

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d\nbody: %s", got, want, recorder.Body.String())
	}
}

// assertRefusalLeaksNothing checks that a refusal's body says nothing about the
// resource or the rule. The reason a decision carries names roles, identifiers
// and actions, and it is written for an operator — apierr.Forbidden and
// apierr.NotFound are what keep it out of the response, and this is what keeps
// them honest.
func assertRefusalLeaksNothing(t *testing.T, body string) {
	t.Helper()

	for _, secret := range []string{"engagement-1", "execution.read", "settings.read", "admin", "not a member"} {
		if strings.Contains(body, secret) {
			t.Errorf("the refusal tells the caller %q, which is part of why it was refused:\n%s", secret, body)
		}
	}
}

// TestAnAnonymousRequestToAProtectedOperationIs401 keeps "you are not signed in"
// and "you may not do this" as different answers. Collapsing them would make a
// session that quietly expired look like a permissions bug to whoever is holding
// the keyboard, and to the client deciding whether to show a login form.
func TestAnAnonymousRequestToAProtectedOperationIs401(t *testing.T) {
	t.Parallel()

	server, _ := authorizingServer(t, ownedBy("engagement-1", false), panicHandler(t))

	recorder := get(server, BasePath+"/engagements/engagement-1/steps")

	if got, want := recorder.Code, http.StatusUnauthorized; got != want {
		t.Fatalf("status = %d, want %d\nbody: %s", got, want, recorder.Body.String())
	}
	if got, want := decodeProblem(t, recorder).Code, gen.ProblemCodeUnauthenticated; got != want {
		t.Errorf("code = %q, want %q", got, want)
	}
}

// TestAnAllowedRequestCarriesItsDecisionToTheHandler is the audit half: M1-015
// records who did what to what, and the values it records have to be the ones
// the decision was made from rather than a handler's reconstruction of them.
func TestAnAllowedRequestCarriesItsDecisionToTheHandler(t *testing.T) {
	t.Parallel()

	own := ownedBy("engagement-1", false)
	own.seats["lead-1"] = authz.EngagementRoleLead

	var recorded Authorization
	var present bool
	server, _ := authorizingServer(t, own, func(w http.ResponseWriter, r *http.Request) {
		recorded, present = authorizationFrom(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	recorder := do(server, asMember(t, "lead-1", authz.PlatformRoleMember,
		httptest.NewRequest(http.MethodGet, BasePath+"/engagements/engagement-1/steps", nil)))

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d\nbody: %s", got, want, recorder.Body.String())
	}
	if !present {
		t.Fatal("the handler ran with no decision in its context; M1-015 would have nothing to record")
	}
	if got, want := recorded.OperationID, "listSteps"; got != want {
		t.Errorf("operation = %q, want %q", got, want)
	}
	if got, want := recorded.Action, authz.ActionExecutionRead; got != want {
		t.Errorf("action = %v, want %v", got, want)
	}
	if got, want := recorded.Resource.EngagementID, "engagement-1"; got != want {
		t.Errorf("engagement = %q, want %q", got, want)
	}
	if !recorded.Allowed || recorded.Reason == "" {
		t.Errorf("decision = %+v, want an allow that says why", recorded)
	}
	if got, want := recorded.Subject.UserID, "lead-1"; got != want {
		t.Errorf("subject = %q, want %q", got, want)
	}
}

// TestBlindModeIsAnsweredWithTheSame404AsANonMember is the HTTP end of the guard
// that internal/store/blind is the query end of. Learning that a step exists is
// most of what blind mode withholds, so the answer has to be the one that admits
// nothing.
func TestBlindModeIsAnsweredWithTheSame404AsANonMember(t *testing.T) {
	t.Parallel()

	own := ownedBy("engagement-1", true)
	own.seats["blue-1"] = authz.EngagementRoleBlue
	server, _ := authorizingServer(t, own, panicHandler(t))

	recorder := do(server, asMember(t, "blue-1", authz.PlatformRoleMember,
		httptest.NewRequest(http.MethodGet, BasePath+"/engagements/engagement-1/steps", nil)))

	if got, want := recorder.Code, http.StatusNotFound; got != want {
		t.Fatalf("status = %d, want %d\nbody: %s", got, want, recorder.Body.String())
	}
}

// TestTheServerRefusesToStartWithAnUnmappedOperation is the mechanism that
// prevents a future unprotected /manage/access, and the one acceptance criterion
// that is about a build rather than a request.
//
// It breaks the *real* document, so it fails if the mapping ever stops being
// required — not only if the fixture drifts.
func TestTheServerRefusesToStartWithAnUnmappedOperation(t *testing.T) {
	t.Parallel()

	doc, err := api.Load()
	if err != nil {
		t.Fatalf("loading the API specification: %v", err)
	}
	// An endpoint somebody added without thinking about permission. It is
	// exactly as plausible as it looks.
	doc.Paths.Set("/manage/access", &openapi3.PathItem{
		Get: &openapi3.Operation{
			OperationID: "listAccess",
			Summary:     "List who may do what.",
			Responses:   okResponses(),
		},
	})

	if _, err := newServer(Deps{Config: testConfig(t), Store: stubStore{}}, doc, nil); err != nil {
		assertStartupFailure(t, err, "listAccess", "Absence is not permission")
	} else {
		t.Fatal("the server was built over a specification with an unmapped operation. " +
			"That endpoint would be reachable by anybody signed in, which is v1's /manage/access")
	}
}

// assertStartupFailure checks that a refusal to build the server names what is
// wrong with the document. A startup failure nobody can act on is a startup
// failure somebody works around.
func assertStartupFailure(t *testing.T, err error, wants ...string) {
	t.Helper()

	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q\nwant it to mention %q", err, want)
		}
	}
}

// TestTheServerRefusesToStartWithNothingToLoadAnEngagementFrom is the same
// discipline one level down. An operation that acts on something an engagement
// owns needs the facts about that engagement; a build with no way to load them
// must not start and then answer 500 at the first request — the failure belongs
// on the machine of whoever added the endpoint.
func TestTheServerRefusesToStartWithNothingToLoadAnEngagementFrom(t *testing.T) {
	t.Parallel()

	deps := Deps{Config: testConfig(t), Store: stubStore{}} // no Ownership
	_, err := newServer(deps, fixtureSpec(t, engagementSpec), nil)
	if err == nil {
		t.Fatal("the server was built with an engagement-scoped operation and nothing to load an engagement from")
	}
	if !strings.Contains(err.Error(), "Deps.Ownership") {
		t.Errorf("error = %q, want it to name what is missing", err)
	}
}

// TestEveryRouteTheServerServesIsOneTheMappingCovers walks the router the
// process builds and checks each of its API routes against the mapping.
//
// The test in api/ proves the *specification* has no gaps. This proves the
// specification is what the server actually routes from — a route registered
// beside the generated ones, on a path no operation describes, would be served
// by a chain whose authorization step has nothing to look up. Together they are
// what makes "no route quietly avoids it" a checked claim rather than an
// intention.
func TestEveryRouteTheServerServesIsOneTheMappingCovers(t *testing.T) {
	t.Parallel()

	doc, err := api.Load()
	if err != nil {
		t.Fatalf("loading the API specification: %v", err)
	}
	requirements, err := api.Requirements(doc)
	if err != nil {
		t.Fatalf("api.Requirements: %v", err)
	}
	mapped := map[string]bool{}
	for _, requirement := range requirements {
		mapped[requirement.Method+" "+BasePath+requirement.Path] = true
	}

	server, _ := newTestServer(t)
	router, ok := server.(chi.Routes)
	if !ok {
		t.Fatal("the server is not a chi router; this test walks its routes")
	}

	var unmapped []string
	err = chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		switch {
		case !strings.HasPrefix(route, BasePath):
			// The SPA's catch-all. It serves a static page, reaches no data and
			// is M0B-010's; it is deliberately not an API route.
			return nil
		case strings.HasSuffix(route, "/*"):
			// chi's own subtree wildcard under the mount, not an endpoint.
			return nil
		case !mapped[method+" "+route]:
			unmapped = append(unmapped, method+" "+route)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the router: %v", err)
	}
	if len(unmapped) > 0 {
		t.Errorf("these routes are served but describe no operation in api/openapi.yaml, so the authorization "+
			"middleware has nothing to look up for them: %s", strings.Join(unmapped, ", "))
	}
}

// --- helpers ---------------------------------------------------------------

// stubOwnership is one engagement and the seats people hold in it. It is a fake
// rather than a mock: the tests are about what the middleware does with the
// facts, so the facts are a struct literal and nothing asserts on the calls.
type stubOwnership struct {
	engagementID string
	blind        bool
	seats        map[string]authz.EngagementRole
}

func ownedBy(engagementID string, blind bool) *stubOwnership {
	return &stubOwnership{
		engagementID: engagementID,
		blind:        blind,
		seats:        map[string]authz.EngagementRole{},
	}
}

func (s *stubOwnership) Facts(_ context.Context, ref ResourceRef) (ResourceFacts, error) {
	if ref.EngagementID != s.engagementID {
		return ResourceFacts{}, apierr.NotFound("engagement", ref.EngagementID)
	}
	// Unrevealed, because the interesting half of blind mode is the step
	// somebody has not been shown.
	return ResourceFacts{EngagementID: s.engagementID, Blind: s.blind, Revealed: false}, nil
}

func (s *stubOwnership) Seat(_ context.Context, engagementID, userID string) (authz.EngagementRole, bool, error) {
	if engagementID != s.engagementID {
		return "", false, nil
	}
	seat, ok := s.seats[userID]
	return seat, ok, nil
}

// authorizingServer builds the production chain over [engagementSpec], with
// handler where the generated route would be — so the validator, the
// authentication step and the authorization middleware in front of it are the
// ones newServer wires for the real specification.
func authorizingServer(t *testing.T, own Ownership, handler http.HandlerFunc) (http.Handler, *logBuffer) {
	t.Helper()

	logs := &logBuffer{}
	deps := Deps{Config: testConfig(t), Store: stubStore{}, Logger: logs.logger(), Ownership: own}
	server, err := newServer(deps, fixtureSpec(t, engagementSpec), func(r chi.Router) {
		r.Get("/engagements/{engagementId}/steps", handler)
	})
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	return server, logs
}

// asMember returns the request as though authentication had already resolved it
// to this caller.
//
// The subject is injected rather than signed in for real because these tests are
// about authorization and a real sign-in would need a user, a password, a
// session and an engagement in the database — none of which would make the
// assertion stronger. auth_test.go covers the resolving; this covers what is
// done with the result.
func asMember(t *testing.T, userID string, role authz.PlatformRole, r *http.Request) *http.Request {
	t.Helper()

	return r.WithContext(authn.WithSubject(r.Context(), authn.Subject{
		UserID:       userID,
		Email:        userID + "@example.test",
		PlatformRole: role,
		Method:       authz.MethodCookie,
		SessionID:    "session-" + userID,
		MFASatisfied: true,
	}))
}

// panicHandler is how "the handler was never entered" is proved. A flag set by
// the handler would prove only that it did not set the flag.
func panicHandler(t *testing.T) http.HandlerFunc {
	t.Helper()

	return func(http.ResponseWriter, *http.Request) {
		panic("a refused request reached the handler behind the authorization middleware")
	}
}

// fixtureSpec parses one of the inline documents in this file.
func fixtureSpec(t *testing.T, spec string) *openapi3.T {
	t.Helper()

	loader := &openapi3.Loader{}
	doc, err := loader.LoadFromData([]byte(spec))
	if err != nil {
		t.Fatalf("loading the fixture spec: %v", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("the fixture spec is not valid: %v", err)
	}
	return doc
}

// okResponses is the minimum an operation needs to be a valid one, for the
// operation this file adds to the real document.
func okResponses() *openapi3.Responses {
	description := "ok"
	responses := openapi3.NewResponses()
	responses.Set("200", &openapi3.ResponseRef{Value: &openapi3.Response{Description: &description}})
	return responses
}
