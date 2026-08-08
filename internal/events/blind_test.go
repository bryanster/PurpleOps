package events_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/store/blind"
)

func TestVisibleActivity(t *testing.T) {
	// Blue subscriber in blind engagement should not see unrevealed step events.
	blueScope := blind.Scope{Blind: true, Seat: "blue"}
	revealedTrue := true
	revealedFalse := false

	tests := []struct {
		name  string
		scope blind.Scope
		data  events.EventData
		want  bool
	}{
		{
			name:  "blue cannot see unrevealed step",
			scope: blueScope,
			data:  events.EventData{ObjectType: events.ObjectStep, Revealed: &revealedFalse},
			want:  false,
		},
		{
			name:  "blue can see revealed step",
			scope: blueScope,
			data:  events.EventData{ObjectType: events.ObjectStep, Revealed: &revealedTrue},
			want:  true,
		},
		{
			name:  "blue cannot see unrevealed execution",
			scope: blueScope,
			data:  events.EventData{ObjectType: events.ObjectExecution, Revealed: &revealedFalse},
			want:  false,
		},
		{
			name:  "blue can see revealed execution",
			scope: blueScope,
			data:  events.EventData{ObjectType: events.ObjectExecution, Revealed: &revealedTrue},
			want:  true,
		},
		{
			name:  "red always sees unrevealed step",
			scope: blind.Scope{Blind: true, Seat: "red"},
			data:  events.EventData{ObjectType: events.ObjectStep, Revealed: &revealedFalse},
			want:  true,
		},
		{
			name:  "non-blind engagement shows everything",
			scope: blind.Scope{Blind: false, Seat: "blue"},
			data:  events.EventData{ObjectType: events.ObjectStep, Revealed: &revealedFalse},
			want:  true,
		},
		{
			name:  "non-step-scoped events always visible",
			scope: blueScope,
			data:  events.EventData{ObjectType: events.ObjectEngagement},
			want:  true,
		},
		{
			name:  "nil revealed treats as visible",
			scope: blueScope,
			data:  events.EventData{ObjectType: events.ObjectStep, Revealed: nil},
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := json.Marshal(tt.data) //nolint:errcheck // test helper, infallible
			ev := events.Event{Data: data}
			got := events.VisibleActivity(tt.scope, ev)
			if got != tt.want {
				t.Errorf("VisibleActivity = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStepIDForEvent(t *testing.T) {
	tests := []struct {
		name string
		d    *events.EventData
		want string
	}{
		{"step object uses objectId", &events.EventData{ObjectType: events.ObjectStep, ObjectID: "step-1"}, "step-1"},
		{"execution uses stepId parent", &events.EventData{ObjectType: events.ObjectExecution, StepID: "step-2"}, "step-2"},
		{"evidence uses stepId parent", &events.EventData{ObjectType: events.ObjectEvidence, StepID: "step-3"}, "step-3"},
		{"comment uses stepId parent", &events.EventData{ObjectType: events.ObjectComment, StepID: "step-4"}, "step-4"},
		{"engagement returns empty", &events.EventData{ObjectType: events.ObjectEngagement, ObjectID: "eng-1"}, ""},
		{"nil returns empty", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := events.StepIDForEvent(tt.d)
			if got != tt.want {
				t.Errorf("StepIDForEvent = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewGapEvent(t *testing.T) {
	ev := events.NewGapEvent("eng-1", "replay truncated")
	if ev.Type != events.TypeStreamGap {
		t.Errorf("Type = %q, want %q", ev.Type, events.TypeStreamGap)
	}
	if ev.Topic != events.EngagementTopic("eng-1") {
		t.Errorf("Topic = %q, want %q", ev.Topic, events.EngagementTopic("eng-1"))
	}
	var d events.GapEventData
	if err := json.Unmarshal(ev.Data, &d); err != nil {
		t.Fatalf("unmarshal gap data: %v", err)
	}
	if d.EngagementID != "eng-1" || d.Reason != "replay truncated" {
		t.Errorf("data = %+v", d)
	}
}

func TestFilterPresenceEvent(t *testing.T) {
	blueScope := blind.Scope{Blind: true, Seat: "blue"}
	noWithhold := blind.Scope{Blind: false, Seat: "blue"}

	stepID := "01900000-0000-7000-8000-000000000099"
	presenceJSON := json.RawMessage(`{"engagementId":"eng-1","userId":"red-1","displayName":"Red","stepId":"` + stepID + `","executionId":""}`)

	// Fake reveal lookup that always says "not revealed".
	neverRevealed := fakeRevealLookup{}

	// Fake reveal lookup that says "revealed".
	alwaysRevealed := fakeRevealLookup{revealed: true}

	tests := []struct {
		name   string
		scope  blind.Scope
		lookup events.RevealLookup
		evType string
		wantID bool // whether stepId should be present in output
	}{
		{
			name:   "blue blind strips unrevealed focus",
			scope:  blueScope,
			lookup: neverRevealed,
			evType: events.TypePresenceJoin,
			wantID: false,
		},
		{
			name:   "blue blind keeps revealed focus",
			scope:  blueScope,
			lookup: alwaysRevealed,
			evType: events.TypePresenceUpdate,
			wantID: true,
		},
		{
			name:   "no withhold passes through",
			scope:  noWithhold,
			lookup: neverRevealed,
			evType: events.TypePresenceJoin,
			wantID: true,
		},
		{
			name:   "presence leave always passes",
			scope:  blueScope,
			lookup: neverRevealed,
			evType: events.TypePresenceLeave,
			wantID: true, // leave events pass through unchanged
		},
		{
			name:   "nil lookup conservatively strips",
			scope:  blueScope,
			lookup: nil,
			evType: events.TypePresenceJoin,
			wantID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := events.Event{
				Type: tt.evType,
				Data: presenceJSON,
			}
			got := events.FilterPresenceEvent(t.Context(), tt.scope, ev, tt.lookup)

			var pd struct {
				StepID string `json:"stepId"`
			}
			if err := json.Unmarshal(got.Data, &pd); err != nil {
				t.Fatalf("unmarshal result: %v", err)
			}
			hasID := pd.StepID != ""
			if hasID != tt.wantID {
				t.Errorf("stepId present = %v, want %v (data=%s)", hasID, tt.wantID, got.Data)
			}
		})
	}
}

type fakeRevealLookup struct {
	revealed bool
}

func (f fakeRevealLookup) IsStepRevealed(_ context.Context, _ string) (bool, error) {
	return f.revealed, nil
}
