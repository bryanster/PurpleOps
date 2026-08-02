package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Login throttling through the real chain (M1-004). The limiter's own behaviour
// — the doubling, the eviction, the exactness under load — is tested against an
// injected clock in internal/authn/throttle; what is being tested here is that a
// browser meets it, and what it is told when it does.

// throttled asserts that a response is the 429 this API sends, and returns the
// Retry-After it carries.
func throttled(t *testing.T, recorder *httptest.ResponseRecorder, what string) time.Duration {
	t.Helper()

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("%s = %d, want 429\nbody: %s", what, recorder.Code, recorder.Body)
	}
	if got, want := recorder.Header().Get("Content-Type"), "application/problem+json"; got != want {
		t.Errorf("%s Content-Type = %q, want %q", what, got, want)
	}
	if !strings.Contains(recorder.Body.String(), string(gen.ProblemCodeRateLimited)) {
		t.Errorf("%s does not carry the %q code: %s", what, gen.ProblemCodeRateLimited, recorder.Body)
	}

	header := recorder.Header().Get("Retry-After")
	if header == "" {
		t.Fatalf("%s carries no Retry-After; a client has nothing to wait for", what)
	}
	seconds, err := strconv.Atoi(header)
	if err != nil {
		t.Fatalf("%s has Retry-After %q, want whole seconds: %v", what, header, err)
	}
	if seconds < 1 {
		t.Errorf("%s has Retry-After %d, want at least 1", what, seconds)
	}
	return time.Duration(seconds) * time.Second
}

// failLogin sends n wrong passwords for email, insisting each is answered 401 —
// which is what makes the first 429 after them meaningful.
func failLogin(t *testing.T, server *authServer, email string, n int) {
	t.Helper()

	for i := range n {
		recorder := server.login(email, "not the right password")
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("failure %d of %d for %s = %d, want 401\nbody: %s",
				i+1, n, email, recorder.Code, recorder.Body)
		}
	}
}

// TestRepeatedFailuresLockTheAccountOut is the regression guard the ticket
// exists for: v1 had login rate limiting, lost it in a refactor, and nothing
// noticed. Delete throttleCredentials from the chain in
// internal/httpapi/server.go and this test fails.
func TestRepeatedFailuresLockTheAccountOut(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = 3
	})
	server.seedUser(t)

	failLogin(t, server, testEmail, 3)

	wait := throttled(t, server.login(testEmail, "not the right password"), "the fourth failure")
	if wait > 15*time.Minute {
		t.Errorf("Retry-After = %v, want no more than the configured lockout", wait)
	}

	// The whole point. A lockout that the right password ends is not a lockout —
	// an attacker who finds the password on the fourth guess must not be let in
	// on the fifth.
	recorder := server.login(testEmail, testPassword)
	throttled(t, recorder, "the right password during the lockout")
	for _, cookie := range recorder.Result().Cookies() {
		t.Errorf("the throttled response set a %s cookie", cookie.Name)
	}
}

// TestTheLockoutEndsWhenItSaysItWill runs a whole cooldown, which is the one
// thing in this file that has to happen in real time. It is configured to half a
// second: the throttled attempt in the middle is refused before the handler
// runs and therefore costs no password hash, so the gap between the lockout
// starting and that assertion is microseconds, and the sleep after it can only
// ever be too long. The doubling and the eviction, which cannot be reached this
// way at all, are unit-tested against an injected clock instead.
func TestTheLockoutEndsWhenItSaysItWill(t *testing.T) {
	t.Parallel()

	const lockout = 500 * time.Millisecond
	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = 2
		cfg.Throttle.AccountLockout = lockout
	})
	server.seedUser(t)

	failLogin(t, server, testEmail, 2)
	throttled(t, server.login(testEmail, testPassword), "the attempt during the lockout")

	time.Sleep(lockout)

	cookie := server.signIn(t)
	if cookie.Value == "" {
		t.Error("the session cookie issued after the lockout is empty")
	}

	// And the count went with it: one failure after signing in is not the second
	// of a run that started before the lockout.
	failLogin(t, server, testEmail, 1)
	if recorder := server.login(testEmail, testPassword); recorder.Code != http.StatusOK {
		t.Errorf("signing in after a single failure = %d, want 200\nbody: %s",
			recorder.Code, recorder.Body)
	}
}

