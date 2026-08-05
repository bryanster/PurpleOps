package authz_test

// The permission matrix (M1-014). PLAN.md §9 asks for "the full (platform role ×
// engagement role × action × resource) matrix asserted in one table, with named
// regression cases". This file is the table; regressions_test.go is the cases.
//
// Every v1 authorization failure was individually obvious in hindsight. What was
// missing was one artifact saying, for every combination, what the answer is —
// so nobody could look at a role and tell whether an omission was a decision. The
// table below is that artifact, and the tests around it are what stop it becoming
// a table with a hole in it.
//
// # How to add a row
//
// Add an [authz.Action] and TestTheMatrixDecidesEveryActionInEveryState fails,
// naming the action. To fix it, add one entry to [matrix]:
//
//  1. Copy the nearest existing entry. Set Action, Resource, Scope and Token to
//     match the rule you added in internal/authz/policy.go — this file states all
//     four independently and the test compares them against the rule table, so a
//     disagreement is a failure rather than a drift.
//  2. Fill in the grids. A platform-scoped action gets Platform and nothing else;
//     an engagement-scoped one gets Standard, BlindRevealed and BlindUnrevealed
//     and no Platform. Every one of the ten cells in a grid must name an outcome
//     — [allow], [deny403], [deny404] or [na]. An empty cell fails the test,
//     because a cell nobody filled in is a decision nobody made.
//
// Do not compute an expectation from the rule table. The value of this file is
// that it is a second, independently written statement of the model: one that
// derived its answers the way [authz.Can] does would agree with a bug.
//
// The auth-method dimension is not a column. It is [lenses] below — four ways of
// arriving at the same cell — because what a service token changes is uniform
// across every action, and stating it once as an invariant is both shorter and
// stronger than restating it 220 times.

import (
	"fmt"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authz"
)

// otherEngagement is an engagement that is never the one under test. Every
// caller in this file is a lead of it, whatever seat they hold in `engagement`,
// so that the "no seat" column means "a member of something else" rather than
// "a member of nothing" — a matrix built from someone who belongs to no
// engagement at all would not notice a rule that let a seat travel.
const otherEngagement = "0192f1a0-0000-7000-8000-000000000002"

// outcome is what [authz.Can] answered, in the vocabulary M1-013 turns into a
// status code. 403 and 404 are different answers and the table records which:
// the 404s are the cases where admitting the resource exists is itself the leak,
// and collapsing them into "denied" would let a change from one to the other
// pass unnoticed.
type outcome string

const (
	// allow: the action is permitted.
	allow outcome = "allow"

	// deny403: refused, and the caller may know the resource exists.
	deny403 outcome = "deny-403"

	// deny404: refused without admitting the resource exists — a non-member on
	// anything an engagement owns, or the blue side on an unrevealed step.
	deny404 outcome = "deny-404"

	// na: this combination is not a question the model asks. It is used for one
	// thing only — an engagement seat on a resource the installation owns, which
	// [authz.Can] cannot see because the resource names no engagement — and it is
	// asserted rather than skipped: the answer must be the one for a caller
	// holding no seat at all.
	na outcome = "n/a"
)

// bySeat is one platform role's answers, across the seats somebody may hold in
// the engagement being acted on. Fields are in the order the table writes them.
type bySeat struct {
	// NonMember is a caller with no seat in this engagement. They are a lead of
	// [otherEngagement], which is what makes this column mean "membership
	// elsewhere buys nothing here".
	NonMember outcome
	Lead      outcome
	Red       outcome
	Blue      outcome
	Observer  outcome
}

// seats is one grid: both platform roles, all five seats.
type seats struct {
	Admin  bySeat
	Member bySeat
}

// reach is whether a service token holds an action at all, before scopes are
// considered. It is a named type rather than a bool because "true" in a table is
// not an argument for anything.
type reach string

