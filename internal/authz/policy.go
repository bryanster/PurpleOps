package authz

import (
	"context"
	"fmt"
	"slices"
)

// Rule is one line of the permission model: who holds one action, and under
// what extra condition.
//
// The model is data rather than control flow. Every rule is a row in [rules]
// below, [Can] is the same twenty lines for all of them, and docs/authz.md is
// rendered from the table — so what the code enforces and what the
// documentation claims cannot drift, and a reviewer checks the model by reading
// a table instead of by tracing branches. v1's model was branches, in
// forty-odd handlers, and no two of them agreed.
type Rule struct {
	// Action is the verb, and Name is its single wire spelling. They are
	// together because an action's name belongs to the rule that defines it;
	// a second lookup table would be a second thing to keep in step.
	Action Action
	Name   string

	// Resource is the type this action acts on. A call naming any other type
	// is denied — asking to manage a user "on" an engagement is a caller bug,
	// and a policy that quietly ignored the mismatch would be answering a
	// question nobody asked.
	Resource ResourceType

	// Platform lists the platform roles that hold this action outright. For an
	// engagement-scoped action that means *on every engagement*, membership or
	// not, which is what PlatformRoleAdmin is for.
	Platform []PlatformRole

	// Engagement lists the engagement roles that hold it within their own
	// engagement, and is empty for a platform-scoped action.
	Engagement []EngagementRole

	// Token is the service-token scope required in addition to the above
	// (M1-011). Every rule names one: an action no scope covers would be an
	// action a token could take unconditionally.
	Token TokenScope

	// Guard is an extra condition, evaluated only after a role grants the
	// action. [GuardNone] for most rules.
	Guard Guard

	// Summary is the one-line description docs/authz.md renders. It is part of
	// the rule because a permission nobody can describe is a permission nobody
	// reviewed.
	Summary string
}

// Guard is a condition beyond "does this role hold this action". There are two,
// and adding another means adding a case to [Guard.blocks] — which is where a
// reviewer will look, because the table names it.
type Guard string

const (
	// GuardNone is the usual case: the role decides and nothing else does.
	GuardNone Guard = ""

	// GuardBlindMode withholds an unrevealed step from the blue side of a
	// blind engagement (PLAN.md §4).
	//
	// It binds to the *engagement role*, so a platform administrator who is
	// also a blue member of the engagement is held to it: they took the blue
	// seat, and an administrator who wants the unblinded view can have it by
	// not sitting in it. An administrator who is not a member is unaffected.
	//
	// internal/store/blind additionally filters unrevealed steps in the query
	// layer (M1-013), so a rule that forgot this guard still could not leak
	// one. Both, deliberately: this is the belt, that is the braces.
	GuardBlindMode Guard = "blind-mode"

	// GuardSessionOnly withholds an action from a request that authenticated
	// with a service token, whatever scopes that token carries and whatever
	// its owner may do (M1-011).
	//
	// It is for the small set of actions whose effect is to change what
	// credentials exist. A token that can mint tokens is a token that can
	// outlive its own revocation, and the two fences do not catch that: the
	// sibling it mints exceeds neither the owner's role nor the scope list, it
	// merely exists afterwards. The fix is that the act of creating a
	// credential requires a person at a keyboard.
	//
	// It refuses rather than conceals. The endpoint is not a secret — every
	// caller can read it in api/openapi.yaml — and the honest answer is "not
	// with this credential", which is a 403.
	GuardSessionOnly Guard = "session-only"
)