func TestASuccessfulSignInClearsTheFailureCount(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = 3
	})
	server.seedUser(t)

	failLogin(t, server, testEmail, 2)
	server.signIn(t)

	// Without the reset, the second of these would be the third failure in a row
	// and would lock the account.
	failLogin(t, server, testEmail, 2)
	if recorder := server.login(testEmail, testPassword); recorder.Code != http.StatusOK {
		t.Errorf("signing in = %d, want 200 — the count did not reset\nbody: %s",
			recorder.Code, recorder.Body)
	}
}

// TestOneAccountsLockoutLeavesTheOthersAlone is the difference between throttling
// and an outage: locking one address must not sign the rest of the team out.
func TestOneAccountsLockoutLeavesTheOthersAlone(t *testing.T) {
	t.Parallel()

	const otherEmail = "bob@example.com"
	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = 3
	})
	server.seedUser(t)
	server.seedUser(t, func(u *identity.NewUser) { u.Email = otherEmail })

	failLogin(t, server, testEmail, 3)
	throttled(t, server.login(testEmail, testPassword), "the locked account")

	if recorder := server.login(otherEmail, testPassword); recorder.Code != http.StatusOK {
		t.Errorf("the untouched account = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}
}

// TestOneSourceSprayingManyAccountsIsLockedOut is the attack the per-account
// limiter cannot see: one password against a hundred addresses, never twice
// against the same one.
func TestOneSourceSprayingManyAccountsIsLockedOut(t *testing.T) {
	t.Parallel()

	const spray = 6
	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.SourceFailures = spray
	})
	server.seedUser(t)

	for i := range spray {
		failLogin(t, server, fmt.Sprintf("victim%d@example.com", i), 1)
	}

	// An address this source has never tried, and an account that exists: both
	// are refused, because it is the source that has run out.
	throttled(t, server.login("victim99@example.com", testPassword), "a fresh address from the same source")
	throttled(t, server.login(testEmail, testPassword), "the real account from the same source")
}

// TestTheThrottleTellsARealAccountAndAnInventedOneApartNoBetterThanTheLoginDoes
// is the enumeration defence, extended to the 429. M1-003 went to some trouble
// to make every failed login byte-identical; a throttle that answered a real
// address differently from an invented one would hand back the oracle.
func TestTheThrottleTellsARealAccountAndAnInventedOneApartNoBetterThanTheLoginDoes(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = 2
	})
	server.seedUser(t)

	failLogin(t, server, testEmail, 2)
	failLogin(t, server, "nobody@example.com", 2)

	real := server.login(testEmail, testPassword)
	invented := server.login("nobody@example.com", testPassword)
	throttled(t, real, "the real account")
	throttled(t, invented, "the invented account")

	if got, want := withoutInstance(t, invented), withoutInstance(t, real); got != want {
		t.Errorf("the invented account answers\n  %s\nand the real one answers\n  %s\n"+
			"— the two are distinguishable", got, want)
	}
	// Retry-After is deliberately not compared. It counts down from when each
	// lockout began, which is a fact about when this test sent its requests and
	// not about whether the account exists — the two took the same path through
	// the limiter to get here, which is what the body proves.
}

// TestOnlyTheCredentialEndpointsAreThrottled keeps the middleware off the rest of
// the API. Counting a 401 from an expired session as a guessed password would
// lock a whole office out at the end of the working day.
func TestOnlyTheCredentialEndpointsAreThrottled(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = 2
		cfg.Throttle.SourceFailures = 2
	})

	for i := range 10 {
		if recorder := server.get(mePath); recorder.Code != http.StatusUnauthorized {
			t.Fatalf("request %d to %s = %d, want 401 every time\nbody: %s",
				i+1, mePath, recorder.Code, recorder.Body)
		}
	}
}

// TestTheThrottleLeavesTheBodyForTheHandler is the one thing this middleware
// could break for every request that is not throttled at all: it reads the body
// to find out which account is being attempted, and has to put it back.
func TestTheThrottleLeavesTheBodyForTheHandler(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	user := server.seedUser(t)

	recorder := server.login(testEmail, testPassword)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}
	if !strings.Contains(recorder.Body.String(), user.ID) {
		t.Errorf("the login response does not describe the user who signed in: %s", recorder.Body)
	}
}
