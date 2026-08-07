package authz_test

// The rule table's own invariants. These are the tests that make the model
// checkable rather than merely present: if adding an [authz.Action] could
// silently mean "nobody holds it", the exhaustive matrix in M1-014 would be
// asserting a table with holes in it.

import (
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
)

// TestEveryActionHasExactlyOneRule is the exhaustiveness gate, and the reason
// Action is an integer enum with a sentinel: adding a constant moves the
// sentinel, Actions() grows, and this fails until somebody decides who holds
// the new verb. Silence is not a permission.
func TestEveryActionHasExactlyOneRule(t *testing.T) {
	covered := map[authz.Action]int{}
	for _, rule := range authz.Rules() {
		covered[rule.Action]++
	}

	for _, action := range authz.Actions() {
		switch covered[action] {
		case 1:
		case 0:
			t.Errorf("no rule covers %s — add a row to the table in internal/authz/policy.go", action)
		default:
			t.Errorf("%d rules cover %s; two rows for one action means one of them is dead", covered[action], action)
		}
	}

	if got, want := len(authz.Rules()), len(authz.Actions()); got != want {
		t.Errorf("the table has %d rules for %d actions, so it covers something that is not one", got, want)
	}
}

// TestEveryRuleIsWellFormed checks the parts of a row that are easy to leave
// out and impossible to notice: the name, the scope a token needs, the summary
// docs/authz.md renders, and roles that actually exist.
func TestEveryRuleIsWellFormed(t *testing.T) {
	for _, rule := range authz.Rules() {
		switch {
		case rule.Name == "":
			t.Errorf("the rule for action %d has no name", int(rule.Action))
		case !strings.Contains(rule.Name, "."):
			t.Errorf("%q is not <resource>.<verb>", rule.Name)
		case rule.Token == "":
			t.Errorf("%s names no token scope, so a service token could take it unconditionally", rule.Name)
		case rule.Summary == "":
			t.Errorf("%s has no summary, so docs/authz.md documents it as a blank", rule.Name)
		case !strings.HasSuffix(rule.Summary, "."):
			t.Errorf("%s's summary is not a sentence: %q", rule.Name, rule.Summary)
		}

		for _, role := range rule.Platform {
			if !role.Valid() {
				t.Errorf("%s grants to the platform role %q, which is not one", rule.Name, role)
			}
		}
		for _, role := range rule.Engagement {
			if !role.Valid() {
				t.Errorf("%s grants to the engagement role %q, which is not one", rule.Name, role)
			}
		}

		if !rule.Resource.EngagementScoped() && len(rule.Engagement) > 0 {
			t.Errorf("%s acts on a %s, which no engagement owns, but grants to engagement roles %v",
				rule.Name, rule.Resource, rule.Engagement)
		}
	}
}

// TestActionNamesAreUniqueAndParse: the wire name is how M1-013 gets from
// x-authz-action in api/openapi.yaml to a constant, so two rules sharing one
// would silently map an endpoint to the wrong rule.
func TestActionNamesAreUniqueAndParse(t *testing.T) {
	seen := map[string]authz.Action{}

	for _, rule := range authz.Rules() {
		if first, dup := seen[rule.Name]; dup {
			t.Errorf("%q names both action %d and action %d", rule.Name, int(first), int(rule.Action))
		}
		seen[rule.Name] = rule.Action

		parsed, ok := authz.ParseAction(rule.Name)
		switch {
		case !ok:
			t.Errorf("ParseAction(%q) found nothing, but the table defines it", rule.Name)
		case parsed != rule.Action:
			t.Errorf("ParseAction(%q) = %s, want itself", rule.Name, parsed)
		}

		if got := rule.Action.String(); got != rule.Name {
			t.Errorf("%s.String() = %q, want %q", rule.Name, got, rule.Name)
		}
	}
}

// TestParseActionRejectsWhatItDoesNotKnow. M1-013 fails the server's startup on
// a false here, so a guess would be an unprotected endpoint.
func TestParseActionRejectsWhatItDoesNotKnow(t *testing.T) {
	for _, name := range []string{"", "unknown", "engagement.destroy", "ENGAGEMENT.READ", "engagement"} {
		if action, ok := authz.ParseAction(name); ok {
			t.Errorf("ParseAction(%q) = %s, want no match", name, action)
		}
	}
}

// TestTheRegressionsAreAbsences reads the three v1 defects out of the table
// directly. M1-014 asserts them through Can as named regression tests; this
// asserts them of the data, so a row edited in the wrong direction fails here
// first and says which row.
func TestTheRegressionsAreAbsences(t *testing.T) {
	rules := map[authz.Action]authz.Rule{}
	for _, rule := range authz.Rules() {
		rules[rule.Action] = rule
	}

	// The /manage/access hole: platform member holds nothing administrative.
	for _, action := range []authz.Action{
		authz.ActionUserRead, authz.ActionUserManage,
		authz.ActionSettingsRead, authz.ActionSettingsManage,
		authz.ActionActivityRead, authz.ActionContentSync, authz.ActionContentManage,
	} {
		if grantsPlatform(rules[action], authz.PlatformRoleMember) {
			t.Errorf("%s grants to the member platform role — this is v1's ungated /manage/access", action)
		}
	}

	// The Spectator fall-through: observer holds exactly one write, and it is
	// commenting.
	for _, rule := range authz.Rules() {
		if !grantsEngagement(rule, authz.EngagementRoleObserver) {
			continue
		}
		if isWrite(rule.Name) && rule.Action != authz.ActionCommentWrite {
			t.Errorf("%s grants to observer — an observer reads and comments and writes nothing else", rule.Name)
		}
	}

	// The two definitions of "blue": neither side holds the other's write.
	if grantsEngagement(rules[authz.ActionExecutionWriteRed], authz.EngagementRoleBlue) {
		t.Error("execution.write_red grants to blue")
	}
	if grantsEngagement(rules[authz.ActionExecutionWriteBlue], authz.EngagementRoleRed) {
		t.Error("execution.write_blue grants to red")
	}
}

