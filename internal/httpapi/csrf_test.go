package httpapi

import (
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/bryanster/blacklight/api"
	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

// CSRF, through the real chain (M1-005). Every request here is built by hand
// rather than with authServer.post, because that helper attaches the header the
// way the SPA does and these tests are about what happens when something does
// not.

const changePasswordBody = `{"currentPassword":"` + testPassword + `","newPassword":"` + testNewPass + `"}`

// totpCodeBody is a syntactically valid code that is not the right one. What
// these tests want is a body the request validator accepts, so that whatever
// answers is the CSRF middleware rather than the schema.
const totpCodeBody = `{"code":"000000"}`

// forge builds the request a cross-site attacker can cause: the browser
// attaches the cookies, and the attacker chooses everything else. header and
// csrfCookie are omitted when empty, which is the state an attacker is actually
// in — they cannot read this origin's cookies, so they cannot echo one.
func (s *authServer) forge(target, body string, sess *http.Cookie, csrfCookie, header string) *httptest.ResponseRecorder {
	return s.forgeMethod(http.MethodPost, target, body, sess, csrfCookie, header)
}

// forgeMethod is forge for the route walk, which has to use each route's own
// method — one of the MFA endpoints is a DELETE, and sending it a POST would
// test the 405 path rather than the CSRF one.
func (s *authServer) forgeMethod(method, target, body string, sess *http.Cookie,
	csrfCookie, header string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if sess != nil {
		request.AddCookie(sess)
	}
	if csrfCookie != "" {
		request.AddCookie(&http.Cookie{Name: session.CSRFCookieName, Value: csrfCookie})
	}
	if header != "" {
		request.Header.Set(CSRFHeader, header)
	}
	return do(s.handler, request)
}

// csrfCookie returns the CSRF cookie a response set, or nil.
func csrfCookieIn(recorder *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == session.CSRFCookieName {
			return cookie
		}
	}
	return nil
}

// --- Refusing ------------------------------------------------------------------

// TestAStateChangingRequestWithNoCSRFTokenIsRefused, and — the half that
// matters — refused before the handler runs. A 403 that arrives after the
// session was revoked is not protection.
func TestAStateChangingRequestWithNoCSRFTokenIsRefused(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)

	recorder := server.forge(logoutPath, "", sess, "", "")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("logout with no CSRF token = %d, want 403\nbody: %s", recorder.Code, recorder.Body)
	}
	if got := decodeProblem(t, recorder).Code; got != gen.ProblemCodeForbidden {
		t.Errorf("code = %q, want %q", got, gen.ProblemCodeForbidden)
	}

	if rows := server.sessions(t); len(rows) != 1 || !rows[0].RevokedAt.IsZero() {
		t.Error("the logout handler ran: the session was revoked by a request that was supposed to be refused")
	}
	if got := server.get(mePath, sess).Code; got != http.StatusOK {
		t.Errorf("the session = %d after the refused logout, want 200", got)
	}
}

