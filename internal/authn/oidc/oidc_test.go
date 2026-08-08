package oidc_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authn/oidc"
	"github.com/bryanster/blacklight/internal/authn/oidc/oidctest"
)

// The protocol half of M1-009, against a mock provider with a generated key
// pair. These are the tests the ticket calls the point of the exercise: the
// happy path proves the flow works, and everything after it proves that a
// provider — or something impersonating one — cannot get past verification by
// misbehaving.

// testKey is 32 bytes of key material. It is not a secret in a test; it is here
// so the sealed state has a key to be sealed under.
const testKey = "kZ2rV8sQ1tYb7NxJ4mWc0PfLgH6dEuAiOoTpRySvXlU="

const redirectURI = "https://blacklight.test/api/v1/auth/oidc/callback"

// newProvider returns a Provider pointed at idp, with the discovery and JWKS
// rate limits disabled: they are the subject of their own tests, and everywhere
// else they would only make a test wait.
func newProvider(t *testing.T, idp *oidctest.Provider, adjust func(*oidc.Options)) *oidc.Provider {
	t.Helper()

	opts := oidc.Options{
		Issuer:             idp.Issuer(),
		ClientID:           idp.ClientID,
		ClientSecret:       idp.ClientSecret,
		Scopes:             []string{"openid", "profile", "email", "groups"},
		GroupsClaim:        "groups",
		RedirectURI:        redirectURI,
		Key:                []byte(testKey),
		CookiePath:         "/api/v1/auth/oidc",
		Secure:             true,
		HTTPClient:         idp.Client(),
		KeyRefetchInterval: -1,
		DiscoveryRetry:     -1,
	}
	if adjust != nil {
		adjust(&opts)
	}
	provider, err := oidc.New(opts)
	if err != nil {
		t.Fatalf("oidc.New: %v", err)
	}
	return provider
}

