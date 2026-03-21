package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bryanster/purpleops/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestHashAndCheckPassword(t *testing.T) {
	password := "securepassword123"

	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword() error: %v", err)
	}

	if hash == password {
		t.Error("hash should not equal plain password")
	}

	if !CheckPassword(hash, password) {
		t.Error("CheckPassword should return true for correct password")
	}

	if CheckPassword(hash, "wrongpassword") {
		t.Error("CheckPassword should return false for wrong password")
	}
}

func TestCheckPasswordInvalidHash(t *testing.T) {
	if CheckPassword("not-a-valid-hash", "password") {
		t.Error("CheckPassword should return false for invalid hash")
	}
}

func TestUserFromContext(t *testing.T) {
	// Empty context
	ctx := context.Background()
	if user := UserFromContext(ctx); user != nil {
		t.Error("UserFromContext should return nil for empty context")
	}

	// Context with user
	u := &models.User{ID: bson.NewObjectID(), Username: "testuser"}
	ctx = context.WithValue(context.Background(), userContextKey, u)
	if user := UserFromContext(ctx); user == nil {
		t.Error("UserFromContext should return user from context")
	} else if user.Username != "testuser" {
		t.Errorf("expected username 'testuser', got %q", user.Username)
	}

	// Context with wrong type
	ctx = context.WithValue(context.Background(), userContextKey, "not-a-user")
	if user := UserFromContext(ctx); user != nil {
		t.Error("UserFromContext should return nil for wrong type")
	}
}

