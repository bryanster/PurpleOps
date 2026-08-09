package authz_test

// The five named regression cases from PLAN.md §9, one test function each, named
// after the v1 bug rather than after the rule that fixes it.
//
// The name is the point. matrix_test.go asserts the whole model and would fail
// for any of these too, but it fails as one wrong cell among 1,840 — and a
// reviewer reading a diff that widens a role needs to see the name of the defect
// they are about to reintroduce, not a coordinate. Deleting one of these is
// therefore a deliberate act with the bug's name on it.
//
// Each is enumerative where the bug was: v1's failures were never "one endpoint
// was wrong", they were "nobody could say which endpoints were checked". So
// TestRegression_ObserverCannotWrite asks about every write there is, rather than
// about the one that was reported.
//
// Every one of these has been verified by breaking its rule in
// internal/authz/policy.go and watching it fail — the results are recorded in
// docs/tickets/done/M1-014-permission-matrix-tests.md. A regression test that
// passes against the bug it is named for is worse than no test, because it is
// evidence somebody will trust.

import (
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
)

// everyScope is a service token carrying every scope this build defines. The
// token tests below use it so that a denial is unambiguously the owner's role
// running out — a token short of a scope would be refused for a reason that has
// nothing to do with the regression.
func everyScope() []authz.TokenScope {
	return authz.TokenScopes()
}

// TestRegression_NonAdminCannotReachUserManagement is v1's `/manage/access`
// (PLAN.md §4): the page that listed accounts, changed their roles and disabled
// them, reachable by anybody who was signed in. The check lived in the navigation
// — the link was hidden from non-admins — and the handler behind it asked nobody
// anything.
//
// So this asks about every administrative action rather than about the one
// endpoint, as every seat, under both authentication methods, with every scope a
// token can carry. The hole was not that one check was wrong; it was that
// "administrative" was not a thing the code had a word for.
func TestRegression_NonAdminCannotReachUserManagement(t *testing.T) {
	// Everything that page did, plus the two settings actions and the activity
	// log, which are the same kind of authority over the installation.
	administrative := []authz.Action{
		authz.ActionUserRead,
		authz.ActionUserManage,
		authz.ActionSettingsRead,
		authz.ActionSettingsManage,
		authz.ActionActivityRead,
		authz.ActionContentSync,
		authz.ActionContentManage,
	}

	for _, action := range administrative {
		resource := resourceFor(t, action)

		for _, seat := range seatsIncludingNone() {
			for _, subject := range []authz.Subject{
				sessionSubject(authz.PlatformRoleMember, seat),
				tokenSubject(authz.PlatformRoleMember, seat, everyScope()),
			} {
				if decision := authz.Can(t.Context(), subject, action, resource); decision.Allowed {
					t.Errorf("a platform member %s, on %s, holds %s: %s\n"+
						"This is v1's ungated /manage/access. Administration of the installation belongs to "+
						"the admin platform role and to nothing else.",
						seatName(seat), subject.Method, action, decision.Reason)
				}
			}
		}

		// The control. Without it this test would pass just as well against a
		// policy that refused everybody, which would prove nothing about roles.
		if decision := authz.Can(t.Context(), sessionSubject(authz.PlatformRoleAdmin, ""),
			action, resource); !decision.Allowed {
			t.Errorf("a platform administrator does not hold %s either, so the assertion above is not "+
				"about the member role: %s", action, decision.Reason)
		}
	}
}