// Role sets, named so the table below reads as a table. Treat them as
// constants: [Rules] clones them on the way out, and nothing here writes to
// them.
var (
	admins   = []PlatformRole{PlatformRoleAdmin}
	everyone = []PlatformRole{PlatformRoleAdmin, PlatformRoleMember}

	// allMembers is every engagement role, which is "any member of this
	// engagement" — the read side, plus commenting.
	allMembers  = []EngagementRole{EngagementRoleLead, EngagementRoleRed, EngagementRoleBlue, EngagementRoleObserver}
	leadOnly    = []EngagementRole{EngagementRoleLead}
	leadAndRed  = []EngagementRole{EngagementRoleLead, EngagementRoleRed}
	leadAndBlue = []EngagementRole{EngagementRoleLead, EngagementRoleBlue}

	// writers is everyone who may record work: not the observer. The Spectator
	// regression is this variable not containing EngagementRoleObserver.
	writers = []EngagementRole{EngagementRoleLead, EngagementRoleRed, EngagementRoleBlue}
)

// rules is the permission model. Reading down the Platform and Engagement
// columns is reading the whole of it; docs/authz.md is this table rendered.
//
// Three v1 defects are visible here as absences, and each has a named
// regression test in M1-014:
//
//   - PlatformRoleMember appears in no platform-administration rule — the
//     ungated /manage/access.
//   - EngagementRoleObserver appears in no write rule but comment.write — the
//     Spectator fall-through.
//   - EngagementRoleRed and EngagementRoleBlue never appear in each other's
//     execution rule — the two contradictory definitions of "blue".
//
// Adding an [Action] without adding a row here fails
// TestEveryActionHasExactlyOneRule. There is no default.
var rules = []Rule{
	// ── Platform ────────────────────────────────────────────────────────────
	{Action: ActionUserRead, Name: "user.read", Resource: ResourceUser,
		Platform: admins, Token: TokenScopeAdminRead,
		Summary: "Read accounts, their platform roles and their status."},
	{Action: ActionUserManage, Name: "user.manage", Resource: ResourceUser,
		Platform: admins, Token: TokenScopeAdminWrite,
		Summary: "Invite, edit, disable and delete accounts, and set platform roles."},
	{Action: ActionSettingsRead, Name: "settings.read", Resource: ResourcePlatform,
		Platform: admins, Token: TokenScopeAdminRead,
		Summary: "Read platform settings, including how hard this installation is to sign in to."},
	{Action: ActionSettingsManage, Name: "settings.manage", Resource: ResourcePlatform,
		Platform: admins, Token: TokenScopeAdminWrite,
		Summary: "Change platform settings, including the MFA policy."},
	{Action: ActionActivityRead, Name: "activity.read", Resource: ResourcePlatform,
		Platform: admins, Token: TokenScopeAdminRead,
		Summary: "Read the installation-wide activity log."},
	{Action: ActionContentRead, Name: "content.read", Resource: ResourceContent,
		Platform: everyone, Token: TokenScopeContentRead,
		Summary: "Read the shared technique and test-case library."},
	{Action: ActionContentSync, Name: "content.sync", Resource: ResourceContent,
		Platform: admins, Token: TokenScopeContentSync,
		Summary: "Sync the library from its upstream sources."},
	// content.manage is deliberately a third action rather than folding enable /
	// disable / delete / metadata edits into content.sync: automation must be
	// able to refresh a library without holding the right to remove one, and
	// the two scopes (content:sync vs content:write) are how that split is
	// expressed on a service token (M2-002).
	{Action: ActionContentManage, Name: "content.manage", Resource: ResourceContent,
		Platform: admins, Token: TokenScopeContentWrite,
		Summary: "Enable, disable, delete and edit content sources, and author custom content."},
	{Action: ActionEngagementCreate, Name: "engagement.create", Resource: ResourcePlatform,
		Platform: everyone, Token: TokenScopeEngagementsWrite,
		Summary: "Create an engagement. Acts on the installation, because the engagement does not exist yet."},

	// Service tokens (M1-011). Everybody holds both, and what stops that being
	// "anybody may read anybody's tokens" is not a rule here — it is that the
	// endpoints and the repository behind them only ever name the caller's own
	// rows, the way GET /auth/me does. That is scoping rather than permission,
	// and this table decides permission.
	//
	// [GuardSessionOnly] is the part that is a decision. Without it a token
	// holding admin:write could mint a sibling with a longer expiry, which
	// would survive the revocation of the token that made it — a compromised
	// credential turning itself into a permanent one, inside the fences,
	// without ever exceeding its owner. The scope named below is therefore
	// unreachable by construction; it is the honest one for the action, so
	// that removing the guard would not silently widen anything.
	{Action: ActionTokenRead, Name: "token.read", Resource: ResourceServiceToken,
		Platform: everyone, Token: TokenScopeAdminRead, Guard: GuardSessionOnly,
		Summary: "List your own service tokens and when they were last used. Never their secrets."},
	{Action: ActionTokenManage, Name: "token.manage", Resource: ResourceServiceToken,
		Platform: everyone, Token: TokenScopeAdminWrite, Guard: GuardSessionOnly,
		Summary: "Create and revoke your own service tokens. A signed-in session only, never a token itself."},

	// The administrative half (M1-018): the tokens *somebody else* holds, which
	// an administrator has to be able to see and end during an incident without
	// disabling the person along with them.
	//
	// They act on a [ResourceUser] and not on a [ResourceServiceToken], because
	// the thing the request names is an account — the question is "what does
	// this account hold", and the answer is a set rather than a row. That also
	// keeps the pair above meaning exactly what they say: your own tokens.
	//
	// admins rather than everyone is the whole of the difference from the pair
	// above, and it is what makes the endpoints safe to point at another
	// account: there is no scoping to the caller in the repository, so the role
	// is the only fence and it is stated here.
	//
	// [GuardSessionOnly] for the reason the pair above carries it, and M1-018
	// asks for it in as many words: a service token must not be in the business
	// of deciding which credentials exist, and an administrator's token is the
	// one this would matter most for — it could otherwise end every other
	// token in the installation.
	{Action: ActionTokenAdminRead, Name: "token.admin_read", Resource: ResourceUser,
		Platform: admins, Token: TokenScopeAdminRead, Guard: GuardSessionOnly,
		Summary: "Read the service tokens one account holds, and when they were last used. Never their secrets."},
	{Action: ActionTokenAdminManage, Name: "token.admin_manage", Resource: ResourceUser,
		Platform: admins, Token: TokenScopeAdminWrite, Guard: GuardSessionOnly,
		Summary: "Revoke somebody else's service token. A signed-in session only, never a token itself."},

	// Sessions (M1-017), and the same two paragraphs apply word for word: the
	// endpoints only ever name the caller's own rows, so "everybody" here is
	// scoping rather than permission, and [GuardSessionOnly] is the decision.
	//
	// The guard matters more here than it does above, because a session is the
	// thing a service token is not. A token that could end its owner's browser
	// sessions would be a leaked credential able to sign the person out while
	// it kept working — and one that could *read* them would be a leaked
	// credential enumerating where its owner is signed in from. Neither is
	// something a token needs, and a token has no session of its own to manage.
	{Action: ActionSessionRead, Name: "session.read", Resource: ResourceSession,
		Platform: everyone, Token: TokenScopeAdminRead, Guard: GuardSessionOnly,
		Summary: "List the browsers you are signed in on. Never the tokens in their cookies."},
	{Action: ActionSessionManage, Name: "session.manage", Resource: ResourceSession,
		Platform: everyone, Token: TokenScopeAdminWrite, Guard: GuardSessionOnly,
		Summary: "Revoke your own sessions, one at a time or everywhere but here. A signed-in session only."},

	// ── Engagement ──────────────────────────────────────────────────────────
	{Action: ActionEngagementRead, Name: "engagement.read", Resource: ResourceEngagement,
		Platform: admins, Engagement: allMembers, Token: TokenScopeEngagementsRead,
		Summary: "See that an engagement exists and read its settings."},
	{Action: ActionEngagementManage, Name: "engagement.manage", Resource: ResourceEngagement,
		Platform: admins, Engagement: leadOnly, Token: TokenScopeEngagementsWrite,
		Summary: "Change an engagement's settings, including whether it runs blind."},
	{Action: ActionEngagementDelete, Name: "engagement.delete", Resource: ResourceEngagement,
		Platform: admins, Engagement: leadOnly, Token: TokenScopeEngagementsWrite,
		Summary: "Delete an engagement and everything in it."},
	{Action: ActionWorkbookWrite, Name: "workbook.write", Resource: ResourceEngagement,
		Platform: admins, Engagement: leadAndRed, Token: TokenScopeEngagementsWrite,
		Summary: "Create, edit, reorder and delete scenarios, steps and their contents."},

	{Action: ActionMemberRead, Name: "member.read", Resource: ResourceMember,
		Platform: admins, Engagement: allMembers, Token: TokenScopeEngagementsRead,
		Summary: "Read who is in an engagement and in what seat."},
	{Action: ActionMemberManage, Name: "member.manage", Resource: ResourceMember,
		Platform: admins, Engagement: leadOnly, Token: TokenScopeEngagementsWrite,
		Summary: "Add, remove and re-seat members of one engagement."},
	{Action: ActionExecutionRead, Name: "execution.read", Resource: ResourceExecution,
		Platform: admins, Engagement: allMembers, Token: TokenScopeEngagementsRead, Guard: GuardBlindMode,
		Summary: "Read an execution and its steps."},
	{Action: ActionExecutionWriteRed, Name: "execution.write_red", Resource: ResourceExecution,
		Platform: admins, Engagement: leadAndRed, Token: TokenScopeEngagementsWrite,
		Summary: "Write the attack side of an execution: what was run, when, and how."},
	{Action: ActionExecutionWriteBlue, Name: "execution.write_blue", Resource: ResourceExecution,
		Platform: admins, Engagement: leadAndBlue, Token: TokenScopeEngagementsWrite, Guard: GuardBlindMode,
		Summary: "Write the detection side of an execution: what was seen, alerted and blocked."},
	{Action: ActionEvidenceRead, Name: "evidence.read", Resource: ResourceEvidence,
		Platform: admins, Engagement: allMembers, Token: TokenScopeEngagementsRead, Guard: GuardBlindMode,
		Summary: "Read evidence metadata and content. Same membership as execution.read, blind-mode guarded."},
	{Action: ActionEvidenceWrite, Name: "evidence.write", Resource: ResourceEvidence,
		Platform: admins, Engagement: writers, Token: TokenScopeEngagementsWrite,
		Summary: "Upload and delete evidence: screenshots, logs, packet captures. Lead, red and blue, not observer."},
	{Action: ActionCommentRead, Name: "comment.read", Resource: ResourceComment,
		Platform: admins, Engagement: allMembers, Token: TokenScopeEngagementsRead,
		Summary: "Read comments and their revision history."},

	{Action: ActionCommentWrite, Name: "comment.write", Resource: ResourceComment,
		Platform: admins, Engagement: allMembers, Token: TokenScopeEngagementsWrite,
		Summary: "Comment. The one write an observer holds, because reading and commenting is the seat."},
	{Action: ActionFindingWrite, Name: "finding.write", Resource: ResourceFinding,
		Platform: admins, Engagement: writers, Token: TokenScopeEngagementsWrite,
		Summary: "Raise and edit findings."},
	{Action: ActionFindingRead, Name: "finding.read", Resource: ResourceFinding,
		Platform: admins, Engagement: allMembers, Token: TokenScopeEngagementsRead,
		Summary: "Read findings in an engagement. Same membership as engagement.read."},
	{Action: ActionReportRead, Name: "report.read", Resource: ResourceReport,
		Platform: admins, Engagement: allMembers, Token: TokenScopeReportsRead,
		Summary: "Read and export an engagement's reports."},
	{Action: ActionReportPublish, Name: "report.publish", Resource: ResourceReport,
		Platform: admins, Engagement: leadOnly, Token: TokenScopeReportsWrite,
		Summary: "Publish a report, which is the act that makes it somebody else's evidence."},
	{Action: ActionReportWrite, Name: "report.write", Resource: ResourceReport,
		Platform: admins, Engagement: allMembers, Token: TokenScopeReportsWrite,
		Summary: "Create and edit report drafts. Every member of the engagement may draft."},
}

