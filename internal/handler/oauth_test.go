package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/config"
	"golang.org/x/oauth2"
)

func initTestSession() {
	auth.InitSessions("test-secret-key-for-oauth", true)
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
	w := httptest.NewRecorder()
	HandleOAuthLogin(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when OAuth not configured, got %d", w.Code)
	}
}

func TestHandleOAuthLoginRedirects(t *testing.T) {
	initTestSession()

	oauthConfig = &oauth2.Config{
		ClientID: "test-client",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://provider.example.com/authorize",
			TokenURL: "https://provider.example.com/token",
		},
		RedirectURL: "https://purpleops.example.com/auth/oauth/callback",
		Scopes:      []string{"openid", "email"},
	}

	r := httptest.NewRequest("GET", "/auth/oauth/login", nil)
	w := httptest.NewRecorder()
	HandleOAuthLogin(w, r)

	if w.Code != http.StatusFound {
		t.Errorf("expected 302 redirect, got %d", w.Code)
	}

	loc := w.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header")
	}

	// Should redirect to provider's authorize endpoint.
	if len(loc) < 40 {
		t.Errorf("Location too short: %q", loc)
	}

	// Verify the state was stored in the session.
	sess := auth.GetSession(r)
	state, ok := sess.Values["oauth_state"].(string)
	if !ok || state == "" {
		t.Error("expected oauth_state to be set in session")
	}
}

// --- HandleOAuthCallback ---

func TestHandleOAuthCallbackNotConfigured(t *testing.T) {
	oauthConfig = nil

	r := httptest.NewRequest("GET", "/auth/oauth/callback", nil)
	w := httptest.NewRecorder()
	HandleOAuthCallback(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 when OAuth not configured, got %d", w.Code)
	}
}

func TestHandleOAuthCallbackInvalidState(t *testing.T) {
	initTestSession()

	oauthConfig = &oauth2.Config{
		ClientID: "test-client",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://provider.example.com/authorize",
			TokenURL: "https://provider.example.com/token",
		},
	}

	// Set a state in session but send a different one in the request.
	r := httptest.NewRequest("GET", "/auth/oauth/callback?state=wrong-state&code=test-code", nil)
	w := httptest.NewRecorder()

	sess := auth.GetSession(r)
	sess.Values["oauth_state"] = "correct-state"
	sess.Save(r, w)

	HandleOAuthCallback(w, r)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

func TestHandleOAuthCallbackMissingState(t *testing.T) {
	initTestSession()

	oauthConfig = &oauth2.Config{
		ClientID: "test-client",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://provider.example.com/authorize",
			TokenURL: "https://provider.example.com/token",
		},
	}

	// No state in session at all.
	r := httptest.NewRequest("GET", "/auth/oauth/callback?state=some-state&code=test-code", nil)
	w := httptest.NewRecorder()

	HandleOAuthCallback(w, r)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect 302, got %d", w.Code)
	}
}

func TestHandleOAuthCallbackProviderError(t *testing.T) {
	initTestSession()

	oauthConfig = &oauth2.Config{
		ClientID: "test-client",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://provider.example.com/authorize",
			TokenURL: "https://provider.example.com/token",
		},
	}

	r := httptest.NewRequest("GET", "/auth/oauth/callback?error=access_denied&error_description=User+denied+access", nil)
	w := httptest.NewRecorder()

	// Set valid state so we get past state check.
	sess := auth.GetSession(r)
	sess.Values["oauth_state"] = "valid-state"
	sess.Save(r, w)

	// Re-create request with state matching.
	r = httptest.NewRequest("GET", "/auth/oauth/callback?state=valid-state&error=access_denied&error_description=User+denied+access", nil)
	w = httptest.NewRecorder()

	sess = auth.GetSession(r)
	sess.Values["oauth_state"] = "valid-state"
	sess.Save(r, w)

	HandleOAuthCallback(w, r)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect 302, got %d", w.Code)
	}
}

func TestHandleOAuthCallbackNoCode(t *testing.T) {
	initTestSession()

	oauthConfig = &oauth2.Config{
		ClientID: "test-client",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://provider.example.com/authorize",
			TokenURL: "https://provider.example.com/token",
		},
	}

	r := httptest.NewRequest("GET", "/auth/oauth/callback?state=valid-state", nil)
	w := httptest.NewRecorder()

	sess := auth.GetSession(r)
	sess.Values["oauth_state"] = "valid-state"
	sess.Save(r, w)

	HandleOAuthCallback(w, r)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect 302, got %d", w.Code)
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
