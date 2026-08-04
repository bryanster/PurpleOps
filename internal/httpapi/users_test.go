package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authn/session"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// User administration over HTTP (M1-016) — the endpoint family PLAN.md §4 names
// as the one v1 shipped ungated, so the regression cases are the point of the
// file rather than an appendix to it.
//
// Everything here drives the real chain against a real temporary DuckDB: real
// accounts, real sign-ins, real session cookies and their CSRF tokens. The
// policy is asserted exhaustively and in microseconds by
// internal/authz/matrix_test.go; what cannot be asserted there is that the
// policy is *reached*, which is what these are for.

const usersPath = BasePath + "/users"

func userPath(id string) string { return usersPath + "/" + id }

// --- Authorization ------------------------------------------------------------

// usersEndpoint is one operation in this ticket, and what a request to it looks
// like. The body is the smallest one the specification accepts: a refusal has to
// come from authorization and not from the request validator, or the row below
// is asserting nothing.
type usersEndpoint struct {
	name   string
	method string
	// path takes the identifier of the account being acted on, so that each
	// caller can be pointed at a real row rather than at an invented UUID that
	// would 404 for a reason this test is not about.
	path func(targetID string) string
	body string
}

func usersEndpoints() []usersEndpoint {
	return []usersEndpoint{
		{"list the accounts", http.MethodGet, func(string) string { return usersPath }, ""},
		{"create an account", http.MethodPost, func(string) string { return usersPath },
			`{"email":"new-account@example.com","displayName":"New","platformRole":"member"}`},
		{"read one account", http.MethodGet, userPath, ""},
		{"rename one account", http.MethodPatch, userPath, `{"displayName":"Renamed"}`},
		// The regression the ticket names in as many words: a platform member
		// patching *themselves* to admin.
		{"make one account an administrator", http.MethodPatch, userPath, `{"platformRole":"admin"}`},
		{"retire one account", http.MethodDelete, userPath, ""},
		{"disable one account", http.MethodPost, func(id string) string { return userPath(id) + "/disable" }, ""},
		{"enable one account", http.MethodPost, func(id string) string { return userPath(id) + "/enable" }, ""},
		{"sign one account out everywhere", http.MethodPost,
			func(id string) string { return userPath(id) + "/sessions/revoke" }, ""},
	}
}

// TestNobodyButAnAdministratorReachesUserAdministration is M1-016's first
// acceptance criterion at the HTTP level: every endpoint in the ticket, against
// every caller who is not an administrator.
//
// The member's target is *themselves*, which is the case that matters. v1's
// /manage/access let anybody signed in grant themselves Admin; here the same
// request is refused before the handler, and the account is read back afterwards
// to prove the handler never ran.
func TestNobodyButAnAdministratorReachesUserAdministration(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t) // the administrator, alice@example.com

	member := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "member@example.com"
		in.DisplayName = "Member"
		in.PlatformRole = authz.PlatformRoleMember
	})
	memberSession := sessionCookie(t, server.login(member.Email, testPassword))

	for _, endpoint := range usersEndpoints() {
		target := endpoint.path(member.ID)

		for _, caller := range []struct {
			name    string
			cookie  *http.Cookie
			want    int
			problem gen.ProblemCode
		}{
			{"a platform member acting on their own account", memberSession,
				http.StatusForbidden, gen.ProblemCodeForbidden},
			{"nobody at all", nil, http.StatusUnauthorized, gen.ProblemCodeUnauthenticated},
		} {
			var recorder *httptest.ResponseRecorder
			if caller.cookie == nil {
				recorder = server.send(endpoint.method, target, endpoint.body)
			} else {
				recorder = server.send(endpoint.method, target, endpoint.body, caller.cookie)
			}

			if recorder.Code != caller.want {
				t.Errorf("%s tried to %s: %d, want %d\nbody: %s",
					caller.name, endpoint.name, recorder.Code, caller.want, recorder.Body)
				continue
			}
			if got := decodeProblem(t, recorder).Code; got != caller.problem {
				t.Errorf("%s tried to %s: problem code %q, want %q",
					caller.name, endpoint.name, got, caller.problem)
			}
		}
	}

	// Nothing was written. A handler that runs and then declines to change
	// anything is a handler somebody will later make change something.
	after, err := identity.NewUsers(server.db).ByID(t.Context(), member.ID)
	if err != nil {
		t.Fatalf("re-reading the member: %v", err)
	}
	// Everything but last_login_at, which the sign-in above legitimately moved
	// and which no endpoint in this ticket writes.
	member.LastLoginAt = after.LastLoginAt
	if after != member {
		t.Errorf("the member's account changed under a request that was supposed to be refused:\nbefore %+v\nafter  %+v",
			member, after)
	}
	if _, err := identity.NewUsers(server.db).ByEmail(t.Context(), "new-account@example.com"); err == nil {
		t.Error("the refused creation created an account")
	}
}