// Rules returns the permission model. docs/authz.md and the permission matrix
// (M1-014) render from it; nothing else should need it.
//
// A deep copy, including the role slices. The alternative is handing out the
// live table and asking callers not to write to it, and a permission model a
// caller can edit at runtime is not a permission model.
func Rules() []Rule {
	out := make([]Rule, len(rules))
	for i, rule := range rules {
		rule.Platform = slices.Clone(rule.Platform)
		rule.Engagement = slices.Clone(rule.Engagement)
		out[i] = rule
	}
	return out
}

// rulesByAction indexes the table. Built once, from the table, and the only
// way [Can] reaches a rule — so an action with no row is unreachable rather
// than defaulted.
var rulesByAction = func() map[Action]Rule {
	byAction := make(map[Action]Rule, len(rules))
	for _, rule := range rules {
		byAction[rule.Action] = rule
	}
	return byAction
}()

// ruleFor returns the rule for an action, and false when there is none.
func ruleFor(action Action) (Rule, bool) {
	rule, ok := rulesByAction[action]
	return rule, ok
}

// Can decides whether subject may take action on resource. It is the only
// function in this system that answers that question (PLAN.md §4), and no
// handler is given what it would need to answer it for itself.
//
// It is pure: no I/O, no database, no clock, and nothing read from ctx. Every
// fact it uses arrives in its arguments, which is what lets M1-014 assert the
// entire model in milliseconds and what stops a rule from quietly becoming a
// query. ctx is in the signature because callers have one and a future
// implementation must not force a signature change on every call site to use
// it — it is deliberately unread today.
//
// Denial is the default at every step. An unknown action, an unauthenticated
// caller, a resource of the wrong type, an engagement resource that does not
// say which engagement, a role no rule lists, a guard, a token pointed outside
// the engagement it is bound to, a missing token scope: each is a deny, and
// each says why.
func Can(ctx context.Context, subject Subject, action Action, resource Resource) Decision {
	_ = ctx

	rule, ok := ruleFor(action)
	if !ok {
		return deny(fmt.Sprintf("%s is not an action this build knows", action))
	}

	if !subject.authenticated() {
		return deny(fmt.Sprintf("nothing authenticated this request, so it holds no %s", rule.Name))
	}

	if resource.Type != rule.Resource {
		return deny(fmt.Sprintf("%s acts on a %s, and the request named a %s",
			rule.Name, rule.Resource, resource.Type))
	}

	if rule.Resource.EngagementScoped() && resource.EngagementID == "" {
		return deny(fmt.Sprintf("%s acts on a %s belonging to an engagement, and the request named none",
			rule.Name, rule.Resource))
	}

	by, decision := grant(subject, rule, resource)
	if !decision.Allowed {
		return decision
	}

	if refusal, blocked := rule.Guard.blocks(subject, by, resource); blocked {
		return refusal
	}

	if subject.Method != MethodServiceToken {
		return decision
	}
	return tokenFences(subject, rule, resource, decision)
}

