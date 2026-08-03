package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bryanster/blacklight/api"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Service tokens through the real chain and a real temporary DuckDB (M1-011).
//
// PLAN.md §4 on v1: "API keys authenticate nothing." Every test in this file is
// about the thing that sentence describes — a credential that is checked, on
// every route, against fences that narrow when the person behind it does.

const (
	tokensPath   = BasePath + "/auth/tokens"
	settingsPath = BasePath + "/settings/mfa"
)

// createToken mints a token through the API, as a person with a browser would,
// and fails the test if it could not.
func (s *authServer) createToken(t *testing.T, sess *http.Cookie, scopes ...authz.TokenScope) gen.CreatedServiceToken {
	t.Helper()

	recorder := s.post(tokensPath, createTokenBody(t, scopes...), sess)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("POST /auth/tokens = %d, want 201\nbody: %s", recorder.Code, recorder.Body)
	}
	return decodeJSON[gen.CreatedServiceToken](t, recorder)
}

// createTokenBody is a valid creation request for the given scopes.
func createTokenBody(t *testing.T, scopes ...authz.TokenScope) string {
	t.Helper()

	words := make([]string, len(scopes))
	for i, scope := range scopes {
		words[i] = string(scope)
	}
	body, err := json.Marshal(map[string]any{
		"name":      "a token for a test",
		"scopes":    words,
		"expiresAt": time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("building the creation body: %v", err)
	}
	return string(body)
}

// withToken performs a request authenticated by a service token and nothing
// else: no session cookie, and no CSRF header — which is half of what is under
// test, because a token-authenticated request is not subject to that check.
func (s *authServer) withToken(method, target, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(""))
	request.Header.Set("Authorization", "Bearer "+token)
	return do(s.handler, request)
}

// setPlatformRole and setStatus reach past the API to change an account, which
// is what an administrator will do through M1-016 and what nothing can do today.
// The point of both is what happens to a token that already exists.
func (s *authServer) setPlatformRole(t *testing.T, userID string, role authz.PlatformRole) {
	t.Helper()
	s.execSQL(t, `UPDATE app."user" SET platform_role = ? WHERE id = ?`, string(role), userID)
}

func (s *authServer) setStatus(t *testing.T, userID string, status identity.Status) {
	t.Helper()
	s.execSQL(t, `UPDATE app."user" SET status = ? WHERE id = ?`, string(status), userID)
}

// --- The regression case ------------------------------------------------------

// TestM1011NoProtectedRouteIsReachableWithoutACredential is the case this ticket
// exists for, named after it so that deleting it is a deliberate act.
//
// v1's API keys authenticated nothing, and the shape of that failure is not "one
// endpoint was wrong" — it is that nobody could enumerate which endpoints were
// checked at all. So this enumerates them, from the router the server actually
// serves, and holds every route the specification does not declare public to
// answering 401 with no credential and with an invented one.
//
// It walks the router rather than a list, so an endpoint added in M2 is covered
// by this test on the day it is added.
func TestM1011NoProtectedRouteIsReachableWithoutACredential(t *testing.T) {
	t.Parallel()

	// The throttle is turned down out of the way: this walk makes three failed
	// credential attempts per route, and what it is about is which routes refuse
	// an unauthenticated caller — not what happens to somebody who asks too
	// often, which is TestGuessingATokenIsThrottled's subject.
	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = 10_000
		cfg.Throttle.SourceFailures = 10_000
	})
	server.seedUser(t)

	doc, err := api.Load()
	if err != nil {
		t.Fatalf("loading the specification: %v", err)
	}
	requirements, err := api.Requirements(doc)
	if err != nil {
		t.Fatalf("reading the authorization requirements: %v", err)
	}

	// Keyed the way the walk below names a route, so a public operation can be
	// recognised without re-deriving the path.
	public := map[string]bool{}
	for _, requirement := range requirements {
		if requirement.Public {
			public[strings.ToUpper(requirement.Method)+" "+BasePath+requirement.Path] = true
		}
	}

	routes, ok := server.handler.(chi.Routes)
	if !ok {
		t.Fatalf("the server is a %T, which cannot be walked; this test has to be rewritten rather than deleted",
			server.handler)
	}

	checked := 0
	walkErr := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.TrimSuffix(route, "/")
		if !strings.HasPrefix(route, BasePath) {
			return nil // The SPA's catch-all, which is not an API route.
		}
		key := method + " " + route
		if public[key] {
			return nil
		}
		checked++

		// The body comes from csrfCoverage where the route takes one, so that a
		// refusal here is authentication's and not the request validator's.
		body, mediaType := csrfCoverage[key].body, mediaTypeOf(csrfCoverage[key].mediaType)

		// A prefix per route, so that the failures are spread the way an
		// attacker's would not be — the account counter is exercised by its own
		// test, and one shared prefix here would lock the walk out halfway
		// through and report that as a passing route.
		invented := fmt.Sprintf("Bearer bl_%010d_%s", checked, strings.Repeat("B", 52))

		for name, authorization := range map[string]string{
			"no credential at all":  "",
			"an invented token":     invented,
			"a token-shaped string": "Bearer not-a-token-at-all",
			"an empty bearer":       "Bearer ",
		} {
			request := httptest.NewRequest(method, route, strings.NewReader(body))
			request.Header.Set("Content-Type", mediaType)
			if authorization != "" {
				request.Header.Set("Authorization", authorization)
			}

			recorder := do(server.handler, request)
			if recorder.Code != http.StatusUnauthorized {
				t.Errorf("%s with %s = %d, want 401 — an unauthenticated caller must not reach this handler\nbody: %s",
					key, name, recorder.Code, recorder.Body)
				continue
			}
			if got := decodeProblemCode(t, recorder); got != gen.ProblemCodeUnauthenticated {
				t.Errorf("%s with %s answered code %q, want %q", key, name, got, gen.ProblemCodeUnauthenticated)
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walking the router: %v", walkErr)
	}

	// A router that stopped reporting routes would leave this test passing about
	// nothing, which is the failure mode of every enumeration test.
	if checked == 0 {
		t.Fatal("the walk found no protected routes; this test has stopped checking anything")
	}
	t.Logf("checked %d protected route(s) against four ways of presenting no credential", checked)
}

