package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Enforced MFA (M1-008), through the real chain and a real temporary DuckDB.
//
// The defect these are written against, from PLAN.md §4: "today MFA=True only
// redirects users who already enrolled, so anyone who skips /mfa/register logs
// in with a password alone." Every test here is about the *order* of two
// questions — policy first, enrolment second — because asking them the other way
// round is what made enforcement optional.

const mfaPolicyPath = BasePath + "/settings/mfa"

// secondEmail is the account the tests that need two people sign the other one
// in with. seedUser's default is testEmail.
const secondEmail = "bob@example.com"

// put sends a JSON body with the session's CSRF token attached, which is what a
// signed-in browser sends.
func (s *authServer) put(target, body string, sess *http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPut, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if sess != nil {
		request.AddCookie(sess)
		s.attachCSRF(request, sess)
	}
	return do(s.handler, request)
}

// setPolicy writes the platform policy as the signed-in administrator holding
// sess, and insists it worked.
//
// Through the endpoint rather than through the store, deliberately: an
// administrator turning enforcement on is the event every test here is about,
// and seeding the row directly would skip the half of it that decides whether
// they were allowed to.
func (s *authServer) setPolicy(t *testing.T, sess *http.Cookie, forAll, forAdmins bool) {
	t.Helper()

	body := fmt.Sprintf(`{"requiredForAll":%t,"requiredForAdmins":%t}`, forAll, forAdmins)
	recorder := s.put(mfaPolicyPath, body, sess)
	if recorder.Code != http.StatusOK {
		t.Fatalf("PUT %s = %d, want 200\nbody: %s", mfaPolicyPath, recorder.Code, recorder.Body)
	}
	stored := decodeJSON[gen.MFAPolicy](t, recorder)
	if stored.RequiredForAll != forAll || stored.RequiredForAdmins != forAdmins {
		t.Fatalf("the stored policy is %+v, want requiredForAll=%t requiredForAdmins=%t",
			stored, forAll, forAdmins)
	}
}

// enforcePolicy turns a requirement on using a throwaway administrator who is
// not the account under test.
//
// The separate account matters: an administrator who turns on a requirement they
// do not satisfy is confined by it themselves at their next request, and a test
// that used one would be measuring two things at once.
func (s *authServer) enforcePolicy(t *testing.T, forAll, forAdmins bool) {
	t.Helper()

	admin := s.seedUser(t, func(u *identity.NewUser) {
		u.Email = "policy-admin@example.com"
		u.PlatformRole = authz.PlatformRoleAdmin
	})
	s.setPolicy(t, s.signInAs(t, admin.Email), forAll, forAdmins)
}

// signInAs signs in an account other than the seeded default and returns its
// session cookie. Every seeded account shares testPassword.
func (s *authServer) signInAs(t *testing.T, email string) *http.Cookie {
	t.Helper()

	recorder := s.login(email, testPassword)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login as %s = %d, want 200\nbody: %s", email, recorder.Code, recorder.Body)
	}
	return sessionCookie(t, recorder)
}

// loginStatus signs in and returns the status and the session cookie, if one was
// set. It is the whole observable outcome of M1-008's state machine.
func (s *authServer) loginStatus(t *testing.T, email string) (gen.LoginStatus, *http.Cookie) {
	t.Helper()

	recorder := s.login(email, testPassword)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}
	result := decodeJSON[gen.LoginResult](t, recorder)

	var sess *http.Cookie
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == session.CookieName && cookie.Value != "" {
			sess = cookie
		}
	}
	return result.Status, sess
}

// --- The regression case ------------------------------------------------------

