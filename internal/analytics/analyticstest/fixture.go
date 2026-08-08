// Package analyticstest builds seeded databases for analytics rollup tests.
//
// The fixture is deterministic: fixed UUIDv7 values, fixed timestamps, no
// time.Now() and no random ids. A fixture that changes between runs cannot
// have hand-computed expectations, and hand-computed expectations are the
// whole point — "is this number right" cannot be answered against ad-hoc data.
//
// The fixture seeds a synthetic ATT&CK v99.0 with ~10 techniques, a baseline
// blind engagement with deliberately covered edge cases, a retest engagement,
// and findings across all four statuses. The expectation tables live on the
// returned [Fixture] struct alongside the IDs, so every rollup test's
// expected values sit beside the data that produces them.
package analyticstest

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// Fixture holds a seeded database and the hand-computed expectations
// against which every M5 rollup test asserts.
type Fixture struct {
	DB *store.DB

	// Content source version for the synthetic ATT&CK.
	SourceVersionID string

	// Engagement IDs.
	BaselineID string
	RetestID   string

	// Scenario IDs (in ordinal order).
	BaselineScenarioIDs []string
	RetestScenarioIDs   []string

	// Step IDs (in ordinal order within each scenario).
	BaselineStepIDs []string
	RetestStepIDs   []string

	// Execution IDs (1:1 with steps).
	BaselineExecIDs []string
	RetestExecIDs   []string

	// Finding IDs.
	FindingIDs []string

	// --- Expectation tables ---

	// BaselineAttempted is the set of technique external-ids that are
	// attempted (status IN complete,blocked) in the baseline, for a
	// non-blue reader (all steps visible).
	BaselineAttempted []string

	// BaselineBlueAttempted is the same, but for the blue seat of the
	// blind baseline — unrevealed steps are excluded.
	BaselineBlueAttempted []string

	// BaselineMTTDSeconds is the sorted list of MTTD values in seconds
	// for all detected executions that have both started_at and
	// detected_at in the baseline (all seats). Used for
	// percentile calculations.
	BaselineMTTDSeconds []float64
	// BaselineBlueMTTDSeconds is the same as BaselineMTTDSeconds but for
	// the blue seat of the blind baseline — unrevealed steps are excluded.
	BaselineBlueMTTDSeconds []float64


	// BaselineOutcomeDistribution maps outcome label → count for the
	// baseline (all seats, attempted executions only).
	BaselineOutcomeDistribution map[string]int

	// BaselineBlueOutcomeDistribution is the same for the blue seat.
	BaselineBlueOutcomeDistribution map[string]int

	// MatrixSize is the total number of techniques in the synthetic
	// ATT&CK v99.0 matrix (10).
	MatrixSize int

	// TacticCoverageCounts maps tactic external-id → count of distinct
	// attempted techniques in that tactic (baseline, all seats).
	TacticCoverageCounts map[string]int

	// FindingStatusCounts maps finding status → count.
	FindingStatusCounts map[string]int

	// --- Cross-engagement comparison ---

	// AddedTechniques are technique external-ids present in retest
	// but not in baseline (all seats).
	AddedTechniques []string

	// RemovedTechniques are technique external-ids present in baseline
	// but not in retest (all seats).
	RemovedTechniques []string

	// CommonTechniques are technique external-ids present in both
	// engagements (all seats).
	CommonTechniques []string
}

// ---------------------------------------------------------------------------
// Fixed identifiers — UUIDv7, deterministic
// ---------------------------------------------------------------------------

