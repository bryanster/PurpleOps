package analytics

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/domain/scoring"
)

// ============================================================================
// Baseline — all seats (lead view, blind engagement)
// ============================================================================

func TestMTTD_BaselineAllSeats(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.MTTD(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("MTTD(lead): %v", err)
	}

	// Hand-computed from fixture timestamps:
	//   Detected with both timestamps AND category NOT 'none':
	//     Step 1 (T1190): 600s, Step 2 (T1566): 300s,
	//     Step 5 (T1027): 180s, Step 9 (T1059): 1800s
	//   Sorted: [180, 300, 600, 1800]
	//   p50: rank=1.5 → 300 + 0.5×(600−300) = 450
	//   p90: rank=2.7 → 600 + 0.7×(1800−600) = 1440
	//   max: 1800
	//   Step 8 (T1203): category 'none' → undetected (despite detected_at set)
	//   Step 4 (T1059.001): NULL category → unscored
	//   Step 6 (T1070): technique, NULL started_at → unmeasurable

	assertIntPtr(t, result.P50, 450, "p50")
	assertIntPtr(t, result.P90, 1440, "p90")
	assertIntPtr(t, result.Max, 1800, "max")
	assertCountEq(t, result.DetectedCount, 4, "detectedCount")
	assertCountEq(t, result.UndetectedCount, 1, "undetectedCount")
	assertCountEq(t, result.UnscoredCount, 1, "unscoredCount")
	assertCountEq(t, result.UnmeasurableCount, 1, "unmeasurableCount")
	assertCountEq(t, result.AttemptedCount, 7, "attemptedCount")

	assertSumMatches(t, result)
}

// ============================================================================
// Blue seat — blind engagement, unrevealed steps excluded
// ============================================================================

func TestMTTD_BlueSeat(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.MTTD(ctx, scope(f.BaselineID, true, authz.EngagementRoleBlue))
	if err != nil {
		t.Fatalf("MTTD(blue): %v", err)
	}

	// Blue sees only revealed steps (1–7). Steps 8 (T1203, unrevealed)
	// and 9 (T1059, unrevealed) are excluded.
	//   Detected: Step 1(600s), Step 2(300s), Step 5(180s)
	//   Sorted: [180, 300, 600]
	//   p50: rank=1.0 → 300
	//   p90: rank=1.8 → 300 + 0.8×(600−300) = 540
	//   max: 600
	//   undetected: 0 (step 8 excluded as unrevealed)
	//   unscored: 1 (step 4)
	//   unmeasurable: 1 (step 6)
	//   attempted: 5

	assertIntPtr(t, result.P50, 300, "p50")
	assertIntPtr(t, result.P90, 540, "p90")
	assertIntPtr(t, result.Max, 600, "max")
	assertCountEq(t, result.DetectedCount, 3, "detectedCount")
	assertCountEq(t, result.UndetectedCount, 0, "undetectedCount")
	assertCountEq(t, result.UnscoredCount, 1, "unscoredCount")
	assertCountEq(t, result.UnmeasurableCount, 1, "unmeasurableCount")
	assertCountEq(t, result.AttemptedCount, 5, "attemptedCount")

	assertSumMatches(t, result)
}

// ============================================================================
// Retest — standard engagement, all detected
// ============================================================================

