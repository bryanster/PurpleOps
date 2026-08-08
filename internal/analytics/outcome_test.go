package analytics

import (
	"context"
	"testing"

	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
	"github.com/bryanster/blacklight/internal/domain/scoring"
)

// TestOutcomeSQLMatchesGo enumerates every category × protection pair and
// asserts that outcomeCase produces the same answer as
// scoring.DeriveOutcome — including the nil cases.
func TestOutcomeSQLMatchesGo(t *testing.T) {
	f := analyticstest.Seed(t)
	ctx := context.Background()

	// Enumerate every known category × protection pair.
	for _, cat := range scoring.AllCategories() {
		for _, prot := range scoring.AllProtections() {
			goOutcome, err := scoring.DeriveOutcome(cat, prot)
			if err != nil {
				t.Fatalf("DeriveOutcome(%q, %q): %v", cat, prot, err)
			}

			var sqlOutcome string
			if err := f.DB.Read().QueryRowContext(ctx,
				`SELECT `+outcomeCase+`
				 FROM (SELECT ?::TEXT AS detection_category,
				              ?::TEXT AS protection) AS execution`,
				string(cat), string(prot),
			).Scan(&sqlOutcome); err != nil {
				t.Fatalf("SQL outcome(%q, %q): %v", cat, prot, err)
			}

			if sqlOutcome != string(goOutcome) {
				t.Errorf("outcome(%q, %q): SQL=%q, Go=%q",
					cat, prot, sqlOutcome, goOutcome)
			}
		}
	}

	// Nil cases: both NULL → "unscored" in SQL, empty string in Go.
	nilPairs := []struct {
		catNull, protNull bool
		label             string
	}{
		{true, true, "both nil"},
		{true, false, "cat nil"},
		{false, true, "prot nil"},
	}
	for _, np := range nilPairs {
		var sqlOutcome string
		query := `SELECT ` + outcomeCase + `
		          FROM (SELECT NULL::TEXT AS detection_category,
		                       NULL::TEXT AS protection) AS execution`
		if err := f.DB.Read().QueryRowContext(ctx, query).Scan(&sqlOutcome); err != nil {
			t.Fatalf("SQL outcome(%s): %v", np.label, err)
		}
		if sqlOutcome != "unscored" {
			t.Errorf("SQL outcome(%s) = %q, want \"unscored\"", np.label, sqlOutcome)
		}

		var catPtr *scoring.Category
		var protPtr *scoring.Protection
		if !np.catNull {
			c := scoring.CategoryTechnique
			catPtr = &c
		}
		if !np.protNull {
			p := scoring.ProtectionNotBlocked
			protPtr = &p
		}
		goOutcome, err := scoring.DeriveOutcomePtr(catPtr, protPtr)
		if err != nil {
			t.Fatalf("DeriveOutcomePtr(%s): %v", np.label, err)
		}
		if goOutcome != "" {
			t.Errorf("DeriveOutcomePtr(%s) = %q, want \"\"", np.label, goOutcome)
		}
	}
}

// TestOutcomeSQLEnumerationComplete verifies that every known category and
// protection appears in at least one execution row of the fixture, so the
// SQL CASE branches are all exercised against real data.
func TestOutcomeSQLEnumerationComplete(t *testing.T) {
	f := analyticstest.Seed(t)
	ctx := context.Background()

	for _, cat := range scoring.AllCategories() {
		var count int
		if err := f.DB.Read().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM app.execution WHERE detection_category = ?`,
			string(cat),
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			t.Errorf("no execution with detection_category = %q in fixture", cat)
		}
	}

	for _, prot := range scoring.AllProtections() {
		var count int
		if err := f.DB.Read().QueryRowContext(ctx,
			`SELECT COUNT(*) FROM app.execution WHERE protection = ?`,
			string(prot),
		).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			t.Errorf("no execution with protection = %q in fixture", prot)
		}
	}
}
