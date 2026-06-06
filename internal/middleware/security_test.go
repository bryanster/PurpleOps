package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newSecurityEngine() *gin.Engine {
	r := gin.New()
	r.Use(SecurityHeaders)
	r.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r
}

func TestSecurityHeaders_XContentTypeOptions(t *testing.T) {
	w := httptest.NewRecorder()
	newSecurityEngine().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if v := w.Header().Get("X-Content-Type-Options"); v != "nosniff" {
		t.Errorf("X-Content-Type-Options: got %q, want nosniff", v)
	}
}

func TestSecurityHeaders_XFrameOptions(t *testing.T) {
	w := httptest.NewRecorder()
	newSecurityEngine().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if v := w.Header().Get("X-Frame-Options"); v != "DENY" {
		t.Errorf("X-Frame-Options: got %q, want DENY", v)
	}
}

func TestSecurityHeaders_ReferrerPolicy(t *testing.T) {
	w := httptest.NewRecorder()
	newSecurityEngine().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if v := w.Header().Get("Referrer-Policy"); v != "strict-origin-when-cross-origin" {
		t.Errorf("Referrer-Policy: got %q", v)
	}
}

func TestSecurityHeaders_CSP(t *testing.T) {
	w := httptest.NewRecorder()
	newSecurityEngine().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	v := w.Header().Get("Content-Security-Policy")
	if v == "" {
		t.Fatal("Content-Security-Policy header missing")
	}
	if !strings.Contains(v, "default-src 'self'") {
		t.Errorf("CSP missing default-src 'self': %q", v)
	}
}

func TestSecurityHeaders_PermissionsPolicy(t *testing.T) {
	w := httptest.NewRecorder()
	newSecurityEngine().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if v := w.Header().Get("Permissions-Policy"); v == "" {
		t.Error("Permissions-Policy header missing")
	}
}

func TestSecurityHeaders_NoHSTSOnHTTP(t *testing.T) {
	w := httptest.NewRecorder()
	newSecurityEngine().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if v := w.Header().Get("Strict-Transport-Security"); v != "" {
		t.Errorf("HSTS should not be set on plain HTTP, got %q", v)
	}
}

func TestSecurityHeaders_HSTSOnHTTPS(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.TLS = &tls.ConnectionState{}

	w := httptest.NewRecorder()
	newSecurityEngine().ServeHTTP(w, req)

	v := w.Header().Get("Strict-Transport-Security")
	if v == "" {
		t.Fatal("Strict-Transport-Security header missing on HTTPS")
	}
	if !strings.Contains(v, "max-age=") {
		t.Errorf("HSTS missing max-age directive: %q", v)
	}
	if !strings.Contains(v, "includeSubDomains") {
		t.Errorf("HSTS missing includeSubDomains: %q", v)
	}
}

func TestSecurityHeaders_CallsNext(t *testing.T) {
	reached := false
	r := gin.New()
	r.Use(SecurityHeaders)
	r.GET("/", func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))

	if !reached {
		t.Error("SecurityHeaders did not call c.Next()")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