// TestOnlyTheRightTokenIsAccepted walks the near misses. Each of these is a
// state an attacker can reach, and the last one is the state the SPA is in.
func TestOnlyTheRightTokenIsAccepted(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)
	valid := server.manager.CSRFToken(session.Token(sess.Value))

	tests := map[string]struct {
		cookie, header string
		want           int
	}{
		"no cookie and no header":      {"", "", http.StatusForbidden},
		"the cookie but no header":     {valid, "", http.StatusForbidden},
		"the header but no cookie":     {"", valid, http.StatusForbidden},
		"a header that is not a token": {valid, "not-the-token", http.StatusForbidden},
		"a cookie that is not a token": {"not-the-token", valid, http.StatusForbidden},
		// The naive double-submit case: an attacker who can write a cookie for
		// this host makes the two halves agree with each other. They still do
		// not agree with the value derived from the session token, which is why
		// the check is not only cookie against header.
		"both, agreeing, but forged": {"forged-by-an-attacker", "forged-by-an-attacker", http.StatusForbidden},
		"both, and right":            {valid, valid, http.StatusNoContent},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// A fresh session per case, so that the one accepted logout does not
			// change what the others are testing.
			sess := server.signIn(t)
			cookie, header := tc.cookie, tc.header
			if cookie == valid || header == valid {
				fresh := server.manager.CSRFToken(session.Token(sess.Value))
				if cookie == valid {
					cookie = fresh
				}
				if header == valid {
					header = fresh
				}
			}

			if got := server.forge(logoutPath, "", sess, cookie, header).Code; got != tc.want {
				t.Errorf("logout = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestASafeMethodNeedsNoToken: GET changes nothing, so there is nothing to
// forge. Requiring a token here would only break every client that reads.
func TestASafeMethodNeedsNoToken(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)

	if got := server.get(mePath, sess).Code; got != http.StatusOK {
		t.Errorf("GET /auth/me with no CSRF token = %d, want 200", got)
	}
}

// TestLoginIsExemptBecauseThereIsNoSessionToProtect. The exemption is by route
// and is listed in csrfExemptRoutes with its reason; this asserts it is real,
// including for a browser that still holds a cookie from a previous session.
func TestLoginIsExemptBecauseThereIsNoSessionToProtect(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)

	body := `{"email":"` + testEmail + `","password":"` + testPassword + `"}`
	if got := server.forge(loginPath, body, nil, "", "").Code; got != http.StatusOK {
		t.Errorf("login with no CSRF token = %d, want 200", got)
	}

	stale := server.signIn(t)
	if got := server.forge(loginPath, body, stale, "", "").Code; got != http.StatusOK {
		t.Errorf("login carrying an old session cookie = %d, want 200 — "+
			"a login form that works or not depending on a stale cookie is worse than what it prevents", got)
	}
}

// --- The exemption cannot be claimed -------------------------------------------

// TestOnlyRealServiceTokenAuthenticationIsExempt is the acceptance criterion
// that says the exemption must key off *how* the request authenticated.
//
// The first half needs M1-011's authentication to exist, so it drives the
// middleware with the subject that step will produce. The second half is the
// attack, and it goes through the real server: sending an Authorization header
// must buy nothing at all.
func TestOnlyRealServiceTokenAuthenticationIsExempt(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)

	t.Run("a request that authenticated by service token", func(t *testing.T) {
		entered := false
		spy := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			entered = true
			w.WriteHeader(http.StatusNoContent)
		})
		middleware := requireCSRF(server.manager, apierr.NewResponder(slog.Default()), slog.Default())

		request := httptest.NewRequest(http.MethodPost, passwordPath, nil)
		request = request.WithContext(authn.WithSubject(request.Context(), authn.Subject{
			UserID: "019f0000-0000-7000-8000-000000000000",
			Method: authn.MethodServiceToken,
		}))

		recorder := do(middleware(spy), request)
		if !entered || recorder.Code != http.StatusNoContent {
			t.Errorf("a service-token request with no CSRF token = %d (handler entered: %t), want 204 and true",
				recorder.Code, entered)
		}
	})

	t.Run("a request that merely sent an Authorization header", func(t *testing.T) {
		sess := server.signIn(t)

		request := httptest.NewRequest(http.MethodPost, logoutPath, nil)
		request.Header.Set("Authorization", "Bearer bl_not_a_real_token")
		request.AddCookie(sess)

		if got := do(server.handler, request).Code; got != http.StatusForbidden {
			t.Errorf("a cookie session with an invented bearer token = %d, want 403 — "+
				"the exemption is not something a client can ask for", got)
		}
	})

	t.Run("an invalid bearer token authenticates nobody", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, passwordPath, strings.NewReader(changePasswordBody))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer bl_not_a_real_token")

		if got := do(server.handler, request).Code; got != http.StatusUnauthorized {
			t.Errorf("an invented bearer token with no cookie = %d, want 401", got)
		}
	})
}

// --- Issuing and rotating --------------------------------------------------------

// TestTheCSRFCookieAccompaniesTheSessionCookie, in both directions: it is set
// when the session is issued and cleared when the session ends, so a browser is
// never left holding half of a pair.
func TestTheCSRFCookieAccompaniesTheSessionCookie(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)

	recorder := server.login(testEmail, testPassword)
	sess := sessionCookie(t, recorder)
	issued := csrfCookieIn(recorder)
	if issued == nil {
		t.Fatalf("login set no %s cookie\nheaders: %v", session.CSRFCookieName, recorder.Header())
	}
	if want := server.manager.CSRFToken(session.Token(sess.Value)); issued.Value != want {
		t.Error("the CSRF cookie does not belong to the session cookie it was sent with")
	}
	// The body carries it too, for a client with no cookie jar to read.
	result := decodeJSON[gen.LoginResult](t, recorder)
	if result.User == nil || result.User.CsrfToken == nil || *result.User.CsrfToken != issued.Value {
		t.Error("the login body's csrfToken does not match the cookie")
	}

	out := server.post(logoutPath, "", sess)
	if out.Code != http.StatusNoContent {
		t.Fatalf("logout = %d, want 204\nbody: %s", out.Code, out.Body)
	}
	cleared := csrfCookieIn(out)
	if cleared == nil {
		t.Fatal("logout cleared the session cookie and left the CSRF cookie behind")
	}
	if cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Errorf("the CSRF cookie was not cleared: value %q, max-age %d", cleared.Value, cleared.MaxAge)
	}
}

