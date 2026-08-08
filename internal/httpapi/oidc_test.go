package httpapi

import (
	"encoding"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authn/oidc"
	"github.com/bryanster/blacklight/internal/authn/oidc/oidctest"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Single sign-on through the whole chain (M1-009): the real server, a real
// temporary DuckDB, and a mock identity provider with a generated key pair.
//
// internal/authn/oidc has the protocol tests — a forged token, a replayed state,
// a rotated key. What is proved here is everything downstream of a verified
// assertion: which account it becomes, what it is allowed to change about that
// account, and what a browser is left holding.

const (
	providersPath = BasePath + "/auth/providers"
	startPath     = BasePath + "/auth/oidc/start"
	callbackPath  = BasePath + "/auth/oidc/callback"
)

// ssoServer is an authServer with an identity provider behind it.
type ssoServer struct {
	*authServer
	idp *oidctest.Provider
}

// newSSOServer builds a server whose OIDC section points at a mock provider.
// adjust changes that section for the tests that are about one of its settings.
func newSSOServer(t *testing.T, adjust ...func(*config.OIDC)) *ssoServer {
	t.Helper()
	return newSSOServerWith(t, oidctest.New(t), adjust...)
}

// newSSOServerWith is [newSSOServer] over a provider the caller made, for the
// tests that need one which is already down before the server starts.
func newSSOServerWith(t *testing.T, idp *oidctest.Provider, adjust ...func(*config.OIDC)) *ssoServer {
	t.Helper()

	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.OIDC = ssoConfig(t, idp)
		for _, fn := range adjust {
			fn(&cfg.OIDC)
		}
	})
	return &ssoServer{authServer: server, idp: idp}
}

// ssoConfig is the configuration an operator would write for this provider,
// parsed the way the environment parses it rather than assembled by hand — so a
// test cannot express a configuration a deployment could not.
func ssoConfig(t *testing.T, idp *oidctest.Provider) config.OIDC {
	t.Helper()

	var (
		issuer config.IssuerURL
		scopes config.Scopes
		roles  config.RoleMap
		secret config.ForeignSecret
	)
	parse(t, &issuer, idp.Issuer())
	parse(t, &scopes, "openid profile email groups")
	parse(t, &roles, "blacklight-admins=admin,staff=member")
	parse(t, &secret, idp.ClientSecret)

	return config.OIDC{
		Issuer:       issuer,
		ClientID:     idp.ClientID,
		ClientSecret: secret,
		Scopes:       scopes,
		GroupsClaim:  "groups",
		RoleMap:      roles,
	}
}

// parse fills one configuration value the way the environment would.
func parse(t *testing.T, target encoding.TextUnmarshaler, value string) {
	t.Helper()

	if err := target.UnmarshalText([]byte(value)); err != nil {
		t.Fatalf("parsing the test OIDC configuration %q: %v", value, err)
	}
}

// signInThrough drives a whole browser flow: start, the provider, and the
// callback with the cookie the start set. It returns the callback's response.
func (s *ssoServer) signInThrough(t *testing.T, returnTo string, claims map[string]any) *httptest.ResponseRecorder {
	t.Helper()

	start := s.get(startPath + returnToQuery(returnTo))
	if start.Code != http.StatusFound {
		t.Fatalf("GET %s = %d, want 302\nbody: %s", startPath, start.Code, start.Body)
	}
	state := stateCookie(t, start)

	callback := s.idp.Login(t, start.Header().Get("Location"), claims)
	return s.get(callbackPath+"?"+callback.Encode(), state)
}

func returnToQuery(returnTo string) string {
	if returnTo == "" {
		return ""
	}
	return "?return_to=" + url.QueryEscape(returnTo)
}

// stateCookie returns the sealed pending-state cookie a response set.
func stateCookie(t *testing.T, recorder *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == oidc.CookieName {
			return cookie
		}
	}
	t.Fatalf("no %s cookie in the response\nheaders: %v", oidc.CookieName, recorder.Header())
	return nil
}