// TestM1008PolicyOnAndNeverEnrolledReachesNoApplicationEndpoint is the case this
// ticket exists for, named after it so that deleting it is a deliberate act.
//
// v1 answered this with a full session, because it checked enrolment state
// instead of policy: the people who had skipped enrolling were exactly the people
// enforcement stopped applying to. Here the password alone buys a session that
// can enrol and nothing else, and every other endpoint — including the one that
// would let them change their password, and the one that would let them take the
// requirement off — is refused.
func TestM1008PolicyOnAndNeverEnrolledReachesNoApplicationEndpoint(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	server.enforcePolicy(t, true, false)

	status, sess := server.loginStatus(t, testEmail)
	if status != gen.LoginStatusMfaEnrolmentRequired {
		t.Fatalf("status = %q, want %q", status, gen.LoginStatusMfaEnrolmentRequired)
	}
	if sess == nil {
		t.Fatal("no session cookie; a caller with nothing to present must still be able to enrol")
	}

	refused := map[string]*httptest.ResponseRecorder{
		"POST /auth/password":                server.post(passwordPath, changePasswordBody, sess),
		"DELETE /auth/mfa/totp":              server.del(totpPath, `{"currentPassword":"`+testPassword+`"}`, sess),
		"POST /auth/mfa/recovery/regenerate": server.post(recoveryRegeneratePath, `{"currentPassword":"`+testPassword+`"}`, sess),
		"GET /settings/mfa":                  server.get(mfaPolicyPath, sess),
		"PUT /settings/mfa":                  server.put(mfaPolicyPath, `{"requiredForAll":false,"requiredForAdmins":false}`, sess),
	}
	for route, recorder := range refused {
		if recorder.Code != http.StatusForbidden {
			t.Errorf("%s = %d with a password-only session, want 403", route, recorder.Code)
			continue
		}
		if got := decodeProblem(t, recorder).Code; got != gen.ProblemCodeMfaEnrolmentRequired {
			t.Errorf("%s answered code %q, want %q — the interface cannot tell this from an ordinary refusal",
				route, got, gen.ProblemCodeMfaEnrolmentRequired)
		}
	}
}

// --- The state machine --------------------------------------------------------

// TestLoginOutcomesUnderPolicy is M1-008's table, every row, plus the
// individually-enforced variant that the platform policy has nothing to do with.
func TestLoginOutcomesUnderPolicy(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		role      authz.PlatformRole
		enforced  bool
		forAll    bool
		forAdmins bool
		enrol     bool
		want      gen.LoginStatus
	}{
		"nothing required, nothing enrolled": {
			role: authz.PlatformRoleMember,
			want: gen.LoginStatusAuthenticated,
		},
		// Stricter than the ticket's table, and deliberately so: somebody who
		// set up an authenticator meant it to be asked for (M1-006).
		"nothing required, enrolled anyway": {
			role:  authz.PlatformRoleMember,
			enrol: true,
			want:  gen.LoginStatusMfaRequired,
		},
		"required for all, enrolled": {
			role:   authz.PlatformRoleMember,
			forAll: true,
			enrol:  true,
			want:   gen.LoginStatusMfaRequired,
		},
		"required for all, not enrolled": {
			role:   authz.PlatformRoleMember,
			forAll: true,
			want:   gen.LoginStatusMfaEnrolmentRequired,
		},
		"required for admins, and they are one": {
			role:      authz.PlatformRoleAdmin,
			forAdmins: true,
			want:      gen.LoginStatusMfaEnrolmentRequired,
		},
		// The half of requiredForAdmins that would be invisible if only the
		// row above existed: it must not catch everybody.
		"required for admins, and they are not": {
			role:      authz.PlatformRoleMember,
			forAdmins: true,
			want:      gen.LoginStatusAuthenticated,
		},
		"enforced individually with the policy off": {
			role:     authz.PlatformRoleMember,
			enforced: true,
			want:     gen.LoginStatusMfaEnrolmentRequired,
		},
		"enforced individually and enrolled": {
			role:     authz.PlatformRoleMember,
			enforced: true,
			enrol:    true,
			want:     gen.LoginStatusMfaRequired,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := newAuthServer(t)
			server.seedUser(t, func(u *identity.NewUser) {
				u.PlatformRole = test.role
				u.MFAEnforced = test.enforced
			})
			if test.enrol {
				server.enrolAndConfirm(t)
			}
			if test.forAll || test.forAdmins {
				server.enforcePolicy(t, test.forAll, test.forAdmins)
			}

			status, sess := server.loginStatus(t, testEmail)
			if status != test.want {
				t.Fatalf("status = %q, want %q", status, test.want)
			}
			// And the cookie agrees with the status, because a client that read
			// one and not the other would still have to be right.
			if wantSession := status != gen.LoginStatusMfaRequired; (sess != nil) != wantSession {
				t.Errorf("session cookie set = %t, want %t for status %q",
					sess != nil, wantSession, status)
			}
		})
	}
}