// TestAnAdministratorReachesEveryUserEndpoint is the other half of the same
// table: the same nine operations, allowed.
//
// One target account per endpoint, because these actually change things — a
// shared one would be disabled by the fourth row and every row after it would be
// testing what happens to a disabled account instead.
func TestAnAdministratorReachesEveryUserEndpoint(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	for i, endpoint := range usersEndpoints() {
		target := server.seedUser(t, func(in *identity.NewUser) {
			in.Email = fmt.Sprintf("target-%d@example.com", i)
			in.DisplayName = "Target"
			in.PlatformRole = authz.PlatformRoleMember
		})

		recorder := server.send(endpoint.method, endpoint.path(target.ID), endpoint.body, admin)
		// 201 for the creation, 200 for everything else.
		if recorder.Code != http.StatusOK && recorder.Code != http.StatusCreated {
			t.Errorf("an administrator tried to %s: %d, want 200 or 201\nbody: %s",
				endpoint.name, recorder.Code, recorder.Body)
		}
	}
}

// TestAServiceTokenNeedsTheAdminScopeForUserAdministration keeps M1-011's first
// fence visible on the new endpoints: a token whose owner is an administrator
// still only reaches what its scopes carry.
func TestAServiceTokenNeedsTheAdminScopeForUserAdministration(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	readOnly := server.createToken(t, admin, authz.TokenScopeAdminRead).Token
	unrelated := server.createToken(t, admin, authz.TokenScopeContentRead).Token

	for name, tc := range map[string]struct {
		token  string
		method string
		body   string
		want   int
	}{
		"admin:read reads the listing":        {readOnly, http.MethodGet, "", http.StatusOK},
		"admin:read cannot create an account": {readOnly, http.MethodPost, newAccountBody("scoped@example.com"), http.StatusForbidden},
		"an unrelated scope cannot even read": {unrelated, http.MethodGet, "", http.StatusForbidden},
		"an unrelated scope cannot write":     {unrelated, http.MethodPost, newAccountBody("nope@example.com"), http.StatusForbidden},
	} {
		request := httptest.NewRequest(tc.method, usersPath, strings.NewReader(tc.body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+tc.token)

		if got := do(server.handler, request).Code; got != tc.want {
			t.Errorf("%s: %d, want %d", name, got, tc.want)
		}
	}
}

// --- The schema is the fence ---------------------------------------------------

// TestSelfServiceCannotNameAPlatformRole is M1-016's second acceptance
// criterion, and the shape of the answer is the criterion: `400` from the
// request validator, because `UpdateSelfRequest` has no such field — not `403`
// from a handler that read the field and decided not to honour it.
//
// PLAN.md §4: "field safety comes from the schema".
func TestSelfServiceCannotNameAPlatformRole(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t, func(in *identity.NewUser) { in.PlatformRole = authz.PlatformRoleMember })
	sess := server.signIn(t)

	for name, body := range map[string]string{
		"a role beside the display name":  `{"displayName":"Renamed","platformRole":"admin"}`,
		"a role on its own":               `{"platformRole":"admin"}`,
		"a status":                        `{"displayName":"Renamed","status":"active"}`,
		"an identifier for somebody else": `{"displayName":"Renamed","id":"0192f1a0-0000-7000-8000-000000000001"}`,
	} {
		recorder := server.send(http.MethodPatch, usersPath+"/me", body, sess)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("PATCH /users/me with %s = %d, want 400 from the request validator\nbody: %s",
				name, recorder.Code, recorder.Body)
			continue
		}
		if got := decodeProblem(t, recorder).Code; got != gen.ProblemCodeValidationFailed {
			t.Errorf("PATCH /users/me with %s answered code %q, want %q", name, got, gen.ProblemCodeValidationFailed)
		}
	}

	// And the account is untouched, including the role that was never a field.
	me := decodeJSON[gen.CurrentUser](t, server.get(mePath, sess))
	if me.PlatformRole != gen.PlatformRole(authz.PlatformRoleMember) {
		t.Errorf("platform role = %q after the refused requests", me.PlatformRole)
	}
}