// TestRegression_ObserverCannotWrite is v1's Spectator fall-through (PLAN.md §4):
// the read-only role that reached write endpoints, because the handlers asked
// "are you a member of this engagement" and the answer for a spectator was yes.
//
// The fix is not a better check in those handlers — it is that no write rule
// lists the observer. This enumerates every write there is, so a rule added in
// M2–M6 that hands one to an observer fails here rather than in an incident.
func TestRegression_ObserverCannotWrite(t *testing.T) {
	writes := []authz.Action{
		authz.ActionEngagementManage,
		authz.ActionEngagementDelete,
		authz.ActionMemberManage,
		authz.ActionExecutionWriteRed,
		authz.ActionExecutionWriteBlue,
		authz.ActionFindingWrite,
		authz.ActionReportPublish,
	}

	for _, action := range writes {
		resource := resourceFor(t, action)

		for _, subject := range []authz.Subject{
			sessionSubject(authz.PlatformRoleMember, authz.EngagementRoleObserver),
			tokenSubject(authz.PlatformRoleMember, authz.EngagementRoleObserver, everyScope()),
		} {
			decision := authz.Can(t.Context(), subject, action, resource)
			if decision.Allowed {
				t.Errorf("an observer holds %s, on %s: %s\nThis is v1's Spectator, which fell through to "+
					"write access. An observer reads and comments.", action, subject.Method, decision.Reason)
				continue
			}
			// 403 rather than 404, and the difference matters: an observer is a
			// member. Concealing the engagement from somebody who was invited to
			// it would be a bug in the other direction, and it is the kind that
			// gets reported as "the app is broken" rather than as a leak.
			if decision.Conceal {
				t.Errorf("%s refused an observer by concealing the engagement: %s\nThey are a member of it "+
					"— they may see it and may not write to it, which is a 403.", action, decision.Reason)
			}
		}
	}

	// The one write the seat does hold. Without this the test above would pass
	// against a role that could do nothing at all, which is a different product.
	if decision := authz.Can(t.Context(),
		sessionSubject(authz.PlatformRoleMember, authz.EngagementRoleObserver),
		authz.ActionCommentWrite, resourceFor(t, authz.ActionCommentWrite)); !decision.Allowed {
		t.Errorf("an observer cannot comment: %s\nReading and commenting is the seat; a seat that cannot "+
			"comment is not the one this role was for.", decision.Reason)
	}

	// Observer also holds report.write (M6-002): drafting a report is exactly
	// the kind of contribution an observer seat is for.
	if decision := authz.Can(t.Context(),
		sessionSubject(authz.PlatformRoleMember, authz.EngagementRoleObserver),
		authz.ActionReportWrite, resourceFor(t, authz.ActionReportWrite)); !decision.Allowed {
		t.Errorf("an observer cannot write a report draft: %s\nDrafting reports is a seat for every member.", decision.Reason)
	}
}

// TestRegression_BlueCannotWriteRedFields is v1's two contradictory definitions
// of "blue" (PLAN.md §4): one handler's idea of the role and another's disagreed,
// and the gap between them was write access to the other side's fields.
//
// "And the reverse" is in the ticket and in this test, because the defect was
// symmetrical: the fix is not that blue is narrower than it was, it is that the
// two sides are different actions with different rules and neither is a subset of
// the other. A red operator writing the detection record is the same defect
// wearing the other hat — it would make the exercise's own evidence unreliable.
func TestRegression_BlueCannotWriteRedFields(t *testing.T) {
	// A standard engagement, so that nothing here is blind mode doing the work.
	// The blind case is the test below.
	execution := resourceFor(t, authz.ActionExecutionWriteRed)

	for _, side := range []struct {
		seat    authz.EngagementRole
		holds   authz.Action
		refused authz.Action
	}{
		{authz.EngagementRoleBlue, authz.ActionExecutionWriteBlue, authz.ActionExecutionWriteRed},
		{authz.EngagementRoleRed, authz.ActionExecutionWriteRed, authz.ActionExecutionWriteBlue},
	} {
		for _, subject := range []authz.Subject{
			sessionSubject(authz.PlatformRoleMember, side.seat),
			tokenSubject(authz.PlatformRoleMember, side.seat, everyScope()),
		} {
			if decision := authz.Can(t.Context(), subject, side.refused, execution); decision.Allowed {
				t.Errorf("the %s seat holds %s, on %s: %s\nThis is v1's two definitions of \"blue\". The "+
					"attack side and the detection side are different actions, and neither role holds the "+
					"other's.", side.seat, side.refused, subject.Method, decision.Reason)
			}
			if decision := authz.Can(t.Context(), subject, side.holds, execution); !decision.Allowed {
				t.Errorf("the %s seat does not hold %s either, on %s: %s\nThe separation is supposed to be "+
					"between the two sides, not a refusal of both.",
					side.seat, side.holds, subject.Method, decision.Reason)
			}
		}
	}
}

