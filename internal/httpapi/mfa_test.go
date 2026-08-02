package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	pqtotp "github.com/pquerna/otp/totp"

	"github.com/bryanster/blacklight/internal/authn/challenge"
	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/authn/totp"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// The TOTP endpoints, through the real chain and a real temporary DuckDB
// (M1-006). As in auth_test.go, nothing here reaches past the HTTP layer to set
// something up that a request could have set up instead — the exception is
// ageing a row, which is the only way to reach an expiry without waiting for
// one.

const (
	enrollPath  = BasePath + "/auth/mfa/totp/enroll"
	confirmPath = BasePath + "/auth/mfa/totp/confirm"
	verifyPath  = BasePath + "/auth/mfa/totp/verify"
	totpPath    = BasePath + "/auth/mfa/totp"
)

// codeFor produces the code an authenticator holding secret would show at a
// moment, using the library directly with this application's parameters. A test
// that computed them a second way would be testing its own arithmetic.
func codeFor(t *testing.T, secret string, at time.Time) string {
	t.Helper()

	code, err := pqtotp.GenerateCodeCustom(secret, at, pqtotp.ValidateOpts{
		Period:    uint(totp.Period / time.Second),
		Digits:    totp.Digits,
		Algorithm: totp.Algorithm,
	})
	if err != nil {
		t.Fatalf("generating a code: %v", err)
	}
	return code
}

func codeBody(code string) string { return fmt.Sprintf(`{"code":%q}`, code) }

// del sends a DELETE with a JSON body and the CSRF token belonging to the
// session cookie, which is what a signed-in browser sends.
func (s *authServer) del(target, body string, sess *http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodDelete, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(sess)
	s.attachCSRF(request, sess)
	return do(s.handler, request)
}

// enrol starts an enrolment and returns the secret the response handed out.
func (s *authServer) enrol(t *testing.T, sess *http.Cookie) string {
	t.Helper()

	recorder := s.post(enrollPath, "", sess)
	if recorder.Code != http.StatusOK {
		t.Fatalf("enroll = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}
	return decodeJSON[gen.TOTPEnrolment](t, recorder).Secret
}

// enrolAndConfirm takes an account all the way to having a confirmed
// authenticator, and returns the secret. It signs in to do it, because that is
// the only way: enrolment needs a session.
func (s *authServer) enrolAndConfirm(t *testing.T) string {
	t.Helper()

	sess := s.signIn(t)
	secret := s.enrol(t, sess)

	recorder := s.post(confirmPath, codeBody(codeFor(t, secret, time.Now())), sess)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("confirm = %d, want 204\nbody: %s", recorder.Code, recorder.Body)
	}
	return secret
}

// pendingCookie returns the MFA challenge cookie a response set, failing the
// test if it set none.
func pendingCookie(t *testing.T, recorder *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == challenge.CookieName {
			return cookie
		}
	}
	t.Fatalf("no %s cookie in the response\nheaders: %v", challenge.CookieName, recorder.Header())
	return nil
}

// noSessionCookie fails the test if a response set one.
func noSessionCookie(t *testing.T, recorder *httptest.ResponseRecorder, when string) {
	t.Helper()

	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == session.CookieName && cookie.Value != "" {
			t.Fatalf("%s set a session cookie", when)
		}
	}
}

// --- Enrolling ------------------------------------------------------------------

