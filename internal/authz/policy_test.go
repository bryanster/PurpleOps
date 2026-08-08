package authz_test

// What [authz.Can] does, at the boundaries. The exhaustive role × action ×
// resource matrix is M1-014's file; these are the cases that define the shape
// of the function rather than the contents of the table — default deny, the
// four ways a call can be malformed, the two fences a service token passes, and
// the guard.
//
// External test package on purpose: everything the middleware, the matrix and
// the doc generator need is exported, and a test that reaches inside would stop
// proving that.

import (
	"context"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
)

// engagement is the engagement most of these tests act on. A UUID-shaped string
// so that a reason containing it looks like the real thing.
const engagement = "0192f1a0-0000-7000-8000-000000000001"

// member builds a subject with one membership.
func member(role authz.EngagementRole) authz.Subject {
	return authz.Subject{
		UserID:       "user-1",
		PlatformRole: authz.PlatformRoleMember,
		Method:       authz.MethodCookie,
		Memberships:  map[string]authz.EngagementRole{engagement: role},
	}
}

// admin is a platform administrator who is a member of nothing.
func admin() authz.Subject {
	return authz.Subject{
		UserID:       "admin-1",
		PlatformRole: authz.PlatformRoleAdmin,
		Method:       authz.MethodCookie,
	}
}

// executionIn is a step of the engagement above, revealed and not blind.
func executionIn(id string) authz.Resource {
	return authz.Resource{Type: authz.ResourceExecution, ID: "exec-1", EngagementID: id, Revealed: true}
}

// TestAnUnknownActionIsDenied is default deny at its most basic: a value no
// rule covers holds nothing, rather than falling through to something.
func TestAnUnknownActionIsDenied(t *testing.T) {
	// Well past the enum, and deliberately not a value any constant has: this
	// is what a build that has been given an action from a newer spec sees.
	unknown := authz.Action(9999)

	decision := authz.Can(t.Context(), admin(), unknown, authz.Resource{Type: authz.ResourcePlatform})

	if decision.Allowed {
		t.Fatalf("Can(admin, action(9999)) allowed it: %s", decision.Reason)
	}
	if !strings.Contains(decision.Reason, "9999") {
		t.Errorf("the reason does not name the action: %s", decision.Reason)
	}
}

// TestTheZeroActionIsDenied: ActionUnknown is the zero value, so a struct field
// nobody filled in must not resolve to the first constant in the block.
func TestTheZeroActionIsDenied(t *testing.T) {
	var unset authz.Action

	if decision := authz.Can(t.Context(), admin(), unset, authz.Resource{Type: authz.ResourceUser}); decision.Allowed {
		t.Fatalf("the zero Action was allowed: %s", decision.Reason)
	}
}

// TestNobodyIsDenied covers the zero subject and the two half-filled ones. An
// unauthenticated caller reaching Can is normal — the authn middleware lets
// anonymous requests through on purpose, because refusing is this package's job.
func TestNobodyIsDenied(t *testing.T) {
	for name, subject := range map[string]authz.Subject{
		"the zero subject": {},
		"a user ID with no method": {
			UserID: "user-1", PlatformRole: authz.PlatformRoleAdmin,
		},
		"a method with no user ID": {
			PlatformRole: authz.PlatformRoleAdmin, Method: authz.MethodCookie,
		},
	} {
		t.Run(name, func(t *testing.T) {
			decision := authz.Can(t.Context(), subject, authz.ActionContentRead,
				authz.Resource{Type: authz.ResourceContent})

			if decision.Allowed {
				t.Fatalf("Can() allowed %s: %s", name, decision.Reason)
			}
			if decision.Reason == "" {
				t.Error("the denial carries no reason")
			}
		})
	}
}

// TestAResourceOfTheWrongTypeIsDenied. Asking to manage a user "on" an
// engagement is a caller bug; answering it would be answering a question nobody
// asked, and the answer might be yes.
func TestAResourceOfTheWrongTypeIsDenied(t *testing.T) {
	decision := authz.Can(t.Context(), admin(), authz.ActionUserManage,
		authz.Resource{Type: authz.ResourceEngagement, EngagementID: engagement})

	if decision.Allowed {
		t.Fatalf("user.manage was allowed against an engagement: %s", decision.Reason)
	}
	if !strings.Contains(decision.Reason, "user") || !strings.Contains(decision.Reason, "engagement") {
		t.Errorf("the reason does not name both types: %s", decision.Reason)
	}
}

// TestAnEngagementResourceMustNameItsEngagement. An engagement-scoped resource
// that cannot say which engagement owns it is denied rather than treated as
// unowned — "unowned" would mean "no membership required", which is the whole
// permission model deleted by an empty string.
func TestAnEngagementResourceMustNameItsEngagement(t *testing.T) {
	decision := authz.Can(t.Context(), member(authz.EngagementRoleLead), authz.ActionExecutionRead,
		authz.Resource{Type: authz.ResourceExecution, ID: "exec-1"})

	if decision.Allowed {
		t.Fatalf("execution.read was allowed with no engagement: %s", decision.Reason)
	}
}