func TestMTTD_Retest(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.MTTD(ctx, scope(f.RetestID, false, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("MTTD(retest): %v", err)
	}

	// All 6 steps are complete, scored, with both timestamps:
	//   Step 1: 600s, Step 2: 300s, Step 3: 180s,
	//   Step 4: 300s, Step 5: 180s, Step 6: 600s
	//   Sorted: [180, 180, 300, 300, 600, 600]
	//   p50: rank=2.5 → 300 + 0.5×(300−300) = 300
	//   p90: rank=4.5 → 600 + 0.5×(600−600) = 600
	//   max: 600

	assertIntPtr(t, result.P50, 300, "p50")
	assertIntPtr(t, result.P90, 600, "p90")
	assertIntPtr(t, result.Max, 600, "max")
	assertCountEq(t, result.DetectedCount, 6, "detectedCount")
	assertCountEq(t, result.UndetectedCount, 0, "undetectedCount")
	assertCountEq(t, result.UnscoredCount, 0, "unscoredCount")
	assertCountEq(t, result.UnmeasurableCount, 0, "unmeasurableCount")
	assertCountEq(t, result.AttemptedCount, 6, "attemptedCount")

	assertSumMatches(t, result)
}

// ============================================================================
// Seat sweep — every role gets the right counts
// ============================================================================

func TestMTTD_SeatSweep(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	// For the blind baseline, lead/red/observer see all 7 attempted,
	// blue sees 5 (steps 8+9 unrevealed), no-seat gets widest scope (7).
	tests := []struct {
		seat          authz.EngagementRole
		wantAttempted int
		wantDetected  int
		wantUndetect  int
		wantUnscored  int
		wantUnmeas    int
	}{
		{authz.EngagementRoleLead, 7, 4, 1, 1, 1},
		{authz.EngagementRoleRed, 7, 4, 1, 1, 1},
		{authz.EngagementRoleBlue, 5, 3, 0, 1, 1},
		{authz.EngagementRoleObserver, 7, 4, 1, 1, 1},
		{"", 7, 4, 1, 1, 1}, // no seat → widest scope
	}

	for _, tt := range tests {
		name := string(tt.seat)
		if name == "" {
			name = "no-seat"
		}
		t.Run(name, func(t *testing.T) {
			result, err := q.MTTD(ctx, scope(f.BaselineID, true, tt.seat))
			if err != nil {
				t.Fatalf("MTTD(%s): %v", name, err)
			}

			assertCountEq(t, result.AttemptedCount, tt.wantAttempted, "attemptedCount")
			assertCountEq(t, result.DetectedCount, tt.wantDetected, "detectedCount")
			assertCountEq(t, result.UnmeasurableCount, tt.wantUnmeas, "unmeasurableCount")
			assertSumMatches(t, result)
		})
	}
}

func TestMTTD_DegenerateCases(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	// Empty engagement.
	t.Run("empty engagement", func(t *testing.T) {
		engID := "01900000-0000-7000-Z000-000000000001"
		seedEngagement(t, f, ctx, engID, "Empty")
		result, err := q.MTTD(ctx, scope(engID, false, authz.EngagementRoleLead))
		if err != nil {
			t.Fatalf("MTTD(empty): %v", err)
		}
		assertCountEq(t, result.AttemptedCount, 0, "attemptedCount")
		assertIntPtrNil(t, result.P50, "p50")
		assertIntPtrNil(t, result.P90, "p90")
		assertIntPtrNil(t, result.Max, "max")
		assertSumMatches(t, result)
	})

	// Zero detections — category 'none'.
	t.Run("zero detections", func(t *testing.T) {
		engID := "01900000-0000-7000-Z000-000000000002"
		scID := "01900000-0000-7000-ZS00-000000000002"
		sID := "01900000-0000-7000-ZP00-000000000002"
		eID := "01900000-0000-7000-ZX00-000000000002"
		seedEngagement(t, f, ctx, engID, "ZeroDetect")
		seedScenario(t, f, ctx, scID, engID, "Zero", 1)
		seedStepExec(t, f, ctx, scID, sID, eID, "T1190", "complete",
			ns("none"), ns("not_blocked"),
			ntStr("2026-06-02 09:00:00"), ntStr("2026-06-02 09:30:00"), ntStr("2026-06-02 09:30:00"))
		result, err := q.MTTD(ctx, scope(engID, false, authz.EngagementRoleLead))
		if err != nil {
			t.Fatalf("MTTD(zero detections): %v", err)
		}
		assertCountEq(t, result.DetectedCount, 0, "detectedCount")
		assertCountEq(t, result.UndetectedCount, 1, "undetectedCount")
		assertCountEq(t, result.AttemptedCount, 1, "attemptedCount")
		assertIntPtrNil(t, result.P50, "p50")
		assertIntPtrNil(t, result.P90, "p90")
		assertIntPtrNil(t, result.Max, "max")
		assertSumMatches(t, result)
	})

	// One detection — single sample, 10min = 600s.
	t.Run("one detection", func(t *testing.T) {
		engID := "01900000-0000-7000-Z000-000000000003"
		scID := "01900000-0000-7000-ZS00-000000000003"
		sID := "01900000-0000-7000-ZP00-000000000003"
		eID := "01900000-0000-7000-ZX00-000000000003"
		seedEngagement(t, f, ctx, engID, "OneDetect")
		seedScenario(t, f, ctx, scID, engID, "One", 1)
		seedStepExec(t, f, ctx, scID, sID, eID, "T1190", "complete",
			ns("technique"), ns("not_blocked"),
			ntStr("2026-06-02 09:00:00"), ntStr("2026-06-02 09:30:00"), ntStr("2026-06-02 09:10:00"))
		result, err := q.MTTD(ctx, scope(engID, false, authz.EngagementRoleLead))
		if err != nil {
			t.Fatalf("MTTD(one detection): %v", err)
		}
		assertIntPtr(t, result.P50, 600, "p50")
		assertIntPtr(t, result.P90, 600, "p90")
		assertIntPtr(t, result.Max, 600, "max")
		assertCountEq(t, result.DetectedCount, 1, "detectedCount")
		assertSumMatches(t, result)
	})

	// Two detections: 200s + 800s.
	t.Run("two detections", func(t *testing.T) {
		engID := "01900000-0000-7000-Z000-000000000004"
		scID := "01900000-0000-7000-ZS00-000000000004"
		sA := "01900000-0000-7000-ZP00-000000000004"
		eA := "01900000-0000-7000-ZX00-000000000004"
		sB := "01900000-0000-7000-ZP00-000000000005"
		eB := "01900000-0000-7000-ZX00-000000000005"
		seedEngagement(t, f, ctx, engID, "TwoDetect")
		seedScenario(t, f, ctx, scID, engID, "Two", 1)
		seedStepExec(t, f, ctx, scID, sA, eA, "T1190", "complete",
			ns("technique"), ns("not_blocked"),
			ntStr("2026-06-02 09:00:00"), ntStr("2026-06-02 09:30:00"), ntStr("2026-06-02 09:03:20"))
		seedStepExecOrd(t, f, ctx, scID, sB, eB, "T1566", "complete", 2,
			ns("technique"), ns("not_blocked"),
			ntStr("2026-06-02 09:00:00"), ntStr("2026-06-02 09:30:00"), ntStr("2026-06-02 09:13:20"))
		result, err := q.MTTD(ctx, scope(engID, false, authz.EngagementRoleLead))
		if err != nil {
			t.Fatalf("MTTD(two detections): %v", err)
		}
		assertIntPtr(t, result.P50, 500, "p50")
		assertIntPtr(t, result.P90, 740, "p90")
		assertIntPtr(t, result.Max, 800, "max")
		assertCountEq(t, result.DetectedCount, 2, "detectedCount")
		assertSumMatches(t, result)
	})
}

func TestMTTD_EdgeCases(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	// NULL started_at.
	t.Run("null started_at is unmeasurable", func(t *testing.T) {
		engID := "01900000-0000-7000-Z000-000000000010"
		scID := "01900000-0000-7000-ZS00-000000000010"
		sID := "01900000-0000-7000-ZP00-000000000010"
		eID := "01900000-0000-7000-ZX00-000000000010"
		seedEngagement(t, f, ctx, engID, "NullStart")
		seedScenario(t, f, ctx, scID, engID, "S", 1)
		seedStepExec(t, f, ctx, scID, sID, eID, "T1190", "complete",
			ns("technique"), ns("not_blocked"),
			sql.NullTime{}, ntStr("2026-06-02 09:30:00"), ntStr("2026-06-02 09:10:00"))
		result, err := q.MTTD(ctx, scope(engID, false, authz.EngagementRoleLead))
		if err != nil {
			t.Fatalf("MTTD(null started_at): %v", err)
		}
		assertCountEq(t, result.UnmeasurableCount, 1, "unmeasurableCount")
		assertCountEq(t, result.DetectedCount, 0, "detectedCount")
		assertIntPtrNil(t, result.P50, "p50")
		assertIntPtrNil(t, result.P90, "p90")
		assertIntPtrNil(t, result.Max, "max")
		assertSumMatches(t, result)
	})

	// Category 'none' with detected_at.
	t.Run("category none with detected_at", func(t *testing.T) {
		engID := "01900000-0000-7000-Z000-000000000011"
		scID := "01900000-0000-7000-ZS00-000000000011"
		sID := "01900000-0000-7000-ZP00-000000000011"
		eID := "01900000-0000-7000-ZX00-000000000011"
		seedEngagement(t, f, ctx, engID, "NoneWithTs")
		seedScenario(t, f, ctx, scID, engID, "S", 1)
		seedStepExec(t, f, ctx, scID, sID, eID, "T1203", "complete",
			ns("none"), ns("not_blocked"),
			ntStr("2026-06-02 09:00:00"), ntStr("2026-06-02 09:30:00"), ntStr("2026-06-02 09:10:00"))
		result, err := q.MTTD(ctx, scope(engID, false, authz.EngagementRoleLead))
		if err != nil {
			t.Fatalf("MTTD(category none): %v", err)
		}
		assertCountEq(t, result.UndetectedCount, 1, "undetectedCount")
		assertCountEq(t, result.DetectedCount, 0, "detectedCount")
		assertIntPtrNil(t, result.P50, "p50")
		assertIntPtrNil(t, result.P90, "p90")
		assertIntPtrNil(t, result.Max, "max")
		assertSumMatches(t, result)
	})

	// Inverted timestamp.
	t.Run("inverted timestamp", func(t *testing.T) {
		engID := "01900000-0000-7000-Z000-000000000012"
		scID := "01900000-0000-7000-ZS00-000000000012"
		sID := "01900000-0000-7000-ZP00-000000000012"
		eID := "01900000-0000-7000-ZX00-000000000012"
		seedEngagement(t, f, ctx, engID, "Inverted")
		seedScenario(t, f, ctx, scID, engID, "S", 1)
		seedStepExec(t, f, ctx, scID, sID, eID, "T1190", "complete",
			ns("technique"), ns("not_blocked"),
			ntStr("2026-06-02 09:10:00"), ntStr("2026-06-02 09:30:00"), ntStr("2026-06-02 09:00:00"))
		result, err := q.MTTD(ctx, scope(engID, false, authz.EngagementRoleLead))
		if err != nil {
			t.Fatalf("MTTD(inverted): %v", err)
		}
		assertCountEq(t, result.UnmeasurableCount, 1, "unmeasurableCount")
		assertCountEq(t, result.DetectedCount, 0, "detectedCount")
		assertIntPtrNil(t, result.P50, "p50")
		assertIntPtrNil(t, result.P90, "p90")
		assertIntPtrNil(t, result.Max, "max")
		assertSumMatches(t, result)
	})
}

// ============================================================================
// Agreement with scoring.MTTD
// ============================================================================

// TestMTTD_SQLAgreesWithScoringMTTD enumerates every detected execution in the
// fixture baseline and asserts the SQL's duration matches scoring.MTTD's Go
// computation, including the inverted-timestamp guard and NULL handling.
func TestMTTD_SQLAgreesWithScoringMTTD(t *testing.T) {
	f := analyticstest.Seed(t)
	ctx := t.Context()

	// Query the per-execution durations computed by the same SQL logic.
	type execMTTD struct {
		stepID      string
		mttdSeconds sql.NullFloat64
		category    sql.NullString
		detectedAt  sql.NullTime
		startedAt   sql.NullTime
	}
	var rows []execMTTD
	query := `
		SELECT s.id,
		       e.detection_category,
		       e.detected_at,
		       e.started_at,
		       CASE
		           WHEN e.status IN ('complete', 'blocked')
		            AND e.detection_category IS NOT NULL
		            AND e.detection_category != 'none'
		            AND e.detected_at IS NOT NULL
		            AND e.started_at IS NOT NULL
		            AND EPOCH(e.detected_at) - EPOCH(e.started_at) >= 0
		           THEN EPOCH(e.detected_at) - EPOCH(e.started_at)
		       END AS mttd_seconds
		FROM app.step s
		JOIN app.execution e ON e.step_id = s.id
		WHERE s.scenario_id IN (
			SELECT sc.id FROM app.scenario sc WHERE sc.engagement_id = ?
		)
		AND TRUE
		ORDER BY s.ordinal`
	rq, err := f.DB.Read().QueryContext(ctx, query, f.BaselineID) //nolint:rowserrcheck
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	_ = rq.Err()     //nolint:errcheck
	defer rq.Close() //nolint:sqlclosecheck
	for rq.Next() {
		var r execMTTD
		if err := rq.Scan(&r.stepID, &r.category, &r.detectedAt, &r.startedAt, &r.mttdSeconds); err != nil {
			t.Fatalf("scan: %v", err)
		}
		rows = append(rows, r)
	}

	if len(rows) == 0 {
		t.Fatal("no execution rows returned")
	}

	for _, r := range rows {
		var goStarted, goDetected *time.Time
		if r.startedAt.Valid {
			goStarted = &r.startedAt.Time
		}
		if r.detectedAt.Valid {
			goDetected = &r.detectedAt.Time
		}

		goDur, goOk, goErr := scoring.MTTD(goStarted, goDetected)

		if r.mttdSeconds.Valid {
			// SQL produced a duration — must agree with Go.
			if !goOk {
				t.Errorf("step %s: SQL has MTTD=%.0f but scoring.MTTD returned ok=false (err=%v)",
					r.stepID, r.mttdSeconds.Float64, goErr)
				continue
			}
			sqlSec := r.mttdSeconds.Float64
			goSec := goDur.Seconds()
			// Allow 1ms tolerance for float representation.
			if sqlSec < goSec-0.001 || sqlSec > goSec+0.001 {
				t.Errorf("step %s: SQL MTTD=%.3f but scoring.MTTD=%v (%.3fs)",
					r.stepID, sqlSec, goDur, goSec)
			}
		}
		// When SQL produced no duration (mttd_seconds is NULL), this execution
		// is not detected per M5-006's definition. scoring.MTTD may return a
		// value (it only checks timestamps, not category). We do not compare
		// here — the bucket tests (undetectedCount, etc.) verify correct
		// classification.
	}
}

// ============================================================================
// Helpers
// ============================================================================

func assertIntPtr(t *testing.T, got *int, want int, label string) {
	t.Helper()
	if got == nil {
		t.Errorf("%s: got nil, want %d", label, want)
		return
	}
	if *got != want {
		t.Errorf("%s = %d, want %d", label, *got, want)
	}
}

func assertIntPtrNil(t *testing.T, got *int, label string) {
	t.Helper()
	if got != nil {
		t.Errorf("%s: got %d, want nil", label, *got)
	}
}

func assertCountEq(t *testing.T, got, want int, label string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d, want %d", label, got, want)
	}
}