// TestEnrolmentReturnsAURIAnAppCanScan covers the first acceptance criterion as
// far as a test can: every field an authenticator app reads out of the URI, and
// that the code the URI describes is one this server accepts. Whether a camera
// resolves the image is the one part of it a person has to check — see the
// implementation notes on M1-006.
func TestEnrolmentReturnsAURIAnAppCanScan(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)

	recorder := server.post(enrollPath, "", sess)
	if recorder.Code != http.StatusOK {
		t.Fatalf("enroll = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}
	enrolment := decodeJSON[gen.TOTPEnrolment](t, recorder)

	// The issuer names this deployment, so two installations do not both show up
	// as "Blacklight"; the label carries the address the account was created with.
	const wantLabel = "otpauth://totp/Blacklight%20%28localhost%29:alice@example.com"
	if !strings.HasPrefix(enrolment.OtpauthUri, wantLabel) {
		t.Errorf("otpauthUri = %q,\nwant it to start %q", enrolment.OtpauthUri, wantLabel)
	}
	if !strings.Contains(enrolment.OtpauthUri, "secret="+enrolment.Secret) {
		t.Error("the URI and the secret field disagree; a person typing one in would get a different entry")
	}
	if !strings.HasPrefix(enrolment.QrCode, "data:image/png;base64,") {
		t.Errorf("qrCode = %.40q, want a PNG data URI", enrolment.QrCode)
	}

	// And the code it produces is one this server accepts, which is the only
	// end-to-end statement that the secret handed out is the secret stored.
	if got := server.post(confirmPath,
		codeBody(codeFor(t, enrolment.Secret, time.Now())), sess).Code; got != http.StatusNoContent {
		t.Errorf("confirm with a code from the enrolment = %d, want 204", got)
	}
}

// TestAnUnconfirmedEnrolmentDoesNotGateLogin is the criterion that stops a
// half-finished enrolment locking somebody out.
func TestAnUnconfirmedEnrolmentDoesNotGateLogin(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	server.enrol(t, server.signIn(t))

	// A second sign-in, as though the browser had been closed on the QR code.
	recorder := server.login(testEmail, testPassword)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}
	if got := decodeJSON[gen.LoginResult](t, recorder).Status; got != gen.LoginStatusAuthenticated {
		t.Errorf("status = %q, want %q — an unconfirmed secret gates nothing",
			got, gen.LoginStatusAuthenticated)
	}
	sessionCookie(t, recorder)

	// And it is not reported as an enrolment either.
	me := decodeJSON[gen.CurrentUser](t, server.get(mePath, sessionCookie(t, recorder)))
	if me.Mfa.Enrolled {
		t.Error("mfa.enrolled is true for a secret nobody has confirmed")
	}
}

func TestConfirmingWithAWrongCodeLeavesTheSecretUnconfirmed(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)
	server.enrol(t, sess)

	recorder := server.post(confirmPath, codeBody("000000"), sess)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("confirm with a wrong code = %d, want 400\nbody: %s", recorder.Code, recorder.Body)
	}
	problem := decodeProblem(t, recorder)
	if problem.Errors == nil || len(*problem.Errors) != 1 || (*problem.Errors)[0].Field != "code" {
		t.Errorf("the failure does not name the code field: %+v", problem)
	}

	if decodeJSON[gen.CurrentUser](t, server.get(mePath, sess)).Mfa.Enrolled {
		t.Error("a wrong code confirmed the enrolment")
	}
	// And the login path is untouched.
	if got := decodeJSON[gen.LoginResult](t, server.login(testEmail, testPassword)).Status; got != gen.LoginStatusAuthenticated {
		t.Errorf("status = %q after a failed confirmation, want %q", got, gen.LoginStatusAuthenticated)
	}
}

// TestConfirmingRotatesTheSessionAndSatisfiesMFA: confirming is a privilege
// change, so the token the browser holds changes with it.
func TestConfirmingRotatesTheSessionAndSatisfiesMFA(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)
	secret := server.enrol(t, sess)

	if before := decodeJSON[gen.CurrentUser](t, server.get(mePath, sess)); before.Mfa.Satisfied {
		t.Fatal("a password-only session reports mfa.satisfied")
	}

	recorder := server.post(confirmPath, codeBody(codeFor(t, secret, time.Now())), sess)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("confirm = %d, want 204\nbody: %s", recorder.Code, recorder.Body)
	}

	rotated := sessionCookie(t, recorder)
	if rotated.Value == sess.Value {
		t.Error("the session token did not change; satisfying a factor is a privilege change")
	}
	if got := server.get(mePath, sess).Code; got != http.StatusUnauthorized {
		t.Errorf("the pre-rotation cookie still works (%d), want 401", got)
	}

	after := decodeJSON[gen.CurrentUser](t, server.get(mePath, rotated))
	if !after.Mfa.Satisfied {
		t.Error("mfa.satisfied is false on the session that presented the factor")
	}
	if !after.Mfa.Enrolled {
		t.Error("mfa.enrolled is false after a successful confirmation")
	}
}