// TestRotationCarriesTheCSRFTokenWithIt. A password change rotates the session
// token (M1-003); if the CSRF cookie did not rotate with it, the next
// state-changing request from the same browser would be a 403.
func TestRotationCarriesTheCSRFTokenWithIt(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)
	before := server.manager.CSRFToken(session.Token(sess.Value))

	recorder := server.post(passwordPath, changePasswordBody, sess)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("change password = %d, want 204\nbody: %s", recorder.Code, recorder.Body)
	}

	rotated := sessionCookie(t, recorder)
	after := csrfCookieIn(recorder)
	switch {
	case after == nil:
		t.Fatal("the rotated session cookie came with no CSRF cookie")
	case after.Value == before:
		t.Error("the CSRF token did not rotate with the session")
	case after.Value != server.manager.CSRFToken(session.Token(rotated.Value)):
		t.Fatal("the new CSRF cookie does not belong to the new session token")
	}

	// And the pair works: the browser can immediately make another
	// state-changing request, which is the thing that would be broken.
	if got := server.forge(logoutPath, "", rotated, after.Value, after.Value).Code; got != http.StatusNoContent {
		t.Errorf("logout right after a rotation = %d, want 204", got)
	}
}

// TestARefusalRepairsTheBrowsersCookie: a client that has lost the cookie — one
// signed in before this protection existed, or one whose jar was cleared — is
// not stuck at 403 forever. The refusal itself carries the value that makes the
// retry work.
func TestARefusalRepairsTheBrowsersCookie(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)

	recorder := server.forge(logoutPath, "", sess, "", "")
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("logout with no CSRF token = %d, want 403", recorder.Code)
	}
	repaired := csrfCookieIn(recorder)
	if repaired == nil {
		t.Fatal("the refusal set no CSRF cookie, so a client that lost it can never recover")
	}
	if got := server.forge(logoutPath, "", sess, repaired.Value, repaired.Value).Code; got != http.StatusNoContent {
		t.Errorf("the retry with the repaired cookie = %d, want 204", got)
	}
}

// TestCurrentUserReportsTheCSRFToken. The SPA reads the cookie; this is for
// everything else, and it is the same value.
func TestCurrentUserReportsTheCSRFToken(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	sess := server.signIn(t)

	me := decodeJSON[gen.CurrentUser](t, server.get(mePath, sess))
	if me.CsrfToken == nil {
		t.Fatal("/auth/me reports no csrfToken")
	}
	if want := server.manager.CSRFToken(session.Token(sess.Value)); *me.CsrfToken != want {
		t.Error("the csrfToken in /auth/me is not this session's")
	}
}

// --- The tests that stop this decaying -------------------------------------------

// csrfCoverage is every state-changing route this server serves, and what a
// syntactically valid request to it looks like. TestEveryMutatingRouteIsCovered
// fails when the router has a route this map does not — so adding a mutating
// endpoint forces whoever adds it to decide, here, whether it is protected or
// exempt, rather than finding out later that it never was.
//
// PLAN.md §4 records that v1's CSRF protection was added and then removed. This
// is the test that would have noticed.
var csrfCoverage = map[string]struct {
	body   string
	exempt bool
}{
	"POST " + BasePath + "/auth/login":    {body: `{"email":"nobody@example.com","password":"whatever it is"}`, exempt: true},
	"POST " + BasePath + "/auth/logout":   {body: ""},
	"POST " + BasePath + "/auth/password": {body: changePasswordBody},

	"POST " + BasePath + "/auth/mfa/totp/enroll":  {body: ""},
	"POST " + BasePath + "/auth/mfa/totp/confirm": {body: totpCodeBody},
	"POST " + BasePath + "/auth/mfa/totp/verify": {
		body:   totpCodeBody,
		exempt: true, // The second half of a sign-in; see csrfExemptRoutes.
	},
	"DELETE " + BasePath + "/auth/mfa/totp": {body: `{"currentPassword":"` + testPassword + `"}`},
}

