package authz

import "fmt"

// Action is a verb this system can be asked to perform. It is a closed
// enumeration: call sites name a constant, never a string, so a typo is a
// compile error rather than a silent deny — or, in v1's case, a silent allow.
//
// The constants are integers rather than strings for one reason: [numActions]
// below is what makes exhaustiveness checkable. Adding a constant moves the
// sentinel, and TestEveryActionHasExactlyOneRule fails until the rule table
// covers it. There is no way to add an action and forget to decide who holds
// it, which is the whole design.
//
// The wire spelling ("engagement.read") lives in the rule table, so an action
// has exactly one name and it is attached to the rule that defines it. M1-013's
// x-authz-action extension resolves through [ParseAction].
type Action int

// One block, because the sentinel at the bottom only counts what is above it.
// Actions are grouped by what they act on; the grouping is a comment rather
// than a second const block so that nothing can be added outside the count.
const (
	// ActionUnknown is the zero value and is never a real action. It exists so
	// that an Action nobody set cannot mean the first constant in the block —
	// "forgot to fill the field" and "asked to read an engagement" must not be
	// the same value.
	ActionUnknown Action = iota

	// Platform-scoped: done to the installation rather than inside an
	// engagement. Every rule below lists admin and omits member. That omission
	// is v1's ungated /manage/access, written as data.
	ActionUserRead
	ActionUserManage
	ActionSettingsRead
	ActionSettingsManage
	ActionActivityRead
	ActionContentRead
	ActionContentSync
	ActionContentManage
	ActionEngagementCreate

	// Service tokens (M1-011). Both are held by everybody, over their own
	// tokens and nobody else's — see the rule table, which says why that is a
	// scoping rule rather than a permission one.
	ActionTokenRead
	ActionTokenManage

	// The same two over *somebody else's* tokens (M1-018), which is a different
	// question and therefore a different action: the two above are held by
	// everybody and scoped to the caller's own rows, and these are held by
	// administrators and name an account. Folding the administrative case into
	// the pair above would make "everybody holds this" stop being true, and
	// folding it into user.manage would drop the session-only guard that keeps
	// a leaked token out of the credential business.
	ActionTokenAdminRead
	ActionTokenAdminManage

	// Sessions (M1-017). The same shape as the two above, over the browsers
	// somebody is signed in on rather than the credentials they issued.
	ActionSessionRead
	ActionSessionManage

	// Engagement-scoped: done inside one engagement, which the caller must be
	// a member of — or a platform administrator, who holds all of these on
	// every engagement.
	ActionEngagementRead
	ActionEngagementManage
	ActionEngagementDelete
	ActionMemberRead
	ActionMemberManage
	ActionExecutionRead
	ActionExecutionWriteRed
	ActionExecutionWriteBlue
	ActionCommentWrite
	ActionFindingWrite
	ActionReportRead
	ActionReportPublish
	ActionWorkbookWrite

	ActionEvidenceRead
	ActionEvidenceWrite

	ActionCommentRead

	// numActions is one past the last action, and must stay last. It is the
	// exhaustiveness check: the rule table is required to cover every value in
	// (ActionUnknown, numActions).
	numActions
)

// Actions returns every action, in declaration order. docs/authz.md and the
// permission matrix (M1-014) enumerate from here rather than from a list
// somebody maintains, so neither can fall behind the enum.
func Actions() []Action {
	actions := make([]Action, 0, numActions-1)
	for a := ActionUnknown + 1; a < numActions; a++ {
		actions = append(actions, a)
	}
	return actions
}

// String returns the wire name — "engagement.read" — or a placeholder naming
// the numeric value for anything outside the enum. It never returns "", so a
// log line about a bogus action still says which one.
func (a Action) String() string {
	if rule, ok := ruleFor(a); ok {
		return rule.Name
	}
	if a == ActionUnknown {
		return "unknown"
	}
	return fmt.Sprintf("action(%d)", int(a))
}

// ParseAction resolves a wire name to its constant. It reports false for
// anything unrecognised, and callers must treat that as a refusal: M1-013 fails
// the server's startup on it, because an operation mapped to an action that
// does not exist is an unprotected endpoint.
func ParseAction(name string) (Action, bool) {
	action, ok := actionsByName[name]
	return action, ok
}

// actionsByName indexes the rule table by wire name. Built from the table, so
// there is no second list to keep in step.
var actionsByName = func() map[string]Action {
	byName := make(map[string]Action, len(rules))
	for _, rule := range rules {
		byName[rule.Name] = rule.Action
	}
	return byName
}()

// TokenScope is the coarse permission a service token carries (M1-011). Scopes
// are few and blunt on purpose — the ticket's words are "resist a per-endpoint
// scope explosion" — so several actions share one.
//
// A scope is the *second* fence, never the first: holding engagements:write
// permits nothing the owner's own role does not already permit. See [Can].
type TokenScope string

const (
	TokenScopeEngagementsRead  TokenScope = "engagements:read"
	TokenScopeEngagementsWrite TokenScope = "engagements:write"
	TokenScopeContentRead      TokenScope = "content:read"
	TokenScopeContentSync      TokenScope = "content:sync"
	TokenScopeContentWrite     TokenScope = "content:write"
	TokenScopeReportsRead      TokenScope = "reports:read"
	TokenScopeReportsWrite     TokenScope = "reports:write"
	TokenScopeAdminRead        TokenScope = "admin:read"
	TokenScopeAdminWrite       TokenScope = "admin:write"
)

// ParseTokenScope resolves a wire name to its constant, and reports false for
// anything no rule requires.
//
// It is what refuses a scope at the moment a token is created (M1-011). Storing
// an unrecognised one would not grant anything — [Can] grants on scopes it
// holds, never on scopes it fails to recognise — but it would hand somebody a
// credential that silently does less than the list they typed, and they would
// find out from a 403 in a pipeline rather than from a 400 at creation.
func ParseTokenScope(name string) (TokenScope, bool) {
	for _, scope := range TokenScopes() {
		if TokenScope(name) == scope {
			return scope, true
		}
	}
	return "", false
}

// TokenScopes returns every scope that some rule requires, in the order the
// rules declare them. M1-011 renders the list into api/openapi.yaml from here.
func TokenScopes() []TokenScope {
	seen := make(map[TokenScope]bool, len(rules))
	scopes := make([]TokenScope, 0, len(rules))
	for _, rule := range rules {
		if !seen[rule.Token] {
			seen[rule.Token] = true
			scopes = append(scopes, rule.Token)
		}
	}
	return scopes
}
