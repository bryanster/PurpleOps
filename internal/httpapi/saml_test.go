package httpapi

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	crewjam "github.com/crewjam/saml"

	"github.com/bryanster/blacklight/internal/authn/oidc/oidctest"
	"github.com/bryanster/blacklight/internal/authn/saml"
	"github.com/bryanster/blacklight/internal/authn/saml/samltest"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// SAML through the whole chain (M1-010): the real server, a real temporary
// DuckDB, and a real identity provider signing with a real key.
//
// internal/authn/saml has the protocol tests — the unsigned assertion, the wrong
// key, the tampered document, the replay. What is proved here is everything
// downstream of a verified assertion: which account it becomes, what a browser
// is left holding, and that the middleware chain lets it through at all.

const (
	samlMetadataPath = BasePath + saml.MetadataPath
	samlStartPath    = BasePath + saml.StartPath
	samlACSFullPath  = BasePath + saml.ACSPath
)

// samlServer is an authServer with a SAML identity provider behind it.
type samlServer struct {
	*authServer
	idp *samltest.Provider

	// acsURL and entityID are what this deployment is, from the identity
	// provider's side. They are computed from the test base URL exactly as the
	// server computes them, which is what makes an assertion minted against them
	// one this server accepts.
	acsURL   string
	entityID string
}

func newSAMLServer(t *testing.T, adjust ...func(*config.SAML)) *samlServer {
	t.Helper()
	return newSAMLServerWith(t, samltest.New(t), adjust...)
}

// newSAMLServerWith is [newSAMLServer] over a provider the caller made, for the
// tests that need one which is already down before the server starts.
func newSAMLServerWith(t *testing.T, idp *samltest.Provider, adjust ...func(*config.SAML)) *samlServer {
	t.Helper()

	certFile, keyFile := samltest.ServiceProviderKeyPair(t, t.TempDir())

	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.SAML = samlConfig(t, idp, certFile, keyFile)
		for _, fn := range adjust {
			fn(&cfg.SAML)
		}
	})

	base := strings.TrimSuffix(testBaseURL, "/") + BasePath
	return &samlServer{
		authServer: server,
		idp:        idp,
		acsURL:     base + saml.ACSPath,
		entityID:   base + saml.MetadataPath,
	}
}

// samlConfig is the configuration an operator would write for this provider,
// parsed the way the environment parses it rather than assembled by hand — so a
// test cannot express a configuration a deployment could not.
func samlConfig(t *testing.T, idp *samltest.Provider, certFile, keyFile string) config.SAML {
	t.Helper()

	var (
		metadata config.URL
		roles    config.RoleMap
		email    config.Names
		name     config.Names
		groups   config.Names
	)
	parse(t, &metadata, idp.MetadataURL())
	parse(t, &roles, "blacklight-admins=admin,staff=member")
	parse(t, &email, "email,mail")
	parse(t, &name, "displayName,name")
	parse(t, &groups, "groups,memberOf")

	return config.SAML{
		MetadataURL:       metadata,
		CertFile:          certFile,
		KeyFile:           keyFile,
		EmailAttribute:    email,
		NameAttribute:     name,
		GroupsAttribute:   groups,
		RoleMap:           roles,
		AllowIDPInitiated: false,
		ClockSkew:         2 * time.Minute,
	}
}

// signInThrough drives a whole browser flow: start, the identity provider, and
// the assertion consumer with the cookie the start set.
func (s *samlServer) signInThrough(t *testing.T, returnTo string, in samltest.Assertion) *httptest.ResponseRecorder {
	t.Helper()

	start := s.get(samlStartPath + returnToQuery(returnTo))
	if start.Code != http.StatusFound {
		t.Fatalf("GET %s = %d, want 302\nbody: %s", samlStartPath, start.Code, start.Body)
	}
	pending := samlCookie(t, start)

	form := s.idp.Login(t, start.Header().Get("Location"), s.acsURL, s.entityID, in)
	return s.postForm(samlACSFullPath, form, pending)
}

