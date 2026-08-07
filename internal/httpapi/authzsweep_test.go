package httpapi

// The HTTP end of M1-014's permission matrix.
//
// internal/authz/matrix_test.go asserts the policy: 1,840 combinations of role,
// seat, action, resource state and credential, in milliseconds, against a pure
// function. What it cannot assert is that the policy is *reached* — that the
// specification maps the endpoint to the right action, that the middleware reads
// the engagement out of the right path segment, that a concealed denial becomes a
// 404 and an ordinary one a 403, and that a signed-in browser and a service token
// get the same answers. v1's checks were correct too, on the endpoints that had
// them.
//
// So this drives real requests: real accounts in a real database, a real sign-in
// with a real session cookie and its CSRF token, and real service tokens minted
// through the API. The six operations are the ones M1-014 names — read an
// engagement, write a red field, write a blue field, manage members, manage
// users, sync content — chosen because between them they cover both resource
// owners, both refusal shapes, and both sides of the red/blue split.
//
// Most of the endpoints do not exist yet: engagements are M3 and content is M2.
// Those are declared in [sweepSpec] below and merged into the real document, the
// way authorize_test.go does — the wiring under test is the middleware, the
// specification extensions and the status codes, all of which are here today.
// When a real endpoint arrives, delete the matching entry from the fixture and
// point its row at the real path, as user management did when M1-016 landed:
// [sweepOp.Real] marks a row that drives the shipped endpoint rather than a stub.

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"

	"github.com/bryanster/blacklight/api"
	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// The identifiers the sweep acts on. The engagement is the one [stubOwnership]
// knows about; anything else is a 404 from the loader rather than from the
// policy, which is a different test.
//
// There is no constant for the account: user management is a real endpoint now,
// so the row that drives it is pointed at an account that really exists — an
// invented identifier would answer 404 for a reason this file is not about.
const (
	sweepEngagement = "0192f1a0-0000-7000-8000-00000000e001"
	sweepExecution  = "0192f1a0-0000-7000-8000-00000000e002"
	sweepScenario   = "0192f1a0-0000-7000-8000-00000000e003"
	sweepStep       = "0192f1a0-0000-7000-8000-00000000e004"
)

// sweepSpec declares the five endpoints that do not exist yet, mapped the way M2
// and M3 will map their own. The mappings are the thing under test as much as the
// handlers are: `x-authz-resource` is what tells the middleware which path
// segment is the engagement, and getting it wrong is an endpoint that asks the
// policy about the wrong thing.
//
// The sixth — user management — is not here. It is `PATCH /users/{userId}` in
// api/openapi.yaml since M1-016, and the sweep drives that.
const sweepSpec = `
openapi: 3.1.0
info: {title: authorization sweep fixture, version: 1.0.0}
servers: [{url: /api/v1}]
components:
  parameters:
    CSRF:
      name: X-CSRF-Token
      in: header
      required: false
      schema: {type: string}
paths:
  /engagements/{engagementId}/executions/{executionId}/blue:
    parameters:
      - {name: engagementId, in: path, required: true, schema: {type: string}}
      - {name: executionId, in: path, required: true, schema: {type: string}}
    put:
      operationId: sweepWriteBlueField
      summary: Write the detection side of one execution.
      x-authz-action: execution.write_blue
      x-authz-resource: {type: execution, param: executionId, engagement: engagementId}
      parameters:
        - $ref: "#/components/parameters/CSRF"
      responses: {"200": {description: ok}}
`

// statuses is one operation's answer to each caller, in the order [sweepCallers]
// declares them. The literal numbers are the point: 403 and 404 are different
// answers, and writing "denied" for both is how the distinction stops being
// checked.
type statuses struct {
	// Admin is a platform administrator who is a member of no engagement. They
	// hold every engagement-scoped action anyway.
	Admin int

	// Lead, Red, Blue and Observer are platform members holding that seat in the
	// engagement under test.
	Lead, Red, Blue, Observer int

	// Outsider is a platform member who is in no engagement at all. Every
	// engagement-scoped refusal for them is a 404: PLAN.md §4 gives a non-member
	// nothing, "including its existence".
	Outsider int
}

