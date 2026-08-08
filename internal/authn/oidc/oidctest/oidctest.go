// Package oidctest is a mock OpenID Connect provider for tests: discovery, a
// key set, an authorization endpoint and a token endpoint, over one httptest
// server with a key pair generated per test.
//
// It exists because the interesting half of M1-009 is what happens when a
// provider misbehaves — a token signed by a key nobody published, an expired
// one, one minted for a different audience, a key rotated between two requests —
// and none of that is reachable against a real provider. A test that only ever
// sees the happy path is a test that proves the library was imported.
//
// It is deliberately a real HTTP server rather than a fake transport. The code
// under test discovers endpoints, follows them, and posts a form to one of them;
// substituting a transport would skip the part where those URLs have to be
// right.
package oidctest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// Provider is a running mock provider. Construct it with [New]; it stops with
// the test.
type Provider struct {
	// ClientID and ClientSecret are what the token endpoint requires. A test
	// that wants to prove a wrong secret is refused changes what it sends, not
	// these.
	ClientID     string
	ClientSecret string

	// t reports what goes wrong inside a request handler. It is Errorf and never
	// Fatalf: these run on the server's goroutines, and FailNow from one of those
	// is undefined. A handler that fails also answers with an error, so the test
	// fails on both sides.
	t testing.TB

	server *httptest.Server

	mu sync.Mutex
	// key is the current signing key. Rotate replaces it.
	key jose.JSONWebKey
	// published is what the JWKS endpoint serves. It is usually the current key
	// and is deliberately separable from it: a provider that has started signing
	// with a key it has not published yet is exactly the rotation case.
	published []jose.JSONWebKey
	// codes are the authorization codes handed out but not yet exchanged, and
	// what each one was issued against.
	codes map[string]issuedCode
	// jwksRequests counts fetches of the key set, which is how a test asserts
	// that the refetch limit is doing something.
	jwksRequests int
	// omitIDToken makes the token endpoint answer without an ID token, which is
	// a provider that has been registered as an OAuth 2.0 client rather than an
	// OpenID Connect one.
	omitIDToken bool
}

// issuedCode is one authorization code and what it is bound to.
type issuedCode struct {
	nonce        string
	challenge    string
	claims       map[string]any
	redirectURI  string
	alreadySpent bool
}

// New starts a provider and registers its shutdown with the test.
func New(t testing.TB) *Provider {
	t.Helper()

	p := &Provider{
		ClientID:     "blacklight-test",
		ClientSecret: "test-client-secret",
		t:            t,
		codes:        map[string]issuedCode{},
	}
	p.key = mustKey(t, "key-1")
	p.published = []jose.JSONWebKey{p.key.Public()}

	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("/keys", p.jwks)
	mux.HandleFunc("/authorize", p.authorize)
	mux.HandleFunc("/token", p.token)

	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)
	return p
}

// Issuer is the identifier to configure Blacklight with.
func (p *Provider) Issuer() string { return p.server.URL }

// Client is an HTTP client that reaches this provider.
func (p *Provider) Client() *http.Client { return p.server.Client() }

// Close stops the server early, so a test can prove what happens when a provider
// that was working stops answering.
func (p *Provider) Close() { p.server.Close() }

// JWKSRequests reports how many times the key set has been fetched.
func (p *Provider) JWKSRequests() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.jwksRequests
}

// Rotate replaces the signing key with a new one under a new identifier, and
// publishes it. This is an ordinary rotation: the next token is signed by a key
// the verifier has never seen, and the verifier is expected to notice.
func (p *Provider) Rotate(t testing.TB, keyID string) {
	t.Helper()

	key := mustKey(t, keyID)
	p.mu.Lock()
	defer p.mu.Unlock()
	p.key = key
	p.published = []jose.JSONWebKey{key.Public()}
}

// OmitIDToken makes the token endpoint answer without one: a client registered
// for plain OAuth 2.0, where there is nothing saying who signed in.
func (p *Provider) OmitIDToken() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.omitIDToken = true
}

// SignWithUnpublishedKey signs the next tokens with a key the JWKS endpoint does
// not serve — a forged token, from the verifier's side.
func (p *Provider) SignWithUnpublishedKey(t testing.TB) {
	t.Helper()

	key := mustKey(t, "unpublished")
	p.mu.Lock()
	defer p.mu.Unlock()
	p.key = key
}

// discovery serves the metadata document.
func (p *Provider) discovery(w http.ResponseWriter, _ *http.Request) {
	p.writeJSON(w, map[string]any{
		"issuer":                                p.server.URL,
		"authorization_endpoint":                p.server.URL + "/authorize",
		"token_endpoint":                        p.server.URL + "/token",
		"jwks_uri":                              p.server.URL + "/keys",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"scopes_supported":                      []string{"openid", "profile", "email", "groups"},
		"code_challenge_methods_supported":      []string{"S256"},
	})
}

func (p *Provider) jwks(w http.ResponseWriter, _ *http.Request) {
	p.mu.Lock()
	p.jwksRequests++
	keys := p.published
	p.mu.Unlock()

	p.writeJSON(w, jose.JSONWebKeySet{Keys: keys})
}