// TestASecondEnrolmentIsRefusedWhileOneIsConfirmed: swapping somebody's second
// factor must cost the current password, which enrolment does not ask for.
func TestASecondEnrolmentIsRefusedWhileOneIsConfirmed(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	secret := server.enrolAndConfirm(t)

	sess := server.signInWith(t, secret)
	if got := server.post(enrollPath, "", sess).Code; got != http.StatusConflict {
		t.Errorf("a second enrolment = %d, want 409", got)
	}
}

// --- Signing in with a second factor -------------------------------------------

// TestLoginWithAConfirmedFactorIsPendingAndVerifyCompletesIt is the whole flow
// the ticket asks for: password → pending → verify → session with
// mfa_satisfied.
func TestLoginWithAConfirmedFactorIsPendingAndVerifyCompletesIt(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	user := server.seedUser(t)
	secret := server.enrolAndConfirm(t)

	recorder := server.login(testEmail, testPassword)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}
	result := decodeJSON[gen.LoginResult](t, recorder)
	if result.Status != gen.LoginStatusMfaRequired {
		t.Fatalf("status = %q, want %q", result.Status, gen.LoginStatusMfaRequired)
	}
	if result.User != nil {
		t.Error("the mfa_required response names the user; nobody has been told who they are yet")
	}
	noSessionCookie(t, recorder, "a login awaiting a second factor")
	pending := pendingCookie(t, recorder)

	// The step the confirmation spent is behind us, so the code that completes
	// the sign-in is the next one — exactly as it would be for a person who has
	// just confirmed and is signing in again.
	next := time.Now().Add(totp.Period)
	verified := server.post(verifyPath, codeBody(codeFor(t, secret, next)), pending)
	if verified.Code != http.StatusOK {
		t.Fatalf("verify = %d, want 200\nbody: %s", verified.Code, verified.Body)
	}
	if got := decodeJSON[gen.LoginResult](t, verified).Status; got != gen.LoginStatusAuthenticated {
		t.Errorf("status = %q, want %q", got, gen.LoginStatusAuthenticated)
	}

	sess := sessionCookie(t, verified)
	me := decodeJSON[gen.CurrentUser](t, server.get(mePath, sess))
	switch {
	case me.Id != user.ID:
		t.Errorf("the session belongs to %q, want %q", me.Id, user.ID)
	case !me.Mfa.Satisfied:
		t.Error("mfa.satisfied is false on a session issued by verification")
	}

	// The pending cookie is dropped on the way out, so the browser is not left
	// holding a spent credential.
	if cleared := pendingCookie(t, verified); cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Errorf("the pending cookie was not cleared: %+v", cleared)
	}
}

// TestAPendingTokenIsNotASession is the criterion that the pending state grants
// access to nothing. It is tried both in its own cookie and in the session
// cookie, because "it is a different cookie name" is only half the answer.
func TestAPendingTokenIsNotASession(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	server.enrolAndConfirm(t)

	pending := pendingCookie(t, server.login(testEmail, testPassword))

	tests := map[string]*http.Cookie{
		"in its own cookie":     pending,
		"in the session cookie": {Name: session.CookieName, Value: pending.Value},
	}
	for name, cookie := range tests {
		t.Run(name, func(t *testing.T) {
			if got := server.get(mePath, cookie).Code; got != http.StatusUnauthorized {
				t.Errorf("GET /auth/me with the pending token %s = %d, want 401", name, got)
			}
		})
	}
}

// TestThePendingStateExpires: five minutes, and then the sign-in has to start
// again.
func TestThePendingStateExpires(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	secret := server.enrolAndConfirm(t)

	pending := pendingCookie(t, server.login(testEmail, testPassword))
	server.execSQL(t, `UPDATE app.mfa_challenge SET expires_at = ?`,
		time.Now().Add(-time.Second).UTC())

	recorder := server.post(verifyPath,
		codeBody(codeFor(t, secret, time.Now().Add(totp.Period))), pending)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("verify against an expired pending state = %d, want 401\nbody: %s",
			recorder.Code, recorder.Body)
	}
	noSessionCookie(t, recorder, "an expired verification")
}