// sweepOp is one endpoint and what each caller gets from it.
type sweepOp struct {
	Name string

	Method string

	// Route is the path as the specification and the router spell it. The
	// concrete URL is derived from it, so there is no second copy to disagree.
	Route string

	// Real marks a row that drives an endpoint this server actually serves,
	// rather than one of the stubs in [sweepSpec]. A real row registers no
	// fixture handler — chi would refuse the duplicate route — so "the request
	// reached the handler" is read off the status rather than off a header the
	// stub sets.
	Real bool

	// Body is what a real row sends. Empty for the stubs, which take none.
	Body string

	Want statuses
}

// sweepOperations is the sweep. Read down a column and you are reading what one
// role can do over HTTP; read across a row and you are reading one endpoint's
// whole answer.
//
// The six numbers in each Want are, in order: the administrator, the lead, red,
// blue, the observer, and somebody in no engagement at all.
var sweepOperations = []sweepOp{
	{
		Name: "read the engagement", Method: http.MethodGet,
		Route: "/engagements/{engagementId}",
		Real:  true,
		Want:  statuses{200, 200, 200, 200, 200, 404},
	},
	{
		// Blue's 403 here and red's 403 below are v1's two definitions of
		// "blue", on the wire.
		Name: "write a red field", Method: http.MethodPatch,
		Route: "/engagements/{engagementId}/executions/{executionId}/execution",
		Real:  true,
		Body:  `{"version":1,"status":"running"}`,
		Want:  statuses{404, 404, 404, 403, 403, 404},
	},
	{
		Name: "write a blue field", Method: http.MethodPut,
		Route: "/engagements/{engagementId}/executions/{executionId}/blue",
		Want:  statuses{200, 200, 403, 200, 403, 404},
	},
	{
		// Admin/lead pass authorization but the user does not exist → 404.
		// Outsider also gets 404, but from concealment rather than user lookup.
		// Both are problem code "not_found" so the assertion holds.
		Name: "manage the members", Method: http.MethodPost,
		Route: "/engagements/{engagementId}/members",
		Real:  true,
		Body:  `{"userId":"ffffffff-ffff-ffff-ffff-ffffffffffff","role":"red"}`,
		Want:  statuses{404, 404, 403, 403, 403, 404},
	},
	{
		// v1's ungated page, rebuilt: this is the real endpoint M1-016 ships,
		// not a stub. Every platform member is refused, and refused with a 403
		// rather than a 404 — the installation is not a secret, and a caller who
		// cannot administer it already knows it is there.
		Name: "manage the users", Method: http.MethodPatch,
		Route: "/users/{userId}",
		Real:  true, Body: `{"displayName":"Swept"}`,
		Want: statuses{200, 403, 403, 403, 403, 403},
	},
	{
		// Real endpoint M2-003 ships. Admin is allowed by authz; the custom
		// source answers 409 (not synced from upstream). Members are refused
		// with 403. ATT&CK, Atomic, Sigma, and CTID have adapters — the sweep
		// pins custom so the conflict stays deterministic without network I/O.
		Name: "sync the content library", Method: http.MethodPost,
		Route: "/content/sources/{sourceId}/sync",
		Real:  true, Body: `{}`,
		Want: statuses{409, 403, 403, 403, 403, 403},
	},

	{
		// workbook.write (M3-004): lead + red + admin, not blue or observer.
		// The scenario does not exist → 404 for those who pass authz.
		Name: "write the workbook", Method: http.MethodDelete,
		Route: "/engagements/{engagementId}/scenarios/{scenarioId}",
		Real:  true,
		Want:  statuses{404, 404, 404, 403, 403, 404},
	},
}

// sweepCaller is one of the six people the sweep is run as.
type sweepCaller struct {
	Name string
	Role authz.PlatformRole

	// Seat is their role in the engagement under test, and is empty for the
	// administrator and the outsider — neither is a member of it.
	Seat authz.EngagementRole

	// Want reads this caller's column out of an operation's row.
	Want func(statuses) int

	user    identity.User
	session *http.Cookie
	token   string
}