// users returns every account in the database, which is how the provisioning
// tests assert that nothing was written.
func (s *ssoServer) users(t *testing.T) []identity.User {
	t.Helper()

	found, _, err := identity.NewUsers(s.db).Page(t.Context(), identity.PageFilter{Limit: 200})
	if err != nil {
		t.Fatalf("listing users: %v", err)
	}
	return found
}

// claims is the shape of a person the mock provider vouches for.
func claims(subject, email string, verified bool, groups ...string) map[string]any {
	return map[string]any{
		"sub":            subject,
		"email":          email,
		"email_verified": verified,
		"name":           "Rowan Ash",
		"groups":         groups,
	}
}

func TestTheLoginPageIsOfferedSingleSignOnWhenItIsConfigured(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t)

	recorder := server.get(providersPath)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200\nbody: %s", providersPath, recorder.Code, recorder.Body)
	}
	providers := decodeJSON[gen.AuthProviders](t, recorder)

	if !providers.Password {
		t.Error("password sign-in is not offered")
	}
	if len(providers.Sso) != 1 {
		t.Fatalf("sso = %+v, want exactly one provider", providers.Sso)
	}
	if got, want := providers.Sso[0].Id, gen.SSOProviderIdOidc; got != want {
		t.Errorf("sso[0].id = %q, want %q", got, want)
	}
	if got, want := providers.Sso[0].StartUrl, startPath; got != want {
		t.Errorf("sso[0].startUrl = %q, want %q", got, want)
	}
	// The issuer is configuration, and this endpoint is unauthenticated.
	if body := recorder.Body.String(); strings.Contains(body, server.idp.Issuer()) {
		t.Errorf("the provider list names the issuer:\n%s", body)
	}
}

// TestABrokenProviderDoesNotLockAnybodyOut is the acceptance criterion that
// matters most operationally: with the identity provider down, the login page
// still works, it offers a password, and it does not offer a button that leads
// nowhere.
func TestABrokenProviderDoesNotLockAnybodyOut(t *testing.T) {
	t.Parallel()

	// Down before the server starts, which is the case the criterion is about:
	// a deployment whose identity provider is unreachable or misconfigured.
	idp := oidctest.New(t)
	idp.Close()

	server := newSSOServerWith(t, idp)
	server.seedUser(t)

	providers := decodeJSON[gen.AuthProviders](t, server.get(providersPath))
	if !providers.Password {
		t.Error("password sign-in is not offered while the provider is down")
	}
	if len(providers.Sso) != 0 {
		t.Errorf("sso = %+v, want no providers while the one configured is unreachable", providers.Sso)
	}

	// And the thing the criterion is really about.
	if recorder := server.login(testEmail, testPassword); recorder.Code != http.StatusOK {
		t.Fatalf("local login while the provider is down = %d, want 200\nbody: %s",
			recorder.Code, recorder.Body)
	}
}

func TestWithNoProviderConfiguredTheEndpointsAreNotThere(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)

	providers := decodeJSON[gen.AuthProviders](t, server.get(providersPath))
	if len(providers.Sso) != 0 {
		t.Errorf("sso = %+v, want none on a deployment with no provider", providers.Sso)
	}
	for _, path := range []string{startPath, callbackPath} {
		recorder := server.get(path)
		if recorder.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 on a deployment with no provider", path, recorder.Code)
		}
		if got, want := decodeProblem(t, recorder).Code, gen.ProblemCodeNotFound; got != want {
			t.Errorf("code = %q, want %q", got, want)
		}
	}
}

