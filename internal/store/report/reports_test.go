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

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

// TestDeleteReportWithBlocks pins the bug behind the Delete button in the
// reports list: DuckDB checks a RESTRICT foreign key against the child's
// index, which does not see the current transaction's own deletes, so removing
// app.report_block and app.report in one transaction failed with "still
// referenced by a foreign key" on every report that had a block — i.e. every
// report anybody had actually worked on.
func TestDeleteReportWithBlocks(t *testing.T) {
	db := storetest.Migrated(t)
	reports := NewReports(db)
	engID := insertEngagement(t, db)
	ctx := context.Background()

	rep, err := reports.Create(ctx, NewReport{
		EngagementID: engID,
		Title:        "Has blocks",
		CreatedBy:    "admin",
	})
	if err != nil {
		t.Fatalf("create report: %v", err)
	}
	if _, err := reports.ReplaceBlocks(ctx, rep.ID, []NewBlock{
		{BlockID: "cover", Params: json.RawMessage(`{}`)},
		{BlockID: "rich_text", Params: json.RawMessage(`{"html":"<p>hi</p>"}`)},
	}); err != nil {
		t.Fatalf("replace blocks: %v", err)
	}

	if err := reports.Delete(ctx, rep.ID); err != nil {
		t.Fatalf("delete report: %v", err)
	}

	if _, err := reports.ByID(ctx, rep.ID); err == nil {
		t.Fatal("report still readable after delete")
	}
	blocks, err := reports.BlocksByReport(ctx, rep.ID)
	if err != nil {
		t.Fatalf("blocks by report: %v", err)
	}
	if len(blocks) != 0 {
		t.Errorf("%d blocks survived the delete, want 0", len(blocks))
	}
}

// TestDeleteReportWithVersionShareAndGrant walks the rest of the graph: a
// published version, a share issued against it, and a grant claimed on that
// share all reference the report transitively, and each level is its own
// RESTRICT parent.
func TestDeleteReportWithVersionShareAndGrant(t *testing.T) {
	db := storetest.Migrated(t)
	reports := NewReports(db)
	versions := NewVersions(db)
	shares := NewShares(db)
	grants := NewGrants(db)
	engID := insertEngagement(t, db)
	ctx := context.Background()

	rep, err := reports.Create(ctx, NewReport{
		EngagementID: engID,
		Title:        "Published",
		CreatedBy:    "admin",
	})
	if err != nil {
		t.Fatalf("create report: %v", err)
	}
	if _, err := reports.ReplaceBlocks(ctx, rep.ID, []NewBlock{
		{BlockID: "cover", Params: json.RawMessage(`{}`)},
	}); err != nil {
		t.Fatalf("replace blocks: %v", err)
	}

	version, err := versions.Insert(ctx, NewVersion{
		ReportID:      rep.ID,
		Ordinal:       1,
		Title:         "Published",
		PublishedBy:   "admin",
		BlindScope:    "full",
		BlocksJSON:    `[]`,
		BrandingJSON:  `{}`,
		HTML:          "<html></html>",
		ContentSHA256: "0000000000000000000000000000000000000000000000000000000000000000",
	})
	if err != nil {
		t.Fatalf("insert version: %v", err)
	}
	share, err := shares.Insert(ctx, NewShare{
		VersionID: version.ID,
		TokenHash: "token-hash",
		CreatedBy: "admin",
	})
	if err != nil {
		t.Fatalf("insert share: %v", err)
	}
	if _, _, err := grants.ClaimInsert(ctx, NewGrant{ShareID: share.ID}, nil); err != nil {
		t.Fatalf("claim grant: %v", err)
	}

	if err := reports.Delete(ctx, rep.ID); err != nil {
		t.Fatalf("delete report: %v", err)
	}

	left, err := versions.CountByReport(ctx, rep.ID)
	if err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if left != 0 {
		t.Errorf("%d versions survived the delete, want 0", left)
	}
	if remaining, err := shares.ListByVersion(ctx, version.ID); err != nil {
		t.Fatalf("list shares: %v", err)
	} else if len(remaining) != 0 {
		t.Errorf("%d shares survived the delete, want 0", len(remaining))
	}
	if remaining, err := grants.ListByShare(ctx, share.ID); err != nil {
		t.Fatalf("list grants: %v", err)
	} else if len(remaining) != 0 {
		t.Errorf("%d grants survived the delete, want 0", len(remaining))
	}
}

// TestDeleteReportMissing keeps the not-found answer: the graph statements are
// all `DELETE … WHERE`, so they pass silently on an unknown id and only the
// final row delete can tell the caller there was nothing there.
func TestDeleteReportMissing(t *testing.T) {
	db := storetest.Migrated(t)
	reports := NewReports(db)
	insertEngagement(t, db)

	err := reports.Delete(context.Background(), "01900000-0000-7000-e000-00000000dead")
	if err == nil {
		t.Fatal("delete of an unknown report returned no error")
	}
}

// ---------------------------------------------------------------------------
// Block counts
// ---------------------------------------------------------------------------

// TestBlockCountsByEngagement covers what the reports list renders per row.
// The list response carries no blocks, so before this count existed the page
// measured an absent array and showed "0 blocks" for every report.
func TestBlockCountsByEngagement(t *testing.T) {
	db := storetest.Migrated(t)
	reports := NewReports(db)
	engID := insertEngagement(t, db)
	ctx := context.Background()

	withBlocks, err := reports.Create(ctx, NewReport{
		EngagementID: engID, Title: "Three blocks", CreatedBy: "admin",
	})
	if err != nil {
		t.Fatalf("create report: %v", err)
	}
	empty, err := reports.Create(ctx, NewReport{
		EngagementID: engID, Title: "No blocks", CreatedBy: "admin",
	})
	if err != nil {
		t.Fatalf("create report: %v", err)
	}
	if _, err := reports.ReplaceBlocks(ctx, withBlocks.ID, []NewBlock{
		{BlockID: "cover", Params: json.RawMessage(`{}`)},
		{BlockID: "rich_text", Params: json.RawMessage(`{"html":"<p>a</p>"}`)},
		{BlockID: "findings_backlog", Params: json.RawMessage(`{}`)},
	}); err != nil {
		t.Fatalf("replace blocks: %v", err)
	}

	counts, err := reports.BlockCountsByEngagement(ctx, engID)
	if err != nil {
		t.Fatalf("block counts: %v", err)
	}
	if got := counts[withBlocks.ID]; got != 3 {
		t.Errorf("count for the three-block report = %d, want 3", got)
	}
	// A report with no blocks has no row to group, so it is absent — and the
	// zero value a caller reads from the missing key is the right answer.
	if got, ok := counts[empty.ID]; ok || got != 0 {
		t.Errorf("count for the empty report = %d (present=%v), want 0 (absent)", got, ok)
	}

	// Counts do not leak across engagements.
	other, err := reports.BlockCountsByEngagement(ctx, "01900000-0000-7000-e000-0000000000fe")
	if err != nil {
		t.Fatalf("block counts for another engagement: %v", err)
	}
	if len(other) != 0 {
		t.Errorf("counts for an unrelated engagement = %v, want empty", other)
	}
}