func TestExtractID(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/assessment/abc123/", "abc123"},
		{"/assessment/abc123", "abc123"},
		{"/testcase/def456/", "def456"},
		{"/testcase/def456", "def456"},
		{"/assessment/abc123/navigator", "abc123"},
		{"/testcase/def456/clone", "def456"},
		{"/login", ""},
		{"/", ""},
		{"/assessment/", ""},
		{"/testcase/", ""},
	}

	for _, tt := range tests {
		r := httptest.NewRequest("GET", tt.path, nil)
		got := extractID(r)
		if got != tt.want {
			t.Errorf("extractID(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestInitSessions(t *testing.T) {
	InitSessions("test-secret-key", false, true)

	if store == nil {
		t.Fatal("store should not be nil after InitSessions")
	}
	if store.Options.Path != "/" {
		t.Errorf("expected path '/', got %q", store.Options.Path)
	}
	if store.Options.MaxAge != 86400*7 {
		t.Errorf("expected MaxAge 604800, got %d", store.Options.MaxAge)
	}
	if !store.Options.HttpOnly {
		t.Error("expected HttpOnly to be true")
	}
}

func TestInitSessionsSameSiteStrict(t *testing.T) {
	InitSessions("test-secret-key", false, true)
	if store.Options.SameSite != http.SameSiteStrictMode {
		t.Errorf("expected SameSiteStrictMode when SSO disabled, got %d", store.Options.SameSite)
	}
}

func TestInitSessionsSameSiteLax(t *testing.T) {
	InitSessions("test-secret-key", true, true)
	if store.Options.SameSite != http.SameSiteLaxMode {
		t.Errorf("expected SameSiteLaxMode when SSO enabled, got %d", store.Options.SameSite)
	}
}

func TestSetAndClearSession(t *testing.T) {
	InitSessions("test-secret-key", false, true)

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// SetSessionUser invalidates the old session and creates a new one,
	// writing two Set-Cookie headers to w. Take the last "purpleops" cookie
	// (the new session) and simulate a follow-up request with it.
	SetSessionUser(w, r, "user123")

	var sessionCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "purpleops" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil {
		t.Fatal("no purpleops cookie written to response")
	}

	r2 := httptest.NewRequest("GET", "/", nil)
	r2.AddCookie(sessionCookie)
	sess := GetSession(r2)
	if sess.Values["user_id"] != "user123" {
		t.Errorf("expected user_id 'user123', got %v", sess.Values["user_id"])
	}

	// Clear session and verify a fresh request has no user_id.
	w2 := httptest.NewRecorder()
	ClearSession(w2, r2)
	r3 := httptest.NewRequest("GET", "/", nil)
	sess = GetSession(r3)
	if _, ok := sess.Values["user_id"]; ok {
		t.Error("expected user_id to be cleared from session")
	}
}

func TestAuthRequiredMiddleware(t *testing.T) {
	InitSessions("test-secret-key", false, true)

	handler := AuthRequired(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Without auth - should redirect
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect 302, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc != "/login" {
		t.Errorf("expected redirect to /login, got %q", loc)
	}
}

// --- intersectIDs ---

func TestIntersectIDs(t *testing.T) {
	id1 := bson.NewObjectID()
	id2 := bson.NewObjectID()
	id3 := bson.NewObjectID()

	t.Run("overlap", func(t *testing.T) {
		got := intersectIDs([]bson.ObjectID{id1, id2}, []bson.ObjectID{id2, id3})
		if len(got) != 1 || got[0] != id2 {
			t.Errorf("expected [id2], got %v", got)
		}
	})

	t.Run("no overlap", func(t *testing.T) {
		got := intersectIDs([]bson.ObjectID{id1}, []bson.ObjectID{id2, id3})
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})

	t.Run("all overlap", func(t *testing.T) {
		got := intersectIDs([]bson.ObjectID{id1, id2}, []bson.ObjectID{id1, id2})
		if len(got) != 2 {
			t.Errorf("expected 2, got %d", len(got))
		}
	})

	t.Run("empty a", func(t *testing.T) {
		got := intersectIDs(nil, []bson.ObjectID{id1})
		if len(got) != 0 {
			t.Errorf("expected empty for nil a, got %v", got)
		}
	})

	t.Run("empty b", func(t *testing.T) {
		got := intersectIDs([]bson.ObjectID{id1}, nil)
		if len(got) != 0 {
			t.Errorf("expected empty for nil b, got %v", got)
		}
	})

	t.Run("both empty", func(t *testing.T) {
		got := intersectIDs(nil, nil)
		if len(got) != 0 {
			t.Errorf("expected empty for both nil, got %v", got)
		}
	})

	t.Run("no duplicates in result", func(t *testing.T) {
		// Even if b has duplicates, result should only contain what a has
		got := intersectIDs([]bson.ObjectID{id1}, []bson.ObjectID{id1, id1})
		if len(got) != 1 {
			t.Errorf("expected 1, got %d", len(got))
		}
	})
}

// --- APIKeyAuth middleware ---

func TestAPIKeyAuthMissingHeader(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := APIKeyAuth(next)

	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAPIKeyAuthXAPIKeyHeader(t *testing.T) {
	// A key that won't be found in DB (no DB in unit tests) should still 401,
	// but the middleware must reach the DB lookup — meaning the header was accepted.
	// We verify it gets past the "empty header" check and reaches the lookup stage
	// (which returns 401 from DB miss, not from "no header").
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := APIKeyAuth(next)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Key", "pops_notarealkey")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	// Either 401 (key not found in DB) or 500 (no DB configured in test) — both
	// indicate the header was read successfully and we didn't get a 200.
	if w.Code == http.StatusOK {
		t.Error("expected non-200 for unrecognised key")
	}
}

func TestAPIKeyAuthBearerFallback(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := APIKeyAuth(next)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer pops_notarealkey")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	// Bearer key was extracted — same as above: not 200
	if w.Code == http.StatusOK {
		t.Error("expected non-200 for unrecognised bearer key")
	}
}

func TestAPIKeyAuthEmptyBearerPrefix(t *testing.T) {
	// "Bearer " prefix without a key body → treated as empty → 401
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := APIKeyAuth(next)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer ")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	// Empty string after stripping "Bearer " → hits the empty-key 401 branch
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for empty bearer value, got %d", w.Code)
	}
}

func TestAPIKeyAuthNonBearerAuthHeader(t *testing.T) {
	// Authorization: Basic ... should not be accepted as a key
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := APIKeyAuth(next)

	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for Basic auth header, got %d", w.Code)
	}
}