// postForm posts the form the identity provider would, which is not
// authServer.post: that helper sends JSON and attaches a CSRF header, and this
// request has neither — it is a cross-site POST from somebody else's server.
func (s *samlServer) postForm(target string, form url.Values, cookies ...*http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	return do(s.handler, request)
}

// samlCookie returns the sealed pending-request cookie a response set.
func samlCookie(t *testing.T, recorder *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == saml.CookieName {
			return cookie
		}
	}
	t.Fatalf("no %s cookie in the response\nheaders: %v", saml.CookieName, recorder.Header())
	return nil
}

// samlPerson is who the identity provider vouches for in these tests.
func samlPerson(nameID, email string, groups ...string) samltest.Person {
	return samltest.Person{
		NameID:      nameID,
		Email:       email,
		DisplayName: "Rowan Ash",
		Groups:      groups,
	}
}

// --- The provider list --------------------------------------------------------

func TestTheLoginPageIsOfferedSAMLWhenItIsConfigured(t *testing.T) {
	t.Parallel()

	server := newSAMLServer(t)

	providers := decodeJSON[gen.AuthProviders](t, server.get(providersPath))
	if !providers.Password {
		t.Error("password sign-in is not offered")
	}
	if len(providers.Sso) != 1 {
		t.Fatalf("sso = %+v, want exactly one provider", providers.Sso)
	}
	if got, want := providers.Sso[0].Id, gen.SSOProviderIdSaml; got != want {
		t.Errorf("sso[0].id = %q, want %q", got, want)
	}
	if got, want := providers.Sso[0].StartUrl, samlStartPath; got != want {
		t.Errorf("sso[0].startUrl = %q, want %q", got, want)
	}
}

// TestABrokenIdentityProviderDoesNotLockAnybodyOut, the SAML half of the
// criterion PLAN.md §4 states and M1-009 already proves for OIDC.
func TestABrokenIdentityProviderDoesNotLockAnybodyOut(t *testing.T) {
	t.Parallel()

	idp := samltest.New(t)
	idp.Close()

	server := newSAMLServerWith(t, idp)
	server.seedUser(t)

	providers := decodeJSON[gen.AuthProviders](t, server.get(providersPath))
	if !providers.Password {
		t.Error("password sign-in is not offered while the identity provider is down")
	}
	if len(providers.Sso) != 0 {
		t.Errorf("sso = %+v, want no providers while the one configured is unreachable", providers.Sso)
	}
	if recorder := server.login(testEmail, testPassword); recorder.Code != http.StatusOK {
		t.Fatalf("local login while the identity provider is down = %d, want 200\nbody: %s",
			recorder.Code, recorder.Body)
	}
}

func TestWithNoSAMLConfiguredTheEndpointsAreNotThere(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)

	for _, path := range []string{samlMetadataPath, samlStartPath} {
		recorder := server.get(path)
		if recorder.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 on a deployment with no SAML", path, recorder.Code)
		}
	}
}

// --- Metadata ------------------------------------------------------------------

// TestTheMetadataEndpointServesSomethingAnIdentityProviderCanRead, and does not
// serve the private key — the acceptance criterion that the key is never exposed
// by any endpoint.
func TestTheMetadataEndpointServesSomethingAnIdentityProviderCanRead(t *testing.T) {
	t.Parallel()

	server := newSAMLServer(t)

	recorder := server.get(samlMetadataPath)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s = %d, want 200\nbody: %s", samlMetadataPath, recorder.Code, recorder.Body)
	}
	if got, want := recorder.Header().Get("Content-Type"), "application/samlmetadata+xml"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}

	var descriptor crewjam.EntityDescriptor
	if err := xml.Unmarshal(recorder.Body.Bytes(), &descriptor); err != nil {
		t.Fatalf("the published metadata is not XML an identity provider could read: %v\n%s",
			err, recorder.Body)
	}
	if descriptor.EntityID != server.entityID {
		t.Errorf("entityID = %q, want %q", descriptor.EntityID, server.entityID)
	}
	if strings.Contains(recorder.Body.String(), "PRIVATE KEY") {
		t.Errorf("the metadata endpoint served the private key:\n%s", recorder.Body)
	}
}

