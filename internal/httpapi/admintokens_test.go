package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Administrative service token management over HTTP (M1-018).
//
// M1-011 built the owner-only endpoints, and left an administrator with exactly
// one answer to a compromised account: disable it, which stops every token it
// holds and stops the person along with them. These two endpoints are the
// narrower instrument — see what this account holds, end this one — and every
// test here is about a property that keeps them from becoming a wider one.

// absentAccount is a well-formed identifier that names nobody. It is what the
// enumeration test compares a real account against.
const absentAccount = "0192f1a0-0000-7000-8000-00000000d0e5"

func userTokensPath(userID string) string { return userPath(userID) + "/tokens" }

func userTokenPath(userID, tokenID string) string { return userTokensPath(userID) + "/" + tokenID }

// adminTokenServer is the cast these tests share: an administrator, an ordinary
// member, and one token the member holds.
type adminTokenServer struct {
	*authServer

	admin  identity.User
	member identity.User

	// adminSession and memberSession are signed-in browsers for each.
	adminSession  *http.Cookie
	memberSession *http.Cookie

	// theirs is a token the member owns, with its secret — so a test can assert
	// on what happens to the *credential* and not only to the row.
	theirs gen.CreatedServiceToken
}

func newAdminTokenServer(t *testing.T) *adminTokenServer {
	t.Helper()

	server := newAuthServer(t)
	admin := server.seedUser(t) // alice@example.com, an administrator
	member := server.seedUser(t, func(in *identity.NewUser) {
		in.Email = "member@example.com"
		in.DisplayName = "Member"
		in.PlatformRole = authz.PlatformRoleMember
	})

	memberSession := server.signInAs(t, member.Email)
	return &adminTokenServer{
		authServer:    server,
		admin:         admin,
		member:        member,
		adminSession:  server.signIn(t),
		memberSession: memberSession,
		// content:read, so that the token has something to do that is not
		// administration — a revocation that stopped it has to be the
		// revocation and not the scope.
		theirs: server.createToken(t, memberSession, authz.TokenScopeContentRead),
	}
}

// --- Authorization ------------------------------------------------------------

// TestAnAccountsTokensAreNotAWayToFindOutWhichAccountsExist is M1-018's first
// acceptance criterion. A caller who may not read these must get the *same*
// answer for a real account as for one that was never there — an endpoint that
// answered differently would be an account enumerator wearing a permission
// check.
//
// It holds by construction rather than by care: `token.admin_read` acts on a
// platform-owned resource, so the middleware asks internal/authz and refuses
// before anything is loaded. Nothing on the refusing path has looked to see
// whether the account is real, and so nothing on it could tell.
func TestAnAccountsTokensAreNotAWayToFindOutWhichAccountsExist(t *testing.T) {
	t.Parallel()

	server := newAdminTokenServer(t)
	tokenID := server.theirs.ServiceToken.Id.String()

	for _, endpoint := range []struct {
		name   string
		method string
		path   func(userID string) string
	}{
		{"list an account's tokens", http.MethodGet, userTokensPath},
		{"revoke one of an account's tokens", http.MethodDelete,
			func(userID string) string { return userTokenPath(userID, tokenID) }},
	} {
		// The member asks about the administrator's account, which exists and
		// holds tokens, and then about one that never existed.
		real := server.send(endpoint.method, endpoint.path(server.admin.ID), "", server.memberSession)
		invented := server.send(endpoint.method, endpoint.path(absentAccount), "", server.memberSession)

		if real.Code != http.StatusForbidden || invented.Code != http.StatusForbidden {
			t.Errorf("a platform member tried to %s: %d for a real account and %d for an invented one, want 403 for both",
				endpoint.name, real.Code, invented.Code)
			continue
		}
		if withoutInstance(t, real) != withoutInstance(t, invented) {
			t.Errorf("a platform member tried to %s and the two refusals differ, which tells them which account is "+
				"real:\n%s\n%s", endpoint.name, real.Body, invented.Body)
		}
		if got := decodeProblem(t, real).Code; got != gen.ProblemCodeForbidden {
			t.Errorf("a platform member tried to %s: problem code %q, want %q",
				endpoint.name, got, gen.ProblemCodeForbidden)
		}

		// And a request carrying nothing is 401 rather than 403: "you are not
		// signed in" and "you may not do this" are different instructions.
		if got := server.send(endpoint.method, endpoint.path(server.admin.ID), "").Code; got != http.StatusUnauthorized {
			t.Errorf("nobody at all tried to %s: %d, want 401", endpoint.name, got)
		}
	}

	// Nothing was revoked by any of the above. A refused request that reached
	// the handler and then declined to act is a handler somebody will later
	// make act.
	if got := server.withToken(http.MethodGet, mePath, server.theirs.Token); got.Code == http.StatusUnauthorized {
		t.Error("the member's token stopped working during a test in which every request was refused")
	}
}

