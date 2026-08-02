package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authn/challenge"
	"github.com/bryanster/blacklight/internal/authn/recovery"
	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Recovery codes end to end (M1-007), through the real chain and a real
// temporary DuckDB. As in mfa_test.go next door, nothing here reaches past the
// HTTP layer to set something up that a request could have set up instead — a
// code that a test minted by hand would not prove that the codes a person is
// shown are the codes the server checks.

const (
	recoveryVerifyPath     = BasePath + "/auth/mfa/recovery/verify"
	recoveryRegeneratePath = BasePath + "/auth/mfa/recovery/regenerate"
)

func recoveryBody(code string) string { return fmt.Sprintf(`{"code":%q}`, code) }

func currentPasswordBody(plaintext string) string {
	return fmt.Sprintf(`{"currentPassword":%q}`, plaintext)
}

// signInWithRecoveryCode signs in with a password and then a printed code,
// which is the whole of the flow this ticket adds.
func (s *authServer) signInWithRecoveryCode(t *testing.T, code string) *httptest.ResponseRecorder {
	t.Helper()

	pending := pendingCookie(t, s.login(testEmail, testPassword))
	return s.post(recoveryVerifyPath, recoveryBody(code), pending)
}

// remainingCodes reads the count off the profile, which is where an interface
// reads it.
func (s *authServer) remainingCodes(t *testing.T, sess *http.Cookie) int {
	t.Helper()

	return decodeJSON[gen.CurrentUser](t, s.get(mePath, sess)).Mfa.RecoveryCodesRemaining
}

// --- Being given a set ------------------------------------------------------

// TestConfirmingAnEnrolmentIssuesTenCodesOnce is the criterion that codes are
// shown exactly once: minted by the confirmation, and unreachable afterwards
// because there is no endpoint that returns them.
func TestConfirmingAnEnrolmentIssuesTenCodesOnce(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	secret, codes := server.enrolAndConfirmWithCodes(t)

	if len(codes) != recovery.SetSize {
		t.Fatalf("%d codes, want %d", len(codes), recovery.SetSize)
	}
	for _, code := range codes {
		if _, err := recovery.Parse(code); err != nil {
			t.Errorf("the response handed out %q, which is not a code it would accept back: %v", code, err)
		}
	}

	sess := server.signInWith(t, secret)
	if got := server.remainingCodes(t, sess); got != recovery.SetSize {
		t.Errorf("mfa.recoveryCodesRemaining = %d, want %d", got, recovery.SetSize)
	}

	// And there is nothing to read them back with. Every route the server
	// serves is walked, so this cannot be satisfied by a list somebody forgets
	// to extend: if a future endpoint returns a live code, it fails here.
	assertNoEndpointReturnsACode(t, server, sess, codes)
}

// assertNoEndpointReturnsACode walks every GET the server has and fails if any
// of them carries one of the codes.
func assertNoEndpointReturnsACode(t *testing.T, server *authServer, sess *http.Cookie, codes []string) {
	t.Helper()

	for _, target := range []string{mePath, recoveryVerifyPath, recoveryRegeneratePath, enrollPath} {
		body := server.get(target, sess).Body.String()
		for _, code := range codes {
			if strings.Contains(body, code) || strings.Contains(body, canonicalOf(t, code)) {
				t.Errorf("GET %s carried a recovery code", target)
			}
		}
	}

	// The log is the other place a credential leaks to, and the one nobody
	// looks at until afterwards.
	logged := server.logs.String()
	for _, code := range codes {
		if strings.Contains(logged, code) || strings.Contains(logged, canonicalOf(t, code)) {
			t.Error("a recovery code reached the log")
		}
	}
}

func canonicalOf(t *testing.T, printed string) string {
	t.Helper()

	code, err := recovery.Parse(printed)
	if err != nil {
		t.Fatalf("Parse(%q): %v", printed, err)
	}
	return code.Reveal()
}

// TestWhatIsStoredIsNotTheCode. A copy of the database must not be a set of
// working codes — the same promise the TOTP secret makes, by a different
// construction.
func TestWhatIsStoredIsNotTheCode(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	_, codes := server.enrolAndConfirmWithCodes(t)

	stored, err := identity.NewRecoveryCodes(server.db).Unused(t.Context(), server.userID(t))
	if err != nil {
		t.Fatalf("reading the stored codes: %v", err)
	}
	if len(stored) != len(codes) {
		t.Fatalf("%d rows for %d codes", len(stored), len(codes))
	}

	for _, row := range stored {
		for _, code := range codes {
			if row.CodeHash == code || row.CodeHash == canonicalOf(t, code) {
				t.Fatal("the code itself is in code_hash")
			}
		}
	}
}

// --- Using one --------------------------------------------------------------

