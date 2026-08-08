package analyticstest

import (
	"testing"
)

// TestSeedIsDeterministic verifies that the fixture seeds produce the
// expected row counts and blind/unrevealed split — so a broken fixture
// fails in its own package rather than as a confusing failure in a rollup
// test.
func TestSeedIsDeterministic(t *testing.T) {
	f := Seed(t)
	db := f.DB

	// Content counts.
	var tacticCount, techniqueCount, ttLinkCount int
	if err := db.Read().QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM content.content_tactic
		 WHERE source_id = '01900000-0000-7000-8000-000000000001' AND version = '99.0'`,
	).Scan(&tacticCount); err != nil {
		t.Fatal(err)
	}
	if tacticCount != 3 {
		t.Errorf("tactics = %d, want 3", tacticCount)
	}

	if err := db.Read().QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM content.content_technique
		 WHERE source_id = '01900000-0000-7000-8000-000000000001' AND version = '99.0'`,
	).Scan(&techniqueCount); err != nil {
		t.Fatal(err)
	}
	if techniqueCount != 10 {
		t.Errorf("techniques = %d, want 10", techniqueCount)
	}

	if err := db.Read().QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM content.content_technique_tactic
		 WHERE source_id = '01900000-0000-7000-8000-000000000001' AND version = '99.0'`,
	).Scan(&ttLinkCount); err != nil {
		t.Fatal(err)
	}
	if ttLinkCount != 11 {
		t.Errorf("technique_tactic links = %d, want 11", ttLinkCount)
	}

	// Baseline engagement exists and is blind.
	var engMode string
	if err := db.Read().QueryRowContext(t.Context(),
		`SELECT mode FROM app.engagement WHERE id = ?`, f.BaselineID,
	).Scan(&engMode); err != nil {
		t.Fatal(err)
	}
	if engMode != "blind" {
		t.Errorf("baseline mode = %q, want \"blind\"", engMode)
	}

	// Baseline step counts.
	var totalSteps, revealedSteps, unrevealedSteps int
	if err := db.Read().QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM app.step
		 WHERE scenario_id IN (SELECT id FROM app.scenario WHERE engagement_id = ?)`,
		f.BaselineID,
	).Scan(&totalSteps); err != nil {
		t.Fatal(err)
	}
	if totalSteps != 9 {
		t.Errorf("baseline steps = %d, want 9", totalSteps)
	}

	if err := db.Read().QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM app.step
		 WHERE scenario_id IN (SELECT id FROM app.scenario WHERE engagement_id = ?)
		   AND revealed_at IS NOT NULL`,
		f.BaselineID,
	).Scan(&revealedSteps); err != nil {
		t.Fatal(err)
	}
	if revealedSteps != 7 {
		t.Errorf("baseline revealed steps = %d, want 7", revealedSteps)
	}

	if err := db.Read().QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM app.step
		 WHERE scenario_id IN (SELECT id FROM app.scenario WHERE engagement_id = ?)
		   AND revealed_at IS NULL`,
		f.BaselineID,
	).Scan(&unrevealedSteps); err != nil {
		t.Fatal(err)
	}
	if unrevealedSteps != 2 {
		t.Errorf("baseline unrevealed steps = %d, want 2", unrevealedSteps)
	}

	// Execution status distribution.
	statusCounts := make(map[string]int)
	rows, err := db.Read().QueryContext(t.Context(),
		`SELECT status, COUNT(*) FROM app.execution
		 WHERE step_id IN (SELECT id FROM app.step
		                  WHERE scenario_id IN
		                    (SELECT id FROM app.scenario WHERE engagement_id = ?))
		 GROUP BY status`,
		f.BaselineID,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var s string
		var c int
		if err := rows.Scan(&s, &c); err != nil {
			t.Fatal(err)
		}
		statusCounts[s] = c
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if statusCounts["complete"] != 6 {
		t.Errorf("complete executions = %d, want 6", statusCounts["complete"])
	}
	if statusCounts["blocked"] != 1 {
		t.Errorf("blocked executions = %d, want 1", statusCounts["blocked"])
	}
	if statusCounts["skipped"] != 1 {
		t.Errorf("skipped executions = %d, want 1", statusCounts["skipped"])
	}
	if statusCounts["pending"] != 1 {
		t.Errorf("pending executions = %d, want 1", statusCounts["pending"])
	}

	// Unscored execution exists (detection_category IS NULL).
	var unscoredCount int
	if err := db.Read().QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM app.execution
		 WHERE step_id IN (SELECT id FROM app.step
		                  WHERE scenario_id IN
		                    (SELECT id FROM app.scenario WHERE engagement_id = ?))
		   AND detection_category IS NULL AND status IN ('complete', 'blocked')`,
		f.BaselineID,
	).Scan(&unscoredCount); err != nil {
		t.Fatal(err)
	}
	if unscoredCount != 1 {
		t.Errorf("unscored attempted executions = %d, want 1", unscoredCount)
	}

	// Retest engagement exists and is standard.
	if err := db.Read().QueryRowContext(t.Context(),
		`SELECT mode FROM app.engagement WHERE id = ?`, f.RetestID,
	).Scan(&engMode); err != nil {
		t.Fatal(err)
	}
	if engMode != "standard" {
		t.Errorf("retest mode = %q, want \"standard\"", engMode)
	}

	// Retest step count.
	var retestSteps int
	if err := db.Read().QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM app.step
		 WHERE scenario_id IN (SELECT id FROM app.scenario WHERE engagement_id = ?)`,
		f.RetestID,
	).Scan(&retestSteps); err != nil {
		t.Fatal(err)
	}
	if retestSteps != 6 {
		t.Errorf("retest steps = %d, want 6", retestSteps)
	}

	// Finding counts.
	var findingCount int
	if err := db.Read().QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM app.finding WHERE engagement_id = ?`, f.BaselineID,
	).Scan(&findingCount); err != nil {
		t.Fatal(err)
	}
	if findingCount != 6 {
		t.Errorf("findings = %d, want 6", findingCount)
	}

	// Expectation tables are non-empty.
	if len(f.BaselineAttempted) == 0 {
		t.Error("BaselineAttempted is empty")
	}
	if len(f.BaselineBlueAttempted) == 0 {
		t.Error("BaselineBlueAttempted is empty")
	}
	if len(f.BaselineMTTDSeconds) == 0 {
		t.Error("BaselineMTTDSeconds is empty")
	}
	if f.MatrixSize != 10 {
		t.Errorf("MatrixSize = %d, want 10", f.MatrixSize)
	}
	if len(f.AddedTechniques) == 0 {
		t.Error("AddedTechniques is empty")
	}
	if len(f.RemovedTechniques) == 0 {
		t.Error("RemovedTechniques is empty")
	}
	if len(f.CommonTechniques) == 0 {
		t.Error("CommonTechniques is empty")
	}

	// Blind blue seat sees fewer techniques.
	if len(f.BaselineBlueAttempted) >= len(f.BaselineAttempted) {
		t.Errorf("blue attempted (%d) should be fewer than all-seats (%d)",
			len(f.BaselineBlueAttempted), len(f.BaselineAttempted))
	}

	// Blue sees no not_detected (unrevealed steps 8-9 carry that outcome).
	if f.BaselineBlueOutcomeDistribution["not_detected"] != 0 {
		t.Errorf("blue should see 0 not_detected, got %d",
			f.BaselineBlueOutcomeDistribution["not_detected"])
	}
}