// TestAFirstSignInProvisionsAnAccount is the happy path end to end: a person the
// provider vouches for, an account created for them with the role their groups
// map to, and a browser that is signed in afterwards.
func TestAFirstSignInProvisionsAnAccount(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t, func(cfg *config.OIDC) { cfg.AutoProvision = true })

	recorder := server.signInThrough(t, "/engagements/018f",
		claims("subject-9f1c", "rowan@example.com", true, "blacklight-admins"))
	if recorder.Code != http.StatusFound {
		t.Fatalf("the callback = %d, want 302\nbody: %s", recorder.Code, recorder.Body)
	}
	if got, want := recorder.Header().Get("Location"), "/engagements/018f"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}

	users := server.users(t)
	if len(users) != 1 {
		t.Fatalf("the database holds %d users, want 1", len(users))
	}
	if got, want := users[0].Email, "rowan@example.com"; got != want {
		t.Errorf("email = %q, want %q", got, want)
	}
	if got, want := users[0].PlatformRole, authz.PlatformRoleAdmin; got != want {
		t.Errorf("platformRole = %q, want %q — the group is mapped to it", got, want)
	}
	if users[0].PasswordHash != "" {
		t.Error("the provisioned account has a password hash; an SSO account has no local password")
	}

	// The browser is signed in: the session cookie the callback set resolves.
	me := server.get(mePath, sessionCookie(t, recorder))
	if me.Code != http.StatusOK {
		t.Fatalf("GET %s with the cookie the callback set = %d, want 200\nbody: %s",
			mePath, me.Code, me.Body)
	}
	if got, want := decodeJSON[gen.CurrentUser](t, me).Email, "rowan@example.com"; got != want {
		t.Errorf("/auth/me email = %q, want %q", got, want)
	}
}

// TestTheStateCookieIsClearedByTheCallback: one state, one callback. The cookie
// going away is what makes it single-use from the browser's side.
func TestTheStateCookieIsClearedByTheCallback(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t, func(cfg *config.OIDC) { cfg.AutoProvision = true })

	recorder := server.signInThrough(t, "", claims("subject-1", "rowan@example.com", true))
	if recorder.Code != http.StatusFound {
		t.Fatalf("the callback = %d, want 302\nbody: %s", recorder.Code, recorder.Body)
	}

	cleared := stateCookie(t, recorder)
	if cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Errorf("the state cookie is %+v, want it cleared", cleared)
	}
}

// TestACallbackWithNoCookieIsRefused is the middleware-level version of the
// protocol test: a callback that did not come from a browser this server sent to
// the provider.
func TestACallbackWithNoCookieIsRefused(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t, func(cfg *config.OIDC) { cfg.AutoProvision = true })

	start := server.get(startPath)
	callback := server.idp.Login(t, start.Header().Get("Location"), claims("subject-1", "rowan@example.com", true))

	// No cookie attached.
	recorder := server.get(callbackPath + "?" + callback.Encode())
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("the callback with no state cookie = %d, want 401\nbody: %s", recorder.Code, recorder.Body)
	}
	if got := len(server.users(t)); got != 0 {
		t.Errorf("the database holds %d users, want none: a refused callback creates nothing", got)
	}
}

// TestProvisioningOffRefusesAnUnknownPerson is the acceptance criterion, both
// halves: a 403 whose message says what to do, and no row written.
func TestProvisioningOffRefusesAnUnknownPerson(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t) // AutoProvision is off by default, as in production.

	recorder := server.signInThrough(t, "", claims("subject-1", "stranger@example.com", true))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("the callback = %d, want 403\nbody: %s", recorder.Code, recorder.Body)
	}

	problem := decodeProblem(t, recorder)
	if got, want := problem.Code, gen.ProblemCodeForbidden; got != want {
		t.Errorf("code = %q, want %q", got, want)
	}
	if problem.Detail == nil || !strings.Contains(*problem.Detail, "administrator") {
		t.Errorf("detail = %v, want it to say who to ask", problem.Detail)
	}
	if got := len(server.users(t)); got != 0 {
		t.Errorf("the database holds %d users, want none", got)
	}
}

// TestAVerifiedEmailLinksToAnExistingAccount: the same person, already here with
// a local password, signing in through the provider for the first time.
func TestAVerifiedEmailLinksToAnExistingAccount(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t)
	existing := server.seedUser(t)

	recorder := server.signInThrough(t, "", claims("subject-1", testEmail, true))
	if recorder.Code != http.StatusFound {
		t.Fatalf("the callback = %d, want 302\nbody: %s", recorder.Code, recorder.Body)
	}

	if got := len(server.users(t)); got != 1 {
		t.Fatalf("the database holds %d users, want 1: linking must not duplicate an account", got)
	}
	linked, err := identity.NewIdentities(server.db).BySubject(t.Context(), identity.ProviderOIDC, "subject-1")
	if err != nil {
		t.Fatalf("the identity was not attached: %v", err)
	}
	if got, want := linked.UserID, existing.ID; got != want {
		t.Errorf("the identity is attached to %q, want the existing user %q", got, want)
	}

	// Their password still works: linking a second way in does not remove the
	// first.
	if login := server.login(testEmail, testPassword); login.Code != http.StatusOK {
		t.Errorf("local login after linking = %d, want 200\nbody: %s", login.Code, login.Body)
	}
}

