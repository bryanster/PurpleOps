package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// newAuthEngine returns a gin engine with session middleware for auth handler tests.
func newAuthEngine() *gin.Engine {
	mw := auth.InitSessions("test-secret-key-for-auth", false, true)
	r := gin.New()
	r.Use(mw)
	return r
}

// withUserMiddleware injects a user into the request context.
func withUserMiddleware(u *models.User) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := auth.WithUser(c.Request.Context(), u)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// --- HandleLoginPost ---

func TestHandleLoginPost_EmptyEmail(t *testing.T) {
	engine := newAuthEngine()
	engine.POST("/login", HandleLoginPost)

	form := url.Values{"email": {""}, "password": {"password"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("empty email: expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("empty email: expected redirect to /login, got %q", loc)
	}
}

func TestHandleLoginPost_EmptyPassword(t *testing.T) {
	engine := newAuthEngine()
	engine.POST("/login", HandleLoginPost)

	form := url.Values{"email": {"user@example.com"}, "password": {""}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("empty password: expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("empty password: expected redirect to /login, got %q", loc)
	}
}

func TestHandleLoginPost_BothEmpty(t *testing.T) {
	engine := newAuthEngine()
	engine.POST("/login", HandleLoginPost)

	form := url.Values{}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("both empty: expected 302, got %d", w.Code)
	}
}

// --- HandleLogout ---

func TestHandleLogout_ClearsSessionAndRedirects(t *testing.T) {
	engine := newAuthEngine()
	engine.GET("/logout", HandleLogout)

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest("GET", "/logout", nil))

	if w.Code != http.StatusFound {
		t.Errorf("logout: expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("logout: expected redirect to /login, got %q", loc)
	}
}

// --- HandlePasswordChangePost ---

func TestHandlePasswordChangePost_NoUser(t *testing.T) {
	// No user in context → redirect to /login; GetSession is never called so no session mw needed.
	engine := gin.New()
	engine.POST("/password/change", HandlePasswordChangePost)

	req := httptest.NewRequest("POST", "/password/change", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("no user: expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("no user: expected redirect to /login, got %q", loc)
	}
}

func TestHandlePasswordChangePost_ShortNewPassword(t *testing.T) {
	hashed, _ := auth.HashPassword("currentpassword")
	user := &models.User{ID: bson.NewObjectID(), Password: hashed, Active: true}

	engine := newAuthEngine()
	engine.Use(withUserMiddleware(user))
	engine.POST("/password/change", HandlePasswordChangePost)

	form := url.Values{
		"password":             {"currentpassword"},
		"new_password":         {"short"},
		"new_password_confirm": {"short"},
	}
	req := httptest.NewRequest("POST", "/password/change", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("short password: expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/password/change" {
		t.Errorf("short password: expected redirect to /password/change, got %q", loc)
	}
}

func TestHandlePasswordChangePost_PasswordMismatch(t *testing.T) {
	hashed, _ := auth.HashPassword("currentpassword")
	user := &models.User{ID: bson.NewObjectID(), Password: hashed, Active: true}

	engine := newAuthEngine()
	engine.Use(withUserMiddleware(user))
	engine.POST("/password/change", HandlePasswordChangePost)

	form := url.Values{
		"password":             {"currentpassword"},
		"new_password":         {"newlongpassword1"},
		"new_password_confirm": {"newlongpassword2"},
	}
	req := httptest.NewRequest("POST", "/password/change", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("password mismatch: expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/password/change" {
		t.Errorf("password mismatch: expected redirect to /password/change, got %q", loc)
	}
}

func TestHandlePasswordChangePost_WrongCurrentPassword(t *testing.T) {
	hashed, _ := auth.HashPassword("correctpassword")
	user := &models.User{ID: bson.NewObjectID(), Password: hashed, Active: true}

	engine := newAuthEngine()
	engine.Use(withUserMiddleware(user))
	engine.POST("/password/change", HandlePasswordChangePost)

	form := url.Values{
		"password":             {"wrongpassword"},
		"new_password":         {"newlongpassword1"},
		"new_password_confirm": {"newlongpassword1"},
	}
	req := httptest.NewRequest("POST", "/password/change", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Errorf("wrong current: expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/password/change" {
		t.Errorf("wrong current: expected redirect to /password/change, got %q", loc)
	}
}

// --- HandleMFAVerifyPost ---

func TestHandleMFAVerifyPost_EmptyCode(t *testing.T) {
	engine := newAuthEngine()
	engine.POST("/mfa/verify", HandleMFAVerifyPost)

	form := url.Values{"code": {""}}
	req := httptest.NewRequest("POST", "/mfa/verify", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	// No mfa_user_id in session and no user in context → redirect to /login.
	if w.Code != http.StatusFound {
		t.Errorf("empty code: expected 302, got %d", w.Code)
	}
}

func TestHandleMFAVerifyPost_NoSession_NoUser(t *testing.T) {
	engine := newAuthEngine()
	engine.POST("/mfa/verify", HandleMFAVerifyPost)

	form := url.Values{"code": {"123456"}}
	req := httptest.NewRequest("POST", "/mfa/verify", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	// No mfa_user_id in session, no user in context → redirect to /login.
	if w.Code != http.StatusFound {
		t.Errorf("no session/user: expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Errorf("no session/user: expected redirect to /login, got %q", loc)
	}
}