// TestRegression_BlueCannotSeeUnrevealedStepsInBlindMode is the blind-mode leak
// (PLAN.md §4): in an engagement run blind, the detection side could read the
// steps they were supposed to be detecting.
//
// Two things are asserted rather than one. That blue is refused — and that the
// refusal conceals, because learning a step exists is most of what blind mode
// withholds, and a 403 on a real step beside a 404 on an imaginary one tells the
// blue side exactly how many are coming.
func TestRegression_BlueCannotSeeUnrevealedStepsInBlindMode(t *testing.T) {
	unrevealed := blindExecution(false)
	revealed := blindExecution(true)
	standard := resourceFor(t, authz.ActionExecutionRead)

	// Both halves: reading the step, and recording a detection against it, which
	// would tell blue the step is there just as loudly.
	for _, action := range []authz.Action{authz.ActionExecutionRead, authz.ActionExecutionWriteBlue} {
		blue := sessionSubject(authz.PlatformRoleMember, authz.EngagementRoleBlue)

		decision := authz.Can(t.Context(), blue, action, unrevealed)
		switch {
		case decision.Allowed:
			t.Errorf("blue holds %s on an unrevealed step of a blind engagement: %s\n"+
				"This is the blind-mode leak: the detection side could read what it was supposed to be "+
				"detecting.", action, decision.Reason)
		case !decision.Conceal:
			t.Errorf("blue was refused %s on an unrevealed step, but not concealed: %s\n"+
				"A 403 here admits the step exists, which is most of what blind mode withholds.",
				action, decision.Reason)
		}

		// The reveal is what lifts it. Without this the test above would pass
		// against a policy that never let blue near an execution, which is not
		// blind mode — it is a broken product with no detection side.
		if decision := authz.Can(t.Context(), blue, action, revealed); !decision.Allowed {
			t.Errorf("blue does not hold %s on a *revealed* step either: %s\nBlind mode withholds until "+
				"the reveal; it is not a permanent exclusion.", action, decision.Reason)
		}

		// And the flag alone does nothing. An engagement that is not blind has
		// no unrevealed steps — every step reads as revealed — so a rule that
		// checked Revealed without checking whether the engagement is blind
		// would show up here.
		if decision := authz.Can(t.Context(), blue, action, standard); !decision.Allowed {
			t.Errorf("blue does not hold %s in a standard engagement: %s\nThe guard is supposed to turn on "+
				"blind mode, not on the reveal flag by itself.", action, decision.Reason)
		}

		// Red is unaffected, on the read. They ran the step.
		red := sessionSubject(authz.PlatformRoleMember, authz.EngagementRoleRed)
		if decision := authz.Can(t.Context(), red, authz.ActionExecutionRead, unrevealed); !decision.Allowed {
			t.Errorf("red cannot read an unrevealed step of their own blind engagement: %s\n"+
				"Blind mode withholds from the detection side; it is not an embargo on the exercise.",
				decision.Reason)
		}
	}

	// The guard binds to the seat, not to the platform role. An administrator who
	// has taken the blue chair is sitting in it — they can have the unblinded
	// view by not sitting there.
	adminInTheBlueSeat := sessionSubject(authz.PlatformRoleAdmin, authz.EngagementRoleBlue)
	if decision := authz.Can(t.Context(), adminInTheBlueSeat,
		authz.ActionExecutionRead, unrevealed); decision.Allowed {
		t.Errorf("a platform administrator sitting in the blue seat reads unrevealed steps: %s\n"+
			"The blind-mode guard is about the chair, not about the account.", decision.Reason)
	}

	// And an administrator who is not a member of the engagement is not in the
	// exercise at all, so there is nothing to blind them to.
	if decision := authz.Can(t.Context(), sessionSubject(authz.PlatformRoleAdmin, ""),
		authz.ActionExecutionRead, unrevealed); !decision.Allowed {
		t.Errorf("a platform administrator who is not a member cannot read a blind engagement's steps: %s\n"+
			"They hold every engagement-scoped action on every engagement; blind mode withholds from a "+
			"seat, and they are not in one.", decision.Reason)
	}
}