// TestTheMetadataEndpointNeedsNoSession — an identity provider administrator
// fetches it before anybody here has an account to fetch it with.
func TestTheMetadataEndpointNeedsNoSession(t *testing.T) {
	t.Parallel()

	server := newSAMLServer(t)

	if got := server.get(samlMetadataPath).Code; got != http.StatusOK {
		t.Errorf("GET %s unauthenticated = %d, want 200", samlMetadataPath, got)
	}
}

// --- Signing in ------------------------------------------------------------------

// TestAFirstSAMLSignInProvisionsAnAccount is the happy path end to end.
func TestAFirstSAMLSignInProvisionsAnAccount(t *testing.T) {
	t.Parallel()

	server := newSAMLServer(t, func(cfg *config.SAML) { cfg.AutoProvision = true })

	recorder := server.signInThrough(t, "/engagements/018f", samltest.Assertion{
		Person: samlPerson("subject-9f1c", "rowan@example.com", "blacklight-admins"),
	})
	if recorder.Code != http.StatusFound {
		t.Fatalf("the assertion consumer = %d, want 302\nbody: %s", recorder.Code, recorder.Body)
	}
	if got, want := recorder.Header().Get("Location"), "/engagements/018f"; got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}

	users := server.samlUsers(t)
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

	me := server.get(mePath, sessionCookie(t, recorder))
	if me.Code != http.StatusOK {
		t.Fatalf("GET %s with the cookie the assertion consumer set = %d, want 200\nbody: %s",
			mePath, me.Code, me.Body)
	}
	if got, want := decodeJSON[gen.CurrentUser](t, me).Email, "rowan@example.com"; got != want {
		t.Errorf("/auth/me email = %q, want %q", got, want)
	}
}

// TestThePendingRequestCookieIsClearedByTheAssertionConsumer: one request, one
// assertion.
func TestThePendingRequestCookieIsClearedByTheAssertionConsumer(t *testing.T) {
	t.Parallel()

	server := newSAMLServer(t, func(cfg *config.SAML) { cfg.AutoProvision = true })

	recorder := server.signInThrough(t, "", samltest.Assertion{
		Person: samlPerson("subject-1", "rowan@example.com"),
	})
	if recorder.Code != http.StatusFound {
		t.Fatalf("the assertion consumer = %d, want 302\nbody: %s", recorder.Code, recorder.Body)
	}

	cleared := samlCookie(t, recorder)
	if cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Errorf("the pending-request cookie is %+v, want it cleared", cleared)
	}
}

// TestThePendingRequestCookieIsSameSiteNone is the one cookie in this
// application that has to be, and it is worth a test rather than a comment: the
// assertion arrives as a cross-site POST, and a browser sends neither a Strict
// nor a Lax cookie on one. Tightening this would mean no sign-in ever completes,
// which is the kind of change that looks like an improvement in review.
func TestThePendingRequestCookieIsSameSiteNone(t *testing.T) {
	t.Parallel()

	server := newSAMLServer(t)

	start := server.get(samlStartPath)
	if start.Code != http.StatusFound {
		t.Fatalf("GET %s = %d, want 302\nbody: %s", samlStartPath, start.Code, start.Body)
	}

	cookie := samlCookie(t, start)
	if cookie.SameSite != http.SameSiteNoneMode {
		t.Errorf("SameSite = %v, want None — the assertion arrives as a cross-site POST and a "+
			"browser would not send this cookie on one", cookie.SameSite)
	}
	if !cookie.Secure {
		t.Error("Secure is false; a browser refuses SameSite=None without it, so the cookie " +
			"would be dropped entirely")
	}
	if !cookie.HttpOnly {
		t.Error("HttpOnly is false")
	}
}

