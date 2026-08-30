package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/config"
)

// TestAccountLockoutIsNotBypassedByAMalformedBearerHeader guards the
// per-account sign-in lockout (M1-004) against the malformed-bearer bypass.
//
// credentialAttempt must key sign-in routes by the account named in the body or
// behind the pending cookie, never by a caller-supplied bearer token. A
// malformed `Authorization: Bearer bl_x` header maps to an empty token prefix,
// which the account limiter ignores — so if the bearer header took precedence,
// a caller could brute-force a password (or a six-digit TOTP code) with only
// the per-source limiter in the way.
func TestAccountLockoutIsNotBypassedByAMalformedBearerHeader(t *testing.T) {
	const failures = 3
	const bearer = "Bearer bl_x"

	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = failures
	})
	server.seedUser(t)

	// `failures` wrong passwords, each carrying a malformed bearer header, must
	// still count against the account: the fourth attempt — even with the
	// correct password — is refused 429 before the handler runs.
	for range failures {
		rec := loginWithAuthorization(t, server, testEmail, "not the right password", bearer)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("wrong password with bearer = %d, want 401\nbody: %s", rec.Code, rec.Body)
		}
	}
	rec := loginWithAuthorization(t, server, testEmail, testPassword, bearer)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("fourth attempt with bearer = %d, want 429 (the lockout must engage)\nbody: %s",
			rec.Code, rec.Body)
	}
}

// loginWithAuthorization posts a login body carrying an Authorization header.
func loginWithAuthorization(t *testing.T, server *authServer, email, password, authorization string) *httptest.ResponseRecorder {
	t.Helper()

	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, password)
	req := httptest.NewRequest(http.MethodPost, loginPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authorization)
	return do(server.handler, req)
}