// Content rows.
const (
	contentSourceVersionID = "01900000-0000-7000-A000-000000000001"

	tacticTA0001 = "01900000-0000-7000-A000-000000000101"
	tacticTA0002 = "01900000-0000-7000-A000-000000000102"
	tacticTA0005 = "01900000-0000-7000-A000-000000000103"

	techT1059     = "01900000-0000-7000-A000-000000000201"
	techT1059_001 = "01900000-0000-7000-A000-000000000202"
	techT1059_003 = "01900000-0000-7000-A000-000000000203"
	techT1566     = "01900000-0000-7000-A000-000000000204"
	techT1566_001 = "01900000-0000-7000-A000-000000000205"
	techT1190     = "01900000-0000-7000-A000-000000000206"
	techT1203     = "01900000-0000-7000-A000-000000000207"
	techT1027     = "01900000-0000-7000-A000-000000000208"
	techT1070     = "01900000-0000-7000-A000-000000000209"
	techT1547     = "01900000-0000-7000-A000-00000000020A"
)

// Engagement-level rows.
const (
	userID        = "01900000-0000-7000-U000-000000000001"
	baselineEngID = "01900000-0000-7000-E000-000000000001"
	retestEngID   = "01900000-0000-7000-E000-000000000002"

	baselineScenario1ID = "01900000-0000-7000-S000-000000000001"
	baselineScenario2ID = "01900000-0000-7000-S000-000000000002"
	retestScenario1ID   = "01900000-0000-7000-S000-000000000003"

	baselineStep1ID = "01900000-0000-7000-P000-000000000001"
	baselineStep2ID = "01900000-0000-7000-P000-000000000002"
	baselineStep3ID = "01900000-0000-7000-P000-000000000003"
	baselineStep4ID = "01900000-0000-7000-P000-000000000004"
	baselineStep5ID = "01900000-0000-7000-P000-000000000005"
	baselineStep6ID = "01900000-0000-7000-P000-000000000006"
	baselineStep7ID = "01900000-0000-7000-P000-000000000007"
	baselineStep8ID = "01900000-0000-7000-P000-000000000008"
	baselineStep9ID = "01900000-0000-7000-P000-000000000009"

	baselineExec1ID = "01900000-0000-7000-X000-000000000001"
	baselineExec2ID = "01900000-0000-7000-X000-000000000002"
	baselineExec3ID = "01900000-0000-7000-X000-000000000003"
	baselineExec4ID = "01900000-0000-7000-X000-000000000004"
	baselineExec5ID = "01900000-0000-7000-X000-000000000005"
	baselineExec6ID = "01900000-0000-7000-X000-000000000006"
	baselineExec7ID = "01900000-0000-7000-X000-000000000007"
	baselineExec8ID = "01900000-0000-7000-X000-000000000008"
	baselineExec9ID = "01900000-0000-7000-X000-000000000009"

	retestStep1ID = "01900000-0000-7000-P000-000000000101"
	retestStep2ID = "01900000-0000-7000-P000-000000000102"
	retestStep3ID = "01900000-0000-7000-P000-000000000103"
	retestStep4ID = "01900000-0000-7000-P000-000000000104"
	retestStep5ID = "01900000-0000-7000-P000-000000000105"
	retestStep6ID = "01900000-0000-7000-P000-000000000106"

	retestExec1ID = "01900000-0000-7000-X000-000000000101"
	retestExec2ID = "01900000-0000-7000-X000-000000000102"
	retestExec3ID = "01900000-0000-7000-X000-000000000103"
	retestExec4ID = "01900000-0000-7000-X000-000000000104"
	retestExec5ID = "01900000-0000-7000-X000-000000000105"
	retestExec6ID = "01900000-0000-7000-X000-000000000106"

	finding1ID = "01900000-0000-7000-F000-000000000001"
	finding2ID = "01900000-0000-7000-F000-000000000002"
	finding3ID = "01900000-0000-7000-F000-000000000003"
	finding4ID = "01900000-0000-7000-F000-000000000004"

	findingHistory1ID = "01900000-0000-7000-H000-000000000001"
	findingHistory2ID = "01900000-0000-7000-H000-000000000002"
	findingHistory3ID = "01900000-0000-7000-H000-000000000003"
	findingHistory4ID = "01900000-0000-7000-H000-000000000004"
)