// sweepCallers are the six, in the order [statuses] writes them.
func sweepCallers() []*sweepCaller {
	return []*sweepCaller{
		{Name: "an administrator who is in no engagement", Role: authz.PlatformRoleAdmin,
			Want: func(s statuses) int { return s.Admin }},
		{Name: "the lead", Role: authz.PlatformRoleMember, Seat: authz.EngagementRoleLead,
			Want: func(s statuses) int { return s.Lead }},
		{Name: "a red operator", Role: authz.PlatformRoleMember, Seat: authz.EngagementRoleRed,
			Want: func(s statuses) int { return s.Red }},
		{Name: "a blue analyst", Role: authz.PlatformRoleMember, Seat: authz.EngagementRoleBlue,
			Want: func(s statuses) int { return s.Blue }},
		{Name: "an observer", Role: authz.PlatformRoleMember, Seat: authz.EngagementRoleObserver,
			Want: func(s statuses) int { return s.Observer }},
		{Name: "somebody in no engagement at all", Role: authz.PlatformRoleMember,
			Want: func(s statuses) int { return s.Outsider }},
	}
}

// TestTheAuthorizationSweep drives every operation as every caller, both signed
// in and automating, and checks the status code and the problem code against the
// table above.
func TestTheAuthorizationSweep(t *testing.T) {
	t.Parallel()

	server := newSweepServer(t)

	for _, op := range sweepOperations {
		target := server.target(op.Route)

		for _, caller := range server.callers {
			want := caller.Want(op.Want)

			// The same request twice: as a browser, and as automation. The
			// expectations are shared on purpose — none of these six actions is
			// session-only, so a service token carrying every scope must reach
			// exactly what its owner reaches and nothing more. A difference here
			// is the scope fence doing something the owner's role did not ask
			// for.
			for _, arrival := range []struct {
				how      string
				recorder *httptest.ResponseRecorder
			}{
				{"signed in", server.asSession(caller, op, target)},
				{"with a service token", server.asToken(caller, op, target)},
			} {
				if got := arrival.recorder.Code; got != want {
					t.Errorf("%s, %s, tried to %s: %d, want %d\nbody: %s",
						caller.Name, arrival.how, op.Name, got, want, arrival.recorder.Body)
					continue
				}
				server.assertRefusalShape(t, arrival.recorder, caller, op, want)
			}
		}
	}
}

// assertRefusalShape checks the half of the answer that is not the status line:
// that an allowed request reached the handler, that a refused one did not, and
// that a refusal is the documented problem shape carrying none of the reasoning.
func (s *sweepServer) assertRefusalShape(t *testing.T, recorder *httptest.ResponseRecorder,
	caller *sweepCaller, op sweepOp, want int) {
	t.Helper()

	// The stub sets a header when it runs. A real endpoint cannot — it is the
	// shipped handler — so for those rows the status is the observation, which
	// is enough: an allowed 200 came out of the handler and a 403 never got
	// there.
	if !op.Real {
		reached := recorder.Header().Get(sweepHandlerHeader) != ""
		if allowed := want == http.StatusOK; reached != allowed {
			t.Errorf("%s tried to %s: the handler %s, and the status was %d. A refused request must not enter "+
				"the handler at all — a handler that runs and then changes nothing is a handler somebody will "+
				"later make change something", caller.Name, op.Name, ranOrNot(reached), want)
		}
	}
	if want == http.StatusOK || want == http.StatusCreated || want == http.StatusAccepted {
		return
	}

	// A 409 from a real endpoint is a product conflict after authorization
	// succeeded (e.g. no adapter registered yet). It is not a refusal.
	wantCode := gen.ProblemCodeForbidden
	switch want {
	case http.StatusNotFound:
		wantCode = gen.ProblemCodeNotFound
	case http.StatusConflict:
		wantCode = gen.ProblemCodeConflict
	}
	if got := decodeProblem(t, recorder).Code; got != wantCode {
		t.Errorf("%s tried to %s: problem code %q, want %q", caller.Name, op.Name, got, wantCode)
	}
	assertRefusalLeaksNothing(t, recorder.Body.String())
	if body := recorder.Body.String(); strings.Contains(body, sweepEngagement) {
		t.Errorf("%s tried to %s and the refusal named the engagement:\n%s", caller.Name, op.Name, body)
	}
}

func ranOrNot(ran bool) string {
	if ran {
		return "ran"
	}
	return "did not run"
}