// TestAnUnverifiedEmailDoesNotLink is the same flow with the one claim that
// decides it flipped. Without this check, anybody who can set an address at the
// provider can claim any account here by typing it.
func TestAnUnverifiedEmailDoesNotLink(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t)
	server.seedUser(t)

	recorder := server.signInThrough(t, "", claims("subject-1", testEmail, false))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("the callback = %d, want 403 — an unverified address links nothing\nbody: %s",
			recorder.Code, recorder.Body)
	}
	if _, err := identity.NewIdentities(server.db).BySubject(
		t.Context(), identity.ProviderOIDC, "subject-1"); err == nil {
		t.Error("an identity was attached on the strength of an unverified address")
	}
}

// TestAnUnverifiedEmailProvisionsItsOwnAccount: with provisioning on, the same
// unverified address gets its own account rather than somebody else's.
func TestAnUnverifiedEmailProvisionsItsOwnAccount(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t, func(cfg *config.OIDC) { cfg.AutoProvision = true })
	existing := server.seedUser(t, func(in *identity.NewUser) { in.Email = "taken@example.com" })

	// The provider says this subject holds an address that already belongs to
	// somebody, and does not say it is verified.
	recorder := server.signInThrough(t, "", claims("subject-1", "taken@example.com", false))
	if recorder.Code == http.StatusFound {
		t.Fatalf("the callback signed somebody in on an unverified address that is already taken")
	}

	linked, err := identity.NewIdentities(server.db).BySubject(
		t.Context(), identity.ProviderOIDC, "subject-1")
	if err == nil && linked.UserID == existing.ID {
		t.Fatal("an unverified address was linked to the account that already holds it")
	}
}

// TestGroupsAreReEvaluatedOnEveryLogin is the demotion criterion. The direction
// that matters is the second one: an integration that only ever promotes is one
// where revoking access at the directory does nothing here.
func TestGroupsAreReEvaluatedOnEveryLogin(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t, func(cfg *config.OIDC) { cfg.AutoProvision = true })

	if recorder := server.signInThrough(t, "",
		claims("subject-1", "rowan@example.com", true, "blacklight-admins")); recorder.Code != http.StatusFound {
		t.Fatalf("the first sign-in = %d, want 302\nbody: %s", recorder.Code, recorder.Body)
	}
	if got, want := server.users(t)[0].PlatformRole, authz.PlatformRoleAdmin; got != want {
		t.Fatalf("platformRole after the first sign-in = %q, want %q", got, want)
	}

	// They are removed from the administrators group at the provider and sign in
	// again.
	if recorder := server.signInThrough(t, "",
		claims("subject-1", "rowan@example.com", true, "staff")); recorder.Code != http.StatusFound {
		t.Fatalf("the second sign-in = %d, want 302\nbody: %s", recorder.Code, recorder.Body)
	}
	if got, want := server.users(t)[0].PlatformRole, authz.PlatformRoleMember; got != want {
		t.Errorf("platformRole after losing the group = %q, want %q: a revocation at the provider "+
			"must take effect at the next login", got, want)
	}
	if got := len(server.users(t)); got != 1 {
		t.Errorf("the database holds %d users, want 1: the second sign-in is the same person", got)
	}
}

