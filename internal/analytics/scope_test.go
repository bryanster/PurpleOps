package analytics

import (
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/store/blind"
)

// TestStepPredicateComposesWithScope verifies that stepPredicate produces
// the correct SQL fragment for every combination of blind flag and seat,
// and that it agrees with blind.Scope.Where.
func TestStepPredicateComposesWithScope(t *testing.T) {
	tests := []struct {
		name       string
		blindFlag  bool
		seat       authz.EngagementRole
		wantFilter bool // does it filter (not TRUE)?
	}{
		{"lead, blind", true, authz.EngagementRoleLead, false},
		{"red, blind", true, authz.EngagementRoleRed, false},
		{"blue, blind", true, authz.EngagementRoleBlue, true},
		{"observer, blind", true, authz.EngagementRoleObserver, false},
		{"no seat, blind", true, "", false},
		{"lead, standard", false, authz.EngagementRoleLead, false},
		{"red, standard", false, authz.EngagementRoleRed, false},
		{"blue, standard", false, authz.EngagementRoleBlue, false},
		{"observer, standard", false, authz.EngagementRoleObserver, false},
		{"no seat, standard", false, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := blind.Scope{Blind: tt.blindFlag, Seat: tt.seat}
			s := Scope{EngagementID: "test-eng", Blind: bs}

			pred := s.stepPredicate()

			if tt.wantFilter {
				if pred == "TRUE" {
					t.Error("expected filtering predicate, got TRUE")
				}
				if pred != "revealed_at IS NOT NULL" {
					t.Errorf("predicate = %q, want \"revealed_at IS NOT NULL\"", pred)
				}
			} else {
				if pred != "TRUE" {
					t.Errorf("expected TRUE, got %q", pred)
				}
			}

			// The predicate must agree with blind.Scope.Where.
			whereResult := bs.Where("revealed_at IS NOT NULL")
			if pred != whereResult {
				t.Errorf("stepPredicate()=%q but blind.Where()=%q", pred, whereResult)
			}
		})
	}
}
