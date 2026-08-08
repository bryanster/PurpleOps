package httpapi

import (
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/store/blind"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// TestStepsToWireFiltersBlindStepsEvenWhenStoreDoesNot confirms the
// belt-and-braces design: even if the store-layer blind filter were disabled,
// the wire-layer filter in stepsToWire still withholds unrevealed steps from
// a blue reader in a blind engagement (M5-002 acceptance criterion 3).
func TestStepsToWireFiltersBlindStepsEvenWhenStoreDoesNot(t *testing.T) {
	now := time.Now().UTC()
	revealed := now
	var unrevealed *time.Time

	steps := []storengagement.Step{
		{
			ID:            "00000000-0000-0000-0000-000000000001",
			ScenarioID:    "00000000-0000-0000-0000-000000000010",
			Ordinal:       1,
			Name:          "Revealed Step",
			TechniqueID:   "T1059",
			AttackVersion: "15.1",
			RevealedAt:    &revealed,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
		{
			ID:            "00000000-0000-0000-0000-000000000002",
			ScenarioID:    "00000000-0000-0000-0000-000000000010",
			Ordinal:       2,
			Name:          "Hidden Step",
			TechniqueID:   "T1059.001",
			AttackVersion: "15.1",
			RevealedAt:    unrevealed,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	}

	// Blue in blind engagement: should see only the revealed step.
	blueBlindScope := blind.Scope{Blind: true, Seat: authz.EngagementRoleBlue}
	got, err := stepsToWire(steps, blueBlindScope)
	if err != nil {
		t.Fatalf("stepsToWire: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("blue blind saw %d steps, want 1 (the store returned both, wire should filter)", len(got))
	}
	if got[0].Name != "Revealed Step" {
		t.Errorf("blue blind saw %q, want \"Revealed Step\"", got[0].Name)
	}

	// Red in blind engagement: sees everything.
	redBlindScope := blind.Scope{Blind: true, Seat: authz.EngagementRoleRed}
	got2, err := stepsToWire(steps, redBlindScope)
	if err != nil {
		t.Fatalf("stepsToWire red: %v", err)
	}
	if len(got2) != 2 {
		t.Errorf("red blind saw %d steps, want 2", len(got2))
	}

	// Non-blind engagement: sees everything.
	stdScope := blind.Scope{Blind: false, Seat: authz.EngagementRoleBlue}
	got3, err := stepsToWire(steps, stdScope)
	if err != nil {
		t.Fatalf("stepsToWire standard: %v", err)
	}
	if len(got3) != 2 {
		t.Errorf("standard mode saw %d steps, want 2", len(got3))
	}
}
