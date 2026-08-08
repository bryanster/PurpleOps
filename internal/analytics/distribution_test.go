package analytics

import (
	"database/sql"
	"testing"

	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/domain/scoring"
)

// ============================================================================
// CategoryDistribution tests
// ============================================================================

func TestCategoryDistribution_BaselineAllSeats(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.CategoryDistribution(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("CategoryDistribution(lead): %v", err)
	}

	if result.Attempted != 7 {
		t.Errorf("Attempted = %d, want 7", result.Attempted)
	}

	// Hand-computed: none=1 (T1203), telemetry=1 (T1566), general=2 (T1190, T1027),
	//   tactic=1 (T1059), technique=1 (T1070), unscored=1 (T1059.001), notAttempted=2
	want := map[string]int{
		"none": 1, "telemetry": 1, "general": 2, "tactic": 1, "technique": 1,
		"unscored": 1, "notAttempted": 2,
	}
	checkDistribution(t, result.Buckets, want)

	// Category counts must sum to attempted.
	sum := sumBuckets(result.Buckets, "notAttempted")
	if sum != result.Attempted {
		t.Errorf("category counts sum to %d, want attempted=%d", sum, result.Attempted)
	}
}

func TestCategoryDistribution_BlueSeat(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.CategoryDistribution(ctx, scope(f.BaselineID, true, authz.EngagementRoleBlue))
	if err != nil {
		t.Fatalf("CategoryDistribution(blue): %v", err)
	}

	if result.Attempted != 5 {
		t.Errorf("Attempted = %d, want 5", result.Attempted)
	}

	// Blue loses unrevealed steps 8 (none) and 9 (tactic).
	want := map[string]int{
		"none": 0, "telemetry": 1, "general": 2, "tactic": 0, "technique": 1,
		"unscored": 1, "notAttempted": 2,
	}
	checkDistribution(t, result.Buckets, want)

	sum := sumBuckets(result.Buckets, "notAttempted")
	if sum != result.Attempted {
		t.Errorf("category counts sum to %d, want attempted=%d", sum, result.Attempted)
	}
}