// TestAPendingStateIsSpentByTheCodeThatUsesIt: one correct code buys one
// session and no more.
func TestAPendingStateIsSpentByTheCodeThatUsesIt(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	secret := server.enrolAndConfirm(t)

	pending := pendingCookie(t, server.login(testEmail, testPassword))
	first := server.post(verifyPath,
		codeBody(codeFor(t, secret, time.Now().Add(totp.Period))), pending)
	if first.Code != http.StatusOK {
		t.Fatalf("verify = %d, want 200\nbody: %s", first.Code, first.Body)
	}

	// The same pending cookie, a later code — so what is being refused is the
	// spent challenge rather than the spent step.
	second := server.post(verifyPath,
		codeBody(codeFor(t, secret, time.Now().Add(3*totp.Period))), pending)
	if second.Code != http.StatusUnauthorized {
		t.Errorf("a second verification on one pending state = %d, want 401\nbody: %s",
			second.Code, second.Body)
	}
	noSessionCookie(t, second, "a replayed pending state")
}

// TestTheAcceptedWindowIsOneStepEitherSide, at the level a browser sees it. The
// arithmetic has its own table in internal/authn/totp; this is the statement
// that the endpoint applies it.
func TestTheAcceptedWindowIsOneStepEitherSide(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		offset time.Duration
		want   int
	}{
		"one step behind": {-totp.Period, http.StatusOK},
		"one step ahead":  {totp.Period, http.StatusOK},
		"two steps ahead": {2 * totp.Period, http.StatusUnauthorized},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// A server each, and the spent step wound back to nothing:
			// confirming spends the current one, and what this case is about is
			// the skew rather than the replay window — which has its own test
			// below.
			server := newAuthServer(t)
			server.seedUser(t)
			secret := server.enrolAndConfirm(t)
			server.execSQL(t, `UPDATE app.user_totp SET last_used_step = 0`)

			pending := pendingCookie(t, server.login(testEmail, testPassword))
			got := server.post(verifyPath,
				codeBody(codeFor(t, secret, time.Now().Add(tt.offset))), pending).Code
			if got != tt.want {
				t.Errorf("verify with a code %s = %d, want %d", name, got, tt.want)
			}
		})
	}
}

// TestACodeCannotBeUsedTwice is replay protection end to end: the same six
// digits, inside their own thirty seconds, on a fresh pending state.
func TestACodeCannotBeUsedTwice(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	secret := server.enrolAndConfirm(t)

	at := time.Now().Add(totp.Period)
	code := codeFor(t, secret, at)

	pending := pendingCookie(t, server.login(testEmail, testPassword))
	if got := server.post(verifyPath, codeBody(code), pending).Code; got != http.StatusOK {
		t.Fatalf("the first use of a code = %d, want 200", got)
	}

	// A new sign-in, so the challenge is fresh and the only thing standing in
	// the way is the spent step.
	replayed := pendingCookie(t, server.login(testEmail, testPassword))
	recorder := server.post(verifyPath, codeBody(code), replayed)
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("a replayed code = %d, want 401\nbody: %s", recorder.Code, recorder.Body)
	}
	noSessionCookie(t, recorder, "a replayed code")
}

// TestEveryVerificationFailureIsTheSameAnswer: a wrong code, a spent one and no
// pending state at all must be indistinguishable, or the endpoint says how close
// a guess was.
func TestEveryVerificationFailureIsTheSameAnswer(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	server.enrolAndConfirm(t)

	pending := pendingCookie(t, server.login(testEmail, testPassword))

	answers := map[string]gen.Problem{}
	for name, cookie := range map[string]*http.Cookie{
		"a wrong code":      pending,
		"no pending state":  {Name: challenge.CookieName, Value: ""},
		"an invented token": {Name: challenge.CookieName, Value: strings.Repeat("A", 43)},
	} {
		recorder := server.post(verifyPath, codeBody("000000"), cookie)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("verify with %s = %d, want 401\nbody: %s", name, recorder.Code, recorder.Body)
		}
		problem := decodeProblem(t, recorder)
		// The request ID is the one field that is supposed to differ — it is
		// how a support desk finds the log line.
		problem.Instance = nil
		answers[name] = problem
	}

	var first gen.Problem
	var firstName string
	for name, problem := range answers {
		if firstName == "" {
			first, firstName = problem, name
			continue
		}
		if !reflect.DeepEqual(problem, first) {
			t.Errorf("the answer to %s differs from the answer to %s:\n%+v\n%+v",
				name, firstName, problem, first)
		}
	}
}

