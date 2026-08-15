package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// insertEngagement seeds a minimal engagement row so the RESTRICT foreign keys
// on app.report / app.report_template have a parent. The report store test
// package has no engagement repository dependency, so it writes the row
// directly.
func insertEngagement(t *testing.T, db *store.DB) string {
	t.Helper()
	const id = "01900000-0000-7000-e000-0000000000ff"
	err := db.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(context.Background(), `
			INSERT INTO app.engagement
				(id, name, client, description, status, starts_on, ends_on,
				 attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at)
			VALUES (?, ?, ?, ?, 'active', DATE '2026-01-01', DATE '2026-01-31',
			        '15.1', 'standard', false, 'admin', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			id, "Round-trip", "Client", "Description",
		)
		return err
	})
	if err != nil {
		t.Fatalf("insert engagement: %v", err)
	}
	return id
}

// compareParams is the exact non-empty params object that exposed the JSON-scan
// bug: a string field stored in a JSON column.
const compareParams = `{"baselineEngagementId":"01900000-0000-7000-e000-000000000001"}`

// TestReportBlocksRoundTripCompareParams pins the DuckDB JSON-scan bug
// (e2e thesis-report.spec.ts pre-existing bug #1): app.report_block.params is a
// JSON column, and DuckDB returns a JSON column as a parsed map, which
// database/sql cannot scan into a string. The column list now casts
// params AS VARCHAR (matching versionSelectColumns for blocks_json), so a
// block with a non-empty params object survives ReplaceBlocks → BlocksByReport.
func TestReportBlocksRoundTripCompareParams(t *testing.T) {
	db := storetest.Migrated(t)
	reports := NewReports(db)
	engID := insertEngagement(t, db)

	rep, err := reports.Create(context.Background(), NewReport{
		EngagementID: engID,
		Title:        "Round-trip",
		CreatedBy:    "admin",
	})
	if err != nil {
		t.Fatalf("create report: %v", err)
	}

	if _, err := reports.ReplaceBlocks(context.Background(), rep.ID, []NewBlock{
		{BlockID: "engagement_compare", Params: json.RawMessage(compareParams)},
	}); err != nil {
		t.Fatalf("replace blocks: %v", err)
	}

	got, err := reports.BlocksByReport(context.Background(), rep.ID)
	if err != nil {
		t.Fatalf("blocks by report: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("read back %d blocks, want 1", len(got))
	}
	if got[0].BlockID != "engagement_compare" {
		t.Errorf("block id = %q, want engagement_compare", got[0].BlockID)
	}
	if string(got[0].Params) != compareParams {
		t.Errorf("params = %s, want %s", got[0].Params, compareParams)
	}
}

// TestTemplateBlocksRoundTripParams is the same round-trip for the template
// block table, which shared the un-cast params scan (app.report_template_block
// is also a JSON column).
func TestTemplateBlocksRoundTripParams(t *testing.T) {
	db := storetest.Migrated(t)
	templates := NewTemplates(db)
	engID := insertEngagement(t, db)

	tmpl, err := templates.Create(context.Background(), NewTemplate{
		EngagementID: engID,
		Name:         "Round-trip",
		CreatedBy:    "admin",
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	if _, err := templates.ReplaceBlocks(context.Background(), tmpl.ID, []NewTemplateBlock{
		{BlockID: "rich_text", Params: json.RawMessage(`{"html":"<p>hi</p>"}`)},
	}); err != nil {
		t.Fatalf("replace template blocks: %v", err)
	}

	got, err := templates.BlocksByTemplate(context.Background(), tmpl.ID)
	if err != nil {
		t.Fatalf("blocks by template: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("read back %d template blocks, want 1", len(got))
	}
	if got[0].BlockID != "rich_text" {
		t.Errorf("block id = %q, want rich_text", got[0].BlockID)
	}
	if string(got[0].Params) != `{"html":"<p>hi</p>"}` {
		t.Errorf("params = %s", got[0].Params)
	}
}
