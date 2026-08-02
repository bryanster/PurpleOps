package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The derivation and the cookie. What the middleware does with them is tested
// in internal/httpapi/csrf_test.go, against the real chain.

// newIssuedToken returns a real token, because the derivation refuses anything
// that is not one.
func newIssuedToken(t *testing.T) Token {
	t.Helper()

	token, err := newToken()
	if err != nil {
		t.Fatalf("newToken: %v", err)
	}
	return token
}

// TestTheCSRFTokenIsNotTheStoredHash is the one that matters most in this file.
//
// The CSRF token is handed to script in a readable cookie and the token hash is
// the database's lookup key for a live session. If the two derivations were the
// same function, any XSS — or anyone who saw one request — would hold the value
// that finds a session row. The domain separator is what keeps them apart, and
// it is the sort of line a later refactor deletes as redundant.
func TestTheCSRFTokenIsNotTheStoredHash(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	token := newIssuedToken(t)

	if manager.CSRFToken(token) == manager.hash(token) {
		t.Error("the CSRF token equals the stored token hash; a readable cookie now carries the database's lookup key")
	}
}

func TestTheCSRFTokenIsDerivedFromTheSessionTokenAndTheSecret(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	token := newIssuedToken(t)
	derived := manager.CSRFToken(token)

	if derived == "" {
		t.Fatal("a real token derived nothing")
	}
	if again := manager.CSRFToken(token); again != derived {
		t.Error("the same token derived two different CSRF tokens; the middleware could never recompute one")
	}

	// Rotation replaces the session token (M1-003), so this is what "the CSRF
	// token rotates with the session" amounts to: a different token, a
	// different CSRF token, with nothing to remember to update.
	if rotated := manager.CSRFToken(newIssuedToken(t)); rotated == derived {
		t.Error("a different session token derived the same CSRF token")
	}

	// A different deployment secret means a different value, which is what
	// stops an attacker who knows the algorithm from computing one.
	other := newTestManager(t, func(o *Options) { o.Secret = []byte("a-different-secret-of-32-bytes!!") })
	if got := other.CSRFToken(token); got == derived {
		t.Error("the derivation ignores the session secret")
	}
}

// TestAMalformedTokenDerivesNothing: "" is the value that can never match a
// header, so a request with a junk session cookie fails the CSRF check as well
// as the authentication one.
func TestAMalformedTokenDerivesNothing(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	for name, token := range map[string]Token{
		"empty":     "",
		"too short": "short",
		"too long":  Token(string(newIssuedToken(t)) + "trailing"),
	} {
		if got := manager.CSRFToken(token); got != "" {
			t.Errorf("CSRFToken(%s) = %q, want empty", name, got)
		}
	}
}

// TestTheCookieFlagsAreNotSwappedOver is the classic mistake, asserted from
// both sides: the session cookie must be unreadable by script and the CSRF
// cookie must be readable, and getting that backwards would leave both tests
// that only check one of them passing.
func TestTheCookieFlagsAreNotSwappedOver(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	token := newIssuedToken(t)
	sessionCookie := manager.Cookie(token, time.Now().Add(time.Hour))
	csrfCookie := manager.CSRFCookie(manager.CSRFToken(token))

	switch {
	case !sessionCookie.HttpOnly:
		t.Error("the session cookie is not HttpOnly; script could read the session token")
	case csrfCookie.HttpOnly:
		t.Error("the CSRF cookie is HttpOnly; the SPA cannot read it, so no request could ever carry the header")
	case csrfCookie.Name != CSRFCookieName:
		t.Errorf("Name = %q, want %q", csrfCookie.Name, CSRFCookieName)
	case !csrfCookie.Secure:
		t.Error("the CSRF cookie is not Secure on a deployment that is not development")
	case csrfCookie.SameSite != http.SameSiteStrictMode:
		t.Errorf("SameSite = %v, want Strict", csrfCookie.SameSite)
	case csrfCookie.Path != "/":
		t.Errorf("Path = %q, want %q", csrfCookie.Path, "/")
	case csrfCookie.Domain != "":
		t.Errorf("Domain = %q, want empty", csrfCookie.Domain)
	}
}

func TestTheClearingCSRFCookieMatchesTheOneItRemoves(t *testing.T) {
	t.Parallel()

	manager := newTestManager(t)
	live := manager.CSRFCookie("a-token")
	cleared := manager.ClearCSRFCookie()

	switch {
	case cleared.Name != live.Name || cleared.Path != live.Path:
		t.Error("the clearing cookie has a different name or path, so a browser keeps the original")
	case cleared.Secure != live.Secure || cleared.HttpOnly != live.HttpOnly || cleared.SameSite != live.SameSite:
		t.Error("the clearing cookie's attributes differ from the cookie being cleared")
	case cleared.Value != "":
		t.Errorf("Value = %q, want empty", cleared.Value)
	case cleared.MaxAge >= 0:
		t.Errorf("MaxAge = %d, want a negative value, which is how a cookie is deleted", cleared.MaxAge)
	}
}

func TestOnlyDevelopmentDropsTheSecureAttributeFromTheCSRFCookie(t *testing.T) {
	t.Parallel()

	insecure := newTestManager(t, func(o *Options) { o.Secure = false })
	if insecure.CSRFCookie("a-token").Secure {
		t.Error("Secure was set although the deployment asked for it not to be")
	}
	if insecure.ClearCSRFCookie().Secure {
		t.Error("the clearing cookie disagrees with the one it has to match")
	}
}

func TestCSRFFromRequestReadsTheCookieAndToleratesItsAbsence(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	if got := CSRFFromRequest(request); got != "" {
		t.Errorf("CSRFFromRequest() with no cookie = %q, want empty", got)
	}

	request.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "the-csrf-token"})
	if got := CSRFFromRequest(request); got != "the-csrf-token" {
		t.Errorf("CSRFFromRequest() = %q, want %q", got, "the-csrf-token")
	}
}
