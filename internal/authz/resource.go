package authz

import "log/slog"

// ResourceType is the kind of thing an action acts on. Each type is owned
// either by the installation or by exactly one engagement, and that ownership
// is what tells [Can] whether to look for a membership.
type ResourceType string

const (
	// ResourcePlatform is the installation itself: its settings, its activity
	// log, and the act of creating an engagement — which is done *to* the
	// installation, because the engagement it would create does not exist yet
	// and so cannot own the decision.
	ResourcePlatform ResourceType = "platform"

	ResourceUser    ResourceType = "user"
	ResourceContent ResourceType = "content"

	// ResourceServiceToken is one of the bearer credentials somebody holds for
	// automating against this installation (M1-011). It is owned by the
	// platform and not by an engagement, even for a token bound to one: a
	// token bound to an engagement is still a credential belonging to a person,
	// and where it may point is a separate question from who may hold it.
	ResourceServiceToken ResourceType = "service_token"

	// ResourceSession is a browser somebody is signed in on (M1-017). Owned by
	// the platform for the same reason a service token is: it is a credential
	// belonging to a person, and no engagement has a say in who holds it.
	ResourceSession ResourceType = "session"

	ResourceEngagement ResourceType = "engagement"
	ResourceMember     ResourceType = "member"
	ResourceExecution  ResourceType = "execution"
	ResourceComment    ResourceType = "comment"
	ResourceFinding    ResourceType = "finding"
	ResourceReport     ResourceType = "report"
	ResourceScenario   ResourceType = "scenario"

)

// resourceOwners says who owns each resource type. A type absent from this map
// is owned by nothing and reaches no rule, because [Rule.Resource] is compared
// against the resource the caller supplied — so an unrecognised type denies.
var resourceOwners = map[ResourceType]owner{
	ResourcePlatform:     ownedByPlatform,
	ResourceUser:         ownedByPlatform,
	ResourceContent:      ownedByPlatform,
	ResourceServiceToken: ownedByPlatform,
	ResourceSession:      ownedByPlatform,
	ResourceEngagement:   ownedByEngagement,
	ResourceMember:       ownedByEngagement,
	ResourceExecution:    ownedByEngagement,
	ResourceComment:      ownedByEngagement,
	ResourceFinding:      ownedByEngagement,
	ResourceReport:       ownedByEngagement,
	ResourceScenario:     ownedByEngagement,
}

// owner is whether a resource type belongs to the installation or to one
// engagement.
type owner int

const (
	ownedByPlatform owner = iota
	ownedByEngagement
)

// EngagementScoped reports whether resources of this type belong to an
// engagement, and therefore require [Resource.EngagementID] to be set.
func (t ResourceType) EngagementScoped() bool {
	return resourceOwners[t] == ownedByEngagement
}

// Resource is the thing being acted on, reduced to the facts a rule can read.
//
// It carries attributes and not identifiers-to-look-up on purpose. [Can] does
// no I/O (M1-012), so anything a rule needs must already be here — if a future
// rule needs a fact this struct does not carry, the fix is a field and a caller
// that loads it, never a query inside the policy. The moment Can can reach the
// database, the exhaustive matrix in M1-014 stops being possible.
type Resource struct {
	Type ResourceType

	// ID identifies the specific resource, and is empty for a whole-collection
	// action ("list the users") or for one that creates something. No rule
	// reads it today; it is here because a denial that cannot name what was
	// denied is not much use in an audit trail (M1-015).
	ID string

	// EngagementID is the engagement this resource belongs to. Required for
	// every engagement-scoped type: a resource that cannot say which
	// engagement owns it is denied rather than treated as unowned, because
	// "unowned" would mean "no membership needed".
	EngagementID string

	// EngagementBlind is whether the owning engagement is running blind — red
	// executes without blue being told what to expect (PLAN.md §4).
	EngagementBlind bool

	// Revealed is whether this step has been revealed to the blue side. It is
	// meaningless outside a blind engagement, where every step reads as
	// revealed.
	Revealed bool
}

// LogValue renders a resource for a log line. Everything here is already an
// identifier or a flag; there is nothing to redact.
func (r Resource) LogValue() slog.Value {
	attrs := []slog.Attr{slog.String("type", string(r.Type))}
	if r.ID != "" {
		attrs = append(attrs, slog.String("id", r.ID))
	}
	if r.EngagementID != "" {
		attrs = append(attrs, slog.String("engagement_id", r.EngagementID))
	}
	if r.EngagementBlind {
		attrs = append(attrs, slog.Bool("blind", true), slog.Bool("revealed", r.Revealed))
	}
	return slog.GroupValue(attrs...)
}