// Fixed timestamps — deterministic, UTC, microsecond precision.
var (
	// ts is a shorthand for creating deterministic timestamps.
	ts = func(y int, month time.Month, d, h, m, s int) time.Time {
		return time.Date(y, month, d, h, m, s, 0, time.UTC)
	}
	epoch = ts(2026, 1, 1, 0, 0, 0)

	// Engagement dates.
	baseCreated  = ts(2026, 6, 1, 10, 0, 0)
	baseStartsOn = ts(2026, 6, 1, 0, 0, 0) // date-only in practice
	baseEndsOn   = ts(2026, 6, 30, 0, 0, 0)

	// Step/execution created at.
	stepCreated = ts(2026, 6, 1, 11, 0, 0)

	// Execution red timestamps.
	execStarted = ts(2026, 6, 2, 9, 0, 0)  // 09:00
	execEnded   = ts(2026, 6, 2, 9, 30, 0) // 09:30

	// Detection timestamps for different MTTD values.
	detected10min = ts(2026, 6, 2, 9, 10, 0) // started+10min
	detected5min  = ts(2026, 6, 2, 9, 5, 0)  // started+5min
	detected3min  = ts(2026, 6, 2, 9, 3, 0)  // started+3min

	// Scoring timestamp.
	scoredAt = ts(2026, 6, 2, 10, 0, 0)
)