// tokenFences applies what a service token is held to beyond its owner's role
// (M1-011, PLAN.md §9).
//
// They run after the role half so that a demoted owner is told they were
// demoted rather than that their token is short a scope: the token did not
// change, and the reason a reader is looking for is the one that did.
//
// The binding is checked before the scope because it conceals and the scope
// does not. A token bound to one engagement, asked about another, must not
// produce an answer that differs from the answer for an engagement that does
// not exist — and a 403 saying "wrong scope" for a real engagement beside a
// 404 for an imaginary one is exactly that difference.
func tokenFences(subject Subject, rule Rule, resource Resource, decision Decision) Decision {
	if bound := subject.TokenEngagementID; bound != "" {
		switch {
		case !rule.Resource.EngagementScoped():
			// Nothing outside an engagement, including the installation
			// itself. There is no engagement to compare against here, so this
			// is not a concealable question: the caller already knows the
			// platform exists.
			return deny(fmt.Sprintf("this token is bound to engagement %s and %s acts on the installation",
				bound, rule.Name))
		case resource.EngagementID != bound:
			return concealed(fmt.Sprintf("this token is bound to engagement %s, and the request named %s",
				bound, resource.EngagementID))
		}
	}

	if !subject.holdsScope(rule.Token) {
		return deny(fmt.Sprintf("%s needs the %s token scope, which this token does not carry",
			rule.Name, rule.Token))
	}
	return decision
}