func TestCategoryDistribution_Retest(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.CategoryDistribution(ctx, scope(f.RetestID, false, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("CategoryDistribution(retest): %v", err)
	}

	if result.Attempted != 6 {
		t.Errorf("Attempted = %d, want 6", result.Attempted)
	}

	want := map[string]int{
		"none": 0, "telemetry": 0, "general": 1, "tactic": 1, "technique": 4,
		"unscored": 0, "notAttempted": 0,
	}
	checkDistribution(t, result.Buckets, want)
}

func TestCategoryDistribution_SeatSweep(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	// Baseline is blind. Lead/red/observer see everything; blue sees less.
	seats := []struct {
		role      authz.EngagementRole
		attempted int
	}{
		{authz.EngagementRoleLead, 7},
		{authz.EngagementRoleRed, 7},
		{authz.EngagementRoleBlue, 5},
		{authz.EngagementRoleObserver, 7},
	}
	for _, s := range seats {
		result, err := q.CategoryDistribution(ctx, scope(f.BaselineID, true, s.role))
		if err != nil {
			t.Errorf("CategoryDistribution(%s): %v", s.role, err)
			continue
		}
		if result.Attempted != s.attempted {
			t.Errorf("CategoryDistribution(%s).Attempted = %d, want %d", s.role, result.Attempted, s.attempted)
		}
	}
}

func TestCategoryDistribution_EmptyEngagement(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	// Use retest engagement which has no skipped/pending — insert a new
	// empty engagement to get true zero.
	result, err := q.CategoryDistribution(ctx, scope("00000000-0000-0000-0000-000000000000", false, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("CategoryDistribution(empty): %v", err)
	}

	if result.Attempted != 0 {
		t.Errorf("Attempted = %d, want 0", result.Attempted)
	}
	for _, b := range result.Buckets {
		if b.Count != 0 {
			t.Errorf("bucket %s = %d, want 0", b.Label, b.Count)
		}
	}
}

// ============================================================================
// ProtectionRate tests
// ============================================================================

func TestProtectionRate_BaselineAllSeats(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.ProtectionRate(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("ProtectionRate(lead): %v", err)
	}

	if result.Attempted != 7 {
		t.Errorf("Attempted = %d, want 7", result.Attempted)
	}

	// Hand-computed: blocked=1 (T1190), partial=1 (T1027), not_blocked=4 (T1566,T1070,T1203,T1059),
	//   n/a=0, unscored=1 (T1059.001)
	want := map[string]int{
		"blocked": 1, "partial": 1, "not_blocked": 4, "n/a": 0, "unscored": 1,
	}
	checkDistribution(t, result.Buckets, want)

	sum := sumBuckets(result.Buckets, "")
	if sum != result.Attempted {
		t.Errorf("protection counts sum to %d, want attempted=%d", sum, result.Attempted)
	}
}

func TestProtectionRate_BlueSeat(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.ProtectionRate(ctx, scope(f.BaselineID, true, authz.EngagementRoleBlue))
	if err != nil {
		t.Fatalf("ProtectionRate(blue): %v", err)
	}

	if result.Attempted != 5 {
		t.Errorf("Attempted = %d, want 5", result.Attempted)
	}

	// Blue loses steps 8 (not_blocked) and 9 (not_blocked).
	want := map[string]int{
		"blocked": 1, "partial": 1, "not_blocked": 2, "n/a": 0, "unscored": 1,
	}
	checkDistribution(t, result.Buckets, want)

	sum := sumBuckets(result.Buckets, "")
	if sum != result.Attempted {
		t.Errorf("protection counts sum to %d, want attempted=%d", sum, result.Attempted)
	}
}

func TestProtectionRate_Retest(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.ProtectionRate(ctx, scope(f.RetestID, false, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("ProtectionRate(retest): %v", err)
	}

	if result.Attempted != 6 {
		t.Errorf("Attempted = %d, want 6", result.Attempted)
	}

	want := map[string]int{
		"blocked": 0, "partial": 0, "not_blocked": 5, "n/a": 1, "unscored": 0,
	}
	checkDistribution(t, result.Buckets, want)
}

func TestProtectionRate_EmptyEngagement(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.ProtectionRate(ctx, scope("00000000-0000-0000-0000-000000000000", false, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("ProtectionRate(empty): %v", err)
	}

	if result.Attempted != 0 {
		t.Errorf("Attempted = %d, want 0", result.Attempted)
	}
	for _, b := range result.Buckets {
		if b.Count != 0 {
			t.Errorf("bucket %s = %d, want 0", b.Label, b.Count)
		}
	}
}

// ============================================================================
// OutcomeMix tests
// ============================================================================

func TestOutcomeMix_BaselineAllSeats(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.OutcomeMix(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("OutcomeMix(lead): %v", err)
	}

	if result.Attempted != 7 {
		t.Errorf("Attempted = %d, want 7", result.Attempted)
	}

	// Hand-computed: prevented=2 (T1190,T1027), detected=3 (T1566,T1070,T1059),
	//   not_detected=1 (T1203), not_applicable=0, unscored=1 (T1059.001)
	want := map[string]int{
		"prevented": 2, "detected": 3, "not_detected": 1, "not_applicable": 0, "unscored": 1,
	}
	checkDistribution(t, result.Buckets, want)

	sum := sumBuckets(result.Buckets, "")
	if sum != result.Attempted {
		t.Errorf("outcome counts sum to %d, want attempted=%d", sum, result.Attempted)
	}

	// Verify agreement with the fixture's stored expectation.
	for _, b := range result.Buckets {
		if b.Label == "unscored" {
			continue // fixture expectation groups unscored differently
		}
		if b.Count != f.BaselineOutcomeDistribution[b.Label] {
			t.Errorf("OutcomeMix %s = %d, fixture expects %d", b.Label, b.Count, f.BaselineOutcomeDistribution[b.Label])
		}
	}
}

func TestOutcomeMix_BlueSeat(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.OutcomeMix(ctx, scope(f.BaselineID, true, authz.EngagementRoleBlue))
	if err != nil {
		t.Fatalf("OutcomeMix(blue): %v", err)
	}

	if result.Attempted != 5 {
		t.Errorf("Attempted = %d, want 5", result.Attempted)
	}

	// Blue loses steps 8 (not_detected) and 9 (detected).
	want := map[string]int{
		"prevented": 2, "detected": 2, "not_detected": 0, "not_applicable": 0, "unscored": 1,
	}
	checkDistribution(t, result.Buckets, want)

	sum := sumBuckets(result.Buckets, "")
	if sum != result.Attempted {
		t.Errorf("outcome counts sum to %d, want attempted=%d", sum, result.Attempted)
	}
}

func TestOutcomeMix_Retest(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.OutcomeMix(ctx, scope(f.RetestID, false, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("OutcomeMix(retest): %v", err)
	}

	if result.Attempted != 6 {
		t.Errorf("Attempted = %d, want 6", result.Attempted)
	}

	want := map[string]int{
		"prevented": 0, "detected": 5, "not_detected": 0, "not_applicable": 1, "unscored": 0,
	}
	checkDistribution(t, result.Buckets, want)
}

func TestOutcomeMix_EmptyEngagement(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.OutcomeMix(ctx, scope("00000000-0000-0000-0000-000000000000", false, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("OutcomeMix(empty): %v", err)
	}

	if result.Attempted != 0 {
		t.Errorf("Attempted = %d, want 0", result.Attempted)
	}
	for _, b := range result.Buckets {
		if b.Count != 0 {
			t.Errorf("bucket %s = %d, want 0", b.Label, b.Count)
		}
	}
}

// ============================================================================
// Outcome drift test — SQL vs Go row-by-row
// ============================================================================

// TestOutcomeMix_AgreesWithDeriveOutcome enumerates every attempted,
// scored execution in the baseline fixture and asserts that OutcomeMix's
// SQL-derived outcome matches scoring.DeriveOutcome run row-by-row in Go.
func TestOutcomeMix_AgreesWithDeriveOutcome(t *testing.T) {
	f := analyticstest.Seed(t)
	ctx := t.Context()

	// Get the SQL outcome per row.
	rows, err := f.DB.Read().QueryContext(ctx,
		`SELECT app.execution.id, `+outcomeCase+` AS outcome
		 FROM app.execution
		 JOIN app.step ON app.step.id = app.execution.step_id
		 WHERE app.step.scenario_id IN (
		     SELECT id FROM app.scenario WHERE engagement_id = ?
		 )
		 AND (`+attemptedPredicate+`)
		 AND (TRUE)
		 ORDER BY app.execution.id`,
		f.BaselineID,
	)
	if err != nil {
		t.Fatalf("querying executions for drift test: %v", err)
	}
	defer rows.Close()

	type rowOutcome struct {
		id, outcome string
	}
	var sqlOutcomes []rowOutcome
	for rows.Next() {
		var r rowOutcome
		if err := rows.Scan(&r.id, &r.outcome); err != nil {
			t.Fatalf("scanning drift row: %v", err)
		}
		sqlOutcomes = append(sqlOutcomes, r)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterating drift rows: %v", err)
	}

	// Get Go outcome per row from the same data.
	// Use COALESCE to avoid nullable scan types.
	goRows, err := f.DB.Read().QueryContext(ctx,
		`SELECT app.execution.id,
		        COALESCE(app.execution.detection_category, '') AS detection_category,
		        COALESCE(app.execution.protection, '') AS protection
		 FROM app.execution
		 JOIN app.step ON app.step.id = app.execution.step_id
		 WHERE app.step.scenario_id IN (
		     SELECT id FROM app.scenario WHERE engagement_id = ?
		 )
		 AND (`+attemptedPredicate+`)
		 AND (TRUE)
		 ORDER BY app.execution.id`,
		f.BaselineID,
	)
	if err != nil {
		t.Fatalf("querying executions for Go derive: %v", err)
	}
	defer goRows.Close()

	var goOutcomes []rowOutcome
	for goRows.Next() {
		var id, catStr, protStr string
		if err := goRows.Scan(&id, &catStr, &protStr); err != nil {
			t.Fatalf("scanning Go derive row: %v", err)
		}
		var goCat *scoring.Category
		var goProt *scoring.Protection
		if catStr != "" {
			c := scoring.Category(catStr)
			goCat = &c
		}
		if protStr != "" {
			p := scoring.Protection(protStr)
			goProt = &p
		}
		outcome, err := scoring.DeriveOutcomePtr(goCat, goProt)
		if err != nil {
			t.Fatalf("DeriveOutcomePtr(%q, %q): %v", catStr, protStr, err)
		}
		goOutcomes = append(goOutcomes, rowOutcome{id: id, outcome: string(outcome)})
	}
	if err := goRows.Err(); err != nil {
		t.Fatalf("iterating Go derive rows: %v", err)
	}

	if len(sqlOutcomes) != len(goOutcomes) {
		t.Fatalf("row count mismatch: SQL=%d, Go=%d", len(sqlOutcomes), len(goOutcomes))
	}
	for i := range sqlOutcomes {
		if sqlOutcomes[i].id != goOutcomes[i].id {
			t.Fatalf("row order mismatch at %d: SQL id=%s, Go id=%s", i, sqlOutcomes[i].id, goOutcomes[i].id)
		}
		sqlOut := sqlOutcomes[i].outcome
		goOut := goOutcomes[i].outcome
		// SQL returns 'unscored' when either column is NULL; Go returns "".
		if goOut == "" {
			goOut = "unscored"
		}
		if sqlOut != goOut {
			t.Errorf("execution %s: SQL outcome = %q, Go DeriveOutcomePtr = %q", sqlOutcomes[i].id, sqlOut, goOut)
		}
	}
}

// ============================================================================
// ModifierDistribution tests
// ============================================================================

func TestModifierDistribution_Baseline(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.ModifierDistribution(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("ModifierDistribution(lead): %v", err)
	}

	// No modifiers in the fixture — all counts zero.
	// Scored+attempted: steps 1,2,5,6,8,9 = 6 (step 4 has NULL category).
	if result.Attempted != 6 {
		t.Errorf("Attempted = %d, want 6", result.Attempted)
	}

	for _, b := range result.Buckets {
		if b.Count != 0 {
			t.Errorf("modifier %s = %d, want 0", b.Label, b.Count)
		}
	}

	// Modifier counts must NOT sum to attempted.
	sum := sumBuckets(result.Buckets, "")
	if sum == result.Attempted {
		t.Errorf("modifier counts sum to %d, matched attempted=%d — should not be equal (modifiers are non-exclusive)", sum, result.Attempted)
	}
}

func TestModifierDistribution_BlueSeat(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.ModifierDistribution(ctx, scope(f.BaselineID, true, authz.EngagementRoleBlue))
	if err != nil {
		t.Fatalf("ModifierDistribution(blue): %v", err)
	}

	// Blue sees steps 1-7 only; scored+attempted: steps 1,2,5,6 = 4.
	if result.Attempted != 4 {
		t.Errorf("Attempted = %d, want 4", result.Attempted)
	}

	for _, b := range result.Buckets {
		if b.Count != 0 {
			t.Errorf("modifier %s = %d, want 0", b.Label, b.Count)
		}
	}
}

func TestModifierDistribution_Retest(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	result, err := q.ModifierDistribution(ctx, scope(f.RetestID, false, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("ModifierDistribution(retest): %v", err)
	}

	// All 6 retest executions are scored+attempted.
	if result.Attempted != 6 {
		t.Errorf("Attempted = %d, want 6", result.Attempted)
	}

	for _, b := range result.Buckets {
		if b.Count != 0 {
			t.Errorf("modifier %s = %d, want 0", b.Label, b.Count)
		}
	}
}

// ============================================================================
// Acceptance-criteria specific tests
// ============================================================================

// TestUnscoredNeverBecomesNone verifies that a NULL detection_category on an
// attempted execution lands in 'unscored', not 'none'.
func TestUnscoredNeverBecomesNone(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	// Step 4 (T1059.001) has complete/NULL/NULL — it should be in unscored.
	result, err := q.CategoryDistribution(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("CategoryDistribution: %v", err)
	}

	sc := bucketMap(result.Buckets)
	// Unscored must be 1 (step 4).
	if sc["unscored"] != 1 {
		t.Errorf("unscored = %d, want 1 — step 4 with NULL category must be unscored", sc["unscored"])
	}
	// None must be 1 (step 8, T1203) — NOT 2.
	if sc["none"] != 1 {
		t.Errorf("none = %d, want 1 — NULL category must not be folded into none", sc["none"])
	}
}

// TestAllBucketsPresent verifies every known category, protection, and
// modifier appears in the response even when its count is zero, using the
// scoring vocabulary so a new enum value uncaught by the test is a failure.
func TestAllBucketsPresent(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	// Category — every known category string present.
	catResult, err := q.CategoryDistribution(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("CategoryDistribution: %v", err)
	}
	catMap := bucketMap(catResult.Buckets)
	for _, s := range scoring.CategoryStrings() {
		if _, ok := catMap[s]; !ok {
			t.Errorf("CategoryDistribution missing bucket %q", s)
		}
	}
	if _, ok := catMap["unscored"]; !ok {
		t.Error("CategoryDistribution missing unscored bucket")
	}
	if _, ok := catMap["notAttempted"]; !ok {
		t.Error("CategoryDistribution missing notAttempted bucket")
	}

	// Protection — every known protection string present.
	protResult, err := q.ProtectionRate(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("ProtectionRate: %v", err)
	}
	protMap := bucketMap(protResult.Buckets)
	for _, s := range scoring.ProtectionStrings() {
		if _, ok := protMap[s]; !ok {
			t.Errorf("ProtectionRate missing bucket %q", s)
		}
	}
	if _, ok := protMap["unscored"]; !ok {
		t.Error("ProtectionRate missing unscored bucket")
	}

	// Outcome — every known outcome label present.
	outResult, err := q.OutcomeMix(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("OutcomeMix: %v", err)
	}
	outMap := bucketMap(outResult.Buckets)
	outcomeLabels := []string{"prevented", "detected", "not_detected", "not_applicable", "unscored"}
	for _, l := range outcomeLabels {
		if _, ok := outMap[l]; !ok {
			t.Errorf("OutcomeMix missing bucket %q", l)
		}
	}

	// Modifier — every known modifier present plus "other".
	modResult, err := q.ModifierDistribution(ctx, scope(f.BaselineID, true, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("ModifierDistribution: %v", err)
	}
	modMap := bucketMap(modResult.Buckets)
	for _, s := range scoring.ModifierStrings() {
		if _, ok := modMap[s]; !ok {
			t.Errorf("ModifierDistribution missing bucket %q", s)
		}
	}
	if _, ok := modMap["other"]; !ok {
		t.Error("ModifierDistribution missing other bucket")
	}
}

// TestModifierDistribution_EdgeCases inserts executions with modifier arrays
// and verifies: single modifier, all five, empty array, and that duplicates
// already collapsed on write don't double-count.
func TestModifierDistribution_EdgeCases(t *testing.T) {
	f := analyticstest.Seed(t)
	q := NewQueries(f.DB)
	ctx := t.Context()

	// Insert a test engagement with executions carrying modifiers.
	engID := "01900000-0000-7000-T000-000000000001"
	scenarioID := "01900000-0000-7000-S000-000000000010"
	userID := "01900000-0000-7000-U000-000000000001"
	ts := "2026-06-01T10:00:00Z"
	execTS := "2026-06-02T09:00:00Z"
	endTS := "2026-06-02T09:30:00Z"
	detTS := "2026-06-02T09:10:00Z"
	scoreTS := "2026-06-02T10:00:00Z"

	if err := f.DB.Write(ctx, func(tx *sql.Tx) error {
		type modExec struct {
			stepID    string
			name      string
			modifiers string
			execID    string
			ordinal   int
		}
		execs := []modExec{
			{"01900000-0000-7000-P000-000000000901", "single", `["alert"]`,
				"01900000-0000-7000-X000-000000000901", 1},
			{"01900000-0000-7000-P000-000000000902", "all five",
				`["alert","correlated","delayed","config_change","residual_artifact"]`,
				"01900000-0000-7000-X000-000000000902", 2},
			{"01900000-0000-7000-P000-000000000903", "empty", `[]`,
				"01900000-0000-7000-X000-000000000903", 3},
			{"01900000-0000-7000-P000-000000000904", "duplicate", `["alert","delayed"]`,
				"01900000-0000-7000-X000-000000000904", 4},
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app.engagement
			 (id, name, client, description, status, starts_on, ends_on,
			  attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at)
			 VALUES (?, 'Mod Test', 'Test', '', 'active', '2026-06-01', '2026-06-30',
			         '99.0', 'standard', false, ?, ?, ?)`,
			engID, userID, ts, ts,
		); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app.scenario (id, engagement_id, ordinal, name, narrative, source,
			 threat_actor, source_ref, plan_id, created_at, updated_at)
			 VALUES (?, ?, 1, 'Mod Scenario', '', 'manual', '', '', '', ?, ?)`,
			scenarioID, engID, ts, ts,
		); err != nil {
			return err
		}

		for _, e := range execs {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO app.step (id, scenario_id, ordinal, name, objective, technique_id,
				 subtechnique_id, tactic_id, "procedure", template_id, target_asset, tools,
				 controls_in_scope, attack_version, revealed_at, created_at, updated_at)
				 VALUES (?, ?, ?, ?, '', 'T1203', '', '', '{}', '', '', '[]', '[]',
				         '99.0', ?, ?, ?)`,
				e.stepID, scenarioID, e.ordinal, e.name, ts, ts, ts,
			); err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx,
				`INSERT INTO app.execution (id, step_id, version, status, executed_by,
				 started_at, ended_at, command_run, source_host, target_host, red_notes,
				 detection_category, detection_modifiers, protection,
				 detected_at, detecting_source, detecting_rule_ref,
				 alert_severity, blue_notes, scored_by, scored_at,
				 created_at, updated_at)
				 VALUES (?, ?, 1, 'complete', ?, ?, ?, '', '', '', '',
				         'general', ?, 'not_blocked',
				         ?, '', '', '', '', ?, ?,
				         ?, ?)`,
				e.execID, e.stepID, userID,
				execTS, endTS,
				e.modifiers,
				detTS,
				userID, scoreTS,
				ts, ts,
			); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("inserting mod test data: %v", err)
	}

	result, err := q.ModifierDistribution(ctx, scope(engID, false, authz.EngagementRoleLead))
	if err != nil {
		t.Fatalf("ModifierDistribution(mod test): %v", err)
	}

	// All 4 executions are scored (detection_category IS NOT NULL).
	if result.Attempted != 4 {
		t.Errorf("Attempted = %d, want 4", result.Attempted)
	}

	want := map[string]int{
		"alert":             3, // single + all_five + duplicate
		"correlated":        1, // all_five only
		"delayed":           2, // all_five + duplicate
		"config_change":     1, // all_five only
		"residual_artifact": 1, // all_five only
	}
	checkDistribution(t, result.Buckets, want)

	// Modifier counts must NOT sum to attempted.
	sum := sumBuckets(result.Buckets, "")
	if sum == result.Attempted {
		t.Errorf("modifier counts sum to %d, matched attempted=%d — should not be equal (modifiers are non-exclusive)", sum, result.Attempted)
	}
	// Verify the correct total: 3+1+2+1+1+0(other) = 8
	if sum != 8 {
		t.Errorf("modifier sum = %d, want 8", sum)
	}
}



// ============================================================================
// Helpers
// ============================================================================

func checkDistribution(t *testing.T, buckets []DistributionBucket, want map[string]int) {
	t.Helper()
	got := bucketMap(buckets)
	for label, wc := range want {
		gc, ok := got[label]
		if !ok {
			t.Errorf("missing bucket %q", label)
			continue
		}
		if gc != wc {
			t.Errorf("bucket %q = %d, want %d", label, gc, wc)
		}
	}
}

func bucketMap(buckets []DistributionBucket) map[string]int {
	m := make(map[string]int, len(buckets))
	for _, b := range buckets {
		m[b.Label] = b.Count
	}
	return m
}

// sumBuckets returns the sum of counts for all buckets except those whose
// label matches skip. An empty skip sums everything.
func sumBuckets(buckets []DistributionBucket, skip string) int {
	var sum int
	for _, b := range buckets {
		if skip != "" && b.Label == skip {
			continue
		}
		sum += b.Count
	}
	return sum
}