// TestAServiceTokenCannotManageAnotherAccountsTokens is [authz.GuardSessionOnly]
// on the new pair, and it is the reason they carry their own actions rather than
// reusing `user.read` and `user.manage`: those two hold no guard, so an
// administrator's leaked credential would have been able to end every other
// credential in the installation.
//
// M1-011's TestATokenCannotMintOrRevokeATokenThroughTheAPI is the same argument
// over the owner-only endpoints. Both matter, and this one more: a token that
// can revoke only its own siblings is a nuisance, and one that can revoke
// everybody's is an outage.
func TestAServiceTokenCannotManageAnotherAccountsTokens(t *testing.T) {
	t.Parallel()

	server := newAdminTokenServer(t)

	// Every scope this build defines, owned by an administrator — so a refusal
	// below cannot be about a missing scope or a junior owner.
	automation := server.createToken(t, server.adminSession, authz.TokenScopes()...)

	listing := server.withToken(http.MethodGet, userTokensPath(server.member.ID), automation.Token)
	if listing.Code != http.StatusForbidden {
		t.Errorf("an administrator's fully scoped token listing another account's tokens = %d, want 403\nbody: %s",
			listing.Code, listing.Body)
	}
	revocation := server.withToken(http.MethodDelete,
		userTokenPath(server.member.ID, server.theirs.ServiceToken.Id.String()), automation.Token)
	if revocation.Code != http.StatusForbidden {
		t.Errorf("an administrator's fully scoped token revoking another account's token = %d, want 403\nbody: %s",
			revocation.Code, revocation.Body)
	}

	// The member's token is untouched, which is the half that would go
	// unnoticed if the refusal happened after the write.
	if got := server.get(tokensPath, server.memberSession); !strings.Contains(got.Body.String(), `"status":"active"`) {
		t.Errorf("the token a refused request named is no longer active:\n%s", got.Body)
	}

	// And the automation token still works for what it is for, so the refusals
	// are about the action rather than about the credential having stopped.
	if got := server.withToken(http.MethodGet, settingsPath, automation.Token).Code; got != http.StatusOK {
		t.Errorf("the administrator's token = %d on an endpoint it holds, want 200", got)
	}
}

// --- Revoking somebody else's ---------------------------------------------------