// grantedBy records which role permitted an action, so that a guard can ask —
// blind mode applies to the blue seat, not to everyone who happens to be
// allowed.
type grantedBy struct {
	// EngagementRole is the seat that granted it, and is empty when the grant
	// came from the platform role instead.
	EngagementRole EngagementRole
}

// grant applies the role half of a rule: the platform roles that hold it
// outright, then the engagement roles that hold it inside their own
// engagement.
//
// Platform first, because [PlatformRoleAdmin] holds every engagement-scoped
// action on every engagement and must not be turned away for not being a
// member of one.
func grant(subject Subject, rule Rule, resource Resource) (grantedBy, Decision) {
	// The seat is read first and reported whichever fence granted the action,
	// because a guard asks about the seat and not about how the grant arrived:
	// an administrator sitting in the blue chair of a blind engagement is
	// sitting in the blue chair. It is "" for a platform-scoped resource and
	// for anybody who is not a member.
	seat, member := subject.MembershipIn(resource.EngagementID)

	for _, role := range rule.Platform {
		if subject.PlatformRole == role {
			return grantedBy{EngagementRole: seat},
				allow(fmt.Sprintf("the %s platform role holds %s", role, rule.Name))
		}
	}

	if !rule.Resource.EngagementScoped() {
		return grantedBy{}, deny(fmt.Sprintf("the %s platform role does not hold %s",
			subject.PlatformRole, rule.Name))
	}

	if !member {
		// Concealed, not merely denied. PLAN.md §4: a non-member gets nothing
		// on an engagement "including its existence", so M1-013 answers this
		// with a 404 — a 403 here would confirm the engagement is real.
		return grantedBy{}, concealed(fmt.Sprintf("not a member of engagement %s", resource.EngagementID))
	}

	for _, role := range rule.Engagement {
		if seat == role {
			return grantedBy{EngagementRole: seat},
				allow(fmt.Sprintf("%s in engagement %s holds %s", seat, resource.EngagementID, rule.Name))
		}
	}

	return grantedBy{}, deny(fmt.Sprintf("%s in engagement %s does not hold %s",
		seat, resource.EngagementID, rule.Name))
}

// blocks applies a guard to an otherwise-granted action, and returns the
// refusal when it withholds it.
//
// It returns a whole [Decision] rather than a reason because the two guards
// refuse differently: blind mode must not admit the step exists, and
// session-only has nothing to hide. Deciding that here keeps "403 or 404" a
// question the policy answers, which is the same reason [Decision.Conceal]
// exists at all.
func (g Guard) blocks(subject Subject, by grantedBy, resource Resource) (Decision, bool) {
	switch g {
	case GuardNone:
		return Decision{}, false

	case GuardBlindMode:
		if by.EngagementRole != EngagementRoleBlue || !resource.EngagementBlind || resource.Revealed {
			return Decision{}, false
		}
		return concealed(fmt.Sprintf("engagement %s runs blind and this %s has not been revealed",
			resource.EngagementID, resource.Type)), true

	case GuardSessionOnly:
		if subject.Method != MethodServiceToken {
			return Decision{}, false
		}
		return deny("this action is not available to a service token, whatever it is scoped for"), true

	default:
		// A guard the table names and this function does not implement. Deny:
		// a condition nobody wrote is not a condition that passes.
		return deny(fmt.Sprintf("the %s guard is not implemented in this build", g)), true
	}
}