// TestARecoveryCodeSignsYouAllTheWayIn is the acceptance criterion that the
// session is genuinely MFA-satisfied and not a half-session: somebody holding a
// printed code has presented a second factor, and asking them for the
// authenticator they have just replaced would defeat the point.
func TestARecoveryCodeSignsYouAllTheWayIn(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	_, codes := server.enrolAndConfirmWithCodes(t)

	recorder := server.signInWithRecoveryCode(t, codes[0])
	if recorder.Code != http.StatusOK {
		t.Fatalf("verify with a recovery code = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}
	if got := decodeJSON[gen.LoginResult](t, recorder).Status; got != gen.LoginStatusAuthenticated {
		t.Errorf("status = %q, want %q", got, gen.LoginStatusAuthenticated)
	}

	sess := sessionCookie(t, recorder)
	me := decodeJSON[gen.CurrentUser](t, server.get(mePath, sess))
	if !me.Mfa.Satisfied {
		t.Error("a session created with a recovery code reports mfa.satisfied false")
	}
	if !me.Mfa.Enrolled {
		t.Error("using a recovery code removed the enrolment; it should not touch it")
	}
	if got := me.Mfa.RecoveryCodesRemaining; got != recovery.SetSize-1 {
		t.Errorf("mfa.recoveryCodesRemaining = %d after using one, want %d", got, recovery.SetSize-1)
	}
}

// TestACodeCannotBeUsedTwice is the reuse criterion, at the level a person
// experiences it.
func TestARecoveryCodeCannotBeUsedTwice(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	_, codes := server.enrolAndConfirmWithCodes(t)

	if got := server.signInWithRecoveryCode(t, codes[0]).Code; got != http.StatusOK {
		t.Fatalf("the first use of a code = %d, want 200", got)
	}
	if got := server.signInWithRecoveryCode(t, codes[0]).Code; got != http.StatusUnauthorized {
		t.Errorf("the second use of the same code = %d, want 401", got)
	}

	// The other nine are untouched: one spent code is one spent code.
	if got := server.signInWithRecoveryCode(t, codes[1]).Code; got != http.StatusOK {
		t.Errorf("a different code = %d after one was spent, want 200", got)
	}
}

// TestACodeIsAcceptedHoweverItWasWrittenDown. The forgiving parse is not a
// convenience, it is the difference between a working recovery path and one
// that fails at the moment it is needed.
func TestACodeIsAcceptedHoweverItWasWrittenDown(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	_, codes := server.enrolAndConfirmWithCodes(t)

	written := map[string]string{
		"as printed":  codes[0],
		"lower case":  strings.ToLower(codes[1]),
		"no hyphens":  strings.ReplaceAll(codes[2], "-", ""),
		"with spaces": strings.ReplaceAll(codes[3], "-", " "),
	}
	for name, typed := range written {
		if got := server.signInWithRecoveryCode(t, typed).Code; got != http.StatusOK {
			t.Errorf("a code typed %s = %d, want 200", name, got)
		}
	}
}

// TestEveryRecoveryFailureIsTheSameAnswer. "That was a real code, but you have
// already used it" is a much smaller search space than "no", and the same
// argument the TOTP endpoint makes applies here.
func TestEveryRecoveryFailureIsTheSameAnswer(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	_, codes := server.enrolAndConfirmWithCodes(t)

	// One code is spent up front, so "already used" is a real state rather than
	// a hypothetical one. It also completes a sign-in, which clears the failure
	// budget before the guesses below start spending it.
	if got := server.signInWithRecoveryCode(t, codes[0]).Code; got != http.StatusOK {
		t.Fatalf("spending a code first = %d, want 200", got)
	}

	pending := pendingCookie(t, server.login(testEmail, testPassword))

	attempts := map[string]struct {
		code   string
		cookie *http.Cookie
	}{
		"a code nobody holds": {"0000-0000-0000-0000-0000", pending},
		"a code already used": {codes[0], pending},
		"no pending state":    {codes[1], &http.Cookie{Name: challenge.CookieName, Value: ""}},
		"an invented token":   {codes[1], &http.Cookie{Name: challenge.CookieName, Value: strings.Repeat("A", 43)}},
	}

	answers := map[string]gen.Problem{}
	for name, attempt := range attempts {
		recorder := server.post(recoveryVerifyPath, recoveryBody(attempt.code), attempt.cookie)
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("verify with %s = %d, want 401\nbody: %s", name, recorder.Code, recorder.Body)
		}
		problem := decodeProblem(t, recorder)
		// The request ID is the one field that is supposed to differ — it is
		// how a support desk finds the log line.
		problem.Instance = nil
		answers[name] = problem
	}

	var (
		first     gen.Problem
		firstName string
	)
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

// TestRecoveryVerificationFailuresAreThrottled. The endpoint is reachable
// before authentication and is another way past a second factor, so it is
// rationed by the same limiter and against the same budget as the password and
// TOTP paths (M1-004).
func TestRecoveryVerificationFailuresAreThrottled(t *testing.T) {
	t.Parallel()

	const failures = 3
	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = failures
	})
	server.seedUser(t)
	_, codes := server.enrolAndConfirmWithCodes(t)

	// One pending state, guessed against repeatedly — which is the attack. A
	// wrong code does not spend the challenge, so the only thing standing
	// between a guesser and the whole set is the limiter.
	pending := pendingCookie(t, server.login(testEmail, testPassword))
	for attempt := 1; attempt <= failures; attempt++ {
		got := server.post(recoveryVerifyPath, recoveryBody("0000-0000-0000-0000-0000"), pending).Code
		if got != http.StatusUnauthorized {
			t.Fatalf("guess %d = %d, want 401", attempt, got)
		}
	}

	// And now even a code that is genuinely theirs is refused, because a
	// lockout the right credential ends is not a lockout.
	recorder := server.post(recoveryVerifyPath, recoveryBody(codes[0]), pending)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("a correct code after %d failures = %d, want 429\nbody: %s",
			failures, recorder.Code, recorder.Body)
	}
	if recorder.Header().Get("Retry-After") == "" {
		t.Error("the 429 carries no Retry-After")
	}

	// The lockout is the account's rather than the endpoint's: guessing codes
	// here closes the password path and the TOTP path too, so the two ways past
	// a second factor share one budget instead of doubling it.
	if got := server.login(testEmail, testPassword).Code; got != http.StatusTooManyRequests {
		t.Errorf("login during a recovery-code lockout = %d, want 429", got)
	}
}

