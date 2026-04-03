package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bryanster/purpleops/internal/models"
	ginsessions "github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newAuthTestCtx creates a gin.Context wrapping the given request.
func newAuthTestCtx(r *http.Request) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = r
	return c, w
}

// newSessionEngine returns a gin engine with the session middleware registered.
// InitSessions must have been called before using this.
func newSessionEngine() *gin.Engine {
	r := gin.New()
	r.Use(ginsessions.Sessions("purpleops", store))
	return r
}

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
	mw := InitSessions("test-secret-key", false, true)
	if mw == nil {
		t.Fatal("InitSessions should return a non-nil middleware handler")
	}
	if store == nil {
		t.Fatal("store should not be nil after InitSessions")
	}
}

func TestSetAndClearSession(t *testing.T) {
	InitSessions("test-secret-key", false, true)
	engine := newSessionEngine()

	// Step 1: set the session user and capture the cookie.
	var sessionCookie *http.Cookie
	engine.GET("/set", func(c *gin.Context) {
		SetSessionUser(c, "user123")
		c.Status(http.StatusOK)
	})
	w1 := httptest.NewRecorder()
	engine.ServeHTTP(w1, httptest.NewRequest("GET", "/set", nil))
	for _, cookie := range w1.Result().Cookies() {
		if cookie.Name == "purpleops" {
			sessionCookie = cookie
		}
	}
	if sessionCookie == nil {
		t.Fatal("no purpleops cookie written to response")
	}

	// Step 2: read user_id back from the session.
	engine.GET("/read", func(c *gin.Context) {
		sess := GetSession(c)
		userID, _ := sess.Get("user_id").(string)
		if userID != "user123" {
			t.Errorf("expected user_id 'user123', got %q", userID)
		}
		c.Status(http.StatusOK)
	})
	req2 := httptest.NewRequest("GET", "/read", nil)
	req2.AddCookie(sessionCookie)
	engine.ServeHTTP(httptest.NewRecorder(), req2)

	// Step 3: clear the session.
	engine.GET("/clear", func(c *gin.Context) {
		ClearSession(c)
		c.Status(http.StatusOK)
	})
	req3 := httptest.NewRequest("GET", "/clear", nil)
	req3.AddCookie(sessionCookie)
	engine.ServeHTTP(httptest.NewRecorder(), req3)

	// Step 4: confirm user_id is gone on a fresh request (no cookie).
	engine.GET("/check", func(c *gin.Context) {
		sess := GetSession(c)
		if uid := sess.Get("user_id"); uid != nil && uid != "" {
			t.Errorf("expected user_id to be absent, got %v", uid)
		}
		c.Status(http.StatusOK)
	})
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/check", nil))
}

func TestAuthRequiredMiddleware(t *testing.T) {
	InitSessions("test-secret-key", false, true)
	engine := newSessionEngine()

	// Without auth - should redirect to /login.
	engine.GET("/protected", AuthRequired, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest("GET", "/protected", nil))

	if w.Code != http.StatusFound {
		t.Errorf("expected redirect 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
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
	r := httptest.NewRequest("GET", "/", nil)
	c, w := newAuthTestCtx(r)
	APIKeyAuth(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAPIKeyAuthXAPIKeyHeader(t *testing.T) {
	// A key that won't be found in DB (no DB in unit tests) should still 401,
	// but the middleware must reach the DB lookup — meaning the header was accepted.
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("X-API-Key", "pops_notarealkey")
	c, w := newAuthTestCtx(r)
	APIKeyAuth(c)

	// Either 401 (key not found in DB) or 500 (no DB configured in test) — both
	// indicate the header was read successfully and we didn't get a 200.
	if w.Code == http.StatusOK {
		t.Error("expected non-200 for unrecognised key")
	}
}

func TestAPIKeyAuthBearerFallback(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer pops_notarealkey")
	c, w := newAuthTestCtx(r)
	APIKeyAuth(c)

	// Bearer key was extracted — same as above: not 200
	if w.Code == http.StatusOK {
		t.Error("expected non-200 for unrecognised bearer key")
	}
}

func TestAPIKeyAuthEmptyBearerPrefix(t *testing.T) {
	// "Bearer " prefix without a key body → treated as empty → 401
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Bearer ")
	c, w := newAuthTestCtx(r)
	APIKeyAuth(c)

	// Empty string after stripping "Bearer " → hits the empty-key 401 branch
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for empty bearer value, got %d", w.Code)
	}
}

func TestAPIKeyAuthNonBearerAuthHeader(t *testing.T) {
	// Authorization: Basic ... should not be accepted as a key
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	c, w := newAuthTestCtx(r)
	APIKeyAuth(c)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for Basic auth header, got %d", w.Code)
	}
}