const (
	// tokensMayHoldIt: a token holds this action when it carries the scope and
	// its owner holds the action. The usual case.
	tokensMayHoldIt reach = "tokens-may-hold-it"

	// sessionOnly: no token holds this action, whatever it carries and whoever
	// owns it — [authz.GuardSessionOnly]. It is for the actions whose effect is
	// to change which credentials exist.
	sessionOnly reach = "session-only"
)

// The resource states each row is asserted against. A platform-scoped action has
// one; an engagement-scoped action has three, because blind mode is a fact about
// the engagement and the reveal is a fact about the thing inside it.
const (
	statePlatform        = "acting on the installation"
	stateStandard        = "in a standard engagement"
	stateBlindRevealed   = "in a blind engagement, on a revealed step"
	stateBlindUnrevealed = "in a blind engagement, on an unrevealed step"
)

// row is one action's whole answer.
type row struct {
	// Action, Resource, Scope and Token restate the rule. They are here so that
	// a row is readable on its own, and checked against internal/authz/policy.go
	// so that restating cannot mean disagreeing.
	Action   authz.Action
	Resource authz.ResourceType
	Scope    authz.TokenScope
	Token    reach

	// Platform is the grid for an action the installation owns, and is empty for
	// an engagement-scoped one.
	Platform seats

	// Standard, BlindRevealed and BlindUnrevealed are the grids for an
	// engagement-scoped action, and are empty for a platform-scoped one. All
	// three are written out even where they are identical: "blind mode changes
	// nothing for a revealed step" is a claim worth asserting per action rather
	// than assuming from the shape of the guard.
	Standard        seats
	BlindRevealed   seats
	BlindUnrevealed seats
}

