package analytics

import (
	"database/sql"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
	"github.com/bryanster/blacklight/internal/authz"
)

// ============================================================================
// Daily burndown — hand-computed against fixture expectations
// ============================================================================

func TestFindingsBurndown_Daily(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.FindingsBurndown(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead), IntervalDaily)
	if err != nil {
		t.Fatalf("FindingsBurndown(daily): %v", err)
	}

	if result.Interval != IntervalDaily {
		t.Errorf("interval = %q, want %q", result.Interval, IntervalDaily)
	}

	got := result.Points
	want := f.BurndownSeries

	if len(got) != len(want) {
		t.Fatalf("got %d points, want %d points", len(got), len(want))
	}

	for i := range want {
		cmp(t, i, got[i], want[i])
	}
}

// ============================================================================
// Reopen case — f3 goes open→resolved→open→resolved
// ============================================================================

func TestFindingsBurndown_Reopen(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.FindingsBurndown(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead), IntervalDaily)
	if err != nil {
		t.Fatalf("FindingsBurndown(daily): %v", err)
	}

	// day 4 (index 3): f3 still open
	assert(t, result.Points, 3, 3, 1, 0, 1, 4)
	// day 5 (index 4): f3 resolved
	assert(t, result.Points, 4, 2, 1, 1, 1, 3)
	// day 6 (index 5): f3 still resolved
	assert(t, result.Points, 5, 2, 1, 1, 1, 3)
	// day 7 (index 6): f3 reopened
	assert(t, result.Points, 6, 3, 1, 0, 1, 4)
	// day 8 (index 7): f3 still open
	assert(t, result.Points, 7, 3, 1, 0, 1, 4)
	// day 9 (index 8): f3 re-resolved
	assert(t, result.Points, 8, 2, 1, 1, 1, 3)
	// day 10 (index 9): f3 stays resolved
	assert(t, result.Points, 9, 2, 1, 1, 1, 3)
}

// ============================================================================
// Accepted risk excluded from totalOpen
// ============================================================================

func TestFindingsBurndown_AcceptedRiskExcluded(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.FindingsBurndown(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead), IntervalDaily)
	if err != nil {
		t.Fatalf("FindingsBurndown(daily): %v", err)
	}

	// Day 4 (index 3): f4=accepted_risk (not in totalOpen)
	assert(t, result.Points, 3, 3, 1, 0, 1, 4)
}

// ============================================================================
// Created before engagement start — f5 created at May 30 but appears day 1
// ============================================================================

func TestFindingsBurndown_PreStartCreated(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.FindingsBurndown(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead), IntervalDaily)
	if err != nil {
		t.Fatalf("FindingsBurndown(daily): %v", err)
	}

	// Day 1 (index 0): f5 (created May 30) should appear as open.
	assert(t, result.Points, 0, 4, 0, 0, 1, 4)
}

// ============================================================================
// Created after engagement end — f6 created at Jul 15 appears at day 30
// ============================================================================

func TestFindingsBurndown_PostEndCreated(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.FindingsBurndown(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead), IntervalDaily)
	if err != nil {
		t.Fatalf("FindingsBurndown(daily): %v", err)
	}

	// Day 29 (index 28): f6 not yet clamped
	assert(t, result.Points, 28, 2, 1, 1, 1, 3)
	// Day 30 (index 29): f6 clamped
	assert(t, result.Points, 29, 3, 1, 1, 1, 4)
}

// ============================================================================
// Ends in future — series stops at today, not ends_on
// ============================================================================

func TestFindingsBurndown_EndsInFuture(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.FindingsBurndown(ctx, scope(f.FutureID, false, authz.EngagementRoleLead), IntervalDaily)
	if err != nil {
		t.Fatalf("FindingsBurndown(future, daily): %v", err)
	}

	if len(result.Points) == 0 {
		t.Fatal("expected non-empty series for future engagement")
	}

	lastDate, err := time.Parse("2006-01-02", result.Points[len(result.Points)-1].Date)
	if err != nil {
		t.Fatalf("parse last date %q: %v", result.Points[len(result.Points)-1].Date, err)
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if lastDate.After(today) {
		t.Errorf("last date %s is after today %s", lastDate.Format("2006-01-02"), today.Format("2006-01-02"))
	}

	maxExpected := int(today.Sub(time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)).Hours()/24) + 1
	if len(result.Points) > maxExpected {
		t.Errorf("got %d points, want ≤ %d (stops at today)", len(result.Points), maxExpected)
	}
}

// ============================================================================
// Empty engagement returns full spine of zeroes
// ============================================================================

func TestFindingsBurndown_EmptyEngagement(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.FindingsBurndown(ctx, scope(f.FutureID, false, authz.EngagementRoleLead), IntervalDaily)
	if err != nil {
		t.Fatalf("FindingsBurndown(future, daily): %v", err)
	}

	if len(result.Points) == 0 {
		t.Fatal("expected non-empty zero series for empty engagement")
	}

	for i, pt := range result.Points {
		if pt.Open != 0 || pt.InProgress != 0 || pt.Resolved != 0 || pt.AcceptedRisk != 0 || pt.TotalOpen != 0 {
			t.Errorf("day %d: got non-zero counts", i)
		}
	}
}

// ============================================================================
// Activity-log independence
// ============================================================================

