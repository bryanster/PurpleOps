package httpapi

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// The self-service session endpoints (M1-017), which are what the account
// screen's "where am I signed in" panel reads and acts on.
//
// Like the service token handlers next door, the scoping is one word repeated:
// every call below passes the caller, so there is no argument any of these
// functions could be given that would reach somebody else's browser. Who may
// call them at all is api/openapi.yaml's `session.read` and `session.manage`,
// decided by the middleware before any function here is entered.

// ListSessions returns the caller's own live sessions.
func (h *handlers) ListSessions(ctx context.Context,
	_ gen.ListSessionsRequestObject) (gen.ListSessionsResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	sessions, err := h.auth.Sessions(ctx, subject)
	if err != nil {
		return nil, err
	}

	// Non-nil, so a caller with none reads `[]` rather than `null` — the
	// generated client types it as an array either way.
	items := make([]gen.Session, 0, len(sessions))
	for _, s := range sessions {
		rendered, err := renderSession(s, subject.SessionID)
		if err != nil {
			return nil, err
		}
		items = append(items, rendered)
	}
	return gen.ListSessions200JSONResponse{Items: items}, nil
}

// RevokeSession ends one of the caller's own sessions.
func (h *handlers) RevokeSession(ctx context.Context,
	request gen.RevokeSessionRequestObject) (gen.RevokeSessionResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.auth.RevokeSession(ctx, subject, request.SessionId.String()); err != nil {
		return nil, err
	}
	return gen.RevokeSession204Response{}, nil
}

// RevokeOtherSessions ends every session the caller holds but this one.
func (h *handlers) RevokeOtherSessions(ctx context.Context,
	_ gen.RevokeOtherSessionsRequestObject) (gen.RevokeOtherSessionsResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	revoked, err := h.auth.RevokeOtherSessions(ctx, subject)
	if err != nil {
		return nil, err
	}
	return gen.RevokeOtherSessions200JSONResponse{Revoked: int(revoked)}, nil
}

// renderSession renders one row for the wire.
//
// current is derived here, from the session this request arrived on, rather
// than by the client comparing something it holds — a browser cannot know its
// own session identifier, because the only thing it was given is the token, and
// that value is deliberately not in this response or any other.
//
// The token hash is not merely omitted: [gen.Session] has no field for it, so
// there is nothing to leave out by mistake.
func renderSession(s identity.Session, currentID string) (gen.Session, error) {
	// Parsed rather than asserted, for the reason serviceToken gives: the
	// column is TEXT, and a row whose identifier is not a UUID is damaged data
	// rather than something a client should be handed and asked to send back.
	id, err := uuid.Parse(s.ID)
	if err != nil {
		return gen.Session{}, apierr.Internal(
			fmt.Errorf("session %q does not have a UUID identifier: %w", s.ID, err))
	}

	out := gen.Session{
		Id:           id,
		Current:      s.ID == currentID,
		CreatedAt:    s.CreatedAt,
		LastSeenAt:   s.LastSeenAt,
		ExpiresAt:    s.ExpiresAt,
		MfaSatisfied: s.MFASatisfied,
	}
	if s.IP != "" {
		out.Ip = &s.IP
	}
	if s.UserAgent != "" {
		out.UserAgent = &s.UserAgent
	}
	return out, nil
}
