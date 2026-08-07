package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/store/activity"
)

// ReplayResult holds the events to replay before live tail begins.
type ReplayResult struct {
	// Events are the catch-up events in oldest-first order (the order they
	// happened). The caller writes them as SSE frames before the live stream.
	Events []Event

	// Truncated is true when the replay was capped (max events exceeded).
	// The caller must send a stream.gap event so the client full-refetches.
	Truncated bool
}

// ReplayAfter returns activity rows for engagementID with id strictly after
// cursor, converted to SSE Events. It returns at most maxEvents rows, oldest
// first.
//
// Cursor is an activity row id (UUIDv7). An empty cursor means "no replay" —
// returns an empty result. Activity ids are UUIDv7 and sortable by creation
// time, so `WHERE id > ? ORDER BY id ASC` gives the chronological stream
// after the cursor.
//
// Visibility filtering is NOT applied here — the caller passes the events
// through [VisibleActivity] before sending.
func (l *Log) ReplayAfter(ctx context.Context, engagementID, cursor string, maxEvents int) (ReplayResult, error) {
	if cursor == "" {
		return ReplayResult{}, nil
	}
	if maxEvents <= 0 {
		return ReplayResult{}, nil
	}

	// Query activity rows with id > cursor for this engagement, oldest first.
	// Limit to maxEvents+1 so we can detect truncation.
	rows, err := l.entries.ReplayAfterCursor(ctx, engagementID, cursor, maxEvents+1)
	if err != nil {
		return ReplayResult{}, fmt.Errorf("events: replay %s: %w", engagementID, err)
	}

	truncated := len(rows) > maxEvents
	if truncated {
		rows = rows[:maxEvents]
	}

	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		// Build an SSE Event from the stored activity row.
		// We don't have the original Entry (with ParentIDs) for replay,
		// so we construct the data from the row fields only.
		// The parent IDs (stepId, executionId, etc.) are not available
		// in the row — they were only in the caller's Entry.
		ev := Event{
			ID:    row.ID,
			Type:  row.Verb,
			At:    row.At,
			Topic: EngagementTopic(engagementID),
			Data:  buildReplayData(row),
		}
		events = append(events, ev)
	}

	return ReplayResult{Events: events, Truncated: truncated}, nil
}

// buildReplayData constructs an SSE event payload from a stored activity row
// for replay. Unlike [buildEventData], it does not have the original Entry
// with ParentIDs — it reconstructs from what the row stores.
func buildReplayData(row activity.Row) json.RawMessage {
	m := map[string]any{
		"engagementId": row.EngagementID,
		"actorId":      row.ActorID,
		"verb":         row.Verb,
		"objectType":   row.ObjectType,
		"objectId":     row.ObjectID,
	}
	raw, err := json.Marshal(m)
	if err != nil {
		raw = []byte(`{}`)
	}
	return raw
}

// GapEventData is the JSON payload for a stream.gap / sync.required event.
type GapEventData struct {
	EngagementID string `json:"engagementId"`
	Reason       string `json:"reason"`
}

// NewGapEvent creates a synthetic stream.gap event for an engagement.
func NewGapEvent(engagementID, reason string) Event {
	data, err := json.Marshal(GapEventData{
		EngagementID: engagementID,
		Reason:       reason,
	})
	if err != nil {
		data = []byte(`{"engagementId":"","reason":"marshal error"}`)
	}
	return Event{
		ID:    "", // synthetic — no activity row
		Type:  TypeStreamGap,
		At:    time.Now().UTC(),
		Topic: EngagementTopic(engagementID),
		Data:  data,
	}
}
