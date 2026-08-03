package httpapi

import (
	"context"
	"errors"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// Memberships is the part of the identity store [membershipOwnership] needs.
// [*identity.Memberships] satisfies it.
type Memberships interface {
	Get(ctx context.Context, engagementID, userID string) (identity.Membership, error)
}

// membershipOwnership answers engagement-scoped authorization questions from
// the membership table alone (M1-015).
//
// Engagements themselves do not exist as rows until M3, so [Facts] accepts any
// engagement id the path named and reports it as not blind and fully revealed.
// Seat still comes from a real membership. A non-member is concealed by
// authz.Can the same way they will be once M3 can prove an engagement is
// missing; an administrator still reaches the empty feed. M3 replaces this
// with a loader that also checks the engagement exists and whether it runs
// blind.
type membershipOwnership struct {
	members Memberships
}

// NewMembershipOwnership returns an [Ownership] backed by the membership
// table. Pass it as [Deps.Ownership] once any engagement-scoped operation is
// in the specification.
func NewMembershipOwnership(members Memberships) Ownership {
	if members == nil {
		panic("httpapi: NewMembershipOwnership called with nil memberships")
	}
	return membershipOwnership{members: members}
}

// Facts reports the engagement named in the path as existing. See the type
// comment for why it does not look anything up yet.
func (membershipOwnership) Facts(_ context.Context, ref ResourceRef) (ResourceFacts, error) {
	if ref.EngagementID == "" {
		return ResourceFacts{}, apierr.NotFound("engagement", ref.ID)
	}
	return ResourceFacts{
		EngagementID: ref.EngagementID,
		Blind:        false,
		Revealed:     true,
	}, nil
}

// Seat returns the caller's role in the engagement, or false when they are
// not a member.
func (o membershipOwnership) Seat(ctx context.Context, engagementID, userID string) (authz.EngagementRole, bool, error) {
	m, err := o.members.Get(ctx, engagementID, userID)
	if errors.Is(err, apierr.ErrNotFound) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return m.Role, true, nil
}
