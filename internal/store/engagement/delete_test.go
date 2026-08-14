package engagement_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// Deleting an engagement is the one operation that touches every table in the
// schema, so these tests are mostly about coverage of that surface: each table
// that can hold a row under an engagement, and the proof that a sibling
// engagement keeps its own.

// engagementTables maps every table that can hold rows under an engagement to a
// WHERE clause selecting exactly that engagement's rows, with one `?` bound to
// the engagement id.
//
// A new table with an engagement-scoped row belongs in this map and in
// deleteEngagementGraph; the test then holds Delete to it automatically. That
// is the point of the map — the previous delete missed five tables, and nothing
// failed until a foreign key did.
var engagementTables = map[string]string{
	"app.engagement":        `id = ?`,
	"app.engagement_member": `engagement_id = ?`,
	"app.scenario":          `engagement_id = ?`,
	"app.step":              `scenario_id IN (SELECT id FROM app.scenario WHERE engagement_id = ?)`,
	"app.execution": `step_id IN (SELECT s.id FROM app.step s
		JOIN app.scenario sc ON s.scenario_id = sc.id WHERE sc.engagement_id = ?)`,
	`app."comment"`: `execution_id IN (SELECT e.id FROM app.execution e
		JOIN app.step s ON e.step_id = s.id
		JOIN app.scenario sc ON s.scenario_id = sc.id WHERE sc.engagement_id = ?)`,
	"app.comment_revision": `comment_id IN (SELECT id FROM app."comment" WHERE execution_id IN (
		SELECT e.id FROM app.execution e
		JOIN app.step s ON e.step_id = s.id
		JOIN app.scenario sc ON s.scenario_id = sc.id WHERE sc.engagement_id = ?))`,
	"app.evidence": `execution_id IN (SELECT e.id FROM app.execution e
		JOIN app.step s ON e.step_id = s.id
		JOIN app.scenario sc ON s.scenario_id = sc.id WHERE sc.engagement_id = ?)
		OR comment_id IN (SELECT id FROM app."comment" WHERE execution_id IN (
		SELECT e.id FROM app.execution e
		JOIN app.step s ON e.step_id = s.id
		JOIN app.scenario sc ON s.scenario_id = sc.id WHERE sc.engagement_id = ?))`,
	"app.finding":                `engagement_id = ?`,
	"app.finding_step":           `finding_id IN (SELECT id FROM app.finding WHERE engagement_id = ?)`,
	"app.finding_status_history": `engagement_id = ?`,
	"app.activity":               `engagement_id = ?`,
	"app.report":                 `engagement_id = ?`,
	"app.report_block":           `report_id IN (SELECT id FROM app.report WHERE engagement_id = ?)`,
	"app.report_version":         `report_id IN (SELECT id FROM app.report WHERE engagement_id = ?)`,
	"app.report_share": `version_id IN (SELECT id FROM app.report_version WHERE report_id IN (
		SELECT id FROM app.report WHERE engagement_id = ?))`,
	"app.report_share_grant": `share_id IN (SELECT id FROM app.report_share WHERE version_id IN (
		SELECT id FROM app.report_version WHERE report_id IN (
		SELECT id FROM app.report WHERE engagement_id = ?)))`,
	"app.report_template": `engagement_id = ?`,
	"app.report_template_block": `template_id IN (
		SELECT id FROM app.report_template WHERE engagement_id = ?)`,
}

// countUnder returns how many rows of table belong to engagement id.
func countUnder(t *testing.T, db *store.DB, table, id string) int {
	t.Helper()
	where, ok := engagementTables[table]
	if !ok {
		t.Fatalf("countUnder: %s is not in engagementTables", table)
	}
	args := make([]any, countPlaceholders(where))
	for i := range args {
		args[i] = id
	}
	var n int
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, table, where)
	if err := db.Read().QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("countUnder %s: %v", table, err)
	}
	return n
}