// TestTheSweepCoversTheOperationsTheTicketNames holds this file to its own
// premise. A sweep that quietly stopped exercising the blue write would still
// pass, and would still look like a sweep — so the set of actions it drives is
// asserted rather than assumed, and so is the fact that every route in it is one
// the specification describes.
func TestTheSweepCoversTheOperationsTheTicketNames(t *testing.T) {
	t.Parallel()

	// M1-014: "The HTTP sweep covers at least: read engagement, write red field,
	// write blue field, manage members, manage users, sync content."
	required := []authz.Action{
		authz.ActionEngagementRead,
		authz.ActionExecutionWriteRed,
		authz.ActionExecutionWriteBlue,
		authz.ActionMemberManage,
		authz.ActionUserManage,
		authz.ActionContentSync,
	}

	doc := sweepDoc(t)
	requirements, err := api.Requirements(doc)
	if err != nil {
		t.Fatalf("api.Requirements over the merged document: %v", err)
	}

	covered := map[authz.Action]bool{}
	for _, op := range sweepOperations {
		requirement, found := sweepRequirementFor(t, requirements, op)
		if !found {
			continue
		}
		covered[requirement.Action] = true
	}

	for _, action := range required {
		if !covered[action] {
			t.Errorf("the sweep drives no endpoint requiring %s, which M1-014 asks it to cover. The unit "+
				"matrix proves the policy; this file is the only thing proving the wiring reaches it", action)
		}
	}
}

// sweepRequirementFor finds the requirement one sweep row's route and method resolve
// to, and fails the test when the fixture and the table disagree about either.
func sweepRequirementFor(t *testing.T, requirements map[string]api.Requirement, op sweepOp) (api.Requirement, bool) {
	t.Helper()

	for _, requirement := range requirements {
		if requirement.Path == op.Route && strings.EqualFold(requirement.Method, op.Method) {
			return requirement, true
		}
	}
	t.Errorf("the sweep drives %s %s, which the fixture specification does not describe — so the request "+
		"would 404 at the validator and the row below would be asserting nothing", op.Method, op.Route)
	return api.Requirement{}, false
}

// --- the fixture ------------------------------------------------------------

// sweepHandlerHeader is set by the handler behind every sweep route, and is how
// "the request reached the handler" is observed. A response body would not do:
// a refusal has one too.
const sweepHandlerHeader = "X-Sweep-Handler"

// sweepServer is the real chain over the merged document, with six signed-in
// callers and a service token each.
type sweepServer struct {
	*authServer
	callers []*sweepCaller

	// targetUser is the account the user-administration row acts on. It is a
	// seventh account rather than one of the callers: patching a caller's own
	// row mid-sweep would change what the rows after it are testing.
	targetUser identity.User
}

