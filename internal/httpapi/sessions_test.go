package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// The self-service session endpoints (M1-017), through the real chain.
//
// What they are for is a person seeing where they are signed in and ending the
// ones they do not recognise, so the cases below are about the two ways that
// could go wrong: a list that shows somebody else's browsers, and a revocation
// that reaches one.

const (
	sessionsPath     = BasePath + "/auth/sessions"
	revokeOthersPath = BasePath + "/auth/sessions/revoke-others"
	otherEmail       = "bob@example.com"
)

// listSessions reads the caller's sessions and insists it worked.
func (s *authServer) listSessions(t *testing.T, sess *http.Cookie) gen.Sessions {
	t.Helper()

	recorder := s.get(sessionsPath, sess)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /auth/sessions = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}
	return decodeJSON[gen.Sessions](t, recorder)
}

func TestTheSessionListIsTheCallersOwnAndMarksTheOneTheyAreOn(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	server.seedUser(t, func(u *identity.NewUser) {
		u.Email = otherEmail
		u.DisplayName = "Bob"
	})

	// Alice on two browsers, Bob on one. Bob's is the row that must not appear.
	first := server.signIn(t)
	second := server.signIn(t)
	server.signInAs(t, otherEmail)

	listed := server.listSessions(t, second)
	if len(listed.Items) != 2 {
		t.Fatalf("listed %d sessions, want the caller's 2 and neither of anybody else's", len(listed.Items))
	}

	current := 0
	for _, item := range listed.Items {
		if item.Current {
			current++
		}
	}
	if current != 1 {
		t.Errorf("%d rows are marked current, want exactly 1 — a client renders that row differently "+
			"and offers no revoke for it", current)
	}

	// The current row is the one the request was made on, not merely the newest.
	// Reading it back through the other cookie is what proves the flag follows
	// the request rather than the ordering.
	viaFirst := server.listSessions(t, first)
	var currentViaFirst gen.Session
	for _, item := range viaFirst.Items {
		if item.Current {
			currentViaFirst = item
		}
	}
	var currentViaSecond gen.Session
	for _, item := range listed.Items {
		if item.Current {
			currentViaSecond = item
		}
	}
	if currentViaFirst.Id == currentViaSecond.Id {
		t.Error("both cookies report the same session as current; the flag is not derived from the request")
	}
}

// TestTheSessionListCarriesNothingThatCouldBeReplayed is the reason this
// endpoint was allowed to exist at all: it describes credentials, so the test
// reads the raw body rather than the decoded struct — a field added to the
// schema later would be invisible to a struct-shaped assertion.
func TestTheSessionListCarriesNothingThatCouldBeReplayed(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	cookie := server.signIn(t)

	recorder := server.get(sessionsPath, cookie)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /auth/sessions = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}

	body := recorder.Body.String()
	if strings.Contains(body, cookie.Value) {
		t.Error("the session token itself is in the response; the value in the cookie must exist nowhere else")
	}
	for _, forbidden := range []string{"token", "hash", "secret"} {
		if strings.Contains(strings.ToLower(body), forbidden) {
			t.Errorf("the response mentions %q: %s", forbidden, body)
		}
	}
}

func TestTheSessionListShowsOnlyWhatWouldStillBeAccepted(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	stale := server.signIn(t)
	current := server.signIn(t)

	// Revoked through the API, so what is under test is the list rather than a
	// row this test wrote by hand.
	revoke(t, server, current, sessionIDOf(t, server, stale))

	listed := server.listSessions(t, current)
	if len(listed.Items) != 1 {
		t.Fatalf("listed %d sessions, want only the live one — a row somebody cannot act on is one they "+
			"would revoke twice", len(listed.Items))
	}
	if !listed.Items[0].Current {
		t.Error("the remaining row is not the current session")
	}
}

func TestRevokingASessionEndsThatBrowserAndLeavesTheOthers(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	doomed := server.signIn(t)
	keeper := server.signIn(t)

	revoke(t, server, keeper, sessionIDOf(t, server, doomed))

	if got := server.get(mePath, doomed).Code; got != http.StatusUnauthorized {
		t.Errorf("the revoked cookie = %d on /auth/me, want 401 — revocation is in the database, not "+
			"merely in the browser", got)
	}
	if got := server.get(mePath, keeper).Code; got != http.StatusOK {
		t.Errorf("the surviving cookie = %d on /auth/me, want 200", got)
	}
}

// TestRevokingATwiceRevokedSessionIsStillFine: the spec says "revoked, or
// already was", and a client that clicks twice must not be told it failed.
func TestRevokingATwiceRevokedSessionIsStillFine(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	doomed := server.signIn(t)
	keeper := server.signIn(t)

	id := sessionIDOf(t, server, doomed)
	revoke(t, server, keeper, id)
	revoke(t, server, keeper, id)
}

