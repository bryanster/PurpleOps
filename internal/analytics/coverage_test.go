package analytics

import (
	"database/sql"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/store/blind"
)

func scope(engID string, isBlind bool, seat authz.EngagementRole) Scope {
	return Scope{
		EngagementID: engID,
		Blind:        blind.Scope{Blind: isBlind, Seat: seat},
	}
}

// ============================================================================
// TechniqueCoverage tests
// ============================================================================

func TestTechniqueCoverage_BaselineAllSeats(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.TechniqueCoverage(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("TechniqueCoverage(lead): %v", err)
	}

	assertCount(t, result.AttemptedTechniques, 7, "attemptedTechniques")
	assertCount(t, result.NotAttemptedTechniques, 2, "notAttemptedTechniques")
	assertCount(t, result.MatrixTechniques, 10, "matrixTechniques")
	assertCount(t, result.UnmatchedTechniques, 0, "unmatchedTechniques")

	workbookSize := len(result.Rows)
	if result.AttemptedTechniques+result.NotAttemptedTechniques != workbookSize {
		t.Errorf("attempted(%d) + notAttempted(%d) != workbookSize(%d)",
			result.AttemptedTechniques, result.NotAttemptedTechniques, workbookSize)
	}

	byID := rowsByID(result.Rows)

	checkRow(t, byID, "T1190", rowExpect{
		attempted: true, matched: true,
		category: "general", categoryOrd: 2, protection: "blocked",
		stepCount: 1,
	})
	checkRow(t, byID, "T1566", rowExpect{
		attempted: true, matched: true,
		category: "telemetry", categoryOrd: 1, protection: "not_blocked",
		stepCount: 1,
	})
	checkRow(t, byID, "T1566.001", rowExpect{
		attempted: false, matched: true,
		category: "", categoryOrd: nilOrd, protection: "",
		isSub: true, parentTechnique: "T1566",
		stepCount: 1,
	})
	checkRow(t, byID, "T1059.001", rowExpect{
		attempted: true, matched: true,
		category: "", categoryOrd: nilOrd, protection: "",
		isSub: true, parentTechnique: "T1059",
		stepCount: 1,
	})
	checkRow(t, byID, "T1027", rowExpect{
		attempted: true, matched: true,
		category: "general", categoryOrd: 2, protection: "partial",
		stepCount: 1,
	})
	checkRow(t, byID, "T1070", rowExpect{
		attempted: true, matched: true,
		category: "technique", categoryOrd: 4, protection: "not_blocked",
		stepCount: 1,
	})
	checkRow(t, byID, "T1547", rowExpect{
		attempted: false, matched: true,
		category: "", categoryOrd: nilOrd, protection: "",
		stepCount: 1,
	})
	checkRow(t, byID, "T1203", rowExpect{
		attempted: true, matched: true,
		category: "none", categoryOrd: 0, protection: "not_blocked",
		stepCount: 1,
	})
	checkRow(t, byID, "T1059", rowExpect{
		attempted: true, matched: true,
		category: "tactic", categoryOrd: 3, protection: "not_blocked",
		stepCount: 1,
	})
}

func TestTechniqueCoverage_BlueSeat(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.TechniqueCoverage(ctx, scope(f.BaselineID, true, authz.EngagementRoleBlue))
	if err != nil {
		t.Fatalf("TechniqueCoverage(blue): %v", err)
	}

	assertCount(t, result.AttemptedTechniques, 5, "attemptedTechniques")
	assertCount(t, result.NotAttemptedTechniques, 2, "notAttemptedTechniques")
	assertCount(t, result.MatrixTechniques, 10, "matrixTechniques")

	byID := rowsByID(result.Rows)

	if _, ok := byID["T1203"]; ok {
		t.Error("T1203 should be hidden from blue seat")
	}
	if _, ok := byID["T1059"]; ok {
		t.Error("T1059 should be hidden from blue seat")
	}

	checkRow(t, byID, "T1190", rowExpect{
		attempted: true, matched: true,
		category: "general", categoryOrd: 2, protection: "blocked",
		stepCount: 1,
	})
	checkRow(t, byID, "T1059.001", rowExpect{
		attempted: true, matched: true,
		category: "", categoryOrd: nilOrd, protection: "",
		isSub: true, parentTechnique: "T1059",
		stepCount: 1,
	})
	checkRow(t, byID, "T1070", rowExpect{
		attempted: true, matched: true,
		category: "technique", categoryOrd: 4, protection: "not_blocked",
		stepCount: 1,
	})
}