// TestSelfServiceRenamesTheCallerAndNobodyElse is the other half: the one field
// that does exist works, and it works on the account that asked.
func TestSelfServiceRenamesTheCallerAndNobodyElse(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t, func(in *identity.NewUser) { in.PlatformRole = authz.PlatformRoleMember })
	other := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "other@example.com"
		in.DisplayName = "Untouched"
		in.PlatformRole = authz.PlatformRoleMember
	})
	sess := server.signIn(t)

	updated := decodeJSON[gen.User](t,
		mustSucceed(t, server.send(http.MethodPatch, usersPath+"/me", `{"displayName":"  Renamed  "}`, sess)))
	if updated.DisplayName != "Renamed" {
		t.Errorf("displayName = %q, want it trimmed to %q", updated.DisplayName, "Renamed")
	}

	untouched, err := identity.NewUsers(server.db).ByID(t.Context(), other.ID)
	if err != nil {
		t.Fatal(err)
	}
	if untouched.DisplayName != "Untouched" {
		t.Errorf("the other account is now named %q", untouched.DisplayName)
	}
}

// --- The last administrator ----------------------------------------------------

// TestTheLastAdministratorCannotBeRemoved is M1-016's third acceptance
// criterion, in both directions the ticket asks for: with exactly one
// administrator every way out is a 409, and with two the same request succeeds.
func TestTheLastAdministratorCannotBeRemoved(t *testing.T) {
	t.Parallel()

	// Each case runs against its own server, because the point of the second
	// half is that the *same* request succeeds once there is somebody else.
	for name, attempt := range map[string]func(*authServer, *http.Cookie, string) *httptest.ResponseRecorder{
		"demoted": func(s *authServer, sess *http.Cookie, id string) *httptest.ResponseRecorder {
			return s.send(http.MethodPatch, userPath(id), `{"platformRole":"member"}`, sess)
		},
		"disabled through the status field": func(s *authServer, sess *http.Cookie, id string) *httptest.ResponseRecorder {
			return s.send(http.MethodPatch, userPath(id), `{"status":"disabled"}`, sess)
		},
		"disabled": func(s *authServer, sess *http.Cookie, id string) *httptest.ResponseRecorder {
			return s.send(http.MethodPost, userPath(id)+"/disable", "", sess)
		},
		"retired": func(s *authServer, sess *http.Cookie, id string) *httptest.ResponseRecorder {
			return s.send(http.MethodDelete, userPath(id), "", sess)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := newAuthServer(t)
			only := server.seedUser(t)
			sess := server.signIn(t)

			recorder := attempt(server, sess, only.ID)
			if recorder.Code != http.StatusConflict {
				t.Fatalf("the only administrator was %s: %d, want 409\nbody: %s", name, recorder.Code, recorder.Body)
			}
			if got := decodeProblem(t, recorder).Code; got != gen.ProblemCodeConflict {
				t.Errorf("problem code %q, want %q", got, gen.ProblemCodeConflict)
			}

			// Rolled back, not half-applied: the guard runs inside the write.
			unchanged, err := identity.NewUsers(server.db).ByID(t.Context(), only.ID)
			if err != nil {
				t.Fatal(err)
			}
			if unchanged.PlatformRole != authz.PlatformRoleAdmin || unchanged.Status != identity.StatusActive {
				t.Fatalf("the refused request was partly applied: role %q, status %q",
					unchanged.PlatformRole, unchanged.Status)
			}

			// A second administrator, and the same request goes through.
			server.seedUser(t, func(in *identity.NewUser) {
				in.Email = "second-admin@example.com"
				in.DisplayName = "Second"
			})
			if got := attempt(server, sess, only.ID).Code; got != http.StatusOK {
				t.Errorf("with a second administrator, being %s = %d, want 200", name, got)
			}
		})
	}
}

