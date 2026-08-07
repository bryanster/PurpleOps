package httpapi

import (
	"context"
	"fmt"

	"github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	identity "github.com/bryanster/blacklight/internal/store/identity"
)

// Membership handlers (M3-003).

// ListEngagementMembers returns the member list for an engagement.
func (h *handlers) ListEngagementMembers(ctx context.Context,
	request gen.ListEngagementMembersRequestObject) (gen.ListEngagementMembersResponseObject, error) {

	members, err := h.engagements.ListMembers(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	out := make([]gen.EngagementMember, 0, len(members))
	for _, m := range members {
		w, err := membershipToWire(ctx, h.users, m)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}

	return gen.ListEngagementMembers200JSONResponse(out), nil
}

// AddEngagementMember adds a user to an engagement.
func (h *handlers) AddEngagementMember(ctx context.Context,
	request gen.AddEngagementMemberRequestObject) (gen.AddEngagementMemberResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: add member: missing body")
	}

	m, err := h.engagements.AddMember(ctx, subject, request.EngagementId.String(), engagement.AddMemberInput{
		UserID: request.Body.UserId,
		Role:   string(request.Body.Role),
	})
	if err != nil {
		return nil, err
	}

	w, err := membershipToWire(ctx, h.users, m)
	if err != nil {
		return nil, err
	}
	return gen.AddEngagementMember201JSONResponse(w), nil
}

// PatchEngagementMember changes a member's engagement role.
func (h *handlers) PatchEngagementMember(ctx context.Context,
	request gen.PatchEngagementMemberRequestObject) (gen.PatchEngagementMemberResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: patch member: missing body")
	}

	updated, err := h.engagements.PatchMemberRole(ctx, subject,
		request.EngagementId.String(),
		request.UserId,
		string(request.Body.Role),
	)
	if err != nil {
		return nil, err
	}

	w, err := membershipToWire(ctx, h.users, updated)
	if err != nil {
		return nil, err
	}
	return gen.PatchEngagementMember200JSONResponse(w), nil
}

// RemoveEngagementMember removes a user from an engagement.
func (h *handlers) RemoveEngagementMember(ctx context.Context,
	request gen.RemoveEngagementMemberRequestObject) (gen.RemoveEngagementMemberResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.engagements.RemoveMember(ctx, subject,
		request.EngagementId.String(),
		request.UserId,
	); err != nil {
		return nil, err
	}

	return gen.RemoveEngagementMember204Response{}, nil
}

// membershipToWire converts a store membership to its OpenAPI representation,
// reading the user row for display fields.
func membershipToWire(ctx context.Context, users membershipUserStore, m identity.Membership) (gen.EngagementMember, error) {
	w := gen.EngagementMember{
		Id:      m.UserID,
		Role:    gen.EngagementRole(m.Role),
		AddedAt: m.AddedAt,
	}

	if users != nil {
		u, err := users.ByID(ctx, m.UserID)
		if err != nil {
			// User may have been deleted since the membership was created.
			// Return the membership with empty display fields rather than 500.
			return w, nil //nolint:nilerr // degraded display is better than broken endpoint
		}
		w.Email = u.Email
		w.DisplayName = u.DisplayName
	}

	return w, nil
}

// membershipUserStore is the user lookup the membership handlers need.
type membershipUserStore interface {
	ByID(ctx context.Context, id string) (identity.User, error)
}