// TestAWrongPasswordIsStill401UnderPolicy is the table's first row. It is worth
// its own test because the requirement is evaluated after the password and must
// not become a way to find out that an account exists.
func TestAWrongPasswordIsStill401UnderPolicy(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	server.enforcePolicy(t, true, false)

	recorder := server.login(testEmail, "not the password")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("login with a wrong password = %d, want 401\nbody: %s", recorder.Code, recorder.Body)
	}
	// Identical to what an address nobody holds gets, which is what stops the
	// requirement leaking who has an account here. Compared as documents rather
	// than field by field, so that a field added later is covered too —
	// everything but `instance`, which is the request ID and differs by
	// construction.
	wrongPassword := decodeJSON[map[string]any](t, recorder)
	unknownAddress := decodeJSON[map[string]any](t,
		server.login("nobody@example.com", "not the password"))
	delete(wrongPassword, "instance")
	delete(unknownAddress, "instance")
	if !reflect.DeepEqual(wrongPassword, unknownAddress) {
		t.Errorf("a wrong password answered %v and an unknown address answered %v",
			wrongPassword, unknownAddress)
	}
}

// --- Sessions that already existed --------------------------------------------

// TestTurningThePolicyOnReachesSessionsAlreadyOpen is the acceptance criterion
// about grandfathering. The requirement is evaluated on every request rather
// than recorded on the session, so a session issued a moment before the policy
// is subject to it a moment after.
func TestTurningThePolicyOnReachesSessionsAlreadyOpen(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)

	sess := server.signIn(t)
	if got := server.get(mePath, sess).Code; got != http.StatusOK {
		t.Fatalf("GET /auth/me before the policy = %d, want 200", got)
	}

	server.enforcePolicy(t, true, false)

	// The same cookie, unchanged, on the same endpoint it worked on: /auth/me
	// still answers, because that is what the enrolment screen reads.
	recorder := server.get(mePath, sess)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /auth/me after the policy = %d, want 200 — the enrolment screen needs it", recorder.Code)
	}
	user := decodeJSON[gen.CurrentUser](t, recorder)
	switch {
	case !user.Mfa.Required:
		t.Error("mfa.required = false after the policy was turned on")
	case user.Mfa.Enforced:
		t.Error("mfa.enforced = true, but nobody set the per-user flag; the two facts have been conflated")
	case user.Mfa.Enrolled:
		t.Error("mfa.enrolled = true for somebody who never enrolled")
	}

	// And everything else is refused, on the session they were already holding.
	refusal := server.post(passwordPath, changePasswordBody, sess)
	if refusal.Code != http.StatusForbidden {
		t.Fatalf("POST /auth/password after the policy = %d, want 403", refusal.Code)
	}
	if got := decodeProblem(t, refusal).Code; got != gen.ProblemCodeMfaEnrolmentRequired {
		t.Errorf("code = %q, want %q", got, gen.ProblemCodeMfaEnrolmentRequired)
	}
}

// TestAnEnrolledSessionThatNeverPresentedAFactorIsSignedOut is the other half of
// the same moment, and the one that would be a dead end if it were confined to
// enrolment: they have an authenticator, so the enrolment endpoint would refuse
// them with a 409 and there would be no way forward at all.
//
// Reachable in one step — sign in on two browsers, enrol in one — so the answer
// has to be somewhere, and 401 is the one that leads back through a sign-in that
// asks for the factor they hold.
func TestAnEnrolledSessionThatNeverPresentedAFactorIsSignedOut(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)

	// The first browser: signed in with a password, before anything was
	// enrolled.
	stale := server.signIn(t)
	// The second: enrols, which is what makes the first one's session one that
	// never presented a factor.
	server.enrolAndConfirm(t)

	server.enforcePolicy(t, true, false)

	recorder := server.get(mePath, stale)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("GET /auth/me on the stale session = %d, want 401\nbody: %s", recorder.Code, recorder.Body)
	}
	if got := decodeProblem(t, recorder).Code; got != gen.ProblemCodeUnauthenticated {
		t.Errorf("code = %q, want %q", got, gen.ProblemCodeUnauthenticated)
	}

	// The session row is left alone rather than revoked: a policy turned on and
	// off again leaves the sessions it interrupted usable.
	sessions := server.sessions(t)
	for _, s := range sessions {
		if !s.RevokedAt.IsZero() {
			t.Errorf("session %s was revoked; the policy is evaluated per request and revokes nothing", s.ID)
		}
	}
}