// TestResourceOwnershipIsDeclaredForEveryRule. A resource type absent from the
// ownership map reads as platform-owned, which would mean an engagement's
// contents needing no membership — the failure is silent, so it is asserted.
func TestResourceOwnershipIsDeclaredForEveryRule(t *testing.T) {
	engagementOwned := map[authz.ResourceType]bool{
		authz.ResourceEngagement: true,
		authz.ResourceMember:     true,
		authz.ResourceExecution:  true,
		authz.ResourceComment:    true,
		authz.ResourceFinding:    true,
		authz.ResourceScenario:   true,
		authz.ResourceEvidence:  true,
		authz.ResourceReport:     true,
	}
	platformOwned := map[authz.ResourceType]bool{
		authz.ResourcePlatform: true,
		authz.ResourceUser:     true,
		authz.ResourceContent:  true,
		// A token bound to one engagement is still owned by the installation:
		// the binding constrains where it may point (M1-011), and who may hold
		// one is not a question about membership.
		authz.ResourceServiceToken: true,
		// A session belongs to the person signed in on it, and the
		// installation is what issued it. No engagement has a say (M1-017).
		authz.ResourceSession: true,
	}

	for _, rule := range authz.Rules() {
		switch {
		case engagementOwned[rule.Resource] && !rule.Resource.EngagementScoped():
			t.Errorf("%s acts on a %s, which belongs to an engagement, but the type is not engagement-scoped",
				rule.Name, rule.Resource)
		case platformOwned[rule.Resource] && rule.Resource.EngagementScoped():
			t.Errorf("%s acts on a %s, which the installation owns, but the type is engagement-scoped",
				rule.Name, rule.Resource)
		case !engagementOwned[rule.Resource] && !platformOwned[rule.Resource]:
			t.Errorf("%s acts on %q, which this test does not know about — declare who owns it",
				rule.Name, rule.Resource)
		}
	}
}

// TestAnUnknownResourceTypeOwnsNothing: a type nobody declared must not read as
// platform-owned by default, because platform-owned means "no membership
// needed". It is denied at the type check in Can instead, which this asserts.
func TestAnUnknownResourceTypeOwnsNothing(t *testing.T) {
	invented := authz.ResourceType("engagement-but-spelled-differently")

	if invented.EngagementScoped() {
		t.Error("an undeclared resource type reads as engagement-scoped, which this test's premise denies")
	}

	decision := authz.Can(t.Context(), admin(), authz.ActionEngagementRead,
		authz.Resource{Type: invented, EngagementID: engagement})
	if decision.Allowed {
		t.Fatalf("engagement.read was allowed against %q: %s", invented, decision.Reason)
	}
}

// TestTokenScopesComeFromTheTable, so M1-011 can render the spec's scope enum
// from here rather than maintaining a second list.
func TestTokenScopesComeFromTheTable(t *testing.T) {
	declared := map[authz.TokenScope]bool{}
	for _, scope := range authz.TokenScopes() {
		if declared[scope] {
			t.Errorf("TokenScopes() lists %q twice", scope)
		}
		declared[scope] = true
	}

	for _, rule := range authz.Rules() {
		if !declared[rule.Token] {
			t.Errorf("%s requires %q, which TokenScopes() does not list", rule.Name, rule.Token)
		}
	}
}

// TestRulesAreHandedOutByValue: the table is the model, and a caller that could
// edit what Rules() returns could grant themselves an action at runtime. The
// role slices are the half that a shallow copy would leave shared.
func TestRulesAreHandedOutByValue(t *testing.T) {
	rules := authz.Rules()
	rules[0] = authz.Rule{}
	rules[1].Platform[0] = authz.PlatformRoleMember
	rules[2].Engagement = append(rules[2].Engagement, authz.EngagementRoleObserver)

	fresh := authz.Rules()
	switch {
	case fresh[0].Name == "":
		t.Error("replacing a row in the slice from Rules() emptied the policy's row")
	case fresh[1].Platform[0] != authz.PlatformRoleAdmin:
		t.Errorf("editing a Platform slice from Rules() demoted %s to %s in the policy",
			fresh[1].Name, fresh[1].Platform[0])
	case len(fresh[2].Engagement) != len(rules[2].Engagement)-1:
		t.Errorf("appending to an Engagement slice from Rules() changed %s in the policy", fresh[2].Name)
	}
}

func grantsPlatform(rule authz.Rule, role authz.PlatformRole) bool {
	for _, granted := range rule.Platform {
		if granted == role {
			return true
		}
	}
	return false
}

func grantsEngagement(rule authz.Rule, role authz.EngagementRole) bool {
	for _, granted := range rule.Engagement {
		if granted == role {
			return true
		}
	}
	return false
}

// isWrite reads the verb out of an action's name. Naming is load-bearing here:
// an action that changes something and is not called write/manage/publish/
// create/delete/sync would slip past this, which is why the names are a
// convention TestEveryRuleIsWellFormed also checks the shape of.
func isWrite(name string) bool {
	_, verb, _ := strings.Cut(name, ".")
	for _, write := range []string{"write", "manage", "publish", "create", "delete", "sync"} {
		if verb == write || strings.HasPrefix(verb, write+"_") {
			return true
		}
	}
	return false
}