// matrix is the model, stated as expectations.
//
// Read down a column and you are reading what one role holds. The absences are
// the point: PlatformRoleMember is deny403 in every administrative row (v1's
// ungated /manage/access), Observer is deny403 in every write row but
// comment.write (the Spectator fall-through), and Red and Blue are deny403 in
// each other's execution row (the two definitions of "blue"). regressions_test.go
// names each of those.
//
// The five cells of a bySeat, in order:
//
//	no seat  lead  red  blue  observer
var matrix = []row{
	// ── Platform ────────────────────────────────────────────────────────────
	//
	// The seat columns are `na` throughout this section: these actions act on
	// the installation, the resource names no engagement, and so the seat is
	// invisible to [authz.Can]. That is the ticket's "an engagement role without
	// an engagement" — written down, and checked to answer the same as no seat,
	// rather than left out.
	{
		Action: authz.ActionUserRead, Resource: authz.ResourceUser,
		Scope: authz.TokenScopeAdminRead, Token: tokensMayHoldIt,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{deny403, na, na, na, na},
		},
	},
	{
		Action: authz.ActionUserManage, Resource: authz.ResourceUser,
		Scope: authz.TokenScopeAdminWrite, Token: tokensMayHoldIt,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{deny403, na, na, na, na},
		},
	},
	{
		Action: authz.ActionSettingsRead, Resource: authz.ResourcePlatform,
		Scope: authz.TokenScopeAdminRead, Token: tokensMayHoldIt,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{deny403, na, na, na, na},
		},
	},
	{
		Action: authz.ActionSettingsManage, Resource: authz.ResourcePlatform,
		Scope: authz.TokenScopeAdminWrite, Token: tokensMayHoldIt,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{deny403, na, na, na, na},
		},
	},
	{
		Action: authz.ActionActivityRead, Resource: authz.ResourcePlatform,
		Scope: authz.TokenScopeAdminRead, Token: tokensMayHoldIt,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{deny403, na, na, na, na},
		},
	},
	{
		// The one platform read everybody holds: the shared library is what an
		// engagement is planned from, and gating it would make membership a
		// prerequisite for learning what a technique is.
		Action: authz.ActionContentRead, Resource: authz.ResourceContent,
		Scope: authz.TokenScopeContentRead, Token: tokensMayHoldIt,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{allow, na, na, na, na},
		},
	},
	{
		Action: authz.ActionContentSync, Resource: authz.ResourceContent,
		Scope: authz.TokenScopeContentSync, Token: tokensMayHoldIt,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{deny403, na, na, na, na},
		},
	},
	{
		// Enable/disable/delete/patch of sources and custom content CRUD. Same
		// seat grid as content.sync, different scope so a pipeline can refresh
		// without the right to remove a library.
		Action: authz.ActionContentManage, Resource: authz.ResourceContent,
		Scope: authz.TokenScopeContentWrite, Token: tokensMayHoldIt,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{deny403, na, na, na, na},
		},
	},
	{
		// Everybody may start an engagement. It acts on the installation
		// because the engagement it would create does not exist yet, and the
		// creator becomes its lead — which is where the seat columns start
		// meaning something.
		Action: authz.ActionEngagementCreate, Resource: authz.ResourcePlatform,
		Scope: authz.TokenScopeEngagementsWrite, Token: tokensMayHoldIt,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{allow, na, na, na, na},
		},
	},
	{
		// Everybody holds both token actions, over their own tokens and nobody
		// else's — the scoping to "own" is the endpoint's, not the policy's.
		// The session-only reach is what stops a token minting a sibling that
		// outlives its revocation, so every token lens below answers deny403
		// here however senior the owner and however broad the scopes.
		Action: authz.ActionTokenRead, Resource: authz.ResourceServiceToken,
		Scope: authz.TokenScopeAdminRead, Token: sessionOnly,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{allow, na, na, na, na},
		},
	},
	{
		Action: authz.ActionTokenManage, Resource: authz.ResourceServiceToken,
		Scope: authz.TokenScopeAdminWrite, Token: sessionOnly,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{allow, na, na, na, na},
		},
	},
	{
		// The administrative half of the two above (M1-018): somebody else's
		// tokens, which is why the account is the resource and why the member
		// row is a refusal rather than the `allow` the pair above carries. A
		// member asking about another account's tokens is refused at exactly
		// the point they would be refused asking about the account itself, and
		// with the same 403 — the installation is not a secret, and an answer
		// that varied with whether the account existed would be an account
		// enumerator.
		//
		// sessionOnly is the part M1-018 asks for in as many words. Every token
		// lens answers deny403 here however senior the owner: an administrator's
		// leaked credential must not be able to end every other credential in
		// the installation.
		Action: authz.ActionTokenAdminRead, Resource: authz.ResourceUser,
		Scope: authz.TokenScopeAdminRead, Token: sessionOnly,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{deny403, na, na, na, na},
		},
	},
	{
		Action: authz.ActionTokenAdminManage, Resource: authz.ResourceUser,
		Scope: authz.TokenScopeAdminWrite, Token: sessionOnly,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{deny403, na, na, na, na},
		},
	},
	{
		// Sessions (M1-017) read the same as tokens do, and for the same two
		// reasons: everybody holds them over their own rows only, and no
		// service token holds them at all — a credential that could enumerate
		// or end its owner's browsers is one that could act against the person
		// holding it.
		Action: authz.ActionSessionRead, Resource: authz.ResourceSession,
		Scope: authz.TokenScopeAdminRead, Token: sessionOnly,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{allow, na, na, na, na},
		},
	},
	{
		Action: authz.ActionSessionManage, Resource: authz.ResourceSession,
		Scope: authz.TokenScopeAdminWrite, Token: sessionOnly,
		Platform: seats{
			Admin:  bySeat{allow, na, na, na, na},
			Member: bySeat{allow, na, na, na, na},
		},
	},

	// ── Engagement ──────────────────────────────────────────────────────────
	//
	// The admin row is allow throughout: a platform administrator holds every
	// engagement-scoped action on every engagement, membership or not. The one
	// exception is blind mode, which binds to the seat rather than to the
	// platform role — an administrator who has taken the blue chair is sitting
	// in it.
	//
	// The member no-seat column is deny404 throughout, never deny403: PLAN.md §4
	// gives a non-member nothing on an engagement "including its existence", and
	// an identifier that answers 403 while its neighbours answer 404 is an
	// engagement somebody has enumerated.
	{
		Action: authz.ActionEngagementRead, Resource: authz.ResourceEngagement,
		Scope: authz.TokenScopeEngagementsRead, Token: tokensMayHoldIt,
		Standard: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, allow},
		},
		BlindRevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, allow},
		},
		// Blind mode withholds steps, not the engagement. Blue knows they are
		// in an exercise; what they are not told is what is coming.
		BlindUnrevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, allow},
		},
	},
	{
		Action: authz.ActionEngagementManage, Resource: authz.ResourceEngagement,
		Scope: authz.TokenScopeEngagementsWrite, Token: tokensMayHoldIt,
		Standard: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, deny403, deny403, deny403},
		},
		BlindRevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, deny403, deny403, deny403},
		},
		BlindUnrevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, deny403, deny403, deny403},
		},
	},
	{
		Action: authz.ActionEngagementDelete, Resource: authz.ResourceEngagement,
		Scope: authz.TokenScopeEngagementsWrite, Token: tokensMayHoldIt,
		Standard: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, deny403, deny403, deny403},
		},
		BlindRevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, deny403, deny403, deny403},
		},
		BlindUnrevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, deny403, deny403, deny403},
		},
	},
	{
		Action: authz.ActionMemberRead, Resource: authz.ResourceMember,
		Scope: authz.TokenScopeEngagementsRead, Token: tokensMayHoldIt,
		Standard: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, allow},
		},
		BlindRevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, allow},
		},
		BlindUnrevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, allow},
		},
	},
	{
		Action: authz.ActionMemberManage, Resource: authz.ResourceMember,
		Scope: authz.TokenScopeEngagementsWrite, Token: tokensMayHoldIt,
		Standard: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, deny403, deny403, deny403},
		},
		BlindRevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, deny403, deny403, deny403},
		},
		BlindUnrevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, deny403, deny403, deny403},
		},
	},
	{
		// Every member reads executions — and then blind mode takes the
		// unrevealed ones back from blue, and from anybody sitting in that
		// chair. The 404 is deliberate: learning that a step exists is most of
		// what blind mode withholds.
		Action: authz.ActionExecutionRead, Resource: authz.ResourceExecution,
		Scope: authz.TokenScopeEngagementsRead, Token: tokensMayHoldIt,
		Standard: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, allow},
		},
		BlindRevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, allow},
		},
		BlindUnrevealed: seats{
			Admin:  bySeat{allow, allow, allow, deny404, allow},
			Member: bySeat{deny404, allow, allow, deny404, allow},
		},
	},
	{
		// Red writes the attack side. Blue's deny403 here and red's deny403 in
		// the row below are v1's two contradictory definitions of "blue",
		// written as data.
		Action: authz.ActionExecutionWriteRed, Resource: authz.ResourceExecution,
		Scope: authz.TokenScopeEngagementsWrite, Token: tokensMayHoldIt,
		Standard: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, deny403, deny403},
		},
		BlindRevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, deny403, deny403},
		},
		// Red is not blinded to their own attack: blind mode withholds from the
		// detection side, and the step they are writing is the one they ran.
		BlindUnrevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, deny403, deny403},
		},
	},
	{
		Action: authz.ActionExecutionWriteBlue, Resource: authz.ResourceExecution,
		Scope: authz.TokenScopeEngagementsWrite, Token: tokensMayHoldIt,
		Standard: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, deny403, allow, deny403},
		},
		BlindRevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, deny403, allow, deny403},
		},
		// The write is withheld along with the read: recording a detection
		// against a step would tell blue the step is there.
		BlindUnrevealed: seats{
			Admin:  bySeat{allow, allow, allow, deny404, allow},
			Member: bySeat{deny404, allow, deny403, deny404, deny403},
		},
	},
	{
		// The one write an observer holds. Reading and commenting is the seat;
		// everything below draws the line under it.
		Action: authz.ActionCommentWrite, Resource: authz.ResourceComment,
		Scope: authz.TokenScopeEngagementsWrite, Token: tokensMayHoldIt,
		Standard: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, allow},
		},
		BlindRevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, allow},
		},
		BlindUnrevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, allow},
		},
	},
	{
		Action: authz.ActionFindingWrite, Resource: authz.ResourceFinding,
		Scope: authz.TokenScopeEngagementsWrite, Token: tokensMayHoldIt,
		Standard: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, deny403},
		},
		BlindRevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, deny403},
		},
		BlindUnrevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, deny403},
		},
	},
	{
		Action: authz.ActionReportRead, Resource: authz.ResourceReport,
		Scope: authz.TokenScopeReportsRead, Token: tokensMayHoldIt,
		Standard: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, allow},
		},
		BlindRevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, allow},
		},
		BlindUnrevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, allow, allow, allow},
		},
	},
	{
		Action: authz.ActionReportPublish, Resource: authz.ResourceReport,
		Scope: authz.TokenScopeReportsWrite, Token: tokensMayHoldIt,
		Standard: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, deny403, deny403, deny403},
		},
		BlindRevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, deny403, deny403, deny403},
		},
		BlindUnrevealed: seats{
			Admin:  bySeat{allow, allow, allow, allow, allow},
			Member: bySeat{deny404, allow, deny403, deny403, deny403},
		},
	},
}