// --- Authenticating -----------------------------------------------------------

// TestATokenReachesWhatItsScopesAndItsOwnerBothAllow is the happy path, and the
// only test here that ends in a 200. Everything else is about the fences.
func TestATokenReachesWhatItsScopesAndItsOwnerBothAllow(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t) // an administrator.
	sess := server.signIn(t)

	created := server.createToken(t, sess, authz.TokenScopeAdminRead)

	if got := server.withToken(http.MethodGet, settingsPath, created.Token).Code; got != http.StatusOK {
		t.Errorf("GET /settings/mfa with an admin's admin:read token = %d, want 200", got)
	}
}

// TestATokenMissingTheScopeIsForbiddenAndNotUnauthenticated. The distinction is
// the acceptance criterion: a client that gets 401 retries with a new credential
// and a client that gets 403 does not, and answering the second with the first
// sends every integration into a retry loop it cannot win.
func TestATokenMissingTheScopeIsForbiddenAndNotUnauthenticated(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)

	// A real, live token belonging to an administrator — just not one scoped for
	// this. The owner could make this call; the token could not.
	created := server.createToken(t, sess, authz.TokenScopeContentRead)

	recorder := server.withToken(http.MethodGet, settingsPath, created.Token)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("GET /settings/mfa with a content:read token = %d, want 403\nbody: %s", recorder.Code, recorder.Body)
	}
	if got := decodeProblemCode(t, recorder); got != gen.ProblemCodeForbidden {
		t.Errorf("code = %q, want %q", got, gen.ProblemCodeForbidden)
	}
}

// TestATokenCannotExceedItsOwnersLivePermissions is PLAN.md §9's named case, at
// the layer where it has to be true: create the token while its owner is an
// administrator, demote the owner, and the same token stops being an
// administrator's — with no change to the token and nothing to invalidate.
func TestATokenCannotExceedItsOwnersLivePermissions(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)
	created := server.createToken(t, sess, authz.TokenScopeAdminRead)

	if got := server.withToken(http.MethodGet, settingsPath, created.Token).Code; got != http.StatusOK {
		t.Fatalf("the administrator's token = %d before the demotion, want 200 — "+
			"this test proves nothing unless it worked first", got)
	}

	server.setPlatformRole(t, server.userID(t), authz.PlatformRoleMember)

	recorder := server.withToken(http.MethodGet, settingsPath, created.Token)
	if recorder.Code != http.StatusForbidden {
		t.Errorf("the same token after the owner was demoted = %d, want 403\nbody: %s", recorder.Code, recorder.Body)
	}
}

// TestDisablingTheOwnerDisablesTheirTokensImmediately. Disabling an account has
// to end its access now rather than when something happens to expire, and a
// token is that account's access.
func TestDisablingTheOwnerDisablesTheirTokensImmediately(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)
	created := server.createToken(t, sess, authz.TokenScopeAdminRead)

	server.setStatus(t, server.userID(t), identity.StatusDisabled)

	recorder := server.withToken(http.MethodGet, settingsPath, created.Token)
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("a disabled owner's token = %d, want 401\nbody: %s", recorder.Code, recorder.Body)
	}
}