// Seed builds a [storetest.Migrated] database and populates it with the
// synthetic ATT&CK v99.0 matrix, a baseline blind engagement, a retest
// engagement, and findings. It returns the [Fixture] with all IDs and
// hand-computed expectation tables.
func Seed(t testing.TB) Fixture {
	t.Helper()

	db := storetest.Migrated(t)
	ctx := context.Background()

	if err := db.Write(ctx, func(tx *sql.Tx) error {
		if err := ensureSchema(ctx, tx); err != nil {
			return err
		}
		if err := seedContent(ctx, tx); err != nil {
			return err
		}
		if err := seedBaseline(ctx, tx); err != nil {
			return err
		}
		if err := seedRetest(ctx, tx); err != nil {
			return err
		}
		if err := seedFindings(ctx, tx); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatalf("analyticstest.Seed: %v", err)
	}

	return Fixture{
		DB:                  db,
		SourceVersionID:     contentSourceVersionID,
		BaselineID:          baselineEngID,
		RetestID:            retestEngID,
		BaselineScenarioIDs: []string{baselineScenario1ID, baselineScenario2ID},
		RetestScenarioIDs:   []string{retestScenario1ID},
		BaselineStepIDs: []string{
			baselineStep1ID, baselineStep2ID, baselineStep3ID, baselineStep4ID,
			baselineStep5ID, baselineStep6ID, baselineStep7ID, baselineStep8ID,
			baselineStep9ID,
		},
		BaselineExecIDs: []string{
			baselineExec1ID, baselineExec2ID, baselineExec3ID, baselineExec4ID,
			baselineExec5ID, baselineExec6ID, baselineExec7ID, baselineExec8ID,
			baselineExec9ID,
		},
		RetestStepIDs: []string{
			retestStep1ID, retestStep2ID, retestStep3ID, retestStep4ID,
			retestStep5ID, retestStep6ID,
		},
		RetestExecIDs: []string{
			retestExec1ID, retestExec2ID, retestExec3ID, retestExec4ID,
			retestExec5ID, retestExec6ID,
		},
		FindingIDs: []string{finding1ID, finding2ID, finding3ID, finding4ID},

		// --- Hand-computed expectations ---

		// Baseline attempted techniques (all seats):
		//   Step 1 T1190 (complete), Step 2 T1566 (complete),
		//   Step 4 T1059.001 (complete), Step 5 T1027 (blocked),
		//   Step 6 T1070 (complete), Step 8 T1203 (complete),
		//   Step 9 T1059 (complete)
		//   = 7 distinct techniques.
		BaselineAttempted: []string{
			"T1059", "T1059.001", "T1566", "T1190", "T1203", "T1027", "T1070",
		},
		// Blue seat of blind baseline (steps 8+9 unrevealed):
		//   Visible attempted: T1190, T1566, T1059.001, T1027, T1070 = 5.
		BaselineBlueAttempted: []string{
			"T1059.001", "T1566", "T1190", "T1027", "T1070",
		},

		// MTTD values (seconds) for detected executions with both timestamps
		// (M5-006 definition: category NOT 'none', both timestamps set):
		//   Step 1: 10min = 600s
		//   Step 2: 5min = 300s
		//   Step 5: 3min = 180s
		//   Step 6: no started_at → excluded (unmeasurable)
		//   Step 8: category 'none' → excluded (undetected)
		//   Step 9: 30min = 1800s
		// Sorted: 180, 300, 600, 1800
		BaselineMTTDSeconds: []float64{180, 300, 600, 1800},

		// Blue seat (steps 8+9 unrevealed, both excluded):
		//   Remaining detected: Step 1(600s), Step 2(300s), Step 5(180s)
		// Sorted: 180, 300, 600
		BaselineBlueMTTDSeconds: []float64{180, 300, 600},


		// Baseline outcome distribution (attempted, all seats):
		//   Step 1: general/blocked → prevented
		//   Step 2: telemetry/not_blocked → detected
		//   Step 4: unscored → unscored
		//   Step 5: general/partial → prevented
		//   Step 6: technique/not_blocked → detected
		//   Step 8: none/not_blocked → not_detected
		//   Step 9: tactic/not_blocked → detected
		BaselineOutcomeDistribution: map[string]int{
			"prevented":    2, // steps 1, 5
			"detected":     3, // steps 2, 6, 9
			"not_detected": 1, // step 8
			"unscored":     1, // step 4
		},

		// Blue seat (steps 8+9 unrevealed):
		//   Visible attempted: steps 1,2,4,5,6
		//   prevented: 2 (steps 1,5)
		//   detected: 2 (steps 2,6)
		//   unscored: 1 (step 4)
		BaselineBlueOutcomeDistribution: map[string]int{
			"prevented": 2,
			"detected":  2,
			"unscored":  1,
		},

		MatrixSize: 10,

		// Tactic coverage (all seats, attempted only):
		//   TA0001: T1190, T1566 = 2
		//   TA0002: T1059, T1059.001, T1203 = 3
		//     (T1547 is not attempted — pending)
		//   TA0005: T1059, T1027, T1070 = 3
		// Note T1059 in both TA0002 and TA0005.
		TacticCoverageCounts: map[string]int{
			"TA0001": 2,
			"TA0002": 3,
			"TA0005": 3,
		},

		// Finding status distribution.
		FindingStatusCounts: map[string]int{
			"open":          1,
			"in_progress":   1,
			"resolved":      1,
			"accepted_risk": 1,
		},

		// Cross-engagement compare (all seats).
		//   Baseline attempted: T1059, T1059.001, T1027, T1070, T1190, T1203, T1566
		//   Retest attempted:   T1059, T1059.001, T1059.003, T1070, T1190, T1566
		//   Added: T1059.003 (retest only)
		//   Removed: T1027, T1203 (baseline only)
		//   Common: T1059, T1059.001, T1070, T1190, T1566
		AddedTechniques:   []string{"T1059.003"},
		RemovedTechniques: []string{"T1027", "T1203"},
		CommonTechniques:  []string{"T1059", "T1059.001", "T1070", "T1190", "T1566"},
	}
}

// ensureSchema creates tables that may not exist yet because their migrations
// are in later tickets. Each statement uses IF NOT EXISTS so that once the
// migration ships, this is a no-op.
func ensureSchema(ctx context.Context, tx *sql.Tx) error {
	// M5-003 has shipped — app.finding_status_history is now created by
	// migration 0017. No temporary tables remain to create.
	return nil
}

// Seed helpers
// ---------------------------------------------------------------------------

func seedContent(ctx context.Context, tx *sql.Tx) error {
	// Source version for the synthetic ATT&CK.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO content.content_source_version
			(id, source_id, version, status, item_count, synced_at, error,
			 raw_sha256, raw_path, raw_bytes, created_at, updated_at)
		 VALUES (?, '01900000-0000-7000-8000-000000000001', '99.0', 'ready', 10, ?, '',
		        '', '', 0, ?, ?)`,
		contentSourceVersionID, epoch, epoch, epoch,
	); err != nil {
		return fmt.Errorf("seed content_source_version: %w", err)
	}

	// Tactics.
	tactics := []struct{ id, externalID, name string }{
		{tacticTA0001, "TA0001", "Initial Access"},
		{tacticTA0002, "TA0002", "Execution"},
		{tacticTA0005, "TA0005", "Defense Evasion"},
	}
	for _, tc := range tactics {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO content.content_tactic
				(id, source_id, version, external_id, name, description, created_at, updated_at)
			 VALUES (?, '01900000-0000-7000-8000-000000000001', '99.0', ?, ?, '', ?, ?)`,
			tc.id, tc.externalID, tc.name, epoch, epoch,
		); err != nil {
			return fmt.Errorf("seed tactic %s: %w", tc.externalID, err)
		}
	}

	// Techniques.
	techniques := []struct {
		id, externalID, name, parentID string
		isSub                          bool
	}{
		{techT1059, "T1059", "Command and Scripting Interpreter", "", false},
		{techT1059_001, "T1059.001", "PowerShell", "T1059", true},
		{techT1059_003, "T1059.003", "Windows Command Shell", "T1059", true},
		{techT1566, "T1566", "Phishing", "", false},
		{techT1566_001, "T1566.001", "Spearphishing Attachment", "T1566", true},
		{techT1190, "T1190", "Exploit Public-Facing Application", "", false},
		{techT1203, "T1203", "Exploitation for Client Execution", "", false},
		{techT1027, "T1027", "Obfuscated Files or Information", "", false},
		{techT1070, "T1070", "Indicator Removal", "", false},
		{techT1547, "T1547", "Boot or Logon Autostart Execution", "", false},
	}
	for _, te := range techniques {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO content.content_technique
				(id, source_id, version, external_id, name, description,
				 is_subtechnique, parent_external_id, created_at, updated_at)
			 VALUES (?, '01900000-0000-7000-8000-000000000001', '99.0', ?, ?, '',
			        ?, ?, ?, ?)`,
			te.id, te.externalID, te.name, te.isSub, te.parentID, epoch, epoch,
		); err != nil {
			return fmt.Errorf("seed technique %s: %w", te.externalID, err)
		}
	}

	// Technique ↔ tactic links.
	// T1059 appears in TA0002 AND TA0005 — the double-count case.
	links := []struct{ tech, tactic string }{
		{"T1059", "TA0002"},
		{"T1059", "TA0005"},
		{"T1059.001", "TA0002"},
		{"T1059.003", "TA0002"},
		{"T1566", "TA0001"},
		{"T1566.001", "TA0001"},
		{"T1190", "TA0001"},
		{"T1203", "TA0002"},
		{"T1027", "TA0005"},
		{"T1070", "TA0005"},
		{"T1547", "TA0002"},
	}
	for _, l := range links {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO content.content_technique_tactic
				(source_id, version, technique_external_id, tactic_external_id)
			 VALUES ('01900000-0000-7000-8000-000000000001', '99.0', ?, ?)`,
			l.tech, l.tactic,
		); err != nil {
			return fmt.Errorf("seed tech_tactic %s→%s: %w", l.tech, l.tactic, err)
		}
	}

	return nil
}