// lens is one way of arriving at a cell: how the request authenticated, and with
// what.
//
// The projections below are the whole of what a service token changes, and they
// are stated once because they are uniform. Two claims, and both are the point of
// M1-011's second and third fences:
//
//   - A fence can turn an allow into a refusal. It can never turn a refusal into
//     an allow, and never softens one — a caller refused 404 as a session is
//     refused 404 with a token, because the role half runs first and a concealed
//     denial has already decided what may be admitted.
//   - Nothing a token carries substitutes for its owner's role. There is no lens
//     below in which a scope makes a difference to a cell that was already
//     denied.
//
// If a future rule makes a token differ from its owner in a way that is not one
// of these, that is a column on [row] and not a fifth lens: it would be an
// action-specific fact, and action-specific facts belong in the table.
type lens struct {
	Name string

	// Subject finishes a subject that already carries the platform role and the
	// membership under test, given the scope this row's action requires.
	Subject func(authz.Subject, authz.TokenScope) authz.Subject

	// Want projects the table's answer — which is written for a session — onto
	// this lens.
	Want func(r row, engagementScoped bool, session outcome) outcome
}

var lenses = []lens{
	{
		Name: "a signed-in session",
		Subject: func(s authz.Subject, _ authz.TokenScope) authz.Subject {
			s.Method = authz.MethodCookie
			return s
		},
		Want: func(_ row, _ bool, session outcome) outcome { return session },
	},
	{
		Name: "a service token carrying the scope this action needs",
		Subject: func(s authz.Subject, scope authz.TokenScope) authz.Subject {
			s.Method = authz.MethodServiceToken
			s.TokenScopes = []authz.TokenScope{scope}
			return s
		},
		Want: func(r row, _ bool, session outcome) outcome {
			if r.Token == sessionOnly {
				return deny403
			}
			// Both fences pass, so the answer is the owner's. That is the
			// design: a token is its owner, narrowed.
			return session
		},
	},
	{
		// A real scope, and the wrong one — rather than a token with no scopes
		// at all, which anything would refuse.
		Name: "a service token carrying some other scope",
		Subject: func(s authz.Subject, scope authz.TokenScope) authz.Subject {
			s.Method = authz.MethodServiceToken
			s.TokenScopes = []authz.TokenScope{otherScopeThan(scope)}
			return s
		},
		Want: func(r row, _ bool, session outcome) outcome {
			if r.Token == sessionOnly {
				return deny403
			}
			if session == allow {
				// A missing scope is not a secret: the caller can read the list
				// this endpoint needs in api/openapi.yaml, so the honest answer
				// is 403.
				return deny403
			}
			return session
		},
	},
	{
		// Bound to [otherEngagement] — which every caller in this file leads,
		// so the refusal is the binding rather than the owner running out of
		// permissions somewhere else.
		Name: "a service token bound to a different engagement",
		Subject: func(s authz.Subject, scope authz.TokenScope) authz.Subject {
			s.Method = authz.MethodServiceToken
			s.TokenScopes = []authz.TokenScope{scope}
			s.TokenEngagementID = otherEngagement
			return s
		},
		Want: func(r row, engagementScoped bool, session outcome) outcome {
			if r.Token == sessionOnly {
				return deny403
			}
			if session != allow {
				return session
			}
			if engagementScoped {
				// The engagement this token may not reach must answer what an
				// engagement that does not exist answers, or the holder of a
				// narrowly bound token can enumerate the rest.
				return deny404
			}
			// Nothing outside an engagement, including the installation. There
			// is no engagement to compare against, and the caller already knows
			// the installation is there, so there is nothing to conceal.
			return deny403
		},
	},
}