// --- Throttling -------------------------------------------------------------

// TestVerificationFailuresAreThrottled: six digits is a small space, so the
// same limiter that rations password guesses rations these (M1-004).
func TestVerificationFailuresAreThrottled(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = 3
	})
	server.seedUser(t)
	server.enrolAndConfirm(t)

	pending := pendingCookie(t, server.login(testEmail, testPassword))

	for attempt := range 3 {
		if got := server.post(verifyPath, codeBody("000000"), pending).Code; got != http.StatusUnauthorized {
			t.Fatalf("guess %d = %d, want 401", attempt+1, got)
		}
	}

	recorder := server.post(verifyPath, codeBody("000000"), pending)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("the fourth guess = %d, want 429\nbody: %s", recorder.Code, recorder.Body)
	}
	if recorder.Header().Get("Retry-After") == "" {
		t.Error("the 429 carries no Retry-After")
	}

	// The lockout is the account's, not the endpoint's: the password path is
	// closed too, which is what makes the two limits one budget.
	if got := server.login(testEmail, testPassword).Code; got != http.StatusTooManyRequests {
		t.Errorf("login during an MFA lockout = %d, want 429", got)
	}
}

// TestAPendingLoginDoesNotClearTheLockoutBudget is the hole that opens if a
// login answering mfa_required is counted as a success: somebody holding the
// password could sign in again between every guess and never run out.
func TestAPendingLoginDoesNotClearTheLockoutBudget(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = 3
	})
	server.seedUser(t)
	server.enrolAndConfirm(t)

	// Two failed guesses, then a fresh sign-in with the right password, then a
	// third guess. If the sign-in cleared the count, this would be a 401.
	pending := pendingCookie(t, server.login(testEmail, testPassword))
	for range 2 {
		if got := server.post(verifyPath, codeBody("000000"), pending).Code; got != http.StatusUnauthorized {
			t.Fatalf("a guess = %d, want 401", got)
		}
	}

	pending = pendingCookie(t, server.login(testEmail, testPassword))
	if got := server.post(verifyPath, codeBody("000000"), pending).Code; got != http.StatusUnauthorized {
		t.Fatalf("the third guess = %d, want 401", got)
	}
	if got := server.post(verifyPath, codeBody("000000"), pending).Code; got != http.StatusTooManyRequests {
		t.Errorf("the fourth guess = %d, want 429 — signing in again reset the budget", got)
	}
}

// --- Removing ---------------------------------------------------------------

func TestDisablingAnAuthenticatorNeedsTheCurrentPassword(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	secret := server.enrolAndConfirm(t)
	sess := server.signInWith(t, secret)

	recorder := server.del(totpPath, `{"currentPassword":"not the password"}`, sess)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("disable with the wrong password = %d, want 400\nbody: %s", recorder.Code, recorder.Body)
	}
	if !decodeJSON[gen.CurrentUser](t, server.get(mePath, sess)).Mfa.Enrolled {
		t.Fatal("the wrong password removed the authenticator")
	}

	if got := server.del(totpPath, `{"currentPassword":"`+testPassword+`"}`, sess).Code; got != http.StatusNoContent {
		t.Fatalf("disable with the right password = %d, want 204", got)
	}
	if decodeJSON[gen.CurrentUser](t, server.get(mePath, sess)).Mfa.Enrolled {
		t.Error("mfa.enrolled is still true after the authenticator was removed")
	}

	// And signing in is a password again.
	if got := decodeJSON[gen.LoginResult](t, server.login(testEmail, testPassword)).Status; got != gen.LoginStatusAuthenticated {
		t.Errorf("status = %q after removing the factor, want %q", got, gen.LoginStatusAuthenticated)
	}
}