// TestARevokedOrExpiredTokenIsUnauthenticated: both are 401, and neither is
// distinguishable from a token nobody ever held.
func TestARevokedOrExpiredTokenIsUnauthenticated(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)

	revoked := server.createToken(t, sess, authz.TokenScopeAdminRead)
	if got := server.request(http.MethodDelete, tokensPath+"/"+revoked.ServiceToken.Id.String(),
		"", jsonMediaType, sess).Code; got != http.StatusNoContent {
		t.Fatalf("revoking = %d, want 204", got)
	}

	expired := server.createToken(t, sess, authz.TokenScopeAdminRead)
	// Aged past its expiry rather than created expired: creation refuses one in
	// the past, which is the other half of the rule and is tested elsewhere.
	server.execSQL(t, `UPDATE app.service_token SET expires_at = ? WHERE id = ?`,
		time.Now().Add(-time.Hour).UTC(), expired.ServiceToken.Id.String())

	for name, token := range map[string]string{
		"revoked": revoked.Token,
		"expired": expired.Token,
	} {
		recorder := server.withToken(http.MethodGet, settingsPath, token)
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("a %s token = %d, want 401\nbody: %s", name, recorder.Code, recorder.Body)
		}
	}
}

// --- The secret ---------------------------------------------------------------

// TestTheSecretAppearsInExactlyOneResponseEverAndInNoLog is the acceptance
// criterion that a full log capture can be grepped for the token and come back
// empty. It exercises the whole life of one: created, listed, used, revoked.
func TestTheSecretAppearsInExactlyOneResponseEverAndInNoLog(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)

	creation := server.post(tokensPath, createTokenBody(t, authz.TokenScopeAdminRead), sess)
	if creation.Code != http.StatusCreated {
		t.Fatalf("POST /auth/tokens = %d, want 201\nbody: %s", creation.Code, creation.Body)
	}
	created := decodeJSON[gen.CreatedServiceToken](t, creation)

	secret := created.Token
	if secret == "" {
		t.Fatal("the creation response carried no token; this test would pass trivially")
	}
	if !strings.Contains(creation.Body.String(), secret) {
		t.Error("the creation response does not contain the token it returned; this test is not checking what it claims")
	}

	// Every other thing that can be done with a token, in order.
	listing := server.get(tokensPath, sess)
	used := server.withToken(http.MethodGet, settingsPath, secret)
	revocation := server.request(http.MethodDelete, tokensPath+"/"+created.ServiceToken.Id.String(),
		"", jsonMediaType, sess)
	after := server.get(tokensPath, sess)

	for name, recorder := range map[string]*httptest.ResponseRecorder{
		"the listing":            listing,
		"a request made with it": used,
		"the revocation":         revocation,
		"the listing after it":   after,
		"the profile":            server.get(mePath, sess),
	} {
		if strings.Contains(recorder.Body.String(), secret) {
			t.Errorf("%s carried the token's secret:\n%s", name, recorder.Body)
		}
	}

	// The log, in full, at debug — which is more than a deployment records and
	// is the point: the secret must not be reachable even by turning the volume
	// up. It is a distinct type whose every rendering is "[redacted]", so this
	// asserts a property of the code rather than the diligence of its callers.
	if logs := server.logs.String(); strings.Contains(logs, secret) {
		t.Errorf("the server log contains the token's secret:\n%s", logs)
	}
	// The secret half alone, in case something logged the parts separately.
	if parts := strings.Split(secret, "_"); len(parts) == 3 {
		if logs := server.logs.String(); strings.Contains(logs, parts[2]) {
			t.Error("the server log contains the secret half of the token")
		}
	}
}

// TestTheLifecycleIsRecorded: creation, use and revocation each leave a line an
// operator can find. M1-015 gives these a durable home in the activity log; this
// is what says they are being reported at all in the meantime.
func TestTheLifecycleIsRecorded(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)

	created := server.createToken(t, sess, authz.TokenScopeAdminRead)
	if got := server.withToken(http.MethodGet, settingsPath, created.Token).Code; got != http.StatusOK {
		t.Fatalf("using the token = %d, want 200", got)
	}
	if got := server.request(http.MethodDelete, tokensPath+"/"+created.ServiceToken.Id.String(),
		"", jsonMediaType, sess).Code; got != http.StatusNoContent {
		t.Fatalf("revoking = %d, want 204", got)
	}

	for _, message := range []string{
		"service token created",
		"service token used for the first time",
		"service token revoked",
	} {
		record := server.logs.find(t, message)
		if got := record["token_id"]; got != created.ServiceToken.Id.String() {
			t.Errorf("%q names token %v, want %s", message, got, created.ServiceToken.Id)
		}
	}
}