// TestRevokingSomebodyElsesSessionIsIndistinguishableFromAnInventedOne is the
// enumeration defence, and the reason the ownership lookup happens before the
// revocation rather than inside it.
func TestRevokingSomebodyElsesSessionIsIndistinguishableFromAnInventedOne(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	server.seedUser(t, func(u *identity.NewUser) {
		u.Email = otherEmail
		u.DisplayName = "Bob"
	})

	alice := server.signIn(t)
	bob := server.signInAs(t, otherEmail)
	bobsSession := sessionIDOf(t, server, bob)

	theirs := server.send(http.MethodDelete, sessionPath(bobsSession), "", alice)
	invented := server.send(http.MethodDelete, sessionPath("0192f1a0-0000-7000-8000-0000000000ff"), "", alice)

	if theirs.Code != http.StatusNotFound {
		t.Errorf("revoking somebody else's session = %d, want 404\nbody: %s", theirs.Code, theirs.Body)
	}
	if invented.Code != http.StatusNotFound {
		t.Errorf("revoking an identifier that names nothing = %d, want 404\nbody: %s", invented.Code, invented.Body)
	}
	if got, want := withoutInstance(t, theirs), withoutInstance(t, invented); got != want {
		t.Errorf("the two answers differ, so the endpoint says which identifiers are real:\n %s\n %s", got, want)
	}

	// And Bob is still signed in, which is the half a status code cannot say.
	if got := server.get(mePath, bob).Code; got != http.StatusOK {
		t.Errorf("Bob's session = %d on /auth/me after somebody else tried to revoke it, want 200", got)
	}
}

func TestRevokeOthersKeepsTheBrowserThatAsked(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	first := server.signIn(t)
	second := server.signIn(t)
	third := server.signIn(t)

	recorder := server.post(revokeOthersPath, "", third)
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /auth/sessions/revoke-others = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}
	if got := decodeJSON[gen.RevokedSessions](t, recorder).Revoked; got != 2 {
		t.Errorf("revoked = %d, want 2 — the count is what a confirmation dialog quotes", got)
	}

	if got := server.get(mePath, third).Code; got != http.StatusOK {
		t.Errorf("the asking session = %d on /auth/me, want 200: this is not a way to sign yourself out", got)
	}
	for name, cookie := range map[string]*http.Cookie{"first": first, "second": second} {
		if got := server.get(mePath, cookie).Code; got != http.StatusUnauthorized {
			t.Errorf("the %s session = %d on /auth/me, want 401", name, got)
		}
	}

	// Idempotent: with nothing else live, the same call is a normal zero.
	again := server.post(revokeOthersPath, "", third)
	if again.Code != http.StatusOK {
		t.Fatalf("the second call = %d, want 200\nbody: %s", again.Code, again.Body)
	}
	if got := decodeJSON[gen.RevokedSessions](t, again).Revoked; got != 0 {
		t.Errorf("revoked = %d on the second call, want 0", got)
	}
}

// TestAServiceTokenCannotSeeOrEndSessions is GuardSessionOnly, reached through
// HTTP. A leaked token that could read this list would enumerate where its owner
// signs in from; one that could post to revoke-others would sign them out.
func TestAServiceTokenCannotSeeOrEndSessions(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	server.seedUser(t)
	cookie := server.signIn(t)

	// The broadest scopes an administrator can mint, so that what refuses the
	// token below is the guard rather than a missing permission.
	created := server.createToken(t, cookie, authz.TokenScopeAdminRead, authz.TokenScopeAdminWrite)
	id := sessionIDOf(t, server, cookie)

	attempts := map[string]struct{ method, target string }{
		"listing them":      {http.MethodGet, sessionsPath},
		"revoking one":      {http.MethodDelete, sessionPath(id)},
		"revoking the rest": {http.MethodPost, revokeOthersPath},
	}
	for name, attempt := range attempts {
		recorder := server.withToken(attempt.method, attempt.target, created.Token)
		if recorder.Code != http.StatusForbidden {
			t.Errorf("%s with a service token = %d, want 403\nbody: %s", name, recorder.Code, recorder.Body)
		}
	}

	// And the session it could not reach is still live.
	if got := server.get(mePath, cookie).Code; got != http.StatusOK {
		t.Errorf("the owner's session = %d on /auth/me, want 200", got)
	}
}

// revoke ends one session through the API and insists the server said 204.
func revoke(t *testing.T, server *authServer, as *http.Cookie, id string) {
	t.Helper()

	recorder := server.send(http.MethodDelete, sessionPath(id), "", as)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("DELETE /auth/sessions/%s = %d, want 204\nbody: %s", id, recorder.Code, recorder.Body)
	}
}

// sessionIDOf returns the identifier of the session a cookie stands for, read
// through the API rather than out of the database: the identifier a client acts
// on is the one this endpoint gave it.
func sessionIDOf(t *testing.T, server *authServer, cookie *http.Cookie) string {
	t.Helper()

	for _, item := range server.listSessions(t, cookie).Items {
		if item.Current {
			return item.Id.String()
		}
	}
	t.Fatal("no row in the caller's own session list is marked current")
	return ""
}

func sessionPath(id string) string {
	return sessionsPath + "/" + id
}