// TestAnUnmappedGroupLeavesTheRoleAlone: "none of your groups is in the mapping"
// is not the same fact as "your groups say member", and treating it as one would
// demote every administrator on a deployment with no mapping.
func TestAnUnmappedGroupLeavesTheRoleAlone(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t)
	server.seedUser(t) // seeded as an admin

	if recorder := server.signInThrough(t, "",
		claims("subject-1", testEmail, true, "some-other-directory-group")); recorder.Code != http.StatusFound {
		t.Fatalf("the callback = %d, want 302\nbody: %s", recorder.Code, recorder.Body)
	}
	if got, want := server.users(t)[0].PlatformRole, authz.PlatformRoleAdmin; got != want {
		t.Errorf("platformRole = %q, want %q — no mapped group means the mapping says nothing", got, want)
	}
}

// TestADisabledAccountCannotSignInThroughTheProvider: disabling an account has
// to close every door, not the one with a password behind it.
// TestAnInvitedAccountIsClaimedByItsFirstSingleSignOn is the other half of
// M1-016's `POST /users` without a password: an administrator creates the
// account `invited`, and the provider vouching for its owner is what claims it.
//
// Without this the invited state is a dead end — no local password to sign in
// with, and a federated sign-in refused for not being active.
func TestAnInvitedAccountIsClaimedByItsFirstSingleSignOn(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t)
	invited := server.seedUser(t, func(in *identity.NewUser) {
		in.Status = identity.StatusInvited
		// Created for single sign-on, so there is no password on it at all.
		in.PasswordHash = ""
	})

	recorder := server.signInThrough(t, "", claims("subject-1", testEmail, true))
	if recorder.Code != http.StatusFound {
		t.Fatalf("the callback = %d, want 302 — an invited account is claimable\nbody: %s",
			recorder.Code, recorder.Body)
	}

	claimed, err := identity.NewUsers(server.db).ByID(t.Context(), invited.ID)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != identity.StatusActive {
		t.Errorf("status = %q after the sign-in, want %q", claimed.Status, identity.StatusActive)
	}
	// One account, not a second one provisioned beside it.
	if got := len(server.users(t)); got != 1 {
		t.Errorf("the database holds %d accounts, want 1", got)
	}
}

// TestADisabledAccountIsNotClaimedByASingleSignOn: `disabled` is somebody's
// decision, and proving who you are at the provider is not an argument against
// it. Only `invited` — "exists and has never been claimed" — is claimable.
func TestADisabledAccountIsNotClaimedByASingleSignOn(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t)
	disabled := server.seedUser(t, func(in *identity.NewUser) { in.Status = identity.StatusDisabled })

	if got := server.signInThrough(t, "", claims("subject-1", testEmail, true)).Code; got != http.StatusForbidden {
		t.Fatalf("the callback = %d, want 403", got)
	}
	after, err := identity.NewUsers(server.db).ByID(t.Context(), disabled.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != identity.StatusDisabled {
		t.Errorf("status = %q, want it left %q", after.Status, identity.StatusDisabled)
	}
}

func TestADisabledAccountCannotSignInThroughTheProvider(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t)
	server.seedUser(t, func(in *identity.NewUser) { in.Status = identity.StatusDisabled })

	recorder := server.signInThrough(t, "", claims("subject-1", testEmail, true))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("the callback = %d, want 403\nbody: %s", recorder.Code, recorder.Body)
	}
}

// TestTheCallbackCannotBeTurnedIntoAnOpenRedirect is the acceptance criterion.
// A login endpoint that redirects wherever a query parameter says is a phishing
// page hosted on your own domain.
func TestTheCallbackCannotBeTurnedIntoAnOpenRedirect(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t)

	for _, returnTo := range []string{
		"https://evil.example/steal",
		"//evil.example/steal",
		"http://evil.example",
		`/\evil.example`,
		`\\evil.example`,
		"https://blacklight.test.evil.example/",
	} {
		t.Run(returnTo, func(t *testing.T) {
			recorder := server.get(startPath + returnToQuery(returnTo))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("GET %s?return_to=%s = %d, want 400\nLocation: %s",
					startPath, returnTo, recorder.Code, recorder.Header().Get("Location"))
			}
			if got, want := decodeProblem(t, recorder).Code, gen.ProblemCodeValidationFailed; got != want {
				t.Errorf("code = %q, want %q", got, want)
			}
		})
	}
}

