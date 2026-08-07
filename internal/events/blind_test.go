package events_test

import (
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