// TestAnAdministratorRevokesSomebodyElsesTokenAndItStopsAtItsNextRequest is
// M1-018's second acceptance criterion, end to end: the credential stops, and
// the owner sees the same revocation the administrator made rather than a copy
// of it.
func TestAnAdministratorRevokesSomebodyElsesTokenAndItStopsAtItsNextRequest(t *testing.T) {
	t.Parallel()

	server := newAdminTokenServer(t)
	tokenID := server.theirs.ServiceToken.Id.String()

	// It works first, or the assertion below is about a token that never did.
	if got := server.withToken(http.MethodGet, mePath, server.theirs.Token); got.Code == http.StatusUnauthorized {
		t.Fatalf("the member's token was refused before anybody revoked it: %d\nbody: %s", got.Code, got.Body)
	}

	// The administrator sees it, without being able to see its secret.
	listed := decodeJSON[gen.ServiceTokens](t,
		server.get(userTokensPath(server.member.ID), server.adminSession))
	switch {
	case len(listed.Items) != 1:
		t.Fatalf("the administrator's view of the account has %d tokens, want 1", len(listed.Items))
	case listed.Items[0].Id != server.theirs.ServiceToken.Id:
		t.Fatalf("the administrator's view returned %s, want %s", listed.Items[0].Id, server.theirs.ServiceToken.Id)
	case listed.Items[0].Status != gen.ServiceTokenStatusActive:
		t.Errorf("the token reads as %q before anybody revoked it, want active", listed.Items[0].Status)
	}

	revocation := server.send(http.MethodDelete, userTokenPath(server.member.ID, tokenID), "", server.adminSession)
	if revocation.Code != http.StatusNoContent {
		t.Fatalf("an administrator revoking the token = %d, want 204\nbody: %s", revocation.Code, revocation.Body)
	}

	// The credential stops at its next request — not at the next sign-in, and
	// not after a cache expires, because nothing caches it.
	if got := server.withToken(http.MethodGet, mePath, server.theirs.Token); got.Code != http.StatusUnauthorized {
		t.Errorf("the revoked token = %d, want 401\nbody: %s", got.Code, got.Body)
	}

	// The owner's own listing shows the same revocation, at the same instant:
	// one row was revoked once, and the two endpoints read it rather than each
	// recording their own version of it.
	own := decodeJSON[gen.ServiceTokens](t, server.get(tokensPath, server.memberSession))
	admins := decodeJSON[gen.ServiceTokens](t,
		server.get(userTokensPath(server.member.ID), server.adminSession))
	if len(own.Items) != 1 || len(admins.Items) != 1 {
		t.Fatalf("the owner sees %d tokens and the administrator sees %d, want 1 each",
			len(own.Items), len(admins.Items))
	}
	switch {
	case own.Items[0].Status != gen.ServiceTokenStatusRevoked:
		t.Errorf("the owner's listing reads %q after an administrator revoked it, want revoked", own.Items[0].Status)
	case own.Items[0].RevokedAt == nil:
		t.Error("the owner's listing carries no revocation time")
	case admins.Items[0].RevokedAt == nil:
		t.Error("the administrator's listing carries no revocation time")
	case !own.Items[0].RevokedAt.Equal(*admins.Items[0].RevokedAt):
		t.Errorf("the owner sees the token revoked at %s and the administrator at %s; there is one revocation",
			own.Items[0].RevokedAt, admins.Items[0].RevokedAt)
	}

	// revokedBy is the column 0010 added, and the question it answers: this was
	// not a rotation the owner did.
	switch {
	case own.Items[0].RevokedBy == nil:
		t.Error("the token does not say who revoked it, so 'was it their own doing?' is unanswerable")
	case own.Items[0].RevokedBy.String() != server.admin.ID:
		t.Errorf("the token was revoked by %s, want the administrator %s",
			own.Items[0].RevokedBy, server.admin.ID)
	}
}

// TestRevokingIsIdempotentAndKeepsTheFirstRevoker: whoever arrives second
// stopped nothing, and a row that recorded them would misdate the incident.
func TestRevokingIsIdempotentAndKeepsTheFirstRevoker(t *testing.T) {
	t.Parallel()

	server := newAdminTokenServer(t)
	tokenID := server.theirs.ServiceToken.Id.String()

	// The owner ends it first, then an administrator ends it again.
	if got := server.send(http.MethodDelete, tokensPath+"/"+tokenID, "", server.memberSession).Code; got != http.StatusNoContent {
		t.Fatalf("the owner revoking their own token = %d, want 204", got)
	}
	first := decodeJSON[gen.ServiceTokens](t, server.get(tokensPath, server.memberSession)).Items[0]

	if got := server.send(http.MethodDelete, userTokenPath(server.member.ID, tokenID), "",
		server.adminSession).Code; got != http.StatusNoContent {
		t.Fatalf("an administrator revoking an already-revoked token = %d, want 204 — the intent is satisfied", got)
	}
	second := decodeJSON[gen.ServiceTokens](t, server.get(tokensPath, server.memberSession)).Items[0]

	switch {
	case second.RevokedAt == nil || first.RevokedAt == nil:
		t.Fatal("a revoked token carries no revocation time")
	case !second.RevokedAt.Equal(*first.RevokedAt):
		t.Errorf("the second revocation moved the time from %s to %s; the first is when access stopped",
			first.RevokedAt, second.RevokedAt)
	case second.RevokedBy == nil || second.RevokedBy.String() != server.member.ID:
		t.Errorf("the second revocation rewrote the revoker to %v, want the owner %s who actually ended it",
			second.RevokedBy, server.member.ID)
	}
}