// TestAMalformedCodeIsRefusedByTheValidator: what is not a code at all does not
// need to reach the handler, and a body the specification rejects is not
// counted as a guess.
func TestAMalformedCodeIsRefusedByTheValidator(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	server.enrolAndConfirmWithCodes(t)

	pending := pendingCookie(t, server.login(testEmail, testPassword))
	if got := server.post(recoveryVerifyPath, recoveryBody("nope"), pending).Code; got != http.StatusBadRequest {
		t.Errorf("a four-character code = %d, want 400", got)
	}
}

// --- Replacing a set --------------------------------------------------------

// TestRegeneratingInvalidatesEveryOutstandingCode is the acceptance criterion,
// and the reason somebody regenerates at all.
func TestRegeneratingInvalidatesEveryOutstandingCode(t *testing.T) {
	t.Parallel()

	// Ten dead codes are ten refused sign-ins, which the default budget would
	// lock the account out partway through. The lockout has its own test; this
	// one is about which codes still work, so it is given room to check all of
	// them rather than a representative three.
	server := newAuthServer(t, func(cfg *config.Config) {
		cfg.Throttle.AccountFailures = 2 * recovery.SetSize
	})
	server.seedUser(t)
	secret, old := server.enrolAndConfirmWithCodes(t)

	sess := server.signInWith(t, secret)
	recorder := server.post(recoveryRegeneratePath, currentPasswordBody(testPassword), sess)
	if recorder.Code != http.StatusOK {
		t.Fatalf("regenerate = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}

	fresh := decodeJSON[gen.RecoveryCodes](t, recorder).Codes
	if len(fresh) != recovery.SetSize {
		t.Fatalf("%d codes, want %d", len(fresh), recovery.SetSize)
	}
	for _, was := range old {
		for _, is := range fresh {
			if was == is {
				t.Error("a regenerated set repeated a code from the previous one")
			}
		}
	}

	// Every old code is dead — the unused ones especially, which is the half
	// that would otherwise still be lying in whatever drawer prompted this.
	for i, was := range old {
		if got := server.signInWithRecoveryCode(t, was).Code; got != http.StatusUnauthorized {
			t.Errorf("the old code %d = %d after regenerating, want 401", i, got)
		}
	}
	// And a new one works.
	if got := server.signInWithRecoveryCode(t, fresh[0]).Code; got != http.StatusOK {
		t.Errorf("a freshly minted code = %d, want 200", got)
	}
}

// TestRegeneratingNeedsTheCurrentPassword. A session left open on a shared
// machine must not be enough to mint credentials that walk past a second
// factor.
func TestRegeneratingNeedsTheCurrentPassword(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	secret, _ := server.enrolAndConfirmWithCodes(t)
	sess := server.signInWith(t, secret)

	if got := server.post(recoveryRegeneratePath,
		currentPasswordBody("not the password"), sess).Code; got != http.StatusBadRequest {
		t.Errorf("regenerate with a wrong password = %d, want 400", got)
	}
	// And nothing was minted: the count is still the original set.
	if got := server.remainingCodes(t, sess); got != recovery.SetSize {
		t.Errorf("mfa.recoveryCodesRemaining = %d after a refused regeneration, want %d",
			got, recovery.SetSize)
	}
}

// TestRegeneratingNeedsASatisfiedSession is the second factor half of the
// requirement — and, in the same test, the reason it is stated as "this session
// satisfied MFA" rather than "present a code from the authenticator": somebody
// who signed in *with a recovery code* can replace their codes, which is
// exactly the person who needs to.
func TestRegeneratingNeedsASatisfiedSession(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)

	// A password-only session, opened before there was a factor to satisfy —
	// which is the only way one can exist alongside a confirmed enrolment, since
	// confirming satisfies the session it was done from and every later sign-in
	// has to present a code.
	plain := server.signIn(t)
	_, codes := server.enrolAndConfirmWithCodes(t)

	if got := server.post(recoveryRegeneratePath, currentPasswordBody(testPassword), plain).Code; got != http.StatusForbidden {
		t.Errorf("regenerate from a password-only session = %d, want 403", got)
	}

	// And the person whose phone is gone: in with a recovery code, out with a
	// new set.
	recovered := sessionCookie(t, server.signInWithRecoveryCode(t, codes[0]))
	if got := server.post(recoveryRegeneratePath, currentPasswordBody(testPassword), recovered).Code; got != http.StatusOK {
		t.Errorf("regenerate from a session created with a recovery code = %d, want 200", got)
	}
}

// TestRegeneratingIsRefusedWithNothingEnrolled: codes stand in for an
// authenticator, so there has to be one to stand in for.
//
// The state is reachable and not hypothetical — a session that satisfied MFA
// and then removed the factor keeps its satisfied flag, because that flag
// records what happened at sign-in rather than what is true now. Without the
// check, such a session could mint a set of credentials outliving the thing
// they replace.
func TestRegeneratingIsRefusedWithNothingEnrolled(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	secret, _ := server.enrolAndConfirmWithCodes(t)
	sess := server.signInWith(t, secret)

	if got := server.del(totpPath, currentPasswordBody(testPassword), sess).Code; got != http.StatusNoContent {
		t.Fatalf("disable = %d, want 204", got)
	}
	if !decodeJSON[gen.CurrentUser](t, server.get(mePath, sess)).Mfa.Satisfied {
		t.Fatal("the session stopped reporting mfa.satisfied; this test no longer reaches what it is about")
	}

	if got := server.post(recoveryRegeneratePath,
		currentPasswordBody(testPassword), sess).Code; got != http.StatusConflict {
		t.Errorf("regenerate with nothing enrolled = %d, want 409", got)
	}
}

// --- Removing them ----------------------------------------------------------

// TestDisablingAnAuthenticatorDeletesTheCodes is the acceptance criterion. A
// second factor that was removed must not still be presentable.
func TestDisablingAnAuthenticatorDeletesTheCodes(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	secret, _ := server.enrolAndConfirmWithCodes(t)
	sess := server.signInWith(t, secret)

	if got := server.del(totpPath, currentPasswordBody(testPassword), sess).Code; got != http.StatusNoContent {
		t.Fatalf("disable = %d, want 204", got)
	}

	if got := server.remainingCodes(t, sess); got != 0 {
		t.Errorf("mfa.recoveryCodesRemaining = %d after removing the authenticator, want 0", got)
	}
	stored, err := identity.NewRecoveryCodes(server.db).Unused(t.Context(), server.userID(t))
	if err != nil {
		t.Fatalf("reading the stored codes: %v", err)
	}
	if len(stored) != 0 {
		t.Errorf("%d code rows survived the authenticator being removed", len(stored))
	}

	// The account signs in with a password alone now, so a recovery code has
	// nothing to complete — there is no pending state to present it against.
	recorder := server.login(testEmail, testPassword)
	if got := decodeJSON[gen.LoginResult](t, recorder).Status; got != gen.LoginStatusAuthenticated {
		t.Fatalf("status = %q after removing the factor, want %q", got, gen.LoginStatusAuthenticated)
	}
}

// TestANewEnrolmentMintsANewSet: enrolling again after removing an
// authenticator hands out ten codes that are not the previous ten.
func TestANewEnrolmentMintsANewSet(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	secret, first := server.enrolAndConfirmWithCodes(t)
	sess := server.signInWith(t, secret)

	if got := server.del(totpPath, currentPasswordBody(testPassword), sess).Code; got != http.StatusNoContent {
		t.Fatalf("disable = %d, want 204", got)
	}

	_, second := server.enrolAndConfirmWithCodes(t)
	for _, was := range first {
		for _, is := range second {
			if was == is {
				t.Fatal("a re-enrolment handed out a code from the previous set")
			}
		}
	}
	if got := server.signInWithRecoveryCode(t, second[0]).Code; got != http.StatusOK {
		t.Errorf("a code from the new set = %d, want 200", got)
	}
}