// TestDisablingIsRefusedWhileMFAIsEnforced: removing the factor would leave an
// account subject to a requirement it can no longer satisfy (M1-008).
func TestDisablingIsRefusedWhileMFAIsEnforced(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	secret := server.enrolAndConfirm(t)
	sess := server.signInWith(t, secret)

	server.execSQL(t, `UPDATE app."user" SET mfa_enforced = TRUE`)

	if got := server.del(totpPath, `{"currentPassword":"`+testPassword+`"}`, sess).Code; got != http.StatusForbidden {
		t.Errorf("disable while MFA is enforced = %d, want 403", got)
	}
	if !decodeJSON[gen.CurrentUser](t, server.get(mePath, sess)).Mfa.Enrolled {
		t.Error("the authenticator was removed anyway")
	}
}

// --- What must never leak ---------------------------------------------------

// TestTheSecretAppearsOnceAndNeverAgain is the acceptance criterion about the
// secret: enrolment is the only response that carries it, and no line of the log
// does.
func TestTheSecretAppearsOnceAndNeverAgain(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	secret := server.enrolAndConfirm(t)
	sess := server.signInWith(t, secret)

	responses := map[string]*httptest.ResponseRecorder{
		"GET /auth/me":     server.get(mePath, sess),
		"POST /auth/login": server.login(testEmail, testPassword),
	}
	for where, recorder := range responses {
		if strings.Contains(recorder.Body.String(), secret) {
			t.Errorf("%s carries the TOTP secret", where)
		}
	}
	if logged := server.logs.String(); strings.Contains(logged, secret) {
		t.Error("the TOTP secret reached the log")
	}

	// Nor is the ciphertext the secret: what is stored must not be the base32
	// string, or "encrypted at rest" would be a comment rather than a fact.
	stored := server.storedTOTPSecret(t)
	if stored == secret {
		t.Error("the secret is stored in the clear")
	}
	if stored == "" {
		t.Error("no ciphertext was stored")
	}
}

// TestThePendingCookieCarriesItsProtections asserts what reaches the browser,
// which is the only place these attributes matter.
func TestThePendingCookieCarriesItsProtections(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	server.enrolAndConfirm(t)

	cookie := pendingCookie(t, server.login(testEmail, testPassword))
	switch {
	case !cookie.HttpOnly:
		t.Error("the pending cookie is readable by script")
	case !cookie.Secure:
		t.Error("the pending cookie is not Secure on a production posture")
	case cookie.SameSite != http.SameSiteStrictMode:
		t.Errorf("SameSite = %v, want Strict", cookie.SameSite)
	case cookie.Path != BasePath+mfaPathPrefix:
		t.Errorf("Path = %q, want %q — it is presented to the MFA endpoints and nothing else",
			cookie.Path, BasePath+mfaPathPrefix)
	case cookie.MaxAge <= 0 || cookie.MaxAge > int((5*time.Minute)/time.Second):
		t.Errorf("MaxAge = %d, want the pending window", cookie.MaxAge)
	}
}

// --- Helpers that need the database -----------------------------------------

// signInWith completes a whole sign-in for an account that has a confirmed
// factor, and returns the session cookie.
func (s *authServer) signInWith(t *testing.T, secret string) *http.Cookie {
	t.Helper()

	pending := pendingCookie(t, s.login(testEmail, testPassword))
	recorder := s.post(verifyPath, codeBody(codeFor(t, secret, time.Now().Add(totp.Period))), pending)
	if recorder.Code != http.StatusOK {
		t.Fatalf("verify = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}
	return sessionCookie(t, recorder)
}

// storedTOTPSecret reads the ciphertext column directly, which is the only way
// to assert that what is on disk is not the secret.
func (s *authServer) storedTOTPSecret(t *testing.T) string {
	t.Helper()

	stored, err := identity.NewTOTPs(s.db).ByUserID(t.Context(), s.userID(t))
	if err != nil {
		t.Fatalf("reading the stored enrolment: %v", err)
	}
	return stored.SecretEncrypted
}