// TestADisabledAdministratorDoesNotCountAsTheOneLeft: the guard counts
// administrators who can sign in, so an installation whose only other
// administrator is disabled is still down to one.
func TestADisabledAdministratorDoesNotCountAsTheOneLeft(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	only := server.seedUser(t)
	sess := server.signIn(t)

	server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "retired-admin@example.com"
		in.DisplayName = "Retired"
		in.Status = identity.StatusDisabled
	})

	if got := server.send(http.MethodPost, userPath(only.ID)+"/disable", "", sess).Code; got != http.StatusConflict {
		t.Errorf("disabling the last administrator who can sign in = %d, want 409", got)
	}
}

// --- Immediate effect ----------------------------------------------------------

// TestDemotionTakesEffectOnAnExistingSession is M1-016's fourth acceptance
// criterion. No re-login: the platform role is read off the account on every
// request, so the cookie the target is already holding changes what it can do
// the moment the account does.
func TestDemotionTakesEffectOnAnExistingSession(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	target := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "second-admin@example.com"
		in.DisplayName = "Second"
	})
	targetSession := sessionCookie(t, server.login(target.Email, testPassword))

	if got := server.get(usersPath, targetSession).Code; got != http.StatusOK {
		t.Fatalf("the second administrator = %d on the listing before anything changed, want 200", got)
	}

	mustSucceed(t, server.send(http.MethodPatch, userPath(target.ID), `{"platformRole":"member"}`, admin))

	// The same cookie, never refreshed.
	if got := server.get(usersPath, targetSession).Code; got != http.StatusForbidden {
		t.Errorf("after the demotion the same session = %d on the listing, want 403", got)
	}
	if got := server.get(mePath, targetSession).Code; got != http.StatusOK {
		t.Errorf("the demoted session = %d on /auth/me, want 200 — a demotion is not a sign-out", got)
	}

	// And back, on the same cookie: "in both directions" is the ticket's phrase.
	mustSucceed(t, server.send(http.MethodPatch, userPath(target.ID), `{"platformRole":"admin"}`, admin))
	if got := server.get(usersPath, targetSession).Code; got != http.StatusOK {
		t.Errorf("after the promotion the same session = %d on the listing, want 200", got)
	}
}

// TestDisablingEndsSessionsAndServiceTokensAtOnce is the fifth criterion. Both
// halves in one test on purpose: they are one act, and an implementation that
// stopped one and not the other would be a credential that outlived the account.
func TestDisablingEndsSessionsAndServiceTokensAtOnce(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	target := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "doomed@example.com"
		in.DisplayName = "Doomed"
		in.PlatformRole = authz.PlatformRoleMember
	})
	targetSession := sessionCookie(t, server.login(target.Email, testPassword))
	token := server.createToken(t, targetSession, authz.TokenScopeAdminRead).Token

	if got := server.get(mePath, targetSession).Code; got != http.StatusOK {
		t.Fatalf("the target's session = %d before anything changed, want 200", got)
	}
	if got := withBearer(server, http.MethodGet, mePath, token).Code; got != http.StatusOK {
		t.Fatalf("the target's token = %d before anything changed, want 200", got)
	}

	mustSucceed(t, server.send(http.MethodPost, userPath(target.ID)+"/disable", "", admin))

	if got := server.get(mePath, targetSession).Code; got != http.StatusUnauthorized {
		t.Errorf("the disabled account's session = %d, want 401", got)
	}
	if got := withBearer(server, http.MethodGet, mePath, token).Code; got != http.StatusUnauthorized {
		t.Errorf("the disabled account's service token = %d, want 401", got)
	}

	// The session row says it was revoked rather than merely being refused, so
	// that re-enabling the account does not hand back a tab somebody left open.
	rows, err := identity.NewSessions(server.db).ListByUser(t.Context(), target.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.RevokedAt.IsZero() {
			t.Errorf("session %s is still live after the account was disabled", row.ID)
		}
	}

	// Enabling gives the account back and not the sessions.
	mustSucceed(t, server.send(http.MethodPost, userPath(target.ID)+"/enable", "", admin))
	if got := server.get(mePath, targetSession).Code; got != http.StatusUnauthorized {
		t.Errorf("a session revoked while the account was disabled = %d after enabling, want 401", got)
	}
	if got := server.login(target.Email, testPassword).Code; got != http.StatusOK {
		t.Errorf("the re-enabled account = %d when signing in again, want 200", got)
	}
}