// TestAReplayedAssertionIsRefusedByTheRealDatabase. The protocol package proves
// the rule against a map; this proves it against DuckDB, through the whole
// chain, which is where it actually has to hold.
func TestAReplayedAssertionIsRefusedByTheRealDatabase(t *testing.T) {
	t.Parallel()

	server := newSAMLServer(t, func(cfg *config.SAML) { cfg.AutoProvision = true })

	start := server.get(samlStartPath)
	if start.Code != http.StatusFound {
		t.Fatalf("GET %s = %d, want 302", samlStartPath, start.Code)
	}
	pending := samlCookie(t, start)
	form := server.idp.Login(t, start.Header().Get("Location"), server.acsURL, server.entityID,
		samltest.Assertion{Person: samlPerson("subject-1", "rowan@example.com")})

	if got := server.postForm(samlACSFullPath, form, pending).Code; got != http.StatusFound {
		t.Fatalf("the first presentation = %d, want 302", got)
	}

	// The same document and the same cookie, inside the same validity window.
	replay := server.postForm(samlACSFullPath, form, pending)
	if replay.Code != http.StatusUnauthorized {
		t.Fatalf("the replay = %d, want 401\nbody: %s", replay.Code, replay.Body)
	}
	if got := len(server.samlUsers(t)); got != 1 {
		t.Errorf("the database holds %d users, want 1: a replay must not create a second", got)
	}
}

// TestAnAssertionForAnExistingAccountLinksRatherThanDuplicating. SAML has no
// `email_verified`, and the address in a signed assertion from the one
// configured identity provider is as trustworthy as the subject beside it — see
// the argument in samlhandlers.go.
func TestAnAssertionForAnExistingAccountLinksRatherThanDuplicating(t *testing.T) {
	t.Parallel()

	server := newSAMLServer(t)
	existing := server.seedUser(t)

	recorder := server.signInThrough(t, "", samltest.Assertion{
		Person: samlPerson("subject-1", testEmail),
	})
	if recorder.Code != http.StatusFound {
		t.Fatalf("the assertion consumer = %d, want 302\nbody: %s", recorder.Code, recorder.Body)
	}

	if got := len(server.samlUsers(t)); got != 1 {
		t.Fatalf("the database holds %d users, want 1: linking must not duplicate an account", got)
	}
	linked, err := identity.NewIdentities(server.db).BySubject(
		t.Context(), identity.ProviderSAML, "subject-1")
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

// TestProvisioningOffRefusesAnUnknownPersonThroughSAML: a 403 whose message says
// what to do, and no row written.
func TestProvisioningOffRefusesAnUnknownPersonThroughSAML(t *testing.T) {
	t.Parallel()

	server := newSAMLServer(t) // AutoProvision is off by default, as in production.

	recorder := server.signInThrough(t, "", samltest.Assertion{
		Person: samlPerson("subject-1", "stranger@example.com"),
	})
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("the assertion consumer = %d, want 403\nbody: %s", recorder.Code, recorder.Body)
	}
	if got := len(server.samlUsers(t)); got != 0 {
		t.Errorf("the database holds %d users, want none", got)
	}
}

// TestGroupsDemoteOnASubsequentSignIn is the direction that matters: an
// integration that only ever promotes is one where revoking access at the
// directory does nothing at all.
func TestGroupsDemoteOnASubsequentSignIn(t *testing.T) {
	t.Parallel()

	server := newSAMLServer(t, func(cfg *config.SAML) { cfg.AutoProvision = true })

	// An administrator, by their groups.
	if got := server.signInThrough(t, "", samltest.Assertion{
		Person: samlPerson("subject-1", "rowan@example.com", "blacklight-admins"),
	}).Code; got != http.StatusFound {
		t.Fatalf("the first sign-in = %d, want 302", got)
	}
	if got := server.samlUsers(t)[0].PlatformRole; got != authz.PlatformRoleAdmin {
		t.Fatalf("platformRole after the first sign-in = %q, want admin", got)
	}

	// Taken out of that group at the directory, and put in one that maps to
	// member. The next sign-in is where this deployment finds out.
	if got := server.signInThrough(t, "", samltest.Assertion{
		Person: samlPerson("subject-1", "rowan@example.com", "staff"),
	}).Code; got != http.StatusFound {
		t.Fatalf("the second sign-in = %d, want 302", got)
	}
	if got := server.samlUsers(t)[0].PlatformRole; got != authz.PlatformRoleMember {
		t.Errorf("platformRole after the second sign-in = %q, want member — a group removed at "+
			"the provider has to take effect here", got)
	}
}