// TestPermissionMatrix asserts every cell, under every lens.
//
// It is one loop rather than a subtest per cell: the coordinates are in the
// failure message, and 1,840 subtests would make a single wrong cell hard to
// find in the output rather than easy.
func TestPermissionMatrix(t *testing.T) {
	started := time.Now()
	ctx := t.Context()
	cells := 0

	for _, r := range matrix {
		for _, v := range r.variants() {
			for _, c := range v.Grid.cells() {
				want := c.Want
				if want == na {
					// Not a skip. `na` says the seat is not part of this
					// question, and the check for that is that the answer is the
					// one for a caller holding no seat at all.
					want = v.Grid.forPlatformRole(c.Platform).NonMember
				}
				for _, l := range lenses {
					cells++
					subject := l.Subject(subjectWith(c.Platform, c.Seat), r.Scope)
					got := outcomeOf(authz.Can(ctx, subject, r.Action, v.Resource))
					if expected := l.Want(r, r.Resource.EngagementScoped(), want); got != expected {
						t.Errorf("%s: a %s %s, %s, on %s\n  got  %s\n  want %s (the matrix says %s for a session)\n"+
							"  reason: %s",
							r.Action, c.Platform, seatName(c.Seat), l.Name, v.State,
							got, expected, c.Want,
							authz.Can(ctx, subject, r.Action, v.Resource).Reason)
					}
				}
			}
		}
	}

	// M1-014: "The matrix runs in under a second (it's pure functions — M1-012
	// made this possible)." It is an acceptance criterion rather than a nicety:
	// the moment [authz.Can] can reach a database, a matrix this size stops
	// being something anybody runs on every commit, and a model nobody asserts
	// is the model v1 had.
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Errorf("the matrix took %s to decide %d cases. Can() has stopped being a pure function of its "+
			"arguments — something in internal/authz is doing I/O", elapsed, cells)
	}
}