func seedBaseline(ctx context.Context, tx *sql.Tx) error {
	// Engagement — blind mode.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO app.engagement
			(id, name, client, description, status, starts_on, ends_on,
			 attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at)
		 VALUES (?, 'Baseline Assessment', 'TestOrg', 'Synthetic baseline for analytics tests.',
		        'active', ?, ?, '99.0', 'blind', false, ?, ?, ?)`,
		baselineEngID, baseStartsOn, baseEndsOn, userID, baseCreated, baseCreated,
	); err != nil {
		return fmt.Errorf("seed baseline engagement: %w", err)
	}

	// Scenarios.
	for i, sc := range []struct{ id, name string }{
		{baselineScenario1ID, "Initial Access & Execution"},
		{baselineScenario2ID, "Defense Evasion & Persistence"},
	} {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app.scenario
				(id, engagement_id, ordinal, name, narrative, source,
				 threat_actor, source_ref, plan_id, created_at, updated_at)
			 VALUES (?, ?, ?, ?, '', 'manual', '', '', '', ?, ?)`,
			sc.id, baselineEngID, i+1, sc.name, stepCreated, stepCreated,
		); err != nil {
			return fmt.Errorf("seed baseline scenario %d: %w", i+1, err)
		}
	}

	// Steps and executions — see fixture design in doc comment.
	type stepRow struct {
		stepID, scenarioID, techniqueID, subtechniqueID string
		ordinal                                         int
		name                                            string
		revealed                                        bool // revealed_at IS NOT NULL?
		status                                          string
		detectionCat                                    sql.NullString
		protection                                      sql.NullString
		startedAt                                       sql.NullTime
		endedAt                                         sql.NullTime
		detectedAt                                      sql.NullTime
	}
	steps := []stepRow{
		// Scenario 1
		{baselineStep1ID, baselineScenario1ID, "T1190", "", 1,
			"Exploit Public-Facing App", true,
			"complete", ns("general"), ns("blocked"), nt(execStarted), nt(execEnded), nt(detected10min)},
		{baselineStep2ID, baselineScenario1ID, "T1566", "", 2,
			"Phishing campaign", true,
			"complete", ns("telemetry"), ns("not_blocked"), nt(execStarted), nt(execEnded), nt(detected5min)},
		{baselineStep3ID, baselineScenario1ID, "T1566.001", "", 3,
			"Spearphish the CFO", true,
			"skipped", ns(""), ns(""), nt(time.Time{}), nt(time.Time{}), nt(time.Time{})},
		{baselineStep4ID, baselineScenario1ID, "T1059.001", "", 4,
			"PowerShell reverse shell", true,
			"complete", ns(""), ns(""), nt(execStarted), nt(execEnded), nt(time.Time{})},
		// Scenario 2
		{baselineStep5ID, baselineScenario2ID, "T1027", "", 1,
			"Obfuscate payload", true,
			"blocked", ns("general"), ns("partial"), nt(execStarted), nt(execEnded), nt(detected3min)},
		{baselineStep6ID, baselineScenario2ID, "T1070", "", 2,
			"Clear Windows event logs", true,
			"complete", ns("technique"), ns("not_blocked"), nt(time.Time{}), nt(execEnded), nt(execEnded)},
		{baselineStep7ID, baselineScenario2ID, "T1547", "", 3,
			"Registry Run key persistence", true,
			"pending", ns(""), ns(""), nt(time.Time{}), nt(time.Time{}), nt(time.Time{})},
		// Unrevealed steps
		{baselineStep8ID, baselineScenario2ID, "T1203", "", 4,
			"Browser exploit chain", false,
			"complete", ns("none"), ns("not_blocked"), nt(execStarted), nt(execEnded), nt(execEnded)},
		{baselineStep9ID, baselineScenario2ID, "T1059", "", 5,
			"Python keylogger", false,
			"complete", ns("tactic"), ns("not_blocked"), nt(execStarted), nt(execEnded), nt(execEnded)},
	}

	for _, s := range steps {
		revealedAt := sql.NullTime{Valid: s.revealed, Time: stepCreated}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app.step
				(id, scenario_id, ordinal, name, objective, technique_id, subtechnique_id,
				 tactic_id, "procedure", template_id, target_asset, tools, controls_in_scope,
				 attack_version, revealed_at, created_at, updated_at)
			 VALUES (?, ?, ?, ?, '', ?, ?, '', '{}', '', '', '[]', '[]',
			        '99.0', ?, ?, ?)`,
			s.stepID, s.scenarioID, s.ordinal, s.name,
			s.techniqueID, nullIfEmpty(s.subtechniqueID),
			nullIfTime(revealedAt), stepCreated, stepCreated,
		); err != nil {
			return fmt.Errorf("seed baseline step %s: %w", s.name, err)
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
			        ?, ?)`,
			execID(s.stepID), s.stepID,
			s.status, userID,
			nullIfTime(s.startedAt), nullIfTime(s.endedAt),
			nullIfNullString(s.detectionCat), nullIfNullString(s.protection),
			nullIfTime(s.detectedAt),
			userID, nullIfTime(nt(scoredAt)),
			stepCreated, stepCreated,
		); err != nil {
			return fmt.Errorf("seed baseline execution for %s: %w", s.name, err)
		}
	}

	return nil
}