// TestAMutatingTokenRequestNeedsNoCSRFToken. The check exists for credentials a
// browser attaches on its own; nothing attaches a bearer token, so requiring one
// here would be a requirement no non-browser client could meet.
//
// A `PUT` rather than a `GET`, because safe methods are exempt anyway and would
// prove nothing about the exemption under test.
func TestAMutatingTokenRequestNeedsNoCSRFToken(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	created := server.createToken(t, server.signIn(t), authz.TokenScopeAdminWrite)

	request := httptest.NewRequest(http.MethodPut, settingsPath,
		strings.NewReader(`{"requiredForAll":true,"requiredForAdmins":true}`))
	request.Header.Set("Content-Type", jsonMediaType)
	request.Header.Set("Authorization", "Bearer "+created.Token)
	// Deliberately no X-CSRF-Token header and no bl_csrf cookie.

	recorder := do(server.handler, request)
	if recorder.Code != http.StatusOK {
		t.Errorf("PUT /settings/mfa with a token and no CSRF token = %d, want 200\nbody: %s",
			recorder.Code, recorder.Body)
	}
}

// --- Managing -----------------------------------------------------------------

// TestATokenCannotMintOrRevokeATokenThroughTheAPI is [authz.GuardSessionOnly]
// where it matters. A leaked token that could mint a longer-lived sibling would
// survive its own revocation, and neither of the other two fences catches that:
// the sibling exceeds neither the owner's role nor the scope list.
func TestATokenCannotMintOrRevokeATokenThroughTheAPI(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)

	// Every scope this build has, so that the refusals below cannot be about a
	// scope somebody forgot to ask for.
	created := server.createToken(t, sess, authz.TokenScopes()...)

	minting := httptest.NewRequest(http.MethodPost, tokensPath,
		strings.NewReader(createTokenBody(t, authz.TokenScopeAdminRead)))
	minting.Header.Set("Content-Type", jsonMediaType)
	minting.Header.Set("Authorization", "Bearer "+created.Token)

	if got := do(server.handler, minting).Code; got != http.StatusForbidden {
		t.Errorf("a fully scoped token creating a token = %d, want 403", got)
	}
	if got := server.withToken(http.MethodGet, tokensPath, created.Token).Code; got != http.StatusForbidden {
		t.Errorf("a fully scoped token listing tokens = %d, want 403", got)
	}
	if got := server.withToken(http.MethodDelete, tokensPath+"/"+created.ServiceToken.Id.String(),
		created.Token).Code; got != http.StatusForbidden {
		t.Errorf("a fully scoped token revoking a token = %d, want 403", got)
	}

	// And the token still works for what it is for, so the refusals above are
	// about the action and not about the credential having stopped working.
	if got := server.withToken(http.MethodGet, settingsPath, created.Token).Code; got != http.StatusOK {
		t.Errorf("the token = %d on an endpoint it holds, want 200", got)
	}
}

// TestListingIsScopedToTheCallerAndCarriesNoSecret. There is no parameter that
// names another account, so the only way to get somebody else's tokens would be
// for the handler to ignore who is asking.
func TestListingIsScopedToTheCallerAndCarriesNoSecret(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	other := server.seedUser(t, func(u *identity.NewUser) {
		u.Email = "somebody-else@example.com"
		u.PlatformRole = authz.PlatformRoleMember
	})

	mine := server.createToken(t, server.signIn(t), authz.TokenScopeAdminRead)
	theirs := server.createToken(t, server.signInAs(t, other.Email), authz.TokenScopeContentRead)

	listed := decodeJSON[gen.ServiceTokens](t, server.get(tokensPath, server.signInAs(t, other.Email)))
	switch {
	case len(listed.Items) != 1:
		t.Fatalf("the other account's listing has %d tokens, want 1 — it is seeing somebody else's", len(listed.Items))
	case listed.Items[0].Id != theirs.ServiceToken.Id:
		t.Errorf("the listing returned %s, want %s", listed.Items[0].Id, theirs.ServiceToken.Id)
	case listed.Items[0].Prefix != theirs.ServiceToken.Prefix:
		t.Errorf("the listing lost the prefix, which is how a person identifies a row")
	}

	// Revoking across accounts is not-found, and indistinguishable from an
	// identifier that names nothing.
	them := server.signInAs(t, other.Email)
	hers := server.request(http.MethodDelete, tokensPath+"/"+mine.ServiceToken.Id.String(), "", jsonMediaType, them)
	invented := server.request(http.MethodDelete,
		tokensPath+"/0192f1a0-0000-7000-8000-0000000000ff", "", jsonMediaType, them)

	if hers.Code != http.StatusNotFound || invented.Code != http.StatusNotFound {
		t.Errorf("revoking somebody else's token = %d and an invented one = %d; both must be 404",
			hers.Code, invented.Code)
	}
	if withoutInstance(t, hers) != withoutInstance(t, invented) {
		t.Errorf("the two 404s differ, which is a way to find out which identifiers are real:\n%s\n%s",
			hers.Body, invented.Body)
	}
}

