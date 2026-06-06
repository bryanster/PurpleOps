package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/config"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

// ginCtx wraps an http.Request in a gin.Context for testing.
func ginCtx(r *http.Request) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = r
	return c, w
}

// newOAuthEngine returns a gin engine with session middleware for OAuth handler tests.
func newOAuthEngine() *gin.Engine {
	mw := auth.InitSessions("test-secret-key-for-oauth", true, true)
	r := gin.New()
	r.Use(mw)
	return r
}

// sessionCookieFromResponse extracts the "purpleops" session cookie from a response.
func sessionCookieFromResponse(w *httptest.ResponseRecorder) *http.Cookie {
	for _, ck := range w.Result().Cookies() {
		if ck.Name == "purpleops" {
			return ck
		}
	}
	return nil
}

// --- InitOAuth ---

func TestInitOAuth(t *testing.T) {
	cfg := &config.Config{
		OAuthClientID:     "test-client-id",
		OAuthClientSecret: "test-client-secret",
		OAuthAuthURL:      "https://provider.example.com/authorize",
		OAuthTokenURL:     "https://provider.example.com/token",
		OAuthRedirectURL:  "https://purpleops.example.com/auth/oauth/callback",
		OAuthScopes:       "openid,email,profile",
	}

	InitOAuth(cfg)

	if oauthConfig == nil {
		t.Fatal("oauthConfig should not be nil after InitOAuth")
	}
	if oauthConfig.ClientID != "test-client-id" {
		t.Errorf("expected ClientID 'test-client-id', got %q", oauthConfig.ClientID)
	}
	if oauthConfig.ClientSecret != "test-client-secret" {
		t.Errorf("expected ClientSecret 'test-client-secret', got %q", oauthConfig.ClientSecret)
	}
	if oauthConfig.RedirectURL != "https://purpleops.example.com/auth/oauth/callback" {
		t.Errorf("unexpected RedirectURL: %q", oauthConfig.RedirectURL)
	}
	if len(oauthConfig.Scopes) != 3 {
		t.Errorf("expected 3 scopes, got %d", len(oauthConfig.Scopes))
	}
	if oauthConfig.Scopes[0] != "openid" {
		t.Errorf("expected first scope 'openid', got %q", oauthConfig.Scopes[0])
	}
}

func TestInitOAuthScopeTrimming(t *testing.T) {
	cfg := &config.Config{
		OAuthScopes: " openid , email , profile ",
	}
	InitOAuth(cfg)

	for _, s := range oauthConfig.Scopes {
		if s != "openid" && s != "email" && s != "profile" {
			t.Errorf("unexpected scope (may have whitespace): %q", s)
		}
	}
}

// --- HandleOAuthLogin ---

func TestHandleOAuthLoginNotConfigured(t *testing.T) {
	oauthConfig = nil

	r := httptest.NewRequest("GET", "/auth/oauth/login", nil)
	c, w := ginCtx(r)
	HandleOAuthLogin(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when OAuth not configured, got %d", w.Code)
	}
}

func TestHandleOAuthLoginRedirects(t *testing.T) {
	engine := newOAuthEngine()
	oauthConfig = &oauth2.Config{
		ClientID: "test-client",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://provider.example.com/authorize",
			TokenURL: "https://provider.example.com/token",
		},
		RedirectURL: "https://purpleops.example.com/auth/oauth/callback",
		Scopes:      []string{"openid", "email"},
	}
	engine.GET("/auth/oauth/login", HandleOAuthLogin)

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest("GET", "/auth/oauth/login", nil))

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header")
	}
	if len(loc) < 40 {
		t.Errorf("Location too short: %q", loc)
	}
}

// --- HandleOAuthCallback ---

func TestHandleOAuthCallbackNotConfigured(t *testing.T) {
	oauthConfig = nil

	r := httptest.NewRequest("GET", "/auth/oauth/callback", nil)
	c, w := ginCtx(r)
	HandleOAuthCallback(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when OAuth not configured, got %d", w.Code)
	}
}