func countPlaceholders(s string) int {
	n := 0
	for _, r := range s {
		if r == '?' {
			n++
		}
	}
	return n
}

// fullGraph builds one engagement with at least one row in every table of
// engagementTables and returns its id. The suffix keeps ids unique so two
// graphs can coexist in one database.
func fullGraph(t *testing.T, db *store.DB, r repos, suffix string) string {
	t.Helper()
	ctx := context.Background()

	eng := mustCreateEngagement(t, r)
	sc := mustCreateScenario(t, r, eng.ID, 1, "Scenario "+suffix)
	step, exec := mustCreateStepWithExecution(t, r, sc.ID, 1, "Step "+suffix)

	if err := writeSQL(t, db,
		`INSERT INTO app.engagement_member VALUES (?, ?, 'lead', 'user-1', now())`,
		eng.ID, "member-"+suffix); err != nil {
		t.Fatalf("member: %v", err)
	}

	// Comment plus one revision.
	cm, err := r.Comments.Create(ctx, engagement.NewComment{
		ExecutionID: exec.ID, AuthorID: "user-1", Body: "first",
	})
	if err != nil {
		t.Fatalf("comment: %v", err)
	}
	if err := writeSQL(t, db,
		`INSERT INTO app.comment_revision VALUES (?, ?, 'older body', 'user-1', now())`,
		"rev-"+suffix, cm.ID); err != nil {
		t.Fatalf("comment_revision: %v", err)
	}

	// Two evidence rows on one blob — one under the execution, one under the
	// comment — so both evidence branches of the delete are exercised and the
	// blob's ref_count has to come down by two.
	if err := writeSQL(t, db,
		`INSERT INTO app.evidence_blob VALUES (?, 1024, 'image/png', ?, 2, now())`,
		"blob-"+suffix, suffix+"/shot.png"); err != nil {
		t.Fatalf("blob: %v", err)
	}
	if _, err := r.Evidence.Create(ctx, engagement.NewEvidence{
		BlobSHA256: "blob-" + suffix, Filename: "a.png", Caption: "on execution",
		Side: engagement.EvidenceSideRed, ExecutionID: exec.ID,
		UploadedBy: "user-1", Size: 1024, MIME: "image/png",
	}); err != nil {
		t.Fatalf("evidence on execution: %v", err)
	}
	if _, err := r.Evidence.Create(ctx, engagement.NewEvidence{
		BlobSHA256: "blob-" + suffix, Filename: "b.png", Caption: "on comment",
		Side: engagement.EvidenceSideRed, CommentID: cm.ID,
		UploadedBy: "user-1", Size: 1024, MIME: "image/png",
	}); err != nil {
		t.Fatalf("evidence on comment: %v", err)
	}

	// Finding, linked to the step. Create also writes the status-history row.
	f, err := r.Findings.Create(ctx, engagement.NewFinding{
		EngagementID: eng.ID, Title: "Finding " + suffix, Description: "d",
		Severity: "high", Recommendation: "fix it", Owner: "user-1", CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatalf("finding: %v", err)
	}
	if err := writeSQL(t, db, `INSERT INTO app.finding_step VALUES (?, ?)`, f.ID, step.ID); err != nil {
		t.Fatalf("finding_step: %v", err)
	}

	if err := writeSQL(t, db,
		`INSERT INTO app.activity VALUES (?, ?, 'user-1', 'engagement.created', 'engagement', ?, NULL, now())`,
		"act-"+suffix, eng.ID, eng.ID); err != nil {
		t.Fatalf("activity: %v", err)
	}

	// Report → block, version → share → grant.
	if err := writeSQL(t, db,
		`INSERT INTO app.report VALUES (?, ?, 'Report', NULL, NULL, NULL, 'user-1', now(), NULL, now())`,
		"rep-"+suffix, eng.ID); err != nil {
		t.Fatalf("report: %v", err)
	}
	if err := writeSQL(t, db,
		`INSERT INTO app.report_block VALUES (?, ?, 0, 'cover', '{}')`,
		"repblk-"+suffix, "rep-"+suffix); err != nil {
		t.Fatalf("report_block: %v", err)
	}
	if err := writeSQL(t, db,
		`INSERT INTO app.report_version VALUES (?, ?, 1, 'Report', 'user-1', now(), FALSE, 'lead_full', '[]', '{}', '<html></html>', NULL, NULL)`,
		"repver-"+suffix, "rep-"+suffix); err != nil {
		t.Fatalf("report_version: %v", err)
	}
	if err := writeSQL(t, db,
		`INSERT INTO app.report_share VALUES (?, ?, 'hash', NULL, NULL, NULL, 'user-1', now(), NULL, NULL)`,
		"share-"+suffix, "repver-"+suffix); err != nil {
		t.Fatalf("report_share: %v", err)
	}
	if err := writeSQL(t, db,
		`INSERT INTO app.report_share_grant VALUES (?, ?, 'user-2', NULL, now(), NULL, now())`,
		"grant-"+suffix, "share-"+suffix); err != nil {
		t.Fatalf("report_share_grant: %v", err)
	}

	// Report template → block.
	if err := writeSQL(t, db,
		`INSERT INTO app.report_template VALUES (?, ?, 'Template', 'user-1', now(), now())`,
		"tmpl-"+suffix, eng.ID); err != nil {
		t.Fatalf("report_template: %v", err)
	}
	if err := writeSQL(t, db,
		`INSERT INTO app.report_template_block VALUES (?, 0, 'cover', '{}')`,
		"tmpl-"+suffix); err != nil {
		t.Fatalf("report_template_block: %v", err)
	}

	return eng.ID
}

// ---------------------------------------------------------------------------
// The schema-level bug: DELETE FROM app.engagement at all
// ---------------------------------------------------------------------------

// 0016_app_domain used to add engagement_member's FK to app.engagement with a
// create-copy-drop-rename, and DuckDB left app.engagement's dependent list
// naming the pre-rename table. Every delete then failed resolving that name,
// whatever the engagement held. 0016 now builds the table under its final name;
// this is the regression guard, deliberately at the raw-SQL level so it fails
// for the schema reason alone and not because some repository changed.
func TestPlainSQLCanDeleteAnEngagement(t *testing.T) {
	db := storetest.Migrated(t)
	if err := writeSQL(t, db, `INSERT INTO app.engagement VALUES
		('e1','n','c','d','draft','2026-01-01','2026-06-01','15.1','standard',false,'u1',now(),now())`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := writeSQL(t, db, `DELETE FROM app.engagement WHERE id = 'e1'`); err != nil {
		t.Fatalf("DELETE FROM app.engagement: %v", err)
	}
}

// The other half of that: the rebuild must not have bought deletability by
// losing the constraint. A membership still has to hold its engagement down.
func TestEngagementMemberStillRestrictsEngagementDelete(t *testing.T) {
	db := storetest.Migrated(t)
	if err := writeSQL(t, db, `INSERT INTO app.engagement VALUES
		('e2','n','c','d','draft','2026-01-01','2026-06-01','15.1','standard',false,'u1',now(),now())`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := writeSQL(t, db,
		`INSERT INTO app.engagement_member VALUES ('e2','u1','lead','u1',now())`); err != nil {
		t.Fatalf("insert member: %v", err)
	}
	if err := writeSQL(t, db, `DELETE FROM app.engagement WHERE id = 'e2'`); err == nil {
		t.Fatal("deleting an engagement that still has members was allowed; " +
			"the RESTRICT foreign key is not being enforced")
	}
}

// ---------------------------------------------------------------------------
// Engagements.Delete
// ---------------------------------------------------------------------------

// The simplest case, and the one the old multi-statement Exec could not do:
// DuckDB binds parameters for the first statement of a script only, so the
// whole delete failed with "incorrect argument count for command" before it
// touched a row.
func TestDeleteRemovesAnEmptyEngagement(t *testing.T) {
	db := storetest.Migrated(t)
	r := newReposOver(db)
	eng := mustCreateEngagement(t, r)

	if err := r.Engagements.Delete(context.Background(), eng.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := r.Engagements.ByID(context.Background(), eng.ID); err == nil {
		t.Fatal("engagement is still readable after Delete")
	}
}

func TestDeleteRemovesEveryRowInTheGraph(t *testing.T) {
	db := storetest.Migrated(t)
	r := newReposOver(db)
	id := fullGraph(t, db, r, "a")

	// The fixture is only worth something if it actually populated every table.
	for table := range engagementTables {
		if countUnder(t, db, table, id) == 0 {
			t.Fatalf("fixture bug: %s has no rows under the engagement before Delete", table)
		}
	}

	if err := r.Engagements.Delete(context.Background(), id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	for table := range engagementTables {
		if n := countUnder(t, db, table, id); n != 0 {
			t.Errorf("%s still holds %d row(s) after Delete", table, n)
		}
	}
}

func TestDeleteLeavesOtherEngagementsAlone(t *testing.T) {
	db := storetest.Migrated(t)
	r := newReposOver(db)
	doomed := fullGraph(t, db, r, "a")
	keep := fullGraph(t, db, r, "b")

	before := make(map[string]int, len(engagementTables))
	for table := range engagementTables {
		before[table] = countUnder(t, db, table, keep)
	}

	if err := r.Engagements.Delete(context.Background(), doomed); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	for table := range engagementTables {
		if got := countUnder(t, db, table, keep); got != before[table] {
			t.Errorf("%s under the surviving engagement: %d row(s), want %d",
				table, got, before[table])
		}
	}
}

// Evidence rows hold a reference on their blob. If deleting the engagement
// dropped the rows without releasing the refs, ref_count could never reach zero
// and the blob file would sit on disk forever with nothing pointing at it.
func TestDeleteReleasesEvidenceBlobRefs(t *testing.T) {
	db := storetest.Migrated(t)
	r := newReposOver(db)
	id := fullGraph(t, db, r, "a")

	var before int
	if err := db.Read().QueryRowContext(context.Background(),
		`SELECT ref_count FROM app.evidence_blob WHERE sha256 = 'blob-a'`).Scan(&before); err != nil {
		t.Fatalf("read ref_count: %v", err)
	}
	if before != 2 {
		t.Fatalf("fixture bug: ref_count = %d before Delete, want 2", before)
	}

	if err := r.Engagements.Delete(context.Background(), id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	var after int
	if err := db.Read().QueryRowContext(context.Background(),
		`SELECT ref_count FROM app.evidence_blob WHERE sha256 = 'blob-a'`).Scan(&after); err != nil {
		t.Fatalf("read ref_count: %v", err)
	}
	// Two evidence rows shared the blob, so both refs must be released — a
	// flat -1 would leave the blob stuck at 1 and never collectable.
	if after != 0 {
		t.Errorf("ref_count = %d after Delete, want 0", after)
	}
}

func TestDeleteReportsNotFoundForAnUnknownEngagement(t *testing.T) {
	db := storetest.Migrated(t)
	r := newReposOver(db)

	err := r.Engagements.Delete(context.Background(), "01a00000-0000-7000-8000-000000000000")
	if err == nil {
		t.Fatal("Delete of an unknown engagement returned no error")
	}
	if !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("Delete of an unknown engagement: %v, want a not-found error", err)
	}
}

// newReposOver is newRepos for tests that also need the *store.DB itself.
func newReposOver(db *store.DB) repos {
	return repos{
		Engagements: engagement.NewEngagements(db),
		Scenarios:   engagement.NewScenarios(db),
		Steps:       engagement.NewSteps(db),
		Executions:  engagement.NewExecutions(db),
		Findings:    engagement.NewFindings(db),
		Comments:    engagement.NewComments(db),
		Evidence:    engagement.NewEvidenceRepo(db),
	}
}