// TestRevokingSessionsLeavesTheAccountUsable is the endpoint an administrator
// reaches for when a laptop goes missing and the person still needs their
// account tomorrow.
func TestRevokingSessionsLeavesTheAccountUsable(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	target := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "lost-laptop@example.com"
		in.DisplayName = "Lost"
		in.PlatformRole = authz.PlatformRoleMember
	})
	first := sessionCookie(t, server.login(target.Email, testPassword))
	second := sessionCookie(t, server.login(target.Email, testPassword))
	token := server.createToken(t, first, authz.TokenScopeAdminRead).Token

	revoked := decodeJSON[gen.RevokedSessions](t,
		mustSucceed(t, server.send(http.MethodPost, userPath(target.ID)+"/sessions/revoke", "", admin)))
	if revoked.Revoked != 2 {
		t.Errorf("revoked = %d, want 2", revoked.Revoked)
	}

	for name, cookie := range map[string]*http.Cookie{"the first session": first, "the second": second} {
		if got := server.get(mePath, cookie).Code; got != http.StatusUnauthorized {
			t.Errorf("%s = %d after the revocation, want 401", name, got)
		}
	}
	// Not a disable: the account still works, and so do the credentials that
	// are not sessions.
	if got := withBearer(server, http.MethodGet, mePath, token).Code; got != http.StatusOK {
		t.Errorf("the account's service token = %d, want 200 — revoking sessions is not disabling the account", got)
	}
	if got := server.login(target.Email, testPassword).Code; got != http.StatusOK {
		t.Errorf("signing in again = %d, want 200", got)
	}

	// Idempotent: there is nothing left to end.
	again := decodeJSON[gen.RevokedSessions](t,
		mustSucceed(t, server.send(http.MethodPost, userPath(target.ID)+"/sessions/revoke", "", admin)))
	if again.Revoked != 1 {
		// The sign-in above created one more.
		t.Errorf("the second revocation ended %d sessions, want 1", again.Revoked)
	}
}

// --- Creating ------------------------------------------------------------------

// TestCreatingAnAccountWithAPasswordMakesItUsableImmediately, and the invite
// link says where to use it.
func TestCreatingAnAccountWithAPassword(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	created := decodeJSON[gen.CreatedUser](t, mustCreated(t, server.post(usersPath,
		`{"email":"Fresh@Example.com","displayName":"Fresh","platformRole":"member",`+
			`"password":"`+testNewPass+`"}`, admin)))

	if created.User.Status != gen.UserStatus(identity.StatusActive) {
		t.Errorf("status = %q, want %q for an account created with a password",
			created.User.Status, identity.StatusActive)
	}
	if created.User.Email != "Fresh@Example.com" {
		t.Errorf("email = %q, want the address as it was typed", created.User.Email)
	}
	if !strings.HasSuffix(created.InviteUrl, "/login") {
		t.Errorf("inviteUrl = %q, want the sign-in page of this deployment", created.InviteUrl)
	}

	if got := server.login("fresh@example.com", testNewPass).Code; got != http.StatusOK {
		t.Errorf("the new account = %d when signing in, want 200", got)
	}
}