func TestHandleOAuthCallbackInvalidState(t *testing.T) {
	engine := newOAuthEngine()
	oauthConfig = &oauth2.Config{
		ClientID: "test-client",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://provider.example.com/authorize",
			TokenURL: "https://provider.example.com/token",
		},
	}
	engine.GET("/test/set-state", func(c *gin.Context) {
		sess := auth.GetSession(c)
		sess.Set("oauth_state", "correct-state")
		auth.SaveSession(c, sess)
		c.Status(http.StatusOK)
	})
	engine.GET("/auth/oauth/callback", HandleOAuthCallback)

	// Step 1: establish session with the correct state.
	w1 := httptest.NewRecorder()
	engine.ServeHTTP(w1, httptest.NewRequest("GET", "/test/set-state", nil))
	ck := sessionCookieFromResponse(w1)

	// Step 2: callback with a mismatched state.
	req := httptest.NewRequest("GET", "/auth/oauth/callback?state=wrong-state&code=test-code", nil)
	if ck != nil {
		req.AddCookie(ck)
	}
	w2 := httptest.NewRecorder()
	engine.ServeHTTP(w2, req)

	if w2.Code != http.StatusFound {
		t.Errorf("expected redirect 302, got %d", w2.Code)
	}
	if loc := w2.Header().Get("Location"); loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestHandleOAuthCallbackMissingState(t *testing.T) {
	engine := newOAuthEngine()
	oauthConfig = &oauth2.Config{
		ClientID: "test-client",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://provider.example.com/authorize",
			TokenURL: "https://provider.example.com/token",
		},
	}
	engine.GET("/auth/oauth/callback", HandleOAuthCallback)

	// No session cookie → no state in session → mismatch → redirect.
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest("GET", "/auth/oauth/callback?state=some-state&code=test-code", nil))

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect 302, got %d", w.Code)
	}
}

func TestHandleOAuthCallbackProviderError(t *testing.T) {
	engine := newOAuthEngine()
	oauthConfig = &oauth2.Config{
		ClientID: "test-client",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://provider.example.com/authorize",
			TokenURL: "https://provider.example.com/token",
		},
	}
	engine.GET("/test/set-state", func(c *gin.Context) {
		sess := auth.GetSession(c)
		sess.Set("oauth_state", "valid-state")
		auth.SaveSession(c, sess)
		c.Status(http.StatusOK)
	})
	engine.GET("/auth/oauth/callback", HandleOAuthCallback)

	// Step 1: set session state.
	w1 := httptest.NewRecorder()
	engine.ServeHTTP(w1, httptest.NewRequest("GET", "/test/set-state", nil))
	ck := sessionCookieFromResponse(w1)

	// Step 2: callback with a provider error.
	req := httptest.NewRequest("GET", "/auth/oauth/callback?state=valid-state&error=access_denied&error_description=User+denied+access", nil)
	if ck != nil {
		req.AddCookie(ck)
	}
	w2 := httptest.NewRecorder()
	engine.ServeHTTP(w2, req)

	if w2.Code != http.StatusFound {
		t.Errorf("expected redirect 302, got %d", w2.Code)
	}
}

func TestHandleOAuthCallbackNoCode(t *testing.T) {
	engine := newOAuthEngine()
	oauthConfig = &oauth2.Config{
		ClientID: "test-client",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://provider.example.com/authorize",
			TokenURL: "https://provider.example.com/token",
		},
	}
	engine.GET("/test/set-state", func(c *gin.Context) {
		sess := auth.GetSession(c)
		sess.Set("oauth_state", "valid-state")
		auth.SaveSession(c, sess)
		c.Status(http.StatusOK)
	})
	engine.GET("/auth/oauth/callback", HandleOAuthCallback)

	// Step 1: set session state.
	w1 := httptest.NewRecorder()
	engine.ServeHTTP(w1, httptest.NewRequest("GET", "/test/set-state", nil))
	ck := sessionCookieFromResponse(w1)

	// Step 2: callback with no authorization code.
	req := httptest.NewRequest("GET", "/auth/oauth/callback?state=valid-state", nil)
	if ck != nil {
		req.AddCookie(ck)
	}
	w2 := httptest.NewRecorder()
	engine.ServeHTTP(w2, req)

	if w2.Code != http.StatusFound {
		t.Errorf("expected redirect 302, got %d", w2.Code)
	}
}