// TestRegression_ServiceTokenCannotExceedOwnerPermissions is v1's "API keys
// authenticate nothing" (PLAN.md §4), stated as the property M1-011 replaced it
// with: a token is its owner, narrowed, and never anything more.
//
// The interesting half is the demotion. A token issued to an administrator is not
// an administrator's token — it is that person's token, and the moment they stop
// being an administrator it stops reaching what only administrators reach, with
// no change to the token and nothing to revoke.
func TestRegression_ServiceTokenCannotExceedOwnerPermissions(t *testing.T) {
	users := resourceFor(t, authz.ActionUserManage)

	// The same token, before and after: identical scopes, identical binding,
	// only the owner's live role differs.
	asAdmin := tokenSubject(authz.PlatformRoleAdmin, "", everyScope())
	afterDemotion := tokenSubject(authz.PlatformRoleMember, "", everyScope())

	if decision := authz.Can(t.Context(), asAdmin, authz.ActionUserManage, users); !decision.Allowed {
		t.Fatalf("a token carrying every scope, owned by an administrator, cannot manage users: %s\n"+
			"The demotion below would then prove nothing.", decision.Reason)
	}
	if decision := authz.Can(t.Context(), afterDemotion, authz.ActionUserManage, users); decision.Allowed {
		t.Errorf("the same token still manages users after its owner was demoted to member: %s\n"+
			"This is v1's API key: a credential that outlived the authority it was issued under.",
			decision.Reason)
	}

	// The same property one level down, inside an engagement: the scope is not a
	// second way in. An observer's token carrying engagements:write may do what
	// an observer may do, which does not include raising findings.
	findings := resourceFor(t, authz.ActionFindingWrite)
	observer := tokenSubject(authz.PlatformRoleMember, authz.EngagementRoleObserver,
		[]authz.TokenScope{authz.TokenScopeEngagementsWrite})
	if decision := authz.Can(t.Context(), observer, authz.ActionFindingWrite, findings); decision.Allowed {
		t.Errorf("an observer's token carrying engagements:write raises findings: %s\n"+
			"A scope is the second fence, never the first — it narrows what the owner may do and cannot "+
			"widen it.", decision.Reason)
	}

	// And the fences are independent: holding the scope is not enough, and being
	// the owner is not enough. A lead's token that does not carry the write scope
	// is refused as surely as an observer's that does.
	leadWithoutTheScope := tokenSubject(authz.PlatformRoleMember, authz.EngagementRoleLead,
		[]authz.TokenScope{authz.TokenScopeEngagementsRead})
	if decision := authz.Can(t.Context(), leadWithoutTheScope,
		authz.ActionFindingWrite, findings); decision.Allowed {
		t.Errorf("a lead's token raises findings without carrying engagements:write: %s\n"+
			"Both fences are required; either one alone is v1.", decision.Reason)
	}
}

// --- helpers ----------------------------------------------------------------

// resourceFor returns the resource an action acts on, taken from the checked-in
// matrix so that the two files cannot disagree about what a question is. It is
// the standard state: a non-blind engagement for anything an engagement owns.
func resourceFor(t *testing.T, action authz.Action) authz.Resource {
	t.Helper()

	for _, r := range matrix {
		if r.Action != action {
			continue
		}
		return r.variants()[0].Resource
	}
	t.Fatalf("no matrix row for %s, so this test cannot say what it would act on", action)
	return authz.Resource{}
}

// blindExecution is a step of a blind engagement, revealed or not.
func blindExecution(revealed bool) authz.Resource {
	return authz.Resource{
		Type:            authz.ResourceExecution,
		ID:              "0192f1a0-0000-7000-8000-000000000010",
		EngagementID:    engagement,
		EngagementBlind: true,
		Revealed:        revealed,
	}
}

// sessionSubject is a caller who signed in with a browser, holding one seat in
// the engagement under test and a lead's seat in another one.
func sessionSubject(role authz.PlatformRole, seat authz.EngagementRole) authz.Subject {
	subject := subjectWith(role, seat)
	subject.Method = authz.MethodCookie
	return subject
}

// tokenSubject is the same caller, arriving on a service token.
func tokenSubject(role authz.PlatformRole, seat authz.EngagementRole,
	scopes []authz.TokenScope) authz.Subject {
	subject := subjectWith(role, seat)
	subject.Method = authz.MethodServiceToken
	subject.TokenScopes = scopes
	return subject
}

// seatsIncludingNone is every engagement role plus the absence of one, which is
// the set a caller can be in with respect to one engagement.
func seatsIncludingNone() []authz.EngagementRole {
	return append(authz.EngagementRoles(), "")
}