func TestTechniqueCoverage_SeatSweep(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	tests := []struct {
		name           string
		seat           authz.EngagementRole
		isBlind        bool
		wantAttempted  int
		wantNotAttempt int
	}{
		{"lead-blind", authz.EngagementRoleLead, true, 7, 2},
		{"red-blind", authz.EngagementRoleRed, true, 7, 2},
		{"blue-blind", authz.EngagementRoleBlue, true, 5, 2},
		{"observer-blind", authz.EngagementRoleObserver, true, 7, 2},
		{"no-seat-blind", "", true, 7, 2},
		{"lead-standard", authz.EngagementRoleLead, false, 7, 2},
		{"blue-standard", authz.EngagementRoleBlue, false, 7, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := q.TechniqueCoverage(ctx, scope(f.BaselineID, tt.isBlind, tt.seat))
			if err != nil {
				t.Fatalf("TechniqueCoverage: %v", err)
			}
			assertCount(t, result.AttemptedTechniques, tt.wantAttempted, "attemptedTechniques")
			assertCount(t, result.NotAttemptedTechniques, tt.wantNotAttempt, "notAttemptedTechniques")
		})
	}
}

func TestTechniqueCoverage_EmptyEngagement(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.TechniqueCoverage(ctx, scope("01900000-0000-7000-E000-000000000099", false, ""))
	if err != nil {
		t.Fatalf("TechniqueCoverage(empty): %v", err)
	}

	if len(result.Rows) != 0 {
		t.Errorf("empty engagement: got %d rows, want 0", len(result.Rows))
	}
	assertCount(t, result.AttemptedTechniques, 0, "attemptedTechniques")
	assertCount(t, result.NotAttemptedTechniques, 0, "notAttemptedTechniques")
	assertCount(t, result.MatrixTechniques, 0, "matrixTechniques")
	assertCount(t, result.UnmatchedTechniques, 0, "unmatchedTechniques")
}

func TestTechniqueCoverage_UnmatchedTechnique(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.TechniqueCoverage(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("TechniqueCoverage: %v", err)
	}
	assertCount(t, result.UnmatchedTechniques, 0, "unmatchedTechniques (baseline)")

	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	if err := f.DB.Write(ctx, func(tx *sql.Tx) error {
		stepID := "01900000-0000-7000-P000-000000000999"
		execID := "01900000-0000-7000-X000-000000000999"

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app.step
				(id, scenario_id, ordinal, name, objective, technique_id, subtechnique_id,
				 tactic_id, "procedure", template_id, target_asset, tools, controls_in_scope,
				 attack_version, revealed_at, created_at, updated_at)
			 VALUES (?, ?, 99, 'Unmatched Step', '', 'T9999', '', '', '{}', '', '', '[]', '[]',
			        '99.0', ?, ?, ?)`,
			stepID, f.BaselineScenarioIDs[0], now, now, now,
		); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app.execution
				(id, step_id, version, status, executed_by, started_at, ended_at,
				 command_run, source_host, target_host, red_notes,
				 detection_category, detection_modifiers, protection,
				 detected_at, detecting_source, detecting_rule_ref,
				 alert_severity, blue_notes, scored_by, scored_at,
				 created_at, updated_at)
			 VALUES (?, ?, 1, 'complete', '01900000-0000-7000-U000-000000000001',
			        NULL, NULL, '', '', '', '',
			        NULL, '[]', NULL,
			        NULL, '', '',
			        '', '', '', NULL,
			        ?, ?)`,
			execID, stepID, now, now,
		); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("seeding unmatched step: %v", err)
	}

	result, err = q.TechniqueCoverage(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("TechniqueCoverage after unmatched insert: %v", err)
	}

	assertCount(t, result.UnmatchedTechniques, 1, "unmatchedTechniques (after insert)")

	byID := rowsByID(result.Rows)
	row, ok := byID["T9999"]
	if !ok {
		t.Fatal("T9999 missing from result after insert")
	}
	if row.Matched {
		t.Error("T9999: matched should be false (not in matrix)")
	}
	if !row.Attempted {
		t.Error("T9999: attempted should be true (status=complete)")
	}
	assertCount(t, result.MatrixTechniques, 10, "matrixTechniques (unchanged)")
}

// ============================================================================
// TacticCoverage tests
// ============================================================================

func TestTacticCoverage_BaselineAllSeats(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.TacticCoverage(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("TacticCoverage(lead): %v", err)
	}

	byID := tacticRowsByID(result.Rows)

	// TA0001 (Initial Access): matrix={T1566, T1566.001, T1190}=3, attempted={T1190, T1566}=2
	checkTacticRow(t, byID, "TA0001", 2, 3, map[string]int{
		"general":   1,
		"telemetry": 1,
	})

	// TA0002 (Execution): matrix={T1059, T1059.001, T1059.003, T1203, T1547}=5,
	// attempted={T1059, T1059.001, T1203}=3
	checkTacticRow(t, byID, "TA0002", 3, 5, map[string]int{
		"tactic":   1,
		"unscored": 1,
		"none":     1,
	})

	// TA0005 (Defense Evasion): matrix={T1059, T1027, T1070}=3,
	// attempted={T1059, T1027, T1070}=3
	checkTacticRow(t, byID, "TA0005", 3, 3, map[string]int{
		"tactic":    1,
		"general":   1,
		"technique": 1,
	})
}