// TestATokenIdentifierBelongingToAnotherAccountIsNotFound: both identifiers are
// part of the statement that does the revoking, so naming the wrong owner is a
// 404 and not a revocation. Without it the account in the path would be
// decoration and the endpoint would revoke by token identifier alone.
func TestATokenIdentifierBelongingToAnotherAccountIsNotFound(t *testing.T) {
	t.Parallel()

	server := newAdminTokenServer(t)
	mine := server.createToken(t, server.adminSession, authz.TokenScopeContentRead)

	// The member's account, the administrator's token.
	mismatched := server.send(http.MethodDelete,
		userTokenPath(server.member.ID, mine.ServiceToken.Id.String()), "", server.adminSession)
	invented := server.send(http.MethodDelete,
		userTokenPath(server.member.ID, "0192f1a0-0000-7000-8000-0000000000ff"), "", server.adminSession)

	if mismatched.Code != http.StatusNotFound || invented.Code != http.StatusNotFound {
		t.Fatalf("a token belonging to another account = %d and an invented one = %d; both must be 404\n%s",
			mismatched.Code, invented.Code, mismatched.Body)
	}
	if withoutInstance(t, mismatched) != withoutInstance(t, invented) {
		t.Errorf("the two 404s differ, which is a way to find out which token identifiers are real:\n%s\n%s",
			mismatched.Body, invented.Body)
	}

	// It really was not revoked — the 404 is a refusal and not a report.
	still := decodeJSON[gen.ServiceTokens](t, server.get(tokensPath, server.adminSession))
	for _, token := range still.Items {
		if token.Id == mine.ServiceToken.Id && token.Status != gen.ServiceTokenStatusActive {
			t.Errorf("the administrator's own token reads %q after a 404, want active", token.Status)
		}
	}

	// An account that does not exist is a 404 too, for the listing as well —
	// the caller here is entitled to know, so this one is a real answer rather
	// than a concealment.
	if got := server.get(userTokensPath(absentAccount), server.adminSession).Code; got != http.StatusNotFound {
		t.Errorf("an administrator listing the tokens of an account that does not exist = %d, want 404", got)
	}
}

// --- What must not have changed ------------------------------------------------

// TestTheOwnerOnlyTokenEndpointsAreUnchanged is M1-018's fourth acceptance
// criterion. The administrative endpoints are additional, not a widening: there
// is still no parameter on `GET /auth/tokens` that names another account, and an
// administrator calling it sees their own tokens like everybody else.
func TestTheOwnerOnlyTokenEndpointsAreUnchanged(t *testing.T) {
	t.Parallel()

	server := newAdminTokenServer(t)
	mine := server.createToken(t, server.adminSession, authz.TokenScopeContentRead)

	own := decodeJSON[gen.ServiceTokens](t, server.get(tokensPath, server.adminSession))
	if len(own.Items) != 1 {
		t.Fatalf("the administrator's own listing has %d tokens, want 1 — it is seeing somebody else's", len(own.Items))
	}
	if own.Items[0].Id != mine.ServiceToken.Id {
		t.Errorf("the administrator's own listing returned %s, want their own %s",
			own.Items[0].Id, mine.ServiceToken.Id)
	}

	// And the owner-only revocation still refuses to reach across accounts,
	// administrator or not: that endpoint has no account parameter, so the only
	// way it could is if it stopped scoping to the caller.
	across := server.send(http.MethodDelete,
		tokensPath+"/"+server.theirs.ServiceToken.Id.String(), "", server.adminSession)
	if across.Code != http.StatusNotFound {
		t.Errorf("an administrator revoking somebody else's token through /auth/tokens = %d, want 404\nbody: %s",
			across.Code, across.Body)
	}
}

// TestNoSecretAppearsInTheAdministrativeResponses extends M1-011's test over the
// two endpoints this ticket adds. It could not carry one — the server stores a
// hash and gen.ServiceToken has no field for a secret — and that is the property
// being pinned, over the responses and over the log at full volume.
func TestNoSecretAppearsInTheAdministrativeResponses(t *testing.T) {
	t.Parallel()

	server := newAdminTokenServer(t)
	secret := server.theirs.Token
	if secret == "" {
		t.Fatal("the member holds no token secret; this test would pass trivially")
	}

	listing := server.get(userTokensPath(server.member.ID), server.adminSession)
	if listing.Code != http.StatusOK {
		t.Fatalf("the administrative listing = %d, want 200\nbody: %s", listing.Code, listing.Body)
	}
	revocation := server.send(http.MethodDelete,
		userTokenPath(server.member.ID, server.theirs.ServiceToken.Id.String()), "", server.adminSession)
	after := server.get(userTokensPath(server.member.ID), server.adminSession)

	for name, body := range map[string]string{
		"the administrative listing":  listing.Body.String(),
		"the revocation":              revocation.Body.String(),
		"the listing after revoking":  after.Body.String(),
		"the owner's own listing now": server.get(tokensPath, server.memberSession).Body.String(),
	} {
		if strings.Contains(body, secret) {
			t.Errorf("%s carried the token's secret:\n%s", name, body)
		}
	}

	// Debug-level, which is more than a deployment records: the secret must not
	// be reachable even by turning the volume up.
	if logs := server.logs.String(); strings.Contains(logs, secret) {
		t.Error("the server log contains the token's secret")
	}
	if parts := strings.Split(secret, "_"); len(parts) == 3 && strings.Contains(server.logs.String(), parts[2]) {
		t.Error("the server log contains the secret half of the token")
	}
}

