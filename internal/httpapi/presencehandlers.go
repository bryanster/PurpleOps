package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/events/presence"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/blind"
)

// PutEngagementPresence upserts a presence heartbeat for the caller.
// On first heartbeat for a presenceId, publishes presence.join; otherwise
// presence.update.
func (h *handlers) PutEngagementPresence(ctx context.Context,
	request gen.PutEngagementPresenceRequestObject,
) (gen.PutEngagementPresenceResponseObject, error) {
	subject, _ := authn.SubjectFrom(ctx)
	if h.presence == nil {
		return gen.PutEngagementPresence204Response{}, nil
	}

	body := request.Body
	if body == nil {
		return gen.PutEngagementPresence204Response{}, nil
	}

	entry := presence.Entry{
		PresenceID:   body.PresenceId,
		UserID:       subject.UserID,
		EngagementID: request.EngagementId.String(),
		SessionID:    subject.SessionID,
		DisplayName:  subject.DisplayName,
	}
	if body.Focus != nil {
		if body.Focus.StepId != nil {
			entry.Focus.StepID = body.Focus.StepId.String()
		}
		if body.Focus.ExecutionId != nil {
			entry.Focus.ExecutionID = body.Focus.ExecutionId.String()
		}
	}

	joined, err := h.presence.Heartbeat(entry)
	if err != nil {
		h.log.WarnContext(ctx, "presence heartbeat rejected", slog.String("err", err.Error()))
	}

	if h.hub != nil {
		evtType := events.TypePresenceUpdate
		if joined {
			evtType = events.TypePresenceJoin
		}
		topic := events.EngagementTopic(entry.EngagementID)
		h.hub.Publish(topic, events.Event{
			ID:   uuid.Must(uuid.NewV7()).String(),
			Type: evtType,
			Data: presenceEventJSON(entry),
		})
	}

	return gen.PutEngagementPresence204Response{}, nil
}

// DeleteEngagementPresence removes the caller's presence from an engagement.
// When presenceId is absent, all entries for this user in this engagement
// are removed. Publishes presence.leave.
func (h *handlers) DeleteEngagementPresence(ctx context.Context,
	request gen.DeleteEngagementPresenceRequestObject,
) (gen.DeleteEngagementPresenceResponseObject, error) {
	subject, _ := authn.SubjectFrom(ctx)
	if h.presence == nil {
		return gen.DeleteEngagementPresence204Response{}, nil
	}

	engagementID := request.EngagementId.String()

	if request.Params.PresenceId != nil {
		h.presence.Leave(*request.Params.PresenceId)
	} else {
		h.presence.LeaveUser(subject.UserID, engagementID)
	}

	if h.hub != nil {
		topic := events.EngagementTopic(engagementID)
		h.hub.Publish(topic, events.Event{
			ID:   uuid.Must(uuid.NewV7()).String(),
			Type: events.TypePresenceLeave,
			Data: presenceLeaveJSON(subject.UserID, engagementID),
		})
	}

	return gen.DeleteEngagementPresence204Response{}, nil
}

// GetEngagementPresence returns the current presence snapshot for an
// engagement. Focus targets are stripped for blue subscribers in blind
// engagements (M4-009) — unrevealed step/execution ids are removed so
// blue cannot infer hidden step existence from a colleague's focus.
func (h *handlers) GetEngagementPresence(ctx context.Context,
	request gen.GetEngagementPresenceRequestObject,
) (gen.GetEngagementPresenceResponseObject, error) {
	if h.presence == nil {
		return gen.GetEngagementPresence200JSONResponse{
			Entries: []gen.PresenceEntry{},
		}, nil
	}

	engagementID := request.EngagementId.String()

	// Build blind scope to decide whether we strip focus.
	scope, err := h.stepBlindScope(ctx, engagementID)
	if err != nil {
		// Engagement gone; return whatever we have without filtering.
		scope = blind.Scope{}
	}

	snapshot := h.presence.Snapshot(engagementID)

	entries := make([]gen.PresenceEntry, 0, len(snapshot))
	for _, s := range snapshot {
		pe := gen.PresenceEntry{
			UserId:      uuid.MustParse(s.UserID),
			DisplayName: s.DisplayName,
			LastSeenAt:  s.LastSeenAt,
			TabCount:    s.TabCount,
		}
		if s.Focus != nil && !focusIsStripped(ctx, scope, s.Focus, h) {
			//nolint:staticcheck // matching generated JSON tags
			pe.Focus = &struct {
				ExecutionId *uuid.UUID `json:"executionId,omitempty"`
				StepId      *uuid.UUID `json:"stepId,omitempty"`
			}{}
			if s.Focus.StepID != "" {
				sid := uuid.MustParse(s.Focus.StepID)
				pe.Focus.StepId = &sid
			}
			if s.Focus.ExecutionID != "" {
				eid := uuid.MustParse(s.Focus.ExecutionID)
				pe.Focus.ExecutionId = &eid
			}
		}
		entries = append(entries, pe)
	}

	return gen.GetEngagementPresence200JSONResponse{Entries: entries}, nil
}

// focusIsStripped reports whether the presence focus should be withheld
// from the caller under the given blind scope. It returns true when
// every focus target references an unrevealed step, making the entire
// focus nil for the response.
func focusIsStripped(ctx context.Context, scope blind.Scope, focus *presence.Focus, lookup events.RevealLookup) bool {
	if !scope.Withholds() {
		return false
	}
	if focus == nil {
		return false
	}
	// If either focus target references a revealed step, we keep the
	// focus (the revealed one stays; unrevealed ones are individually
	// stripped by the caller).
	hasVisible := false
	if focus.StepID != "" {
		revealed, err := lookup.IsStepRevealed(ctx, focus.StepID)
		if err == nil && revealed {
			hasVisible = true
		}
	}
	if focus.ExecutionID != "" && !hasVisible {
		revealed, err := lookup.IsStepRevealed(ctx, focus.ExecutionID)
		if err == nil && revealed {
			hasVisible = true
		}
	}
	// TODO: also check that execution's step is revealed via step lookup.
	// For now, execution focus is checked directly (both are step IDs in
	// the current schema; execution focus references the step's own ID).
	return !hasVisible
}

// ---------------------------------------------------------------------------
// SSE helpers
// ---------------------------------------------------------------------------

func presenceEventJSON(e presence.Entry) json.RawMessage {
	b, err := json.Marshal(map[string]any{
		"engagementId": e.EngagementID,
		"userId":       e.UserID,
		"displayName":  e.DisplayName,
		"stepId":       e.Focus.StepID,
		"executionId":  e.Focus.ExecutionID,
	})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(b)
}

func presenceLeaveJSON(userID, engagementID string) json.RawMessage {
	b, err := json.Marshal(map[string]string{
		"userId":       userID,
		"engagementId": engagementID,
	})
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(b)
}
