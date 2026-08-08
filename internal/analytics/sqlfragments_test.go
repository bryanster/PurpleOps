package analytics

import (
	"testing"

	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
)

// TestSQLFragmentsCompile verifies that the named SQL fragments are valid
// against the real schema. They are referenced here so the linter does not
// complain about unused constants before M5-004 ships.
func TestSQLFragmentsCompile(t *testing.T) {
	f := analyticstest.Seed(t)

	// attemptedPredicate must be valid SQL.
	var count int
	if err := f.DB.Read().QueryRowContext(t.Context(),
		`SELECT COUNT(*) FROM app.execution WHERE `+attemptedPredicate,
	).Scan(&count); err != nil {
		t.Fatalf("attemptedPredicate: %v", err)
	}
	if count == 0 {
		t.Error("attemptedPredicate matched nothing — fixture has no attempted executions?")
	}

	// outcomeCase must be valid SQL and agree with the fixture.
	if err := f.DB.Read().QueryRowContext(t.Context(),
		`SELECT `+outcomeCase+`
		 FROM app.execution
		 WHERE step_id IN (SELECT id FROM app.step
		                   WHERE scenario_id IN
		                     (SELECT id FROM app.scenario WHERE engagement_id = ?))
		 LIMIT 1`,
		f.BaselineID,
	).Scan(new(string)); err != nil {
		t.Fatalf("outcomeCase: %v", err)
	}
}