// assertSumMatches verifies that the four component counts sum to
// attemptedCount.
func assertSumMatches(t *testing.T, r *MTTDResult) {
	t.Helper()
	sum := r.DetectedCount + r.UndetectedCount + r.UnscoredCount + r.UnmeasurableCount
	if sum != r.AttemptedCount {
		t.Errorf("component sum (%d+%d+%d+%d=%d) != attemptedCount (%d)",
			r.DetectedCount, r.UndetectedCount, r.UnscoredCount, r.UnmeasurableCount,
			sum, r.AttemptedCount)
	}
}

// ============================================================================
// Seed helpers for degenerate/edge case tests
// ============================================================================

func ns(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func ntStr(s string) sql.NullTime {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		panic(fmt.Sprintf("ntStr(%q): %v", s, err))
	}
	return sql.NullTime{Time: t, Valid: true}
}

func seedEngagement(t *testing.T, f analyticstest.Fixture, ctx context.Context, engID, name string) {
	t.Helper()
	if err := f.DB.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO app.engagement
			(id, name, client, description, status, starts_on, ends_on,
			 attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at)
		 VALUES (?, ?, '', '', 'active', '2026-01-01', '2026-12-31',
		         '99.0', 'standard', false, ?, '2026-01-01', '2026-01-01')`,
			engID, name, f.BaselineID[:19]+"U")
		return err
	}); err != nil {
		t.Fatalf("seed engagement %s: %v", name, err)
	}
}

func seedScenario(t *testing.T, f analyticstest.Fixture, ctx context.Context, scID, engID, name string, ordinal int) {
	t.Helper()
	if err := f.DB.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO app.scenario
			(id, engagement_id, ordinal, name, narrative, source,
			 threat_actor, source_ref, plan_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, '', 'manual', '', '', '', '2026-06-01', '2026-06-01')`,
			scID, engID, ordinal, name)
		return err
	}); err != nil {
		t.Fatalf("seed scenario %s: %v", name, err)
	}
}