func TestTacticCoverage_BlueSeat(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.TacticCoverage(ctx, scope(f.BaselineID, true, authz.EngagementRoleBlue))
	if err != nil {
		t.Fatalf("TacticCoverage(blue): %v", err)
	}

	byID := tacticRowsByID(result.Rows)

	// TA0001: T1190, T1566 both revealed — 2 attempted out of 3.
	checkTacticRow(t, byID, "TA0001", 2, 3, map[string]int{
		"general":   1,
		"telemetry": 1,
	})

	// TA0002: only T1059.001 revealed (T1059 and T1203 hidden), unscored.
	checkTacticRow(t, byID, "TA0002", 1, 5, map[string]int{
		"unscored": 1,
	})

	// TA0005: T1027 and T1070 revealed, T1059 hidden. 2 attempted out of 3.
	checkTacticRow(t, byID, "TA0005", 2, 3, map[string]int{
		"general":   1,
		"technique": 1,
	})
}

// ============================================================================
// Helpers
// ============================================================================

type rowExpect struct {
	attempted       bool
	matched         bool
	category        string
	categoryOrd     int // nilOrd means nil
	protection      string
	isSub           bool
	parentTechnique string
	stepCount       int
}

const nilOrd = -1

func rowsByID(rows []TechniqueCoverageRow) map[string]TechniqueCoverageRow {
	m := make(map[string]TechniqueCoverageRow, len(rows))
	for _, r := range rows {
		m[r.TechniqueID] = r
	}
	return m
}

func checkRow(t *testing.T, byID map[string]TechniqueCoverageRow, id string, want rowExpect) {
	t.Helper()
	row, ok := byID[id]
	if !ok {
		t.Errorf("technique %s: missing from result", id)
		return
	}
	if row.Attempted != want.attempted {
		t.Errorf("%s: attempted = %v, want %v", id, row.Attempted, want.attempted)
	}
	if row.Matched != want.matched {
		t.Errorf("%s: matched = %v, want %v", id, row.Matched, want.matched)
	}
	if row.BestCategory != want.category {
		t.Errorf("%s: bestCategory = %q, want %q", id, row.BestCategory, want.category)
	}
	if want.categoryOrd == nilOrd {
		if row.BestCategoryOrdinal != nil {
			t.Errorf("%s: bestCategoryOrdinal = %v, want nil", id, *row.BestCategoryOrdinal)
		}
	} else {
		if row.BestCategoryOrdinal == nil {
			t.Errorf("%s: bestCategoryOrdinal = nil, want %d", id, want.categoryOrd)
		} else if *row.BestCategoryOrdinal != want.categoryOrd {
			t.Errorf("%s: bestCategoryOrdinal = %d, want %d", id, *row.BestCategoryOrdinal, want.categoryOrd)
		}
	}
	if row.BestProtection != want.protection {
		t.Errorf("%s: bestProtection = %q, want %q", id, row.BestProtection, want.protection)
	}
	if row.IsSubtechnique != want.isSub {
		t.Errorf("%s: isSubtechnique = %v, want %v", id, row.IsSubtechnique, want.isSub)
	}
	if row.ParentTechniqueID != want.parentTechnique {
		t.Errorf("%s: parentTechniqueID = %q, want %q", id, row.ParentTechniqueID, want.parentTechnique)
	}
	if row.StepCount != want.stepCount {
		t.Errorf("%s: stepCount = %d, want %d", id, row.StepCount, want.stepCount)
	}
}

func tacticRowsByID(rows []TacticCoverageRow) map[string]TacticCoverageRow {
	m := make(map[string]TacticCoverageRow, len(rows))
	for _, r := range rows {
		m[r.TacticID] = r
	}
	return m
}

func checkTacticRow(t *testing.T, byID map[string]TacticCoverageRow, id string, wantAttempted, wantMatrix int, wantDist map[string]int) {
	t.Helper()
	row, ok := byID[id]
	if !ok {
		t.Errorf("tactic %s: missing from result", id)
		return
	}
	if row.TechniquesAttempted != wantAttempted {
		t.Errorf("%s: techniquesAttempted = %d, want %d", id, row.TechniquesAttempted, wantAttempted)
	}
	if row.TechniquesInMatrix != wantMatrix {
		t.Errorf("%s: techniquesInMatrix = %d, want %d", id, row.TechniquesInMatrix, wantMatrix)
	}
	for cat, wantCount := range wantDist {
		got := row.CategoryDistribution[cat]
		if got != wantCount {
			t.Errorf("%s: categoryDistribution[%q] = %d, want %d", id, cat, got, wantCount)
		}
	}
	for cat, got := range row.CategoryDistribution {
		if _, expected := wantDist[cat]; !expected {
			t.Errorf("%s: unexpected category %q = %d in distribution", id, cat, got)
		}
	}
}

func assertCount(t *testing.T, got, want int, label string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d, want %d", label, got, want)
	}
}