// TestCreatingAnAccountWithoutAPasswordInvitesIt: no password means single
// sign-on, and an invited account has no local way in until it is claimed.
func TestCreatingAnAccountWithoutAPasswordInvitesIt(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	created := decodeJSON[gen.CreatedUser](t,
		mustCreated(t, server.post(usersPath, newAccountBody("sso-only@example.com"), admin)))
	if created.User.Status != gen.UserStatus(identity.StatusInvited) {
		t.Errorf("status = %q, want %q for an account created without a password",
			created.User.Status, identity.StatusInvited)
	}

	if got := server.login("sso-only@example.com", testPassword).Code; got != http.StatusUnauthorized {
		t.Errorf("an invited account = %d when signing in with a password, want 401", got)
	}
}

// TestCreatingAnAccountRefusesAnAddressAlreadyInUse is the sixth criterion, and
// the casing is the whole of it: the conflict has to come from the normalized
// column rather than from a comparison somebody wrote in Go.
func TestCreatingAnAccountRefusesAnAddressAlreadyInUse(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t) // alice@example.com
	admin := server.signIn(t)

	for _, spelling := range []string{
		"alice@example.com", "ALICE@EXAMPLE.COM", "Alice@Example.com", "  alice@example.com  ",
	} {
		recorder := server.post(usersPath, newAccountBody(spelling), admin)
		if recorder.Code != http.StatusConflict {
			t.Errorf("creating %q = %d, want 409 (and never a 500)\nbody: %s", spelling, recorder.Code, recorder.Body)
			continue
		}
		if got := decodeProblem(t, recorder).Code; got != gen.ProblemCodeConflict {
			t.Errorf("creating %q answered code %q, want %q", spelling, got, gen.ProblemCodeConflict)
		}
	}
}

// TestCreatingAnAccountRefusesAnUnusableCombination: an invited account has no
// local sign-in, so a password on one would be a password nobody could ever use.
func TestCreatingAnAccountRefusesAnUnusableCombination(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	for name, body := range map[string]string{
		"invited, with a password": `{"email":"a@example.com","displayName":"A","platformRole":"member",` +
			`"status":"invited","password":"` + testNewPass + `"}`,
		"a password the policy refuses": `{"email":"b@example.com","displayName":"B","platformRole":"member",` +
			`"password":"password"}`,
		"a blank display name": `{"email":"c@example.com","displayName":"   ","platformRole":"member"}`,
	} {
		recorder := server.post(usersPath, body, admin)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400\nbody: %s", name, recorder.Code, recorder.Body)
			continue
		}
		if problem := decodeProblem(t, recorder); len(*problem.Errors) == 0 {
			t.Errorf("%s answered no field errors; a form has nowhere to put the message", name)
		}
	}
}

// --- What a response may carry -------------------------------------------------