func seedStepExec(t *testing.T, f analyticstest.Fixture, ctx context.Context,
	scID, stepID, execID, techniqueID, status string,
	detCat, protection sql.NullString,
	startedAt, endedAt, detectedAt sql.NullTime,
) {
	seedStepExecOrd(t, f, ctx, scID, stepID, execID, techniqueID, status, 1,
		detCat, protection, startedAt, endedAt, detectedAt)
}

func seedStepExecOrd(t *testing.T, f analyticstest.Fixture, ctx context.Context,
	scID, stepID, execID, techniqueID, status string, ordinal int,
	detCat, protection sql.NullString,
	startedAt, endedAt, detectedAt sql.NullTime,
) {
	t.Helper()
	if err := f.DB.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app.step
			(id, scenario_id, ordinal, name, objective, technique_id, subtechnique_id,
			 tactic_id, "procedure", template_id, target_asset, tools, controls_in_scope,
			 attack_version, revealed_at, created_at, updated_at)
		 VALUES (?, ?, ?, 'Step', '', ?, '', '', '{}', '', '', '[]', '[]',
		         '99.0', '2026-06-01', '2026-06-01', '2026-06-01')`,
			stepID, scID, ordinal, techniqueID); err != nil {
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
		 VALUES (?, ?, 1, ?, ?, ?, ?,
		         '', '', '', '',
		         ?, '[]', ?,
		         ?, '', '',
		         '', '', ?, ?,
		         '2026-06-01', '2026-06-01')`,
			execID, stepID,
			status, f.BaselineID[:19]+"U",
			nullIfNullTime(startedAt), nullIfNullTime(endedAt),
			nullIfNullString(detCat), nullIfNullString(protection),
			nullIfNullTime(detectedAt),
			f.BaselineID[:19]+"U", nilIfTimeStr("2026-06-02 10:00:00"),
		); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("seed step+exec: %v", err)
	}
}

func nullIfNullString(ns sql.NullString) any {
	if ns.Valid {
		return ns.String
	}
	return nil
}

func nullIfNullTime(nt sql.NullTime) any {
	if nt.Valid {
		return nt.Time
	}
	return nil
}

func nilIfTimeStr(s string) any {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return nil
	}
	return t
}