// --- The way out --------------------------------------------------------------

// TestConfirmingAnEnrolmentEndsTheConfinementInOneStep is the acceptance
// criterion that there is no second sign-in: the session that was confined is
// rotated onto a new token and is fully privileged on the response to the
// confirmation itself.
func TestConfirmingAnEnrolmentEndsTheConfinementInOneStep(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	server.enforcePolicy(t, true, false)

	status, confined := server.loginStatus(t, testEmail)
	if status != gen.LoginStatusMfaEnrolmentRequired {
		t.Fatalf("status = %q, want %q", status, gen.LoginStatusMfaEnrolmentRequired)
	}

	secret := server.enrol(t, confined)
	recorder := server.post(confirmPath, codeBody(codeFor(t, secret, time.Now())), confined)
	if recorder.Code != http.StatusOK {
		t.Fatalf("confirm = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}

	// The token changed — satisfying a factor is a privilege change (M1-003) —
	// and the old one is gone with it.
	rotated := sessionCookie(t, recorder)
	if rotated.Value == confined.Value {
		t.Error("the session token did not change; a privilege change must rotate it")
	}
	if got := server.get(mePath, confined).Code; got != http.StatusUnauthorized {
		t.Errorf("the pre-confirmation cookie still answers %d on /auth/me, want 401", got)
	}

	// And the new one is an ordinary session: the endpoint that was refused a
	// moment ago is not.
	if got := server.post(passwordPath, changePasswordBody, rotated).Code; got != http.StatusNoContent {
		t.Errorf("POST /auth/password after confirming = %d, want 204 — the confinement outlived the enrolment", got)
	}
}

// TestAConfinedSessionCanOnlyReachTheEnrolmentRoutes walks the router and holds
// every route to the allowlist, so that an endpoint added later is refused
// unless somebody writes down why it should not be.
func TestAConfinedSessionCanOnlyReachTheEnrolmentRoutes(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	server.enforcePolicy(t, true, false)

	routes, ok := server.handler.(chi.Routes)
	if !ok {
		t.Fatalf("the server is a %T, which cannot be walked; this test has to be rewritten rather than deleted", server.handler)
	}

	seen := map[string]bool{}
	err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.TrimSuffix(route, "/")
		if !strings.HasPrefix(route, BasePath) {
			// The SPA's catch-all. It is not an API route and the gate does not
			// sit in front of it.
			return nil
		}
		key := method + " " + route
		seen[key] = true

		// A fresh confined session per route, because one of the routes under
		// test is logout and a shared session would be revoked halfway through
		// the walk — leaving every route after it answering 401 and this test
		// passing for the wrong reason.
		status, confined := server.loginStatus(t, testEmail)
		if status != gen.LoginStatusMfaEnrolmentRequired {
			t.Fatalf("status = %q, want %q", status, gen.LoginStatusMfaEnrolmentRequired)
		}

		// The body has to be one the specification accepts: request validation
		// runs before the gate, so a malformed body would be a 400 and would
		// say nothing about whether the route is gated. The bodies come from
		// csrfCoverage, which already holds one per mutating route — a second
		// Substitute path parameters with valid UUIDs for request validation.
		url := strings.NewReplacer(
			"{engagementId}", "0192f1a0-0000-7000-8000-00000000e001",
			"{scenarioId}", "0192f1a0-0000-7000-8000-00000000e003",
			"{stepId}", "0192f1a0-0000-7000-8000-00000000e004",
		).Replace(route)
		recorder := server.request(method, url,
			csrfCoverage[key].body, mediaTypeOf(csrfCoverage[key].mediaType), confined)
		_, allowed := enrolmentOnlyRoutes[key]

		refusedByTheGate := recorder.Code == http.StatusForbidden &&
			decodeProblemCode(t, recorder) == gen.ProblemCodeMfaEnrolmentRequired
		switch {
		case allowed && refusedByTheGate:
			t.Errorf("%s is in enrolmentOnlyRoutes and was refused by the gate anyway", key)
		case !allowed && !refusedByTheGate && recorder.Code == http.StatusBadRequest:
			// The request validator rejected the request before the MFA
			// gate ran — typically a missing required query parameter
			// (e.g. baseline on /analytics/compare). The route is
			// effectively blocked from confined sessions because a valid
			// request would be refused by the gate, and an invalid one
			// never reaches it.
		case !allowed && !refusedByTheGate:
			t.Errorf("%s answered %d to a session confined to enrolment, want a 403 with %q — "+
				"either it is behind the gate, or it belongs in enrolmentOnlyRoutes with a reason",
				key, recorder.Code, gen.ProblemCodeMfaEnrolmentRequired)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the router: %v", err)
	}

	for key := range enrolmentOnlyRoutes {
		if !seen[key] {
			t.Errorf("enrolmentOnlyRoutes lists %s, which the router does not serve; the list has gone stale", key)
		}
	}
}

// request sends method to target with a session cookie and, for the methods that
// need one, its CSRF token. It exists for the route walk above, which has to
// issue every method the router serves.
func (s *authServer) request(method, target, body, mediaType string, sess *http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", mediaType)
	if sess != nil {
		request.AddCookie(sess)
		s.attachCSRF(request, sess)
	}
	return do(s.handler, request)
}

// decodeProblemCode reads the code out of a response that may not be a problem
// document at all, which is what the route walk needs: a route that answered 200
// has no code, and that is an answer rather than a failure.
func decodeProblemCode(t *testing.T, recorder *httptest.ResponseRecorder) gen.ProblemCode {
	t.Helper()

	if recorder.Header().Get("Content-Type") != "application/problem+json" {
		return ""
	}
	return decodeProblem(t, recorder).Code
}

// --- What enforcement must not do ---------------------------------------------

// TestTurningThePolicyOffKeepsEveryEnrolment: enforcement is a rule about
// signing in, not a lifecycle for authenticators. Deleting them when the rule is
// relaxed would mean turning it back on made everybody enrol again — and would
// silently take away the recovery codes that are somebody's way back in.
func TestTurningThePolicyOffKeepsEveryEnrolment(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	secret := server.enrolAndConfirm(t)
	server.enforcePolicy(t, true, false)

	// The administrator turning it off is the seeded account itself, which
	// satisfies the requirement by presenting the factor — itself the
	// enforcement working.
	sess := server.signInWith(t, secret)
	server.setPolicy(t, sess, false, false)

	// The authenticator is still there, and still asked for: a confirmed factor
	// gates a sign-in whatever the policy says.
	status, _ := server.loginStatus(t, testEmail)
	if status != gen.LoginStatusMfaRequired {
		t.Fatalf("status = %q after the policy was turned off, want %q — the enrolment was destroyed",
			status, gen.LoginStatusMfaRequired)
	}
	user := decodeJSON[gen.CurrentUser](t, server.get(mePath, sess))
	switch {
	case !user.Mfa.Enrolled:
		t.Error("mfa.enrolled = false; turning the policy off deleted the enrolment")
	case user.Mfa.Required:
		t.Error("mfa.required = true although the policy is off and no flag is set")
	case user.Mfa.RecoveryCodesRemaining != 10:
		t.Errorf("recoveryCodesRemaining = %d, want 10; the codes went with the policy",
			user.Mfa.RecoveryCodesRemaining)
	}
}

// TestAnAdministratorCannotRemoveTheirOwnFactorWhileItIsRequired is the
// criterion about not leaving somebody enforced-but-unenrolled. The endpoint
// refuses rather than obliging and stranding them.
func TestAnAdministratorCannotRemoveTheirOwnFactorWhileItIsRequired(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	secret := server.enrolAndConfirm(t)
	server.enforcePolicy(t, false, true) // Administrators only; the seeded user is one.

	sess := server.signInWith(t, secret)
	recorder := server.del(totpPath, `{"currentPassword":"`+testPassword+`"}`, sess)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("DELETE /auth/mfa/totp = %d, want 403\nbody: %s", recorder.Code, recorder.Body)
	}
	// forbidden and not mfa_enrolment_required: they are enrolled, and what they
	// may not do is stop being.
	if got := decodeProblem(t, recorder).Code; got != gen.ProblemCodeForbidden {
		t.Errorf("code = %q, want %q", got, gen.ProblemCodeForbidden)
	}

	// And the authenticator is still there, which is the point of refusing.
	user := decodeJSON[gen.CurrentUser](t, server.get(mePath, sess))
	if !user.Mfa.Enrolled {
		t.Error("mfa.enrolled = false; the refusal did not prevent the removal")
	}
}

// --- The policy endpoints ------------------------------------------------------

// TestThePolicyIsOffOnAFreshDeployment: absence is the default, and it has to be
// this one. A fresh installation whose first administrator is confined to
// enrolling before they have seen the product is an installation nobody
// finishes.
func TestThePolicyIsOffOnAFreshDeployment(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)

	policy := decodeJSON[gen.MFAPolicy](t, server.get(mfaPolicyPath, sess))
	if policy.RequiredForAll || policy.RequiredForAdmins {
		t.Errorf("a database nobody has configured reports %+v, want both false", policy)
	}
	user := decodeJSON[gen.CurrentUser](t, server.get(mePath, sess))
	if user.Mfa.Required {
		t.Error("mfa.required = true on a deployment with no policy and no flag")
	}
}

