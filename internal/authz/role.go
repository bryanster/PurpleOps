package authz

// The role vocabulary. It lives here, and only here.
//
// v1's defining authorization bug was two contradictory definitions of "blue"
// (PLAN.md §4): one handler's idea of the role and another's disagreed, and the
// gap between them was write access. The fix is not a better comparison — it is
// having one place that owns the words, so there is no second definition to
// disagree with. TestRoleLiteralsLiveOnlyInThisPackage enforces that: a role
// string anywhere else in the Go tree fails the build.
//
// These types used to live in internal/store/identity. They moved because
// [Can] must not import a database package (M1-012), and because the store was
// never the right owner: where a role is *persisted* is a detail, what it
// *means* is the policy's business. identity now imports these.

// PlatformRole is what somebody may do to this installation: manage users,
// content and every engagement, or take part in the ones they are a member of.
//
// It is deliberately two values. Anything finer belongs to [EngagementRole],
// and v1's single fuzzy level is the mistake PLAN.md §4 is correcting.
type PlatformRole string

const (
	// PlatformRoleAdmin administers the installation and holds every
	// engagement-scoped action on every engagement, membership or not. That
	// reach is the point of the role; docs/authz.md renders it as a column so
	// it is impossible to miss.
	PlatformRoleAdmin PlatformRole = "admin"

	// PlatformRoleMember takes part in the engagements they belong to and does
	// nothing to the installation. Every platform-scoped rule omits it, which
	// is the /manage/access regression stated as data rather than as an `if`.
	PlatformRoleMember PlatformRole = "member"
)

// PlatformRoles are the platform roles, in order of authority. The matrix tests
// (M1-014) and docs/authz.md enumerate from here, so a role added without a
// column is not possible.
func PlatformRoles() []PlatformRole {
	return []PlatformRole{PlatformRoleAdmin, PlatformRoleMember}
}

// Valid reports whether r is one of the two. An unrecognised role is not a
// weaker role: it holds nothing, because no rule lists it.
func (r PlatformRole) Valid() bool {
	return r == PlatformRoleAdmin || r == PlatformRoleMember
}

// EngagementRole is what somebody may do inside one engagement. Red and blue
// are separate so that blind mode and the split write endpoints in PLAN.md §4
// have something to decide on.
type EngagementRole string

const (
	// EngagementRoleLead runs one engagement: its settings, its membership and
	// its published output. Their authority stops at their own engagement —
	// being a lead somewhere confers nothing anywhere else.
	EngagementRoleLead EngagementRole = "lead"

	// EngagementRoleRed writes the attack side of an execution and never the
	// detection side. The separation is structural: [ActionExecutionWriteRed]
	// and [ActionExecutionWriteBlue] are different actions with different
	// rules, so there is no field list to get wrong.
	EngagementRoleRed EngagementRole = "red"

	// EngagementRoleBlue writes the detection side and never the attack side.
	// In a blind engagement they additionally cannot reach an unrevealed step
	// — see [GuardBlindMode].
	EngagementRoleBlue EngagementRole = "blue"

	// EngagementRoleObserver reads and comments. It writes nothing else, ever:
	// v1's Spectator fell through to write access, and the fix is that no
	// write rule lists this role.
	EngagementRoleObserver EngagementRole = "observer"
)

// EngagementRoles are the engagement roles, in order of authority.
func EngagementRoles() []EngagementRole {
	return []EngagementRole{
		EngagementRoleLead,
		EngagementRoleRed,
		EngagementRoleBlue,
		EngagementRoleObserver,
	}
}

// Valid reports whether r is one of the four.
func (r EngagementRole) Valid() bool {
	switch r {
	case EngagementRoleLead, EngagementRoleRed, EngagementRoleBlue, EngagementRoleObserver:
		return true
	default:
		return false
	}
}

// Method is how a request proved who it is.
//
// It is here rather than in internal/authn for the same reason the roles are:
// [Can] reads it — a service token is fenced by its scopes as well as by its
// owner's role (PLAN.md §9) — and CSRF turns on it (M1-005). Two packages
// deciding on one distinction means one definition of it.
type Method string

const (
	// MethodNone is an anonymous request. It is the zero value, so a Subject
	// that nothing authenticated cannot pass for one that something did.
	MethodNone Method = ""

	// MethodCookie is a browser session cookie (M1-003). These requests carry
	// ambient authority — the browser attaches the cookie whether or not this
	// application asked it to — and are the ones CSRF protection is for.
	MethodCookie Method = "cookie"

	// MethodServiceToken is an Authorization: Bearer service token (M1-011).
	// Nothing attaches one on a caller's behalf, so there is no cross-site
	// request to forge and CSRF does not apply (PLAN.md §4).
	//
	// [Can] holds these requests to two fences rather than one: the owner's
	// live role, and the token's scopes.
	MethodServiceToken Method = "service_token"
)