// authorize is the endpoint a browser would be redirected to. It does not
// pretend to be a login screen: it records what the request asked for, mints a
// code, and redirects straight back.
func (p *Provider) authorize(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	redirect, err := url.Parse(query.Get("redirect_uri"))
	if err != nil || redirect.String() == "" {
		http.Error(w, "bad redirect_uri", http.StatusBadRequest)
		return
	}
	if query.Get("code_challenge") == "" || query.Get("code_challenge_method") != "S256" {
		// The mock refuses what a well-configured provider would refuse, so a
		// regression that dropped PKCE fails here rather than passing quietly.
		http.Error(w, "the authorization request carries no S256 PKCE challenge", http.StatusBadRequest)
		return
	}

	code := p.randomString()
	p.mu.Lock()
	p.codes[code] = issuedCode{
		nonce:       query.Get("nonce"),
		challenge:   query.Get("code_challenge"),
		redirectURI: query.Get("redirect_uri"),
	}
	p.mu.Unlock()

	back := *redirect
	values := back.Query()
	values.Set("code", code)
	values.Set("state", query.Get("state"))
	back.RawQuery = values.Encode()
	http.Redirect(w, r, back.String(), http.StatusFound)
}

// Login drives the authorization endpoint the way a browser would and returns
// the callback query the provider redirected to, with the claims a test asked
// for attached to the code.
//
// It exists so a test can complete a whole sign-in without a browser: give it
// the authorization URL the code under test produced, and it hands back the
// query parameters that would have arrived at the callback.
func (p *Provider) Login(t testing.TB, authorizationURL string, claims map[string]any) url.Values {
	t.Helper()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, authorizationURL, nil)
	if err != nil {
		t.Fatalf("oidctest: building the authorization request: %v", err)
	}
	client := p.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("oidctest: following the authorization URL: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("oidctest: the authorization endpoint answered %s, want a redirect", resp.Status)
	}
	back, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("oidctest: parsing the callback URL: %v", err)
	}
	query := back.Query()

	p.mu.Lock()
	defer p.mu.Unlock()
	issued, ok := p.codes[query.Get("code")]
	if !ok {
		t.Fatalf("oidctest: the provider redirected with a code it did not issue")
	}
	issued.claims = claims
	p.codes[query.Get("code")] = issued
	return query
}

// token is the token endpoint: it checks the client credentials and the PKCE
// verifier, then mints an ID token from the claims the test attached.
func (p *Provider) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		p.writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	clientID, clientSecret := r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	if id, secret, ok := r.BasicAuth(); ok {
		clientID, clientSecret = id, secret
	}
	if clientID != p.ClientID || (p.ClientSecret != "" && clientSecret != p.ClientSecret) {
		p.writeError(w, http.StatusUnauthorized, "invalid_client")
		return
	}

	code := r.PostForm.Get("code")
	p.mu.Lock()
	omit := p.omitIDToken
	issued, ok := p.codes[code]
	if ok && issued.alreadySpent {
		// One code, one exchange. A real provider refuses a replay, and so does
		// this one — a test that replays a callback should fail here.
		p.mu.Unlock()
		p.writeError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	if ok {
		issued.alreadySpent = true
		p.codes[code] = issued
	}
	p.mu.Unlock()

	if !ok {
		p.writeError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	if challenge(r.PostForm.Get("code_verifier")) != issued.challenge {
		p.writeError(w, http.StatusBadRequest, "invalid_grant")
		return
	}
	if redirect := r.PostForm.Get("redirect_uri"); redirect != issued.redirectURI {
		p.writeError(w, http.StatusBadRequest, "invalid_grant")
		return
	}

	body := map[string]any{
		"access_token": p.randomString(),
		"token_type":   "Bearer",
		"expires_in":   3600,
	}
	if !omit {
		body["id_token"] = p.IDToken(issued.claims, issued.nonce)
	}
	p.writeJSON(w, body)
}

// IDToken mints a signed ID token from the claims given, filling in the ones a
// test did not name: issuer, audience, subject, and a validity window around
// now. A test that wants one of those wrong sets it explicitly.
//
// It is exported because some tests need a token without a whole exchange —
// the verification cases, which are about the token and not about the flow.
func (p *Provider) IDToken(claims map[string]any, nonce string) string {
	now := time.Now()
	full := map[string]any{
		"iss": p.server.URL,
		"aud": p.ClientID,
		"sub": "subject-1",
		"iat": now.Unix(),
		"nbf": now.Add(-time.Minute).Unix(),
		"exp": now.Add(time.Hour).Unix(),
	}
	if nonce != "" {
		full["nonce"] = nonce
	}
	for name, value := range claims {
		full[name] = value
	}

	p.mu.Lock()
	key := p.key
	p.mu.Unlock()

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		p.t.Errorf("oidctest: building a signer: %v", err)
		return ""
	}
	payload, err := json.Marshal(full)
	if err != nil {
		p.t.Errorf("oidctest: encoding the claims: %v", err)
		return ""
	}
	signed, err := signer.Sign(payload)
	if err != nil {
		p.t.Errorf("oidctest: signing the ID token: %v", err)
		return ""
	}
	serialized, err := signed.CompactSerialize()
	if err != nil {
		p.t.Errorf("oidctest: serializing the ID token: %v", err)
		return ""
	}
	return serialized
}

// mustKey generates an RSA key with the given identifier. 2048 bits: a test
// suite that generates several of these should not spend a second on each.
func mustKey(t testing.TB, keyID string) jose.JSONWebKey {
	t.Helper()

	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("oidctest: generating a key: %v", err)
	}
	return jose.JSONWebKey{
		Key:       private,
		KeyID:     keyID,
		Algorithm: string(jose.RS256),
		Use:       "sig",
	}
}

// challenge computes the S256 PKCE challenge for a verifier, which is what the
// token endpoint compares against what the authorization request carried.
func challenge(verifier string) string {
	if verifier == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (p *Provider) randomString() string {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		p.t.Errorf("oidctest: reading random bytes: %v", err)
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (p *Provider) writeJSON(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		p.t.Errorf("oidctest: writing a response: %v", err)
	}
}

func (p *Provider) writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": code}); err != nil {
		p.t.Errorf("oidctest: writing a %d: %v", status, err)
	}
}