func seedRetest(ctx context.Context, tx *sql.Tx) error {
	// Engagement — standard mode (not blind).
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO app.engagement
			(id, name, client, description, status, starts_on, ends_on,
			 attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at)
		 VALUES (?, 'Retest Assessment', 'TestOrg', 'Synthetic retest for analytics tests.',
		        'active', ?, ?, '99.0', 'standard', false, ?, ?, ?)`,
		retestEngID, baseStartsOn, baseEndsOn, userID, baseCreated, baseCreated,
	); err != nil {
		return fmt.Errorf("seed retest engagement: %w", err)
	}

	// One scenario.
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO app.scenario
			(id, engagement_id, ordinal, name, narrative, source,
			 threat_actor, source_ref, plan_id, created_at, updated_at)
		 VALUES (?, ?, 1, 'Retest Run', '', 'manual', '', '', '', ?, ?)`,
		retestScenario1ID, retestEngID, stepCreated, stepCreated,
	); err != nil {
		return fmt.Errorf("seed retest scenario: %w", err)
	}

	type stepRow struct {
		stepID                         string
		techniqueID, subtechniqueID    string
		ordinal                        int
		name                           string
		status                         string
		detectionCat, protection       sql.NullString
		startedAt, endedAt, detectedAt sql.NullTime
	}
	// Retest: overlaps baseline on T1190, T1566, T1059.001, T1070, T1059.
	// Adds T1059.003 (new). Drops T1027 and T1203 (baseline only).
	// Better scores where comparable.
	steps := []stepRow{
		{retestStep1ID, "T1190", "", 1, "Exploit Public-Facing App (retest)",
			"complete", ns("technique"), ns("not_blocked"),
			nt(execStarted), nt(execEnded), nt(detected10min)},
		{retestStep2ID, "T1566", "", 2, "Phishing campaign (retest)",
			"complete", ns("technique"), ns("not_blocked"),
			nt(execStarted), nt(execEnded), nt(detected5min)},
		{retestStep3ID, "T1059.001", "", 3, "PowerShell C2 (retest)",
			"complete", ns("general"), ns("not_blocked"),
			nt(execStarted), nt(execEnded), nt(detected3min)},
		{retestStep4ID, "T1070", "", 4, "Clear event logs (retest)",
			"complete", ns("technique"), ns("n/a"),
			nt(execStarted), nt(execEnded), nt(detected5min)},
		{retestStep5ID, "T1059.003", "", 5, "Windows Command Shell payload",
			"complete", ns("tactic"), ns("not_blocked"),
			nt(execStarted), nt(execEnded), nt(detected3min)},
		{retestStep6ID, "T1059", "", 6, "Python keylogger (retest)",
			"complete", ns("technique"), ns("not_blocked"),
			nt(execStarted), nt(execEnded), nt(detected10min)},
	}

	for _, s := range steps {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app.step
				(id, scenario_id, ordinal, name, objective, technique_id, subtechnique_id,
				 tactic_id, "procedure", template_id, target_asset, tools, controls_in_scope,
				 attack_version, revealed_at, created_at, updated_at)
			 VALUES (?, ?, ?, ?, '', ?, '', '', '{}', '', '', '[]', '[]',
			        '99.0', ?, ?, ?)`,
			s.stepID, retestScenario1ID, s.ordinal, s.name,
			s.techniqueID, nullIfEmpty(s.subtechniqueID),
			stepCreated, // revealed_at = stepCreated (not blind, so all revealed)
			stepCreated, stepCreated,
		); err != nil {
			return fmt.Errorf("seed retest step %s: %w", s.name, err)
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
			        ?, ?)`,
			execID(s.stepID), s.stepID,
			s.status, userID,
			nullIfTime(s.startedAt), nullIfTime(s.endedAt),
			nullIfNullString(s.detectionCat), nullIfNullString(s.protection),
			nullIfTime(s.detectedAt),
			userID, nt(scoredAt),
			stepCreated, stepCreated,
		); err != nil {
			return fmt.Errorf("seed retest execution for %s: %w", s.name, err)
		}
	}

	return nil
}