func TestFindingsBurndown_ActivityLogIndependence(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result1, err := q.FindingsBurndown(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead), IntervalDaily)
	if err != nil {
		t.Fatalf("FindingsBurndown #1: %v", err)
	}
	openBefore := result1.Points[4].Open
	if err := f.DB.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO app.activity (id, engagement_id, actor_id, verb, object_type, object_id, delta, "at")
			 VALUES (uuid(), ?, ?, 'finding.updated', 'finding', ?, '{"status":"resolved"}', CURRENT_TIMESTAMP)`,
			f.BaselineID, "01900000-0000-7000-U000-000000000001", f.FindingIDs[0],
		)
		return err
	}); err != nil {
		t.Fatalf("insert activity: %v", err)
	}

	result2, err := q.FindingsBurndown(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead), IntervalDaily)
	if err != nil {
		t.Fatalf("FindingsBurndown #2: %v", err)
	}
	openAfter := result2.Points[4].Open

	if openAfter != openBefore {
		t.Errorf("activity log mutation changed burndown open count from %d to %d", openBefore, openAfter)
	}
}

// ============================================================================
// Interval switch / cap
// ============================================================================

func TestFindingsBurndown_IntervalSwitch(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	daily, err := q.FindingsBurndown(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead), IntervalDaily)
	if err != nil {
		t.Fatalf("FindingsBurndown(daily): %v", err)
	}
	if daily.Interval != IntervalDaily {
		t.Errorf("explicit daily: interval = %q, want %q", daily.Interval, IntervalDaily)
	}

	weekly, err := q.FindingsBurndown(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead), IntervalWeekly)
	if err != nil {
		t.Fatalf("FindingsBurndown(weekly): %v", err)
	}
	if weekly.Interval != IntervalWeekly {
		t.Errorf("explicit weekly: interval = %q, want %q", weekly.Interval, IntervalWeekly)
	}
	if len(weekly.Points) >= len(daily.Points) {
		t.Errorf("weekly points (%d) >= daily points (%d)", len(weekly.Points), len(daily.Points))
	}

	defaultInterval, err := q.FindingsBurndown(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead), "")
	if err != nil {
		t.Fatalf("FindingsBurndown(default): %v", err)
	}
	if defaultInterval.Interval != IntervalDaily {
		t.Errorf("default interval for 30-day engagement: got %q, want %q", defaultInterval.Interval, IntervalDaily)
	}
}

// ============================================================================
// FindingsBySeverity snapshot
// ============================================================================

func TestFindingsBySeverity(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	snapshot, err := q.FindingsBySeverity(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("FindingsBySeverity: %v", err)
	}

	if len(snapshot.Buckets) == 0 {
		t.Fatal("expected non-empty severity buckets")
	}

	totalFindings := 0
	for _, b := range snapshot.Buckets {
		totalFindings += b.Open + b.InProgress + b.Resolved + b.AcceptedRisk
	}
	if totalFindings != 6 {
		t.Errorf("total findings in snapshot = %d, want 6", totalFindings)
	}

	var b *SeverityBucket
	for i := range snapshot.Buckets {
		if snapshot.Buckets[i].Severity == "medium" {
			b = &snapshot.Buckets[i]
			break
		}
	}
	if b == nil {
		t.Fatal("expected 'medium' severity bucket")
	}
	if b.Open != 3 {
		t.Errorf("medium open = %d, want 3", b.Open)
	}
	if b.InProgress != 1 {
		t.Errorf("medium inProgress = %d, want 1", b.InProgress)
	}
	if b.Resolved != 1 {
		t.Errorf("medium resolved = %d, want 1", b.Resolved)
	}
	if b.AcceptedRisk != 1 {
		t.Errorf("medium acceptedRisk = %d, want 1", b.AcceptedRisk)
	}
	if b.TotalOpen != 4 {
		t.Errorf("medium totalOpen = %d, want 4", b.TotalOpen)
	}
}

// ============================================================================
// Helpers
// ============================================================================

func assert(t *testing.T, points []BurndownPoint, dayIdx int, open, inProgress, resolved, acceptedRisk, totalOpen int) {
	t.Helper()
	if dayIdx >= len(points) {
		t.Fatalf("day index %d out of range (len=%d)", dayIdx, len(points))
	}
	got := points[dayIdx]
	check := func(field string, gotv, wantv int) {
		if gotv != wantv {
			t.Errorf("day %d (idx %d): %s = %d, want %d", dayIdx+1, dayIdx, field, gotv, wantv)
		}
	}
	check("open", got.Open, open)
	check("inProgress", got.InProgress, inProgress)
	check("resolved", got.Resolved, resolved)
	check("acceptedRisk", got.AcceptedRisk, acceptedRisk)
	check("totalOpen", got.TotalOpen, totalOpen)
}

func cmp(t *testing.T, idx int, got BurndownPoint, want analyticstest.BurndownDay) {
	t.Helper()
	check := func(field string, gotv, wantv int) {
		if gotv != wantv {
			t.Errorf("day %d (idx %d): %s = %d, want %d", idx+1, idx, field, gotv, wantv)
		}
	}
	check("open", got.Open, want.Open)
	check("inProgress", got.InProgress, want.InProgress)
	check("resolved", got.Resolved, want.Resolved)
	check("acceptedRisk", got.AcceptedRisk, want.AcceptedRisk)
	check("totalOpen", got.TotalOpen, want.TotalOpen)
}