// TestWhatCreationRefusesThroughTheAPI: the specification catches the shapes and
// internal/authn/servicetoken catches the policy, and a caller should be able to
// tell what is wrong from either.
func TestWhatCreationRefusesThroughTheAPI(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)

	future := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	for name, body := range map[string]string{
		"no scopes":       fmt.Sprintf(`{"name":"x","scopes":[],"expiresAt":%q}`, future),
		"a made-up scope": fmt.Sprintf(`{"name":"x","scopes":["engagements:destroy"],"expiresAt":%q}`, future),
		"no name":         fmt.Sprintf(`{"name":"","scopes":["content:read"],"expiresAt":%q}`, future),
		"no expiry":       `{"name":"x","scopes":["content:read"]}`,
		"an expiry in the past": fmt.Sprintf(`{"name":"x","scopes":["content:read"],"expiresAt":%q}`,
			time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)),
		"an expiry beyond the maximum": fmt.Sprintf(`{"name":"x","scopes":["content:read"],"expiresAt":%q}`,
			time.Now().Add(400*24*time.Hour).UTC().Format(time.RFC3339)),
		"a field nobody declared": fmt.Sprintf(
			`{"name":"x","scopes":["content:read"],"expiresAt":%q,"ownerUserId":"somebody-else"}`, future),
	} {
		recorder := server.post(tokensPath, body, sess)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("creating with %s = %d, want 400\nbody: %s", name, recorder.Code, recorder.Body)
		}
	}
}

// --- Throttling ---------------------------------------------------------------

// TestGuessingATokenIsThrottled. The credential is checked on every route rather
// than on one sign-in endpoint, so an attacker guessing secrets would otherwise
// simply guess them somewhere the throttle was not looking.
func TestGuessingATokenIsThrottled(t *testing.T) {
	t.Parallel()

	// Low thresholds, so the lockout is reached in a handful of requests rather
	// than fifty.
	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = 3
		cfg.Throttle.SourceFailures = 100
	})
	server.seedUser(t)

	// One prefix, guessed repeatedly: the account the failures are counted
	// against. The route is the profile, which is not a sign-in endpoint at all.
	guess := "Bearer bl_AAAAAAAAAA_" + strings.Repeat("B", 52)

	var last int
	for range 5 {
		request := httptest.NewRequest(http.MethodGet, mePath, nil)
		request.Header.Set("Authorization", guess)
		last = do(server.handler, request).Code
	}

	if last != http.StatusTooManyRequests {
		t.Errorf("the fifth guess against one prefix = %d, want 429 — guessing a token is not rationed", last)
	}
}

// TestASuccessfulTokenRequestIsNotAGuess: a valid token making its ordinary
// requests must never accumulate towards a lockout, or a busy integration locks
// itself out.
func TestASuccessfulTokenRequestIsNotAGuess(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = 2
		cfg.Throttle.SourceFailures = 100
	})
	server.seedUser(t)
	created := server.createToken(t, server.signIn(t), authz.TokenScopeAdminRead)

	for i := range 5 {
		if got := server.withToken(http.MethodGet, settingsPath, created.Token).Code; got != http.StatusOK {
			t.Fatalf("request %d with a valid token = %d, want 200", i+1, got)
		}
	}
}

// TestARefusedScopeIsNotCountedAsAGuess is the other half, and the reason the
// throttle reads the authentication step's verdict rather than the response
// status: a token repeatedly refused for want of a scope has authenticated
// perfectly well every time.
func TestARefusedScopeIsNotCountedAsAGuess(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = 2
		cfg.Throttle.SourceFailures = 100
	})
	server.seedUser(t)
	created := server.createToken(t, server.signIn(t), authz.TokenScopeContentRead)

	var last int
	for range 5 {
		last = server.withToken(http.MethodGet, settingsPath, created.Token).Code
	}
	if last != http.StatusForbidden {
		t.Errorf("the fifth scope-refused request = %d, want 403 — a valid token was locked out for being refused", last)
	}
}