func seedFindings(ctx context.Context, tx *sql.Tx) error {
	type finding struct {
		id, status, createdFromExec string
	}
	findings := []finding{
		{finding1ID, "open", baselineExec1ID},
		{finding2ID, "in_progress", baselineExec5ID},
		{finding3ID, "resolved", baselineExec8ID},
		{finding4ID, "accepted_risk", ""}, // freeform, not from an execution
	}
	for _, f := range findings {
		cfExec := nullIfEmpty(f.createdFromExec)
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app.finding
				(id, engagement_id, title, description, severity, recommendation,
				 "owner", status, created_from_execution, created_at, updated_at)
			 VALUES (?, ?, 'Finding ' || ?, '', 'medium', '',
			        ?, ?, ?, ?, ?)`,
			f.id, baselineEngID, f.id, userID, f.status, cfExec, stepCreated, stepCreated,
		); err != nil {
			return fmt.Errorf("seed finding %s: %w", f.id, err)
		}
	}

	// Finding-step links.
	links := []struct{ findingID, stepID string }{
		{finding1ID, baselineStep1ID},
		{finding2ID, baselineStep5ID},
		{finding3ID, baselineStep8ID},
	}
	for _, l := range links {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO app.finding_step (finding_id, step_id) VALUES (?, ?)`,
			l.findingID, l.stepID,
		); err != nil {
			return fmt.Errorf("seed finding_step %s→%s: %w", l.findingID, l.stepID, err)
		}
	}

	// Status history — one transition per finding (the initial creation).
	// M5-003 will expand this; for now, record initial status + one transition
	// for findings that have changed status.
		historyRows := []struct {
			id, findingID, fromStatus, toStatus string
			ts                                  time.Time
		}{
			// Creation rows: fromStatus empty → NULL.
			{findingHistory1ID, finding1ID, "", "open", stepCreated},
			{findingHistory2ID, finding2ID, "", "open", stepCreated},
			// Transition: finding2 went from open to in_progress.
			{findingHistory3ID, finding2ID, "open", "in_progress", ts(2026, 6, 3, 10, 0, 0)},
			// Creation row for finding3.
			{findingHistory4ID, finding3ID, "", "open", stepCreated},
		}
		for _, h := range historyRows {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO app.finding_status_history
					(id, finding_id, engagement_id, from_status, to_status, changed_by, changed_at)
				 VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?)`,
				h.id, h.findingID, baselineEngID, h.fromStatus, h.toStatus, userID, h.ts,
			); err != nil {
				return fmt.Errorf("seed finding_status_history %s: %w", h.id, err)
			}
		}

	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func ns(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nt(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullIfNullString(ns sql.NullString) any {
	if !ns.Valid {
		return nil
	}
	return ns.String
}

func nullIfTime(nt sql.NullTime) any {
	if !nt.Valid {
		return nil
	}
	return nt.Time
}

// execID derives the execution id from the step id — they're deterministic
// pairs, with the -P- → -X- swap at position 19.
func execID(stepID string) string {
	return stepID[:19] + "X" + stepID[20:]
	// step IDs: …-P000-…, exec IDs: …-X000-…
}