// signIn drives a whole sign-in and returns what Complete made of it. It is the
// shape every test below is a variation on.
func signIn(t *testing.T, provider *oidc.Provider, idp *oidctest.Provider, claims map[string]any) (oidc.Identity, error) {
	t.Helper()

	start, err := provider.Start(context.Background(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	callback := idp.Login(t, start.AuthorizationURL, claims)
	return provider.Complete(context.Background(), oidc.Callback{
		State:  callback.Get("state"),
		Code:   callback.Get("code"),
		Sealed: start.Sealed,
	})
}

func TestASignInReturnsTheVerifiedClaims(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)

	identity, err := signIn(t, provider, idp, map[string]any{
		"sub":            "9f1c",
		"email":          "rowan@example.com",
		"email_verified": true,
		"name":           "Rowan Ash",
		"groups":         []string{"blacklight-admins", "everyone"},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if got, want := identity.Subject, "9f1c"; got != want {
		t.Errorf("Subject = %q, want %q", got, want)
	}
	if got, want := identity.Email, "rowan@example.com"; got != want {
		t.Errorf("Email = %q, want %q", got, want)
	}
	if !identity.EmailVerified {
		t.Error("EmailVerified is false, want true")
	}
	if got, want := identity.DisplayName, "Rowan Ash"; got != want {
		t.Errorf("DisplayName = %q, want %q", got, want)
	}
	if got, want := strings.Join(identity.Groups, ","), "blacklight-admins,everyone"; got != want {
		t.Errorf("Groups = %q, want %q", got, want)
	}
}

// TestTheAuthorizationRequestCarriesEverythingItMust is the one test that reads
// the outbound URL. PKCE, state and nonce are each the whole defence against one
// attack, and a change that dropped one would otherwise show up as nothing at
// all — every test here would still pass, because the mock is not the attacker.
func TestTheAuthorizationRequestCarriesEverythingItMust(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)

	start, err := provider.Start(context.Background(), "/engagements/1")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	parsed, err := url.Parse(start.AuthorizationURL)
	if err != nil {
		t.Fatalf("parsing the authorization URL: %v", err)
	}
	query := parsed.Query()

	for name, want := range map[string]string{
		"response_type":         "code",
		"client_id":             idp.ClientID,
		"redirect_uri":          redirectURI,
		"code_challenge_method": "S256",
	} {
		if got := query.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	for _, name := range []string{"state", "nonce", "code_challenge"} {
		if query.Get(name) == "" {
			t.Errorf("the authorization URL carries no %s", name)
		}
	}
	if got, want := query.Get("scope"), "openid profile email groups"; got != want {
		t.Errorf("scope = %q, want %q", got, want)
	}
	// The verifier itself must never leave this server: sending it alongside the
	// challenge would make PKCE decorative.
	if strings.Contains(start.AuthorizationURL, "code_verifier") {
		t.Error("the authorization URL carries the PKCE verifier")
	}
	if start.Sealed == "" {
		t.Fatal("Start returned no sealed state")
	}
	// The pending state is sealed: none of what it holds is readable by the
	// browser carrying it.
	for _, secret := range []string{query.Get("nonce"), query.Get("state")} {
		if strings.Contains(start.Sealed, secret) {
			t.Error("the sealed state carries a value in the clear")
		}
	}
}

func TestACallbackWithNoStateCookieIsRefused(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)

	start, err := provider.Start(context.Background(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	callback := idp.Login(t, start.AuthorizationURL, nil)

	_, err = provider.Complete(context.Background(), oidc.Callback{
		State: callback.Get("state"),
		Code:  callback.Get("code"),
		// No Sealed: the browser that comes back is not the one that left.
	})
	if !errors.Is(err, oidc.ErrNoPendingSignIn) {
		t.Fatalf("Complete = %v, want ErrNoPendingSignIn", err)
	}
}

func TestACallbackWithSomebodyElsesStateIsRefused(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)

	// Two sign-ins started in two browsers. The callback of one, presented with
	// the cookie of the other, is login CSRF.
	mine, err := provider.Start(context.Background(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	theirs, err := provider.Start(context.Background(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	callback := idp.Login(t, theirs.AuthorizationURL, nil)

	_, err = provider.Complete(context.Background(), oidc.Callback{
		State:  callback.Get("state"),
		Code:   callback.Get("code"),
		Sealed: mine.Sealed,
	})
	if !errors.Is(err, oidc.ErrNoPendingSignIn) {
		t.Fatalf("Complete = %v, want ErrNoPendingSignIn", err)
	}
}

func TestACallbackWithNoStateParameterIsRefused(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)

	start, err := provider.Start(context.Background(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	callback := idp.Login(t, start.AuthorizationURL, nil)

	_, err = provider.Complete(context.Background(), oidc.Callback{
		Code:   callback.Get("code"),
		Sealed: start.Sealed,
	})
	if !errors.Is(err, oidc.ErrNoPendingSignIn) {
		t.Fatalf("Complete = %v, want ErrNoPendingSignIn", err)
	}
}

// TestAReplayedCallbackIsRefused covers the second half of "single use". The
// cookie is cleared by the handler, so the ordinary replay arrives with no
// state at all — and a browser that somehow still held the cookie is stopped by
// the provider, which spends an authorization code exactly once.
func TestAReplayedCallbackIsRefused(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)

	start, err := provider.Start(context.Background(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	callback := idp.Login(t, start.AuthorizationURL, nil)
	in := oidc.Callback{
		State:  callback.Get("state"),
		Code:   callback.Get("code"),
		Sealed: start.Sealed,
	}
	if _, err := provider.Complete(context.Background(), in); err != nil {
		t.Fatalf("the first Complete failed: %v", err)
	}

	_, err = provider.Complete(context.Background(), in)
	if !errors.Is(err, oidc.ErrRejected) {
		t.Fatalf("the replayed callback = %v, want ErrRejected", err)
	}
}

func TestAnExpiredStateIsRefused(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, func(o *oidc.Options) {
		o.StateTTL = time.Millisecond
	})

	start, err := provider.Start(context.Background(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	callback := idp.Login(t, start.AuthorizationURL, nil)
	time.Sleep(5 * time.Millisecond)

	_, err = provider.Complete(context.Background(), oidc.Callback{
		State:  callback.Get("state"),
		Code:   callback.Get("code"),
		Sealed: start.Sealed,
	})
	if !errors.Is(err, oidc.ErrNoPendingSignIn) {
		t.Fatalf("Complete = %v, want ErrNoPendingSignIn", err)
	}
}

// TestStateFromAnotherDeploymentIsRefused: the seal is what makes the state
// unforgeable, and a deployment with a different encryption key must not be able
// to hand one to this one.
func TestStateFromAnotherDeploymentIsRefused(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	ours := newProvider(t, idp, nil)
	theirs := newProvider(t, idp, func(o *oidc.Options) {
		o.Key = []byte("9Qd3JmE7uZpA0xTnCiL5wHrYbVsK2fGoP4jXeM8tUcR=")
	})

	start, err := theirs.Start(context.Background(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	callback := idp.Login(t, start.AuthorizationURL, nil)

	_, err = ours.Complete(context.Background(), oidc.Callback{
		State:  callback.Get("state"),
		Code:   callback.Get("code"),
		Sealed: start.Sealed,
	})
	if !errors.Is(err, oidc.ErrNoPendingSignIn) {
		t.Fatalf("Complete = %v, want ErrNoPendingSignIn", err)
	}
}

// TestAMismatchedNonceIsRefused is the acceptance criterion, and it is the check
// no library performs for you: the token verifies perfectly and belongs to a
// different exchange.
func TestAMismatchedNonceIsRefused(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)

	_, err := signIn(t, provider, idp, map[string]any{"nonce": "a nonce from somewhere else"})
	if !errors.Is(err, oidc.ErrRejected) {
		t.Fatalf("Complete = %v, want ErrRejected", err)
	}
	if !strings.Contains(err.Error(), "nonce") {
		t.Errorf("the error does not mention the nonce: %v", err)
	}
}

// The three token-verification cases the ticket names, one test each. They are
// worth having precisely because "the library handles it" is a claim rather than
// a fact until something asserts it.
func TestAnIDTokenSignedByAnUnknownKeyIsRefused(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)
	idp.SignWithUnpublishedKey(t)

	_, err := signIn(t, provider, idp, nil)
	if !errors.Is(err, oidc.ErrRejected) {
		t.Fatalf("Complete = %v, want ErrRejected", err)
	}
}

func TestAnExpiredIDTokenIsRefused(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)

	_, err := signIn(t, provider, idp, map[string]any{
		"exp": time.Now().Add(-time.Minute).Unix(),
	})
	if !errors.Is(err, oidc.ErrRejected) {
		t.Fatalf("Complete = %v, want ErrRejected", err)
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("the error does not say the token expired: %v", err)
	}
}

func TestAnIDTokenForAnotherAudienceIsRefused(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)

	_, err := signIn(t, provider, idp, map[string]any{"aud": "some-other-application"})
	if !errors.Is(err, oidc.ErrRejected) {
		t.Fatalf("Complete = %v, want ErrRejected", err)
	}
}

func TestAnIDTokenFromAnotherIssuerIsRefused(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)

	_, err := signIn(t, provider, idp, map[string]any{"iss": "https://not-the-provider.example"})
	if !errors.Is(err, oidc.ErrRejected) {
		t.Fatalf("Complete = %v, want ErrRejected", err)
	}
}

func TestATokenResponseWithNoIDTokenIsRefused(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)
	idp.OmitIDToken()

	_, err := signIn(t, provider, idp, nil)
	if !errors.Is(err, oidc.ErrRejected) {
		t.Fatalf("Complete = %v, want ErrRejected", err)
	}
}

func TestAProviderRefusalIsReported(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)

	start, err := provider.Start(context.Background(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	// What arrives when somebody declines the consent screen: the state is
	// genuine and there is no code.
	state, err := stateOf(t, start.AuthorizationURL)
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Complete(context.Background(), oidc.Callback{
		State:            state,
		Error:            "access_denied",
		ErrorDescription: "the user refused consent",
		Sealed:           start.Sealed,
	})
	if !errors.Is(err, oidc.ErrRejected) {
		t.Fatalf("Complete = %v, want ErrRejected", err)
	}
}

// TestAKeyRotationIsHandledWithoutARestart is the rotation acceptance criterion:
// the provider signs with a key published after the last fetch, and the next
// sign-in works.
func TestAKeyRotationIsHandledWithoutARestart(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)

	if _, err := signIn(t, provider, idp, nil); err != nil {
		t.Fatalf("the sign-in before the rotation failed: %v", err)
	}
	before := idp.JWKSRequests()

	idp.Rotate(t, "key-2")

	if _, err := signIn(t, provider, idp, nil); err != nil {
		t.Fatalf("the sign-in after the rotation failed: %v", err)
	}
	if after := idp.JWKSRequests(); after <= before {
		t.Errorf("the key set was fetched %d times before the rotation and %d after; "+
			"an unknown key must trigger a refetch", before, after)
	}
}

// TestARotationIsHonouredAsSoonAsTheLimitAllows is the same criterion with the
// rate limit left *on*, which is how it is deployed.
//
// It exists because the version of this that disabled the limit passed while a
// real Keycloak rotation failed for a full minute: the refetch a rotation needs
// is the same refetch the limit refuses. What the limit costs is bounded here in
// so many words — one interval, and no more — and the interval is chosen in
// keys.go with this test's failure mode in mind.
func TestARotationIsHonouredAsSoonAsTheLimitAllows(t *testing.T) {
	t.Parallel()

	clock := &testClock{now: time.Now()}
	idp := oidctest.New(t)
	provider := newProvider(t, idp, func(o *oidc.Options) {
		o.KeyRefetchInterval = time.Minute
		o.Now = clock.read
	})

	if _, err := signIn(t, provider, idp, nil); err != nil {
		t.Fatalf("the sign-in before the rotation failed: %v", err)
	}

	idp.Rotate(t, "key-2")

	// Straight away: the key set was fetched a moment ago, so the new key cannot
	// be fetched and the sign-in is refused rather than accepted on trust.
	if _, err := signIn(t, provider, idp, nil); !errors.Is(err, oidc.ErrRejected) {
		t.Fatalf("the sign-in immediately after the rotation = %v, want ErrRejected", err)
	}

	// One interval later, with nothing restarted and nothing reconfigured.
	clock.advance(time.Minute + time.Second)
	if _, err := signIn(t, provider, idp, nil); err != nil {
		t.Fatalf("the sign-in an interval after the rotation failed: %v", err)
	}
}

// testClock is a clock a test moves by hand, for the behaviour that is about an
// interval passing rather than about time itself.
type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) read() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// TestTheKeySetRefetchIsRateLimited is the other half of that criterion, and the
// reason this package does not use the library's key set. Tokens signed by a key
// nobody published are free to make; fetching the JWKS for each one turns this
// server into a load generator pointed at the identity provider.
func TestTheKeySetRefetchIsRateLimited(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, func(o *oidc.Options) {
		o.KeyRefetchInterval = time.Hour
	})

	// One good sign-in, which fetches the key set once.
	if _, err := signIn(t, provider, idp, nil); err != nil {
		t.Fatalf("the first sign-in failed: %v", err)
	}
	fetches := idp.JWKSRequests()

	idp.SignWithUnpublishedKey(t)
	for range 20 {
		if _, err := signIn(t, provider, idp, nil); !errors.Is(err, oidc.ErrRejected) {
			t.Fatalf("a token signed by an unpublished key = %v, want ErrRejected", err)
		}
	}

	// One more fetch is expected: the first forged token is indistinguishable
	// from a rotation, so it is honoured. The other nineteen must cost nothing.
	if got, want := idp.JWKSRequests(), fetches+1; got > want {
		t.Errorf("the key set was fetched %d times, want at most %d: twenty forged tokens "+
			"must not become twenty requests to the provider", got, want)
	}
}

// TestAProviderThatIsDownDoesNotBreakTheServer is the "must not lock everyone
// out" criterion at this layer: discovery fails, and it fails as an error the
// caller can report rather than as a panic or a hang.
func TestAProviderThatIsDownDoesNotBreakTheServer(t *testing.T) {
	t.Parallel()

	idp := oidctest.New(t)
	provider := newProvider(t, idp, nil)
	idp.Close()

	if provider.Available(context.Background()) {
		t.Error("Available() is true for a provider that cannot be reached")
	}
	if _, err := provider.Start(context.Background(), ""); !errors.Is(err, oidc.ErrUnavailable) {
		t.Errorf("Start = %v, want ErrUnavailable", err)
	}
}

// TestAProviderThatComesBackIsNoticed: discovery is retried rather than being a
// one-shot at startup, so an identity provider that was down when this server
// booted starts working without anybody restarting it.
func TestAProviderThatComesBackIsNoticed(t *testing.T) {
	t.Parallel()

	flaky := newFlakyProvider(t)
	provider, err := oidc.New(oidc.Options{
		Issuer:         flaky.issuer(),
		ClientID:       "blacklight",
		RedirectURI:    redirectURI,
		GroupsClaim:    "groups",
		Key:            []byte(testKey),
		CookiePath:     "/api/v1/auth/oidc",
		HTTPClient:     flaky.server.Client(),
		DiscoveryRetry: -1,
	})
	if err != nil {
		t.Fatalf("oidc.New: %v", err)
	}

	if err := provider.Refresh(context.Background()); !errors.Is(err, oidc.ErrUnavailable) {
		t.Fatalf("Refresh against a provider that is down = %v, want ErrUnavailable", err)
	}
	if provider.Available(context.Background()) {
		t.Error("Available() is true while the provider is down")
	}

	flaky.up.Store(true)

	if err := provider.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh after the provider came back = %v, want it to succeed", err)
	}
	if !provider.Available(context.Background()) {
		t.Error("Available() is false after discovery succeeded")
	}
}

// TestDiscoveryIsRateLimited: the endpoint that asks whether single sign-on is
// available is public, so the retry behind it has to be bounded by time.
func TestDiscoveryIsRateLimited(t *testing.T) {
	t.Parallel()

	flaky := newFlakyProvider(t)
	provider, err := oidc.New(oidc.Options{
		Issuer:         flaky.issuer(),
		ClientID:       "blacklight",
		RedirectURI:    redirectURI,
		GroupsClaim:    "groups",
		Key:            []byte(testKey),
		CookiePath:     "/api/v1/auth/oidc",
		HTTPClient:     flaky.server.Client(),
		DiscoveryRetry: time.Hour,
	})
	if err != nil {
		t.Fatalf("oidc.New: %v", err)
	}

	for range 10 {
		if _, err := provider.Start(context.Background(), ""); !errors.Is(err, oidc.ErrUnavailable) {
			t.Fatalf("Start = %v, want ErrUnavailable", err)
		}
	}
	if got := flaky.requests.Load(); got > 1 {
		t.Errorf("the provider was asked for its metadata %d times, want 1: ten sign-in attempts "+
			"against a provider that is down must not be ten requests", got)
	}
}

// flakyProvider is a provider that can be turned on and off, for the tests about
// what happens when one is unreachable. It serves discovery and nothing else —
// none of those tests gets as far as a token.
type flakyProvider struct {
	server   *httptest.Server
	up       atomic.Bool
	requests atomic.Int64
}

func newFlakyProvider(t *testing.T) *flakyProvider {
	t.Helper()

	flaky := &flakyProvider{}
	flaky.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flaky.requests.Add(1)
		if !flaky.up.Load() {
			http.Error(w, "down", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		base := flaky.issuer()
		if err := json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                base,
			"authorization_endpoint":                base + "/authorize",
			"token_endpoint":                        base + "/token",
			"jwks_uri":                              base + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		}); err != nil {
			t.Errorf("writing the discovery document: %v", err)
		}
	}))
	t.Cleanup(flaky.server.Close)
	return flaky
}

func (f *flakyProvider) issuer() string { return f.server.URL }

// stateOf reads the state out of an authorization URL, for the tests that
// fabricate a callback rather than driving one.
func stateOf(t *testing.T, authorizationURL string) (string, error) {
	t.Helper()

	parsed, err := url.Parse(authorizationURL)
	if err != nil {
		return "", err
	}
	return parsed.Query().Get("state"), nil
}