// TestOnlyAnAdministratorReadsOrChangesThePolicy is v1's /manage/access, which
// shipped without an admin gate and let any user make themselves Admin. The
// check lives in the service until M1-013 moves every authorization decision
// into one middleware; either way it must not be absent.
func TestOnlyAnAdministratorReadsOrChangesThePolicy(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t, func(u *identity.NewUser) { u.PlatformRole = authz.PlatformRoleMember })
	sess := server.signIn(t)

	body := `{"requiredForAll":true,"requiredForAdmins":true}`
	for name, recorder := range map[string]*httptest.ResponseRecorder{
		"GET": server.get(mfaPolicyPath, sess),
		"PUT": server.put(mfaPolicyPath, body, sess),
	} {
		if recorder.Code != http.StatusForbidden {
			t.Errorf("%s %s as a member = %d, want 403\nbody: %s",
				name, mfaPolicyPath, recorder.Code, recorder.Body)
			continue
		}
		if got := decodeProblem(t, recorder).Code; got != gen.ProblemCodeForbidden {
			t.Errorf("%s answered code %q, want %q", name, got, gen.ProblemCodeForbidden)
		}
	}

	// And nothing was written: a refusal that half-applied would be worse than
	// one that did not refuse at all.
	admin := server.seedUser(t, func(u *identity.NewUser) {
		u.Email = secondEmail
		u.PlatformRole = authz.PlatformRoleAdmin
	})
	adminSession := server.signInAs(t, admin.Email)
	policy := decodeJSON[gen.MFAPolicy](t, server.get(mfaPolicyPath, adminSession))
	if policy.RequiredForAll || policy.RequiredForAdmins {
		t.Errorf("the policy is %+v after a refused write, want both false", policy)
	}
}

// TestThePolicyEndpointNeedsASession is the other half: unauthenticated is a 401
// rather than a 403, so a client knows to sign in rather than to give up.
func TestThePolicyEndpointNeedsASession(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)

	if got := server.get(mfaPolicyPath).Code; got != http.StatusUnauthorized {
		t.Errorf("GET %s with no session = %d, want 401", mfaPolicyPath, got)
	}
}