// TestARefusedAssertionAnswers401AndWritesNothing, for the two kinds of forgery
// most likely to arrive: unsigned, and signed by somebody else's key. The
// protocol package proves each check; what this proves is the status code and
// that nothing reached the database.
func TestARefusedAssertionAnswers401AndWritesNothing(t *testing.T) {
	t.Parallel()

	for name, assertion := range map[string]samltest.Assertion{
		"unsigned":  {Unsigned: true},
		"wrong key": {WrongKey: true},
		"tampered":  {Tamper: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := newSAMLServer(t, func(cfg *config.SAML) { cfg.AutoProvision = true })
			assertion.Person = samlPerson("subject-1", "rowan@example.com")

			recorder := server.signInThrough(t, "", assertion)
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("the assertion consumer = %d, want 401\nbody: %s", recorder.Code, recorder.Body)
			}
			if got, want := decodeProblem(t, recorder).Code, gen.ProblemCodeUnauthenticated; got != want {
				t.Errorf("code = %q, want %q", got, want)
			}
			if got := len(server.samlUsers(t)); got != 0 {
				t.Errorf("the database holds %d users, want none: a refused assertion creates nothing", got)
			}
		})
	}
}

// TestTheAssertionConsumerNeedsNoCSRFToken. It is a cross-site POST from the
// identity provider by design; a check it could never pass would mean no sign-in
// ever completes. See csrfExemptRoutes for what stands in its place.
func TestTheAssertionConsumerNeedsNoCSRFToken(t *testing.T) {
	t.Parallel()

	server := newSAMLServer(t, func(cfg *config.SAML) { cfg.AutoProvision = true })

	// signInThrough sends no CSRF header and no CSRF cookie at all.
	if got := server.signInThrough(t, "", samltest.Assertion{
		Person: samlPerson("subject-1", "rowan@example.com"),
	}).Code; got != http.StatusFound {
		t.Errorf("the assertion consumer with no CSRF token = %d, want 302", got)
	}
}

// TestAMisconfiguredSAMLDoesNotBreakOIDC is the acceptance criterion about the
// two living beside each other. Both are configured; SAML's identity provider is
// down; OIDC still signs somebody in.
func TestAMisconfiguredSAMLDoesNotBreakOIDC(t *testing.T) {
	t.Parallel()

	broken := samltest.New(t)
	broken.Close()
	certFile, keyFile := samltest.ServiceProviderKeyPair(t, t.TempDir())

	idp := oidctest.New(t)
	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.OIDC = ssoConfig(t, idp)
		cfg.OIDC.AutoProvision = true
		cfg.SAML = samlConfig(t, broken, certFile, keyFile)
	})
	sso := &ssoServer{authServer: server, idp: idp}

	providers := decodeJSON[gen.AuthProviders](t, server.get(providersPath))
	if len(providers.Sso) != 1 || providers.Sso[0].Id != gen.SSOProviderIdOidc {
		t.Fatalf("sso = %+v, want OIDC alone: SAML's provider is unreachable", providers.Sso)
	}

	recorder := sso.signInThrough(t, "", claims("subject-1", "rowan@example.com", true))
	if recorder.Code != http.StatusFound {
		t.Fatalf("an OIDC sign-in beside a broken SAML = %d, want 302\nbody: %s",
			recorder.Code, recorder.Body)
	}
}

// samlUsers returns every account in the database, which is how the provisioning
// tests assert that nothing was written.
func (s *samlServer) samlUsers(t *testing.T) []identity.User {
	t.Helper()

	found, _, err := identity.NewUsers(s.db).Page(t.Context(), identity.PageFilter{Limit: 200})
	if err != nil {
		t.Fatalf("listing users: %v", err)
	}
	return found
}