// TestNoUserResponseCarriesASecret is the seventh criterion, asserted on the
// bytes rather than on the struct: a field added to the wire type later would
// slip past a struct-shaped assertion, and this is the assertion that would not
// let it.
func TestNoUserResponseCarriesASecret(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	admin := server.seedUser(t)
	sess := server.signIn(t)

	// An account with every kind of secret attached to it: a password hash, a
	// confirmed authenticator, and recovery codes.
	target := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "secretive@example.com"
		in.DisplayName = "Secretive"
		in.PlatformRole = authz.PlatformRoleMember
	})
	const totpCiphertext = "totp-secret-ciphertext-value"
	if _, err := identity.NewTOTPs(server.db).Enroll(t.Context(), identity.NewTOTP{
		UserID: target.ID, SecretEncrypted: totpCiphertext,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := identity.NewRecoveryCodes(server.db).Replace(t.Context(), target.ID,
		[]string{"recovery-code-hash-value"}); err != nil {
		t.Fatal(err)
	}

	banned := map[string]string{
		"an Argon2id hash":         "$argon2",
		"the stored password hash": testPasswordHash(),
		"the authenticator secret": totpCiphertext,
		"a recovery code hash":     "recovery-code-hash-value",
		"a session token":          sess.Value,
		"the CSRF token":           server.manager.CSRFToken(session.Token(sess.Value)),
	}

	for name, target := range map[string]string{
		"the listing":       usersPath,
		"one account":       userPath(target.ID),
		"the administrator": userPath(admin.ID),
	} {
		body := mustSucceed(t, server.get(target, sess)).Body.String()
		for what, secret := range banned {
			if secret != "" && strings.Contains(body, secret) {
				t.Errorf("%s carries %s:\n%s", name, what, body)
			}
		}
	}
}

// --- Paging and searching ------------------------------------------------------

// TestTheListingPagesAndSearchesAtScale is the eighth criterion. A thousand
// accounts, created in the store rather than through the API: what is under test
// is the listing, and paying for a thousand Argon2id derivations to get there
// would make this the slowest test in the repository for no extra coverage.
func TestTheListingPagesAndSearchesAtScale(t *testing.T) {
	t.Parallel()

	const total = 1000
	server := newAuthServer(t)
	server.seedUser(t)
	admin := server.signIn(t)

	users := identity.NewUsers(server.db)
	for i := range total {
		if _, err := users.Create(t.Context(), identity.NewUser{
			Email:        fmt.Sprintf("person-%04d@example.com", i),
			DisplayName:  fmt.Sprintf("Person %04d", i),
			PlatformRole: authz.PlatformRoleMember,
			Status:       identity.StatusActive,
		}); err != nil {
			t.Fatalf("seeding account %d: %v", i, err)
		}
	}

	// Walking every page visits every account exactly once and terminates.
	seen := map[string]bool{}
	pages, cursor := 0, ""
	for {
		target := usersPath + "?limit=200"
		if cursor != "" {
			target += "&cursor=" + cursor
		}
		page := decodeJSON[gen.UserPage](t, mustSucceed(t, server.get(target, admin)))
		pages++
		if pages > 20 {
			t.Fatal("the listing never reported a last page; the cursor is not advancing")
		}
		for _, u := range page.Items {
			if seen[u.Id.String()] {
				t.Fatalf("the listing returned %s twice", u.Email)
			}
			seen[u.Id.String()] = true
		}
		next, err := page.NextCursor.Get()
		if err != nil || next == "" {
			break
		}
		cursor = next
	}
	if want := total + 1; len(seen) != want { // the thousand, plus the administrator
		t.Errorf("walking the listing saw %d accounts, want %d", len(seen), want)
	}

	// The cap is the specification's, and it is enforced by the validator rather
	// than by a handler quietly clamping — a caller that asks for too much is
	// told so.
	if got := server.get(usersPath+"?limit=201", admin).Code; got != http.StatusBadRequest {
		t.Errorf("limit=201 = %d, want 400", got)
	}

	// Search narrows to the one account whose name contains the fragment.
	found := decodeJSON[gen.UserPage](t, mustSucceed(t, server.get(usersPath+"?q=PERSON+0777", admin)))
	if len(found.Items) != 1 || found.Items[0].Email != "person-0777@example.com" {
		t.Errorf("q=PERSON+0777 returned %d account(s), want just person-0777@example.com", len(found.Items))
	}

	// And the filters are an AND with each other.
	admins := decodeJSON[gen.UserPage](t, mustSucceed(t, server.get(usersPath+"?role=admin&status=active", admin)))
	if len(admins.Items) != 1 {
		t.Errorf("role=admin&status=active returned %d accounts, want 1", len(admins.Items))
	}
}

// --- The activity log ----------------------------------------------------------

// TestEveryMutationIsRecorded is the ninth criterion: the acting administrator
// and the before/after, for each of the mutations in this ticket.
func TestEveryMutationIsRecorded(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	admin := server.seedUser(t)
	sess := server.signIn(t)

	created := decodeJSON[gen.CreatedUser](t,
		mustCreated(t, server.post(usersPath, newAccountBody("recorded@example.com"), sess)))
	id := created.User.Id.String()

	mustSucceed(t, server.send(http.MethodPatch, userPath(id),
		`{"displayName":"Recorded","platformRole":"admin","mfaEnforced":true}`, sess))
	mustSucceed(t, server.send(http.MethodPost, userPath(id)+"/enable", "", sess))
	mustSucceed(t, server.send(http.MethodPost, userPath(id)+"/sessions/revoke", "", sess))
	mustSucceed(t, server.send(http.MethodPost, userPath(id)+"/disable", "", sess))

	entries := decodeJSON[gen.ActivityPage](t, mustSucceed(t, server.get(BasePath+"/activity?limit=200", sess)))

	byVerb := map[string]gen.ActivityEntry{}
	for _, entry := range entries.Items {
		if entry.ObjectId == id {
			byVerb[entry.Verb] = entry
		}
	}

	for _, verb := range []string{
		"user.created", "user.updated", "user.role_changed",
		"user.enabled", "user.disabled", "user.sessions_revoked",
	} {
		entry, found := byVerb[verb]
		if !found {
			t.Errorf("no %s entry for the account that was just administered", verb)
			continue
		}
		if entry.ActorId == nil || entry.ActorId.String() != admin.ID {
			t.Errorf("%s names the actor %v, want the administrator %s", verb, entry.ActorId, admin.ID)
		}
		if entry.Delta == nil || len(*entry.Delta) == 0 {
			t.Errorf("%s carries no delta, so it says what happened but not what changed", verb)
		}
	}

	// The one that has to carry before *and* after, because it is the change an
	// incident review is looking for.
	role := byVerb["user.role_changed"]
	if role.Delta == nil {
		t.Fatal("user.role_changed carries no delta")
	}
	change, ok := (*role.Delta)["platform_role"].(map[string]any)
	if !ok {
		t.Fatalf("user.role_changed's delta is %v, want a platform_role before/after", *role.Delta)
	}
	if change["from"] != string(authz.PlatformRoleMember) || change["to"] != string(authz.PlatformRoleAdmin) {
		t.Errorf("platform_role = %v, want member → admin", change)
	}
}

// TestARefusedChangeLeavesNoActivityRow: the guard runs before the log row on
// the same transaction, so a 409 does not leave a feed claiming the change
// happened.
func TestARefusedChangeLeavesNoActivityRow(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	only := server.seedUser(t)
	sess := server.signIn(t)

	if got := server.send(http.MethodPatch, userPath(only.ID), `{"platformRole":"member"}`, sess).Code; got != http.StatusConflict {
		t.Fatalf("demoting the last administrator = %d, want 409", got)
	}

	entries := decodeJSON[gen.ActivityPage](t, mustSucceed(t, server.get(BasePath+"/activity?limit=200", sess)))
	for _, entry := range entries.Items {
		if entry.Verb == "user.role_changed" {
			t.Errorf("a refused demotion left a %s row behind:\n%v", entry.Verb, entry)
		}
	}
}

// --- Helpers -------------------------------------------------------------------

// newAccountBody is the smallest valid creation request: an SSO account, which
// is the shape that needs no password and so costs no derivation.
func newAccountBody(email string) string {
	return fmt.Sprintf(`{"email":%q,"displayName":"Someone","platformRole":"member"}`, email)
}

// withBearer performs a request as automation rather than as a browser.
func withBearer(s *authServer, method, target, token string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	return do(s.handler, request)
}

// mustSucceed and mustCreated fail the test with the body when a request that
// was supposed to work did not — which is far more useful than the decode error
// a caller would otherwise get.
func mustSucceed(t *testing.T, recorder *httptest.ResponseRecorder) *httptest.ResponseRecorder {
	t.Helper()

	if recorder.Code != http.StatusOK {
		t.Fatalf("request = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}
	return recorder
}

func mustCreated(t *testing.T, recorder *httptest.ResponseRecorder) *httptest.ResponseRecorder {
	t.Helper()

	if recorder.Code != http.StatusCreated {
		t.Fatalf("request = %d, want 201\nbody: %s", recorder.Code, recorder.Body)
	}
	return recorder
}