// newSweepServer builds it: one database, six accounts, six sign-ins, six tokens.
//
// The sign-ins are real rather than injected subjects. Injecting one would skip
// the layer where a session becomes a role, and this file exists to test the
// layers between the request and the policy — including the CSRF check that every
// state-changing request in the sweep has to satisfy.
func newSweepServer(t *testing.T) *sweepServer {
	t.Helper()

	own := ownedBy(sweepEngagement, false)
	db := storetest.Migrated(t)
	logs := &logBuffer{}

	// Seed the engagement the sweep routes reference, so that the real
	// GET /engagements/{engagementId} handler (M3-002) does not 500 on a
	// missing row.
	if err := db.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.Exec(
			`INSERT INTO app.engagement
			(id, name, client, description, status, starts_on, ends_on,
			 attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at)
			VALUES (?, 'Sweep', '', '', 'draft', '2025-01-01', '2025-01-01',
			        '15.1', 'standard', false,
			        '0192f1a0-0000-7000-8000-000000000000', '2025-01-01T00:00:00Z', '2025-01-01T00:00:00Z')`,
			sweepEngagement)
		return err
	}); err != nil {
		t.Fatalf("seeding sweep engagement: %v", err)
	}
	cfg := testConfig(t)

	handler, err := newServer(
		Deps{Config: cfg, Store: db, Logger: logs.logger(), Ownership: own},
		sweepDoc(t),
		func(r chi.Router) {
			for _, op := range sweepOperations {
				if op.Real {
					// The server already serves this one. Registering a second
					// handler on the same pattern is a chi panic, and a stub in
					// front of the real handler would be a sweep that proves the
					// stub is protected.
					continue
				}
				r.MethodFunc(op.Method, op.Route, func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set(sweepHandlerHeader, "ran")
					w.WriteHeader(http.StatusOK)
				})
			}
		})
	if err != nil {
		t.Fatalf("building the sweep server: %v", err)
	}

	manager, err := session.New(identity.NewSessions(db), session.OptionsFrom(cfg))
	if err != nil {
		t.Fatalf("building the test session manager: %v", err)
	}
	server := &sweepServer{
		authServer: &authServer{handler: handler, db: db, logs: logs, manager: manager},
		callers:    sweepCallers(),
	}

	for i, caller := range server.callers {
		email := fmt.Sprintf("sweep-%d@example.test", i)
		caller.user = server.seedUser(t, func(in *identity.NewUser) {
			in.Email = email
			in.DisplayName = caller.Name
			in.PlatformRole = caller.Role
		})
		if caller.Seat != "" {
			own.seats[caller.user.ID] = caller.Seat
		}

		recorder := server.login(email, testPassword)
		if recorder.Code != http.StatusOK {
			t.Fatalf("signing in %s: %d\nbody: %s", caller.Name, recorder.Code, recorder.Body)
		}
		caller.session = sessionCookie(t, recorder)

		// Every scope, so that a refusal below is unambiguously the owner's role
		// and never a missing scope. That is M1-011's first fence, which this
		// sweep is not the test for — servicetoken_test.go is.
		caller.token = server.createToken(t, caller.session, authz.TokenScopes()...).Token
	}

	server.targetUser = server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "sweep-target@example.test"
		in.DisplayName = "the account the sweep edits"
		in.PlatformRole = authz.PlatformRoleMember
	})
	return server
}

// sweepDoc is api/openapi.yaml with the six fixture endpoints merged in.
//
// Merged rather than replaced: the callers below sign in and mint tokens through
// the real endpoints, so the real document has to be there — and a fixture that
// replaced it would be testing a server nobody runs.
func sweepDoc(t *testing.T) *openapi3.T {
	t.Helper()

	doc, err := api.Load()
	if err != nil {
		t.Fatalf("loading the API specification: %v", err)
	}
	for path, item := range fixtureSpec(t, sweepSpec).Paths.Map() {
		if doc.Paths.Value(path) != nil {
			t.Fatalf("the real specification already describes %s, so the fixture would be shadowing a real "+
				"endpoint. Point the sweep at the real one instead", path)
		}
		doc.Paths.Set(path, item)
	}
	return doc
}

// target turns a route into the URL a request is sent to.
func (s *sweepServer) target(route string) string {
	return BasePath + strings.NewReplacer(
		"{engagementId}", sweepEngagement,
		"{scenarioId}", sweepScenario,
		"{stepId}", sweepStep,
		"{executionId}", sweepExecution,
		"{userId}", s.targetUser.ID,
		"{sourceId}", storecontent.SourceIDCustom,
	).Replace(route)
}

// asSession performs one request as a signed-in browser: the session cookie, and
// the CSRF cookie and header that go with it on a state-changing method.
func (s *sweepServer) asSession(caller *sweepCaller, op sweepOp, target string) *httptest.ResponseRecorder {
	body := strings.ReplaceAll(op.Body, "{targetUserID}", s.targetUser.ID)
	request := httptest.NewRequest(op.Method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", jsonMediaType)
	request.AddCookie(caller.session)
	s.attachCSRF(request, caller.session)
	return do(s.handler, request)
}

// asToken performs the same request as automation: the bearer token, no cookie,
// and no CSRF header — a token-authenticated request is not subject to that
// check, because nothing attaches one on a caller's behalf.
func (s *sweepServer) asToken(caller *sweepCaller, op sweepOp, target string) *httptest.ResponseRecorder {
	body := strings.ReplaceAll(op.Body, "{targetUserID}", s.targetUser.ID)
	request := httptest.NewRequest(op.Method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", jsonMediaType)
	request.Header.Set("Authorization", "Bearer "+caller.token)
	return do(s.handler, request)
}