// TestTheMatrixDecidesEveryActionInEveryState is the exhaustiveness gate, and the
// reason the file above is worth having. Adding an [authz.Action] fails this test
// by name until somebody writes down what every role may do with it: silence is
// not an answer, and an accidental default is not a decision.
//
// It also holds the four restated columns to the rule table. A row that says the
// wrong resource type or the wrong scope is a row asserting a question nobody
// asks, which would pass while proving nothing.
func TestTheMatrixDecidesEveryActionInEveryState(t *testing.T) {
	rules := map[authz.Action]authz.Rule{}
	for _, rule := range authz.Rules() {
		rules[rule.Action] = rule
	}

	rows := map[authz.Action]row{}
	for _, r := range matrix {
		if _, twice := rows[r.Action]; twice {
			t.Errorf("%s has two entries in the matrix; one of them is dead and nobody knows which", r.Action)
		}
		rows[r.Action] = r
	}

	for _, action := range authz.Actions() {
		rule, ruled := rules[action]

		r, listed := rows[action]
		if !listed {
			// An action with no rule has no name — the wire spelling lives on
			// the rule — so say where the constant is instead of printing
			// `action(23)` and leaving the reader to count.
			named := action.String()
			if !ruled {
				named = fmt.Sprintf("%s, the %d%s constant in the block in internal/authz/action.go, which "+
					"has no rule either", action, int(action), ordinal(int(action)))
			}
			t.Errorf("the matrix decides nothing about %s. Add an entry for it to `matrix` in "+
				"internal/authz/matrix_test.go — the comment at the top of that file says how. An action "+
				"nobody wrote a row for is an action nobody decided who holds.", named)
			continue
		}

		if r.Resource != rule.Resource {
			t.Errorf("the matrix asserts %s on a %s; the rule says it acts on a %s, so every cell in that "+
				"row is answering the wrong question", action, r.Resource, rule.Resource)
		}
		if r.Scope != rule.Token {
			t.Errorf("the matrix says %s needs the %s token scope; the rule says %s",
				action, r.Scope, rule.Token)
		}
		switch {
		case r.Token == "":
			t.Errorf("the matrix does not say whether a service token may hold %s — set Token to "+
				"tokensMayHoldIt or sessionOnly", action)
		case (rule.Guard == authz.GuardSessionOnly) != (r.Token == sessionOnly):
			t.Errorf("the matrix says %s is %q; the rule's guard is %q", action, r.Token, rule.Guard)
		}

		assertGrids(t, r)
	}

	for action := range rows {
		if _, real := rules[action]; !real {
			t.Errorf("the matrix has a row for %s, which no rule covers. Delete the row, or the rule that "+
				"went with it left an assertion behind that proves nothing", action)
		}
	}

	if otherScopeThan(authz.TokenScopeAdminRead) == "" {
		t.Error("there is only one token scope, so the \"carrying some other scope\" lens has nothing to " +
			"carry and is asserting the same thing as the lens above it")
	}
}