// --- The activity log ------------------------------------------------------------

// TestAnAdministrativeRevocationIsRecordedApartFromARoutineOne is M1-018's
// activity requirement: the verb says who did it to whose.
//
// Two verbs rather than one with a delta field somebody has to notice. An
// incident review filters for "an administrator ended somebody's credential",
// and a filter that also returns every routine rotation in the installation is
// a filter nobody uses — the same distinction M1-016 drew between
// `user.sessions_revoked` and `session.logout`.
func TestAnAdministrativeRevocationIsRecordedApartFromARoutineOne(t *testing.T) {
	t.Parallel()

	server := newAdminTokenServer(t)
	ownRotation := server.createToken(t, server.memberSession, authz.TokenScopeContentRead)

	// The owner rotates one of their own, and an administrator ends the other.
	if got := server.send(http.MethodDelete, tokensPath+"/"+ownRotation.ServiceToken.Id.String(),
		"", server.memberSession).Code; got != http.StatusNoContent {
		t.Fatalf("the owner revoking their own token = %d, want 204", got)
	}
	if got := server.send(http.MethodDelete,
		userTokenPath(server.member.ID, server.theirs.ServiceToken.Id.String()),
		"", server.adminSession).Code; got != http.StatusNoContent {
		t.Fatalf("an administrator revoking the member's token = %d, want 204", got)
	}

	rows := server.activityRows(t)
	routine := findActivity(rows, events.VerbTokenRevoked, ownRotation.ServiceToken.Id.String())
	administrative := findActivity(rows,
		events.VerbTokenAdminRevoked, server.theirs.ServiceToken.Id.String())

	switch {
	case routine == nil:
		t.Errorf("the owner's own revocation left no %s row", events.VerbTokenRevoked)
	case routine.ActorId == nil || routine.ActorId.String() != server.member.ID:
		t.Errorf("%s names actor %v, want the owner %s", events.VerbTokenRevoked, routine.ActorId, server.member.ID)
	}
	switch {
	case administrative == nil:
		t.Fatalf("an administrator's revocation left no %s row; it is indistinguishable from a rotation",
			events.VerbTokenAdminRevoked)
	case administrative.ActorId == nil || administrative.ActorId.String() != server.admin.ID:
		t.Errorf("%s names actor %v, want the administrator %s",
			events.VerbTokenAdminRevoked, administrative.ActorId, server.admin.ID)
	}

	// Whose access stopped. A feed entry naming only the token is one a reader
	// has to go and look up, at the moment they can least afford to.
	delta, err := json.Marshal(administrative.Delta)
	if err != nil {
		t.Fatalf("re-encoding the delta: %v", err)
	}
	if !strings.Contains(string(delta), server.member.ID) {
		t.Errorf("the administrative revocation does not say whose token it was: %s", delta)
	}

	// And the routine one is not filed under the administrative verb, which is
	// the half that makes the filter worth having.
	if findActivity(rows,
		events.VerbTokenAdminRevoked, ownRotation.ServiceToken.Id.String()) != nil {
		t.Errorf("the owner rotating their own token was recorded as %s", events.VerbTokenAdminRevoked)
	}
}

// activityRows reads the platform feed as the administrator, which is the only
// caller that may.
func (s *adminTokenServer) activityRows(t *testing.T) []gen.ActivityEntry {
	t.Helper()

	recorder := s.get(BasePath+"/activity?limit=200", s.adminSession)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /activity = %d, want 200\nbody: %s", recorder.Code, recorder.Body)
	}
	return decodeJSON[gen.ActivityPage](t, recorder).Items
}

// findActivity returns the row with this verb about this object, or nil.
func findActivity(rows []gen.ActivityEntry, verb events.Verb, objectID string) *gen.ActivityEntry {
	for i, row := range rows {
		if row.Verb == string(verb) && row.ObjectId == objectID {
			return &rows[i]
		}
	}
	return nil
}