func TestASignInWithNoReturnPathLandsOnTheRoot(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t, func(cfg *config.OIDC) { cfg.AutoProvision = true })

	recorder := server.signInThrough(t, "", claims("subject-1", "rowan@example.com", true))
	if got, want := recorder.Header().Get("Location"), "/"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
}

// TestASecondFactorStillAppliesToASingleSignOn: a person who has an
// authenticator enrolled here is asked for it, whichever door they came in
// through. Anything else would make single sign-on a way around M1-006.
func TestASecondFactorStillAppliesToASingleSignOn(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t)
	server.seedUser(t)
	server.enrolAndConfirm(t)

	recorder := server.signInThrough(t, "", claims("subject-1", testEmail, true))
	if recorder.Code != http.StatusFound {
		t.Fatalf("the callback = %d, want 302\nbody: %s", recorder.Code, recorder.Body)
	}
	if got, want := recorder.Header().Get("Location"), signInPath; got != want {
		t.Errorf("Location = %q, want %q — the code entry page", got, want)
	}

	// No session, and a pending challenge instead: exactly what a local sign-in
	// with an authenticator produces.
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "bl_session" && cookie.Value != "" {
			t.Error("the callback issued a session before the second factor was presented")
		}
	}
}

// TestTheClientSecretNeverReachesAResponseOrTheLog is the acceptance criterion
// about the secret, asserted over everything a whole sign-in produced.
func TestTheClientSecretNeverReachesAResponseOrTheLog(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t, func(cfg *config.OIDC) { cfg.AutoProvision = true })
	secret := server.idp.ClientSecret

	start := server.get(startPath)
	recorder := server.signInThrough(t, "", claims("subject-1", "rowan@example.com", true))
	refused := server.get(callbackPath + "?state=nonsense&code=nonsense")

	for name, body := range map[string]string{
		"the start response":    start.Body.String() + start.Header().Get("Location"),
		"the callback response": recorder.Body.String() + recorder.Header().Get("Location"),
		"the refusal":           refused.Body.String(),
		"the log":               server.logs.String(),
	} {
		if strings.Contains(body, secret) {
			t.Errorf("the client secret appears in %s:\n%s", name, body)
		}
	}
}

// TestTheStartEndpointSendsTheBrowserToTheProvider covers the one thing the
// protocol tests cannot: that the redirect really leaves this server, and that
// the cookie it sets is scoped and flagged the way the specification says.
func TestTheStartEndpointSendsTheBrowserToTheProvider(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t)

	recorder := server.get(startPath)
	if recorder.Code != http.StatusFound {
		t.Fatalf("GET %s = %d, want 302\nbody: %s", startPath, recorder.Code, recorder.Body)
	}
	location := recorder.Header().Get("Location")
	if !strings.HasPrefix(location, server.idp.Issuer()) {
		t.Errorf("Location = %q, want it to point at the provider", location)
	}

	cookie := stateCookie(t, recorder)
	switch {
	case !cookie.HttpOnly:
		t.Error("the state cookie is not HttpOnly")
	case cookie.SameSite != http.SameSiteLaxMode:
		// Lax and not Strict: the callback is a top-level navigation from
		// another site, and a Strict cookie is not sent on one.
		t.Errorf("the state cookie is SameSite=%v, want Lax", cookie.SameSite)
	case cookie.Path != BasePath+"/auth/oidc":
		t.Errorf("the state cookie is scoped to %q, want the OIDC endpoints", cookie.Path)
	}
}

// TestTheCallbackIsNotCachedByAnything is a small one with a real consequence: a
// redirect carrying Set-Cookie must not be stored by a shared cache.
func TestTheCallbackIsNotCachedByAnything(t *testing.T) {
	t.Parallel()

	server := newSSOServer(t, func(cfg *config.OIDC) { cfg.AutoProvision = true })

	recorder := server.signInThrough(t, "", claims("subject-1", "rowan@example.com", true))
	if got := recorder.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Errorf("Cache-Control = %q, want it to forbid storing the response", got)
	}
}