// assertGrids checks that a row fills in exactly the grids its resource calls for,
// and that none of them has an undecided cell.
func assertGrids(t *testing.T, r row) {
	t.Helper()

	if r.Resource.EngagementScoped() {
		if r.Platform != (seats{}) {
			t.Errorf("%s acts on something an engagement owns, so it has no Platform grid — the answer "+
				"depends on which engagement, and that grid cannot say", r.Action)
		}
	} else if r.Standard != (seats{}) || r.BlindRevealed != (seats{}) || r.BlindUnrevealed != (seats{}) {
		t.Errorf("%s acts on something the installation owns, so blind mode cannot apply to it — "+
			"delete the engagement grids", r.Action)
	}

	for _, v := range r.variants() {
		for _, undecided := range v.Grid.undecided() {
			t.Errorf("the matrix leaves %s %s undecided for %s. Name an outcome — an empty cell is a "+
				"decision nobody made", r.Action, v.State, undecided)
		}
	}
}

// TestNAIsOnlyUsedWhereTheSeatIsInvisible keeps the escape hatch from becoming
// one. `na` is checked rather than skipped, but it still asserts less than a
// named outcome does, so it is confined to the one case it describes: a seat on a
// resource that names no engagement.
func TestNAIsOnlyUsedWhereTheSeatIsInvisible(t *testing.T) {
	for _, r := range matrix {
		for _, v := range r.variants() {
			for _, c := range v.Grid.cells() {
				if c.Want != na {
					continue
				}
				switch {
				case r.Resource.EngagementScoped():
					t.Errorf("%s %s marks a %s %s n/a. This action acts inside an engagement, where the "+
						"seat is the whole question", r.Action, v.State, c.Platform, seatName(c.Seat))
				case c.Seat == "":
					t.Errorf("%s marks a %s holding no seat n/a. That is the answer the other cells are "+
						"measured against, so it has to be a real one", r.Action, c.Platform)
				}
			}
		}
	}
}

// --- the machinery ----------------------------------------------------------

// variant is one grid together with the resource it is about.
type variant struct {
	State    string
	Grid     seats
	Resource authz.Resource
}