// TestAPlatformAdminHoldsEveryEngagement is the reach the role is for: PLAN.md
// §4 gives admin "every engagement", membership or not.
func TestAPlatformAdminHoldsEveryEngagement(t *testing.T) {
	decision := authz.Can(t.Context(), admin(), authz.ActionExecutionWriteRed, executionIn(engagement))

	if !decision.Allowed {
		t.Fatalf("a platform admin was refused execution.write_red: %s", decision.Reason)
	}
	if !strings.Contains(decision.Reason, string(authz.PlatformRoleAdmin)) {
		t.Errorf("the reason does not say which role allowed it: %s", decision.Reason)
	}
}

// TestANonMemberIsConcealedRatherThanRefused. PLAN.md §4: a non-member gets
// nothing on an engagement "including its existence" — so the denial is marked
// for M1-013 to answer 404. A 403 would confirm the engagement is real.
func TestANonMemberIsConcealedRatherThanRefused(t *testing.T) {
	other := "0192f1a0-0000-7000-8000-00000000ffff"

	decision := authz.Can(t.Context(), member(authz.EngagementRoleLead), authz.ActionEngagementRead,
		authz.Resource{Type: authz.ResourceEngagement, ID: other, EngagementID: other})

	switch {
	case decision.Allowed:
		t.Fatalf("a lead of one engagement could read another: %s", decision.Reason)
	case !decision.Conceal:
		t.Errorf("the denial is not concealed, so M1-013 would answer 403 and confirm %s exists", other)
	}
}

// TestAMemberInTheWrongSeatIsRefusedButNotConcealed: they already know the
// engagement exists, so hiding it from them would be a lie they can see through
// and a 404 where a 403 is the honest answer.
func TestAMemberInTheWrongSeatIsRefusedButNotConcealed(t *testing.T) {
	decision := authz.Can(t.Context(), member(authz.EngagementRoleObserver), authz.ActionExecutionWriteRed,
		executionIn(engagement))

	switch {
	case decision.Allowed:
		t.Fatalf("an observer was allowed to write red fields: %s", decision.Reason)
	case decision.Conceal:
		t.Error("the denial is concealed, but a member already knows the engagement exists")
	case !strings.Contains(decision.Reason, string(authz.EngagementRoleObserver)):
		t.Errorf("the reason does not name the seat: %s", decision.Reason)
	}
}

// TestBlindModeWithholdsAnUnrevealedStepFromBlue, and from nobody else. The
// guard is why execution.read is not simply "any member".
func TestBlindModeWithholdsAnUnrevealedStepFromBlue(t *testing.T) {
	unrevealed := authz.Resource{
		Type: authz.ResourceExecution, ID: "exec-1", EngagementID: engagement,
		EngagementBlind: true, Revealed: false,
	}

	for _, test := range []struct {
		name     string
		subject  authz.Subject
		resource authz.Resource
		want     bool
	}{
		{"blue, blind, unrevealed", member(authz.EngagementRoleBlue), unrevealed, false},
		{"blue, blind, revealed", member(authz.EngagementRoleBlue), reveal(unrevealed), true},
		{"blue, not blind", member(authz.EngagementRoleBlue), unblind(unrevealed), true},
		{"red, blind, unrevealed", member(authz.EngagementRoleRed), unrevealed, true},
		{"lead, blind, unrevealed", member(authz.EngagementRoleLead), unrevealed, true},
		{"a non-member admin, blind, unrevealed", admin(), unrevealed, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			decision := authz.Can(t.Context(), test.subject, authz.ActionExecutionRead, test.resource)

			if decision.Allowed != test.want {
				t.Fatalf("Can(execution.read) = %t, want %t: %s", decision.Allowed, test.want, decision.Reason)
			}
			if !decision.Allowed && !decision.Conceal {
				t.Error("the blind-mode denial is not concealed, so the step's existence leaks in the status code")
			}
		})
	}
}

// TestBlindModeAlsoWithholdsTheWrite. Writing detection against a step you may
// not read is the same leak from the other end: a write that succeeds confirms
// the step is there.
func TestBlindModeAlsoWithholdsTheWrite(t *testing.T) {
	unrevealed := authz.Resource{
		Type: authz.ResourceExecution, ID: "exec-1", EngagementID: engagement,
		EngagementBlind: true,
	}

	if decision := authz.Can(t.Context(), member(authz.EngagementRoleBlue),
		authz.ActionExecutionWriteBlue, unrevealed); decision.Allowed {
		t.Fatalf("blue wrote detection onto an unrevealed step: %s", decision.Reason)
	}
}