func TestEveryMutatingRouteIsCoveredByCSRF(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)

	routes, ok := server.handler.(chi.Routes)
	if !ok {
		t.Fatalf("the server is a %T, which cannot be walked; this test has to be rewritten rather than deleted", server.handler)
	}

	seen := map[string]bool{}
	err := chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if slicesContains(csrfSafeMethods, method) {
			return nil
		}
		key := method + " " + strings.TrimSuffix(route, "/")
		seen[key] = true

		expectation, listed := csrfCoverage[key]
		if !listed {
			t.Errorf("%s is a state-changing route that this test does not know about. "+
				"Add it to csrfCoverage: either it is behind the CSRF check, or it is in "+
				"csrfExemptRoutes with a reason", key)
			return nil
		}

		// A request with a live session and no token at all. Anything not
		// exempt must be refused before its handler runs.
		sess := server.signIn(t)
		got := server.forgeMethod(method, strings.TrimSuffix(route, "/"), expectation.body, sess, "", "").Code
		switch {
		case expectation.exempt && got == http.StatusForbidden:
			t.Errorf("%s is listed as exempt but was refused with 403", key)
		case !expectation.exempt && got != http.StatusForbidden:
			t.Errorf("%s answered %d without a CSRF token, want 403 — it is not behind the check", key, got)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the router: %v", err)
	}

	for key := range csrfCoverage {
		if !seen[key] {
			t.Errorf("csrfCoverage lists %s, which the router does not serve; the list has gone stale", key)
		}
	}
	for key := range csrfExemptRoutes {
		if expectation, listed := csrfCoverage[key]; !listed || !expectation.exempt {
			t.Errorf("%s is exempt in csrfExemptRoutes but csrfCoverage does not say so", key)
		}
	}
}

// TestTheCSRFComparisonIsConstantTime reads the source, because a byte
// comparison is too fast to time reliably and a test that tried would be a
// flake rather than a check. What it is really guarding against is somebody
// simplifying csrfMatches into `header == expected`.
func TestTheCSRFComparisonIsConstantTime(t *testing.T) {
	t.Parallel()

	file, err := parser.ParseFile(token.NewFileSet(), "csrf.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing csrf.go: %v", err)
	}

	compares := false
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if !isFunc {
			continue
		}
		switch fn.Name.Name {
		case "constantTimeEqual":
			ast.Inspect(fn, func(node ast.Node) bool {
				call, isCall := node.(*ast.CallExpr)
				if !isCall {
					return true
				}
				if selector, isSelector := call.Fun.(*ast.SelectorExpr); isSelector {
					pkg, isIdent := selector.X.(*ast.Ident)
					compares = compares || (isIdent && pkg.Name == "subtle" && selector.Sel.Name == "ConstantTimeCompare")
				}
				return true
			})
		case "csrfMatches":
			ast.Inspect(fn, func(node ast.Node) bool {
				binary, isBinary := node.(*ast.BinaryExpr)
				if !isBinary || (binary.Op != token.EQL && binary.Op != token.NEQ) {
					return true
				}
				// Comparing against a literal is fine: that is the check for an
				// absent value, and its timing says nothing secret.
				if isBasicLit(binary.X) || isBasicLit(binary.Y) {
					return true
				}
				t.Error("csrfMatches compares two values with == or !=; token comparison must go through constantTimeEqual")
				return true
			})
		}
	}
	if !compares {
		t.Error("constantTimeEqual does not call subtle.ConstantTimeCompare")
	}
}

func isBasicLit(expr ast.Expr) bool {
	_, ok := expr.(*ast.BasicLit)
	return ok
}

// slicesContains is here rather than slices.Contains so that this test file
// does not have to care what type csrfSafeMethods is.
func slicesContains(haystack []string, needle string) bool {
	for _, straw := range haystack {
		if straw == needle {
			return true
		}
	}
	return false
}

// TestTheCSRFHeaderMatchesTheSpecification ties this package's constant to the
// parameter declared in api/openapi.yaml, which is what the generated
// TypeScript client documents to whoever writes against it.
func TestTheCSRFHeaderMatchesTheSpecification(t *testing.T) {
	t.Parallel()

	doc, err := api.Load()
	if err != nil {
		t.Fatalf("loading the API specification: %v", err)
	}
	parameter := doc.Components.Parameters["CSRFToken"]
	if parameter == nil || parameter.Value == nil {
		t.Fatal("the document declares no CSRFToken parameter")
	}
	if got := parameter.Value.Name; got != CSRFHeader {
		t.Errorf("the specification calls the header %q and this package calls it %q", got, CSRFHeader)
	}
	if got := parameter.Value.In; got != "header" {
		t.Errorf("CSRFToken is declared in %q, want %q", got, "header")
	}
}