// variants returns the states this row is asserted in: one for a platform-scoped
// action, three for an engagement-scoped one.
func (r row) variants() []variant {
	const id = "0192f1a0-0000-7000-8000-000000000010"

	if !r.Resource.EngagementScoped() {
		return []variant{{statePlatform, r.Platform, authz.Resource{Type: r.Resource, ID: id}}}
	}

	// Revealed is deliberately false on the standard engagement. Outside blind
	// mode the flag means nothing, and a guard that read it without checking
	// whether the engagement is blind would show up as a difference between this
	// grid and the one below.
	standard := authz.Resource{Type: r.Resource, ID: id, EngagementID: engagement}
	unrevealed := standard
	unrevealed.EngagementBlind = true
	revealed := unrevealed
	revealed.Revealed = true

	return []variant{
		{stateStandard, r.Standard, standard},
		{stateBlindRevealed, r.BlindRevealed, revealed},
		{stateBlindUnrevealed, r.BlindUnrevealed, unrevealed},
	}
}

// cell is one expectation, with the coordinates a failure has to name.
type cell struct {
	Platform authz.PlatformRole

	// Seat is the role this caller holds in the engagement being acted on, and
	// is empty for one holding none.
	Seat authz.EngagementRole

	Want outcome
}

// cells flattens a grid, in the order the table writes it.
func (s seats) cells() []cell {
	var flat []cell
	for _, side := range []struct {
		role   authz.PlatformRole
		bySeat bySeat
	}{
		{authz.PlatformRoleAdmin, s.Admin},
		{authz.PlatformRoleMember, s.Member},
	} {
		flat = append(flat,
			cell{side.role, "", side.bySeat.NonMember},
			cell{side.role, authz.EngagementRoleLead, side.bySeat.Lead},
			cell{side.role, authz.EngagementRoleRed, side.bySeat.Red},
			cell{side.role, authz.EngagementRoleBlue, side.bySeat.Blue},
			cell{side.role, authz.EngagementRoleObserver, side.bySeat.Observer},
		)
	}
	return flat
}

// forPlatformRole returns one side of a grid.
func (s seats) forPlatformRole(role authz.PlatformRole) bySeat {
	if role == authz.PlatformRoleAdmin {
		return s.Admin
	}
	return s.Member
}

// undecided names the cells nobody filled in.
func (s seats) undecided() []string {
	var missing []string
	for _, c := range s.cells() {
		if c.Want == "" {
			missing = append(missing, fmt.Sprintf("a %s %s", c.Platform, seatName(c.Seat)))
		}
	}
	return missing
}

// subjectWith builds the caller one cell is about: a platform role, a seat in the
// engagement under test, and — always — a lead's seat in a different one.
func subjectWith(role authz.PlatformRole, seat authz.EngagementRole) authz.Subject {
	subject := authz.Subject{
		UserID:       "caller-1",
		PlatformRole: role,
		Memberships:  map[string]authz.EngagementRole{otherEngagement: authz.EngagementRoleLead},
	}
	if seat != "" {
		subject.Memberships[engagement] = seat
	}
	return subject
}

// outcomeOf reads a decision in the vocabulary the table is written in. The
// concealed denial is a separate answer rather than a flavour of refusal,
// because M1-013 turns it into a different status code and the difference is the
// whole of what a non-member is told.
func outcomeOf(decision authz.Decision) outcome {
	switch {
	case decision.Allowed:
		return allow
	case decision.Conceal:
		return deny404
	default:
		return deny403
	}
}

// otherScopeThan returns a real token scope that is not the one given.
func otherScopeThan(scope authz.TokenScope) authz.TokenScope {
	for _, candidate := range authz.TokenScopes() {
		if candidate != scope {
			return candidate
		}
	}
	return ""
}

// ordinal is the suffix for a position in a list, so that a failure can say
// "the 23rd constant" rather than making the reader count the block.
func ordinal(n int) string {
	switch {
	case n%100 >= 11 && n%100 <= 13:
		return "th"
	case n%10 == 1:
		return "st"
	case n%10 == 2:
		return "nd"
	case n%10 == 3:
		return "rd"
	default:
		return "th"
	}
}

// seatName renders a seat for a failure message, including the absence of one.
func seatName(seat authz.EngagementRole) string {
	if seat == "" {
		return "holding no seat in this engagement"
	}
	return "in the " + string(seat) + " seat"
}