// TestAnAdminInTheBlueSeatIsHeldToBlindMode. Documented behaviour rather than
// an accident: taking the seat is taking the seat, and an administrator who
// wants the unblinded view can have it by not being a member.
func TestAnAdminInTheBlueSeatIsHeldToBlindMode(t *testing.T) {
	subject := admin()
	subject.Memberships = map[string]authz.EngagementRole{engagement: authz.EngagementRoleBlue}

	decision := authz.Can(t.Context(), subject, authz.ActionExecutionRead, authz.Resource{
		Type: authz.ResourceExecution, ID: "exec-1", EngagementID: engagement, EngagementBlind: true,
	})

	if decision.Allowed {
		t.Fatalf("an admin sitting in the blue seat read an unrevealed step: %s", decision.Reason)
	}
}

// TestAServiceTokenNeedsTheScope is the second fence. The owner is an
// administrator, so the first fence passes and only the scope is in question.
func TestAServiceTokenNeedsTheScope(t *testing.T) {
	subject := admin()
	subject.Method = authz.MethodServiceToken
	subject.TokenScopes = []authz.TokenScope{authz.TokenScopeAdminRead}

	if decision := authz.Can(t.Context(), subject, authz.ActionUserRead,
		authz.Resource{Type: authz.ResourceUser}); !decision.Allowed {
		t.Fatalf("a token scoped admin:read could not read users: %s", decision.Reason)
	}

	decision := authz.Can(t.Context(), subject, authz.ActionUserManage, authz.Resource{Type: authz.ResourceUser})
	switch {
	case decision.Allowed:
		t.Fatalf("a token scoped admin:read managed users: %s", decision.Reason)
	case !strings.Contains(decision.Reason, string(authz.TokenScopeAdminWrite)):
		t.Errorf("the reason does not name the missing scope: %s", decision.Reason)
	}
}

// TestAServiceTokenCannotExceedItsOwner is the first fence, and the case
// PLAN.md §9 names: a token minted while its holder was an administrator stops
// being an administrator's token the moment they are demoted, with no change to
// the token.
func TestAServiceTokenCannotExceedItsOwner(t *testing.T) {
	token := authz.Subject{
		UserID:       "user-1",
		PlatformRole: authz.PlatformRoleAdmin,
		Method:       authz.MethodServiceToken,
		TokenScopes:  []authz.TokenScope{authz.TokenScopeAdminWrite},
	}

	if decision := authz.Can(t.Context(), token, authz.ActionUserManage,
		authz.Resource{Type: authz.ResourceUser}); !decision.Allowed {
		t.Fatalf("an admin's token could not manage users: %s", decision.Reason)
	}

	// The same token, one demotion later. Nothing about the token changed.
	token.PlatformRole = authz.PlatformRoleMember

	decision := authz.Can(t.Context(), token, authz.ActionUserManage, authz.Resource{Type: authz.ResourceUser})
	switch {
	case decision.Allowed:
		t.Fatalf("the demoted owner's token still managed users: %s", decision.Reason)
	case !strings.Contains(decision.Reason, string(authz.PlatformRoleMember)):
		t.Errorf("the reason blames the scope rather than the demotion: %s", decision.Reason)
	}
}

// TestAnEngagementBoundTokenReachesNothingElse is the third fence: a token
// created against one engagement holds nothing anywhere else, however broad its
// scopes and however senior its owner. The denial for another engagement
// conceals, because a token that answered 403 for a real engagement and 404 for
// an imaginary one would be an engagement enumerator.
func TestAnEngagementBoundTokenReachesNothingElse(t *testing.T) {
	const other = "0192f1a0-0000-7000-8000-00000000000f"

	// An administrator's token — so nothing below can be blamed on the role —
	// carrying every scope the actions under test require.
	token := admin()
	token.Method = authz.MethodServiceToken
	token.TokenScopes = []authz.TokenScope{authz.TokenScopeEngagementsRead, authz.TokenScopeAdminRead}
	token.TokenEngagementID = engagement

	if decision := authz.Can(t.Context(), token, authz.ActionExecutionRead,
		executionIn(engagement)); !decision.Allowed {
		t.Fatalf("a token bound to %s could not read its own engagement: %s", engagement, decision.Reason)
	}

	elsewhere := authz.Can(t.Context(), token, authz.ActionExecutionRead, executionIn(other))
	switch {
	case elsewhere.Allowed:
		t.Errorf("a token bound to %s read a step of %s: %s", engagement, other, elsewhere.Reason)
	case !elsewhere.Conceal:
		t.Errorf("the refusal for another engagement admits it exists: %s", elsewhere.Reason)
	}

	// And nothing on the installation, which is not concealed: the caller
	// already knows the platform is there.
	platform := authz.Can(t.Context(), token, authz.ActionSettingsRead,
		authz.Resource{Type: authz.ResourcePlatform})
	switch {
	case platform.Allowed:
		t.Errorf("a token bound to an engagement read platform settings: %s", platform.Reason)
	case platform.Conceal:
		t.Errorf("the refusal conceals the installation, which every caller can see: %s", platform.Reason)
	}
}