// --- randomState ---

func TestRandomState(t *testing.T) {
	s1, err := randomState()
	if err != nil {
		t.Fatalf("randomState() error: %v", err)
	}
	if len(s1) != 32 { // 16 bytes = 32 hex chars
		t.Errorf("expected 32 char state, got %d chars", len(s1))
	}

	s2, err := randomState()
	if err != nil {
		t.Fatalf("randomState() error: %v", err)
	}
	if s1 == s2 {
		t.Error("two consecutive random states should differ")
	}
}

// --- fetchOAuthUserInfo ---

func TestFetchOAuthUserInfo(t *testing.T) {
	// Set up a test server that returns user info JSON.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := map[string]interface{}{
			"email": "user@example.com",
			"name":  "Test User",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer ts.Close()

	// Configure oauth to use test server.
	oauthConfig = &oauth2.Config{
		ClientID: "test",
		Endpoint: oauth2.Endpoint{
			AuthURL:  ts.URL + "/authorize",
			TokenURL: ts.URL + "/token",
		},
	}

	config.Cfg = &config.Config{
		OAuthUserInfoURL: ts.URL + "/userinfo",
	}

	r := httptest.NewRequest("GET", "/", nil)
	token := &oauth2.Token{AccessToken: "test-token"}
	token = token.WithExtra(map[string]interface{}{})

	email, username, err := fetchOAuthUserInfo(r, token)
	if err != nil {
		t.Fatalf("fetchOAuthUserInfo() error: %v", err)
	}
	if email != "user@example.com" {
		t.Errorf("expected email 'user@example.com', got %q", email)
	}
	if username != "Test User" {
		t.Errorf("expected username 'Test User', got %q", username)
	}
}

func TestFetchOAuthUserInfoAlternateFields(t *testing.T) {
	// Azure AD style response with "mail" and "displayName".
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := map[string]interface{}{
			"mail":        "azure@example.com",
			"displayName": "Azure User",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer ts.Close()

	oauthConfig = &oauth2.Config{
		ClientID: "test",
		Endpoint: oauth2.Endpoint{
			AuthURL:  ts.URL + "/authorize",
			TokenURL: ts.URL + "/token",
		},
	}
	config.Cfg = &config.Config{
		OAuthUserInfoURL: ts.URL + "/userinfo",
	}

	r := httptest.NewRequest("GET", "/", nil)
	token := &oauth2.Token{AccessToken: "test-token"}
	token = token.WithExtra(map[string]interface{}{})

	email, username, err := fetchOAuthUserInfo(r, token)
	if err != nil {
		t.Fatalf("fetchOAuthUserInfo() error: %v", err)
	}
	if email != "azure@example.com" {
		t.Errorf("expected email 'azure@example.com', got %q", email)
	}
	if username != "Azure User" {
		t.Errorf("expected username 'Azure User', got %q", username)
	}
}

func TestFetchOAuthUserInfoGitHubStyle(t *testing.T) {
	// GitHub-style response with "login".
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := map[string]interface{}{
			"email": "gh@example.com",
			"login": "ghuser",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer ts.Close()

	oauthConfig = &oauth2.Config{
		ClientID: "test",
		Endpoint: oauth2.Endpoint{
			AuthURL:  ts.URL + "/authorize",
			TokenURL: ts.URL + "/token",
		},
	}
	config.Cfg = &config.Config{
		OAuthUserInfoURL: ts.URL + "/userinfo",
	}

	r := httptest.NewRequest("GET", "/", nil)
	token := &oauth2.Token{AccessToken: "test-token"}
	token = token.WithExtra(map[string]interface{}{})

	email, username, err := fetchOAuthUserInfo(r, token)
	if err != nil {
		t.Fatalf("fetchOAuthUserInfo() error: %v", err)
	}
	if email != "gh@example.com" {
		t.Errorf("expected email 'gh@example.com', got %q", email)
	}
	// "name" is checked first; if absent, falls through to "displayName", "preferred_username", "login"
	if username != "ghuser" {
		t.Errorf("expected username 'ghuser', got %q", username)
	}
}

func TestFetchOAuthUserInfoNoEmail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := map[string]interface{}{
			"name": "No Email User",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	}))
	defer ts.Close()

	oauthConfig = &oauth2.Config{
		ClientID: "test",
		Endpoint: oauth2.Endpoint{
			AuthURL:  ts.URL + "/authorize",
			TokenURL: ts.URL + "/token",
		},
	}
	config.Cfg = &config.Config{
		OAuthUserInfoURL: ts.URL + "/userinfo",
	}

	r := httptest.NewRequest("GET", "/", nil)
	token := &oauth2.Token{AccessToken: "test-token"}
	token = token.WithExtra(map[string]interface{}{})

	email, username, err := fetchOAuthUserInfo(r, token)
	if err != nil {
		t.Fatalf("fetchOAuthUserInfo() error: %v", err)
	}
	if email != "" {
		t.Errorf("expected empty email, got %q", email)
	}
	if username != "No Email User" {
		t.Errorf("expected username 'No Email User', got %q", username)
	}
}

