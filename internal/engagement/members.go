package engagement

import (
	"context"
	"fmt"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	identity "github.com/bryanster/blacklight/internal/store/identity"
)

// MemberStore is the subset of [identity.Memberships] the membership domain
// layer needs. The concrete [*identity.Memberships] satisfies it.
type MemberStore interface {
	Add(ctx context.Context, in identity.NewMembership) (identity.Membership, error)
	SetRole(ctx context.Context, engagementID, userID string, role authz.EngagementRole) (identity.Membership, error)
	Remove(ctx context.Context, engagementID, userID string) error
	ListByEngagement(ctx context.Context, engagementID string) ([]identity.Membership, error)
}

// UserStore is the subset of [identity.Users] the membership domain layer
// needs. The concrete [*identity.Users] satisfies it.
type UserStore interface {
	ByID(ctx context.Context, id string) (identity.User, error)
}

// AddMemberInput is the caller's half of adding a member.
// Role is an engagement role string (lead, red, blue, observer).
type AddMemberInput struct {
	UserID string
	Role   string
}

// ListMembers returns the membership rows for an engagement.
func (s *Service) ListMembers(ctx context.Context, engagementID string) ([]identity.Membership, error) {
	return s.memberships.ListByEngagement(ctx, engagementID)
}

// AddMember seats a user in an engagement. Only active users may be added.
// Adding someone who is already a member is a 409.
func (s *Service) AddMember(ctx context.Context, actor authn.Subject, engagementID string, in AddMemberInput) (identity.Membership, error) {
	role := authz.EngagementRole(in.Role)
	if !role.Valid() {
		return identity.Membership{}, fmt.Errorf("engagement: invalid role %q", in.Role)
	}

	m, err := s.memberships.Add(ctx, identity.NewMembership{
		EngagementID: engagementID,
		UserID:       in.UserID,
		Role:         role,
		AddedBy:      actor.UserID,
	})
	if err != nil {
		return identity.Membership{}, err
	}

	s.recordMemberActivity(ctx, actor.UserID, engagementID, events.VerbMemberAdded, events.ObjectMember, in.UserID, map[string]any{
		"role": in.Role,
	})

	return m, nil
}

// PatchMemberRole changes a member's engagement role. The last lead cannot
// be demoted. A lead may change their own role only if another lead remains.
// role is an engagement role string (lead, red, blue, observer).
func (s *Service) PatchMemberRole(ctx context.Context, actor authn.Subject, engagementID, userID, role string) (identity.Membership, error) {
	r := authz.EngagementRole(role)
	if !r.Valid() {
		return identity.Membership{}, fmt.Errorf("engagement: invalid role %q", role)
	}

	if err := s.checkLastLead(ctx, engagementID, userID, r); err != nil {
		return identity.Membership{}, err
	}

	updated, err := s.memberships.SetRole(ctx, engagementID, userID, r)
	if err != nil {
		return identity.Membership{}, err
	}

	s.recordMemberActivity(ctx, actor.UserID, engagementID, events.VerbMemberRoleChanged, events.ObjectMember, userID, map[string]any{
		"role": role,
	})

	return updated, nil
}

// RemoveMember takes a user out of an engagement. The last lead cannot be
// removed. A lead may remove themselves only if another lead remains.
func (s *Service) RemoveMember(ctx context.Context, actor authn.Subject, engagementID, userID string) error {
	if err := s.checkLastLead(ctx, engagementID, userID, authz.EngagementRoleObserver); err != nil {
		return err
	}

	if err := s.memberships.Remove(ctx, engagementID, userID); err != nil {
		return err
	}

	s.recordMemberActivity(ctx, actor.UserID, engagementID, events.VerbMemberRemoved, events.ObjectMember, userID, nil)

	return nil
}

// checkLastLead returns a 409 if userID is the last lead in the engagement
// and removing/demoting them would leave the engagement with no lead.
// targetRole of EngagementRoleLead means no change (patch keeping lead).
// Any other role (or observer, used for removal) triggers the check.
func (s *Service) checkLastLead(ctx context.Context, engagementID, userID string, targetRole authz.EngagementRole) error {
	if targetRole == authz.EngagementRoleLead {
		return nil
	}

	members, err := s.memberships.ListByEngagement(ctx, engagementID)
	if err != nil {
		return err
	}

	var isLead bool
	for _, m := range members {
		if m.UserID == userID && m.Role == authz.EngagementRoleLead {
			isLead = true
			break
		}
	}
	if !isLead {
		return nil
	}

	otherLeads := 0
	for _, m := range members {
		if m.UserID != userID && m.Role == authz.EngagementRoleLead {
			otherLeads++
		}
	}
	if otherLeads == 0 {
		return apierr.Conflict("cannot remove or demote the last lead of an engagement")
	}

	return nil
}

// recordMemberActivity writes an activity entry for a membership change.
func (s *Service) recordMemberActivity(ctx context.Context, actorID, engagementID string, verb events.Verb, objectType, objectID string, delta map[string]any) {
	if s.activity == nil {
		return
	}
	entry := events.Entry{
		EngagementID: engagementID,
		ActorID:      actorID,
		Verb:         verb,
		ObjectType:   objectType,
		ObjectID:     objectID,
	}
	if delta != nil {
		entry.Delta = events.Delta(delta)
	}
	// RecordAlone is best-effort: the mutation already succeeded.
	//nolint:errcheck // best-effort audit trail; failure is logged by the store
	s.activity.RecordAlone(ctx, entry)
}