// TestAnUnboundTokenIsNotFencedByAnEmptyBinding: "" means unbound and must not
// be compared against a resource's engagement, which would deny everything.
func TestAnUnboundTokenIsNotFencedByAnEmptyBinding(t *testing.T) {
	token := admin()
	token.Method = authz.MethodServiceToken
	token.TokenScopes = []authz.TokenScope{authz.TokenScopeEngagementsRead}

	if decision := authz.Can(t.Context(), token, authz.ActionExecutionRead,
		executionIn(engagement)); !decision.Allowed {
		t.Fatalf("an unbound token was fenced out of an engagement: %s", decision.Reason)
	}
}

// TestATokenCannotMintATokenHoweverItIsScoped is [authz.GuardSessionOnly]: the
// one thing the two fences do not catch is a credential creating a credential,
// because the sibling exceeds neither the owner nor the scope list — it merely
// outlives the revocation of the token that made it.
func TestATokenCannotMintATokenHoweverItIsScoped(t *testing.T) {
	token := admin()
	token.Method = authz.MethodServiceToken
	token.TokenScopes = authz.TokenScopes() // every scope this build has.

	for _, action := range []authz.Action{authz.ActionTokenRead, authz.ActionTokenManage} {
		decision := authz.Can(t.Context(), token, action, authz.Resource{Type: authz.ResourceServiceToken})
		switch {
		case decision.Allowed:
			t.Errorf("a fully scoped administrator's token holds %s: %s", action, decision.Reason)
		case decision.Conceal:
			t.Errorf("%s was refused with a concealment; the endpoint is in the published spec: %s",
				action, decision.Reason)
		}
	}

	// The same actions, from a session: an ordinary member manages their own
	// tokens, which is what makes the refusals above about the credential and
	// not about the role.
	for _, action := range []authz.Action{authz.ActionTokenRead, authz.ActionTokenManage} {
		if decision := authz.Can(t.Context(), member(authz.EngagementRoleObserver), action,
			authz.Resource{Type: authz.ResourceServiceToken}); !decision.Allowed {
			t.Errorf("a signed-in member does not hold %s over their own tokens: %s", action, decision.Reason)
		}
	}
}

// TestASessionIgnoresTokenScopes: the scope fence exists for tokens only, and a
// browser session carrying none must not be fenced by their absence.
func TestASessionIgnoresTokenScopes(t *testing.T) {
	if decision := authz.Can(t.Context(), admin(), authz.ActionUserManage,
		authz.Resource{Type: authz.ResourceUser}); !decision.Allowed {
		t.Fatalf("a signed-in administrator was refused user.manage: %s", decision.Reason)
	}
}

// TestEveryDecisionCarriesAReason sweeps the whole table against several
// subjects and asserts the one property every answer must have. A denial with
// no reason is v1's bare 403: indistinguishable from a rule nobody wrote.
func TestEveryDecisionCarriesAReason(t *testing.T) {
	subjects := map[string]authz.Subject{
		"admin":    admin(),
		"lead":     member(authz.EngagementRoleLead),
		"observer": member(authz.EngagementRoleObserver),
		"nobody":   {},
	}

	for _, rule := range authz.Rules() {
		for name, subject := range subjects {
			resource := authz.Resource{Type: rule.Resource, ID: "resource-1", Revealed: true}
			if rule.Resource.EngagementScoped() {
				resource.EngagementID = engagement
			}

			decision := authz.Can(t.Context(), subject, rule.Action, resource)
			if decision.Reason == "" {
				t.Errorf("Can(%s, %s) returned allowed=%t with no reason", name, rule.Name, decision.Allowed)
			}
		}
	}
}

// TestCanIsAPureFunctionOfItsArguments: the same question twice, from a context
// that has been cancelled in between. A different answer would mean something
// outside the arguments got a vote, and the M1-014 matrix would be asserting a
// model that does not hold at runtime.
func TestCanIsAPureFunctionOfItsArguments(t *testing.T) {
	subject, resource := member(authz.EngagementRoleRed), executionIn(engagement)

	first := authz.Can(t.Context(), subject, authz.ActionExecutionWriteRed, resource)

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	second := authz.Can(cancelled, subject, authz.ActionExecutionWriteRed, resource)

	if first != second {
		t.Errorf("Can() answered %+v then %+v for the same arguments", first, second)
	}
}

func reveal(r authz.Resource) authz.Resource  { r.Revealed = true; return r }
func unblind(r authz.Resource) authz.Resource { r.EngagementBlind = false; return r }