func TestFetchOAuthUserInfoInvalidJSON(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not-json"))
	}))
	defer ts.Close()

	oauthConfig = &oauth2.Config{
		ClientID: "test",
		Endpoint: oauth2.Endpoint{
			AuthURL:  ts.URL + "/authorize",
			TokenURL: ts.URL + "/token",
		},
	}
	config.Cfg = &config.Config{
		OAuthUserInfoURL: ts.URL + "/userinfo",
	}

	r := httptest.NewRequest("GET", "/", nil)
	token := &oauth2.Token{AccessToken: "test-token"}
	token = token.WithExtra(map[string]interface{}{})

	_, _, err := fetchOAuthUserInfo(r, token)
	if err == nil {
		t.Error("expected error for invalid JSON response")
	}
}

func TestFetchOAuthUserInfoHTTPError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid_token"}`))
	}))
	defer ts.Close()

	oauthConfig = &oauth2.Config{
		ClientID: "test",
		Endpoint: oauth2.Endpoint{
			AuthURL:  ts.URL + "/authorize",
			TokenURL: ts.URL + "/token",
		},
	}
	config.Cfg = &config.Config{
		OAuthUserInfoURL: ts.URL + "/userinfo",
	}

	r := httptest.NewRequest("GET", "/", nil)
	token := &oauth2.Token{AccessToken: "test-token"}
	token = token.WithExtra(map[string]interface{}{})

	_, _, err := fetchOAuthUserInfo(r, token)
	if err == nil {
		t.Error("expected error for HTTP 401 response")
	}
}

func TestFetchOAuthUserInfoHTTP500(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	oauthConfig = &oauth2.Config{
		ClientID: "test",
		Endpoint: oauth2.Endpoint{
			AuthURL:  ts.URL + "/authorize",
			TokenURL: ts.URL + "/token",
		},
	}
	config.Cfg = &config.Config{
		OAuthUserInfoURL: ts.URL + "/userinfo",
	}

	r := httptest.NewRequest("GET", "/", nil)
	token := &oauth2.Token{AccessToken: "test-token"}
	token = token.WithExtra(map[string]interface{}{})

	_, _, err := fetchOAuthUserInfo(r, token)
	if err == nil {
		t.Error("expected error for HTTP 500 response")
	}
}
