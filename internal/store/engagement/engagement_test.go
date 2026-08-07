package engagement_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// These tests run against a real migrated DuckDB file (see storetest), because
// most of what this package promises is a promise the schema makes.

// repos is every repository over one database.
type repos struct {
	Engagements *engagement.Engagements
	Scenarios   *engagement.Scenarios
	Steps       *engagement.Steps
	Executions  *engagement.Executions
	Findings    *engagement.Findings
	Comments    *engagement.Comments
	Evidence    *engagement.EvidenceRepo
}

func newRepos(t *testing.T) repos {
	t.Helper()
	db := storetest.Migrated(t)
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

// writeSQL runs one statement through the store's serialized writer.
func writeSQL(t *testing.T, db *store.DB, stmt string, args ...any) error {
	t.Helper()
	return db.Write(context.Background(), func(tx *sql.Tx) error {
		_, err := tx.ExecContext(context.Background(), stmt, args...)
		return err
	})
}

// newEngagement creates a valid engagement for use in tests that need a parent row.
func mustCreateEngagement(t *testing.T, r repos) engagement.Engagement {
	t.Helper()
	e, err := r.Engagements.Create(context.Background(), engagement.NewEngagement{
		Name:              "Test Engagement",
		Client:            "Test Corp",
		Description:       "A test assessment",
		StartsOn:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndsOn:            time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		AttackVersion:     "15.1",
		Mode:              engagement.EngagementModeStandard,
		AutoRevealOnStart: false,
		CreatedBy:         "user-1",
	})
	if err != nil {
		t.Fatalf("mustCreateEngagement: %v", err)
	}
	return e
}

// mustCreateScenario creates a scenario under the given engagement.
func mustCreateScenario(t *testing.T, r repos, engagementID string, ordinal int, name string) engagement.Scenario {
	t.Helper()
	s, err := r.Scenarios.Create(context.Background(), engagement.NewScenario{
		EngagementID: engagementID,
		Ordinal:      ordinal,
		Name:         name,
		Narrative:    "Test narrative",
		Source:       engagement.ScenarioSourceManual,
		ThreatActor:  "APT999",
		SourceRef:    "",
		PlanID:       "",
	})
	if err != nil {
		t.Fatalf("mustCreateScenario: %v", err)
	}
	return s
}

// mustCreateStepWithExecution creates a step and its pending execution.
func mustCreateStepWithExecution(t *testing.T, r repos, scenarioID string, ordinal int, name string) (engagement.Step, engagement.Execution) {
	t.Helper()
	step, exec, err := r.Steps.CreateWithExecution(context.Background(), engagement.NewStep{
		ScenarioID:      scenarioID,
		Ordinal:         ordinal,
		Name:            name,
		Objective:       "Test objective",
		TechniqueID:     "T1059",
		SubtechniqueID:  "T1059.001",
		TacticID:        "TA0002",
		Procedure:       json.RawMessage(`{"executor":"sh","command":"whoami"}`),
		TemplateID:      "",
		TargetAsset:     "test-host",
		Tools:           json.RawMessage(`["tool1"]`),
		ControlsInScope: json.RawMessage(`["EDR"]`),
		AttackVersion:   "15.1",
	})
	if err != nil {
		t.Fatalf("mustCreateStepWithExecution: %v", err)
	}
	return step, exec
}

// ---------------------------------------------------------------------------
// Schema existence
// ---------------------------------------------------------------------------

func TestTheAppDomainSchemaExists(t *testing.T) {
	db := storetest.Migrated(t)

	tables := []string{
		"engagement", "scenario", "step", "execution",
		"finding", "finding_step", "evidence_blob", "evidence",
		"comment", "comment_revision",
	}
	for _, table := range tables {
		var count int
		if err := db.Read().QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'app' AND table_name = ?`, table,
		).Scan(&count); err != nil {
			t.Errorf("checking table app.%s: %v", table, err)
		} else if count != 1 {
			t.Errorf("table app.%s should exist, got count %d", table, count)
		}
	}

	// Check the FK on engagement_member was added.
	var fkCount int
	if err := db.Read().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM duckdb_constraints()
			WHERE schema_name = 'app' AND table_name = 'engagement_member'
			AND constraint_type = 'FOREIGN KEY' AND constraint_text LIKE '%engagement%'`,
	).Scan(&fkCount); err != nil {
		t.Errorf("checking engagement_member FK: %v", err)
	} else if fkCount == 0 {
		t.Error("engagement_member should have an FK on engagement_id")
	}
}

// ---------------------------------------------------------------------------
// Constraint tests
// ---------------------------------------------------------------------------
func TestTheSchemaRefusesDuplicateExecutionPerStep_DirectSQL(t *testing.T) {
	db := storetest.Migrated(t)

	// Create an engagement, scenario, step via repos.
	r := repos{
		Engagements: engagement.NewEngagements(db),
		Scenarios:   engagement.NewScenarios(db),
		Steps:       engagement.NewSteps(db),
		Executions:  engagement.NewExecutions(db),
	}
	eng := mustCreateEngagement(t, r)
	sc := mustCreateScenario(t, r, eng.ID, 1, "Scenario 1")
	step, _ := mustCreateStepWithExecution(t, r, sc.ID, 1, "Step 1")

	// Try to insert a second execution for the same step via raw SQL.
	err := writeSQL(t, db,
		`INSERT INTO app.execution (id, step_id, version, status, executed_by, command_run, source_host, target_host, red_notes, detection_modifiers, detecting_source, detecting_rule_ref, alert_severity, blue_notes, scored_by, created_at, updated_at)
		VALUES ('fake-id', ?, 1, 'pending', '', '', '', '', '', '[]', '', '', '', '', '', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`,
		step.ID,
	)
	if err == nil {
		t.Error("second execution for same step_id should fail unique constraint")
	} else if !strings.Contains(err.Error(), "duplicate") && !strings.Contains(err.Error(), "unique") && !strings.Contains(err.Error(), "constraint") {
		t.Errorf("expected constraint violation, got: %v", err)
	}
}

func TestTheSchemaRefusesBadEnumValues(t *testing.T) {
	db := storetest.Migrated(t)

	tests := []struct {
		table string
		col   string
		bad   string
		stmt  string
	}{
		{
			table: "engagement",
			col:   "status",
			bad:   "bogus",
			stmt:  `INSERT INTO app.engagement (id, name, client, description, status, starts_on, ends_on, attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at) VALUES ('e1', 'x', 'x', 'x', ?, '2026-01-01', '2026-06-01', '15.1', 'standard', false, 'u1', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`,
		},
		{
			table: "engagement",
			col:   "mode",
			bad:   "bogus",
			stmt:  `INSERT INTO app.engagement (id, name, client, description, status, starts_on, ends_on, attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at) VALUES ('e2', 'x', 'x', 'x', 'draft', '2026-01-01', '2026-06-01', '15.1', ?, false, 'u1', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`,
		},
		{
			table: "execution",
			col:   "status",
			bad:   "bogus",
			stmt:  `INSERT INTO app.execution (id, step_id, version, status, executed_by, command_run, source_host, target_host, red_notes, detection_modifiers, detecting_source, detecting_rule_ref, alert_severity, blue_notes, scored_by, created_at, updated_at) VALUES ('x1', 'x1', 1, ?, '', '', '', '', '', '[]', '', '', '', '', '', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`,
		},
		{
			table: "execution",
			col:   "detection_category",
			bad:   "bogus",
			stmt:  `INSERT INTO app.execution (id, step_id, version, status, executed_by, command_run, source_host, target_host, red_notes, detection_category, detection_modifiers, detecting_source, detecting_rule_ref, alert_severity, blue_notes, scored_by, created_at, updated_at) VALUES ('x2', 'x2', 1, 'pending', '', '', '', '', '', ?, '[]', '', '', '', '', '', '2026-01-01 00:00:00', '2026-01-01 00:00:00')`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.table+"/"+tc.col, func(t *testing.T) {
			err := writeSQL(t, db, tc.stmt, tc.bad)
			if err == nil {
				t.Errorf("bad %s value %q in %s should fail CHECK", tc.col, tc.bad, tc.table)
			}
		})
	}
}

func TestEngagementMemberFKRefusesUnknownEngagement(t *testing.T) {
	db := storetest.Migrated(t)

	err := writeSQL(t, db,
		`INSERT INTO app.engagement_member (engagement_id, user_id, role, added_at)
		VALUES ('nonexistent', 'user-1', 'lead', '2026-01-01 00:00:00')`,
	)
	if err == nil {
		t.Error("insert with nonexistent engagement_id should fail FK")
	}
}

func TestEvidenceParentXOR(t *testing.T) {
	db := storetest.Migrated(t)

	// Both NULL — should fail.
	err := writeSQL(t, db,
		`INSERT INTO app.evidence (id, blob_sha256, filename, caption, side, execution_id, comment_id, uploaded_by, uploaded_at, size, mime)
		VALUES ('ev1', 'fake', 'f.txt', '', 'red', NULL, NULL, 'u1', '2026-01-01 00:00:00', 0, 'text/plain')`,
	)
	if err == nil {
		t.Error("evidence with both parents NULL should fail CHECK")
	}
}

// ---------------------------------------------------------------------------
// Repository round-trip tests
// ---------------------------------------------------------------------------

func TestEngagementRoundTrip(t *testing.T) {
	r := newRepos(t)

	e, err := r.Engagements.Create(context.Background(), engagement.NewEngagement{
		Name:              "ACME Assessment",
		Client:            "ACME Corp",
		Description:       "Annual red team",
		StartsOn:          time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		EndsOn:            time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		AttackVersion:     "16.0",
		Mode:              engagement.EngagementModeBlind,
		AutoRevealOnStart: true,
		CreatedBy:         "user-1",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.ID == "" {
		t.Error("engagement ID is empty")
	}
	if e.Status != engagement.EngagementStatusDraft {
		t.Errorf("new engagement should be draft, got %s", e.Status)
	}
	if e.Mode != engagement.EngagementModeBlind {
		t.Errorf("expected blind mode, got %s", e.Mode)
	}
	if e.AttackVersion != "16.0" {
		t.Errorf("expected attack_version 16.0, got %s", e.AttackVersion)
	}

	// Read it back.
	got, err := r.Engagements.ByID(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.Name != "ACME Assessment" {
		t.Errorf("Name = %q, want %q", got.Name, "ACME Assessment")
	}
}

func TestScenarioRoundTrip(t *testing.T) {
	r := newRepos(t)
	eng := mustCreateEngagement(t, r)

	s, err := r.Scenarios.Create(context.Background(), engagement.NewScenario{
		EngagementID: eng.ID,
		Ordinal:      1,
		Name:         "Initial Access",
		Narrative:    "Gained access via phishing",
		Source:       engagement.ScenarioSourceManual,
		ThreatActor:  "APT42",
		SourceRef:    "",
		PlanID:       "",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.ID == "" {
		t.Error("scenario ID is empty")
	}
	if s.Ordinal != 1 {
		t.Errorf("Ordinal = %d, want 1", s.Ordinal)
	}

	// Read back.
	got, err := r.Scenarios.ByID(context.Background(), s.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.Name != "Initial Access" {
		t.Errorf("Name = %q", got.Name)
	}

	// List by engagement.
	list, err := r.Scenarios.ListByEngagement(context.Background(), eng.ID)
	if err != nil {
		t.Fatalf("ListByEngagement: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 scenario, got %d", len(list))
	}
}

func TestStepWithExecutionRoundTrip(t *testing.T) {
	r := newRepos(t)
	eng := mustCreateEngagement(t, r)
	sc := mustCreateScenario(t, r, eng.ID, 1, "Scenario 1")

	step, exec, err := r.Steps.CreateWithExecution(context.Background(), engagement.NewStep{
		ScenarioID:      sc.ID,
		Ordinal:         1,
		Name:            "PowerShell Execution",
		Objective:       "Execute payload",
		TechniqueID:     "T1059.001",
		SubtechniqueID:  "T1059.001",
		TacticID:        "TA0002",
		Procedure:       json.RawMessage(`{"executor":"powershell","command":"Invoke-Expression ..."}`),
		TemplateID:      "tpl-1",
		TargetAsset:     "WS01",
		Tools:           json.RawMessage(`["PowerShell Empire"]`),
		ControlsInScope: json.RawMessage(`["CrowdStrike","Windows Defender"]`),
		AttackVersion:   "15.1",
	})
	if err != nil {
		t.Fatalf("CreateWithExecution: %v", err)
	}
	if step.ID == "" {
		t.Error("step ID is empty")
	}
	if step.TechniqueID != "T1059.001" {
		t.Errorf("TechniqueID = %q", step.TechniqueID)
	}
	if exec.ID == "" {
		t.Error("execution ID is empty")
	}
	if exec.StepID != step.ID {
		t.Errorf("execution StepID = %q, want %q", exec.StepID, step.ID)
	}
	if exec.Status != engagement.ExecutionStatusPending {
		t.Errorf("new execution status = %s, want pending", exec.Status)
	}
	if exec.Version != 1 {
		t.Errorf("new execution version = %d, want 1", exec.Version)
	}

	// Read step back.
	gotStep, err := r.Steps.ByID(context.Background(), step.ID)
	if err != nil {
		t.Fatalf("Steps.ByID: %v", err)
	}
	if gotStep.Name != "PowerShell Execution" {
		t.Errorf("Name = %q", gotStep.Name)
	}

	// Read execution back.
	gotExec, err := r.Executions.ByStepID(context.Background(), step.ID)
	if err != nil {
		t.Fatalf("Executions.ByStepID: %v", err)
	}
	if gotExec.Version != 1 {
		t.Errorf("Version = %d", gotExec.Version)
	}
}

func TestExecutionVersionIncrement(t *testing.T) {
	r := newRepos(t)
	eng := mustCreateEngagement(t, r)
	sc := mustCreateScenario(t, r, eng.ID, 1, "Scenario 1")
	_, exec := mustCreateStepWithExecution(t, r, sc.ID, 1, "Step 1")

	// Increment from version 1.
	newVer, err := r.Executions.IncrementVersion(context.Background(), exec.ID, 1)
	if err != nil {
		t.Fatalf("IncrementVersion: %v", err)
	}
	if newVer != 2 {
		t.Errorf("new version = %d, want 2", newVer)
	}

	// Read back to confirm.
	got, err := r.Executions.ByID(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.Version != 2 {
		t.Errorf("stored version = %d, want 2", got.Version)
	}

	// Version conflict: try with stale version.
	_, err = r.Executions.IncrementVersion(context.Background(), exec.ID, 1)
	if err == nil {
		t.Error("expected version conflict error for stale version 1")
	}
}

func TestFindingRoundTrip(t *testing.T) {
	r := newRepos(t)
	eng := mustCreateEngagement(t, r)

	f, err := r.Findings.Create(context.Background(), engagement.NewFinding{
		EngagementID:         eng.ID,
		Title:                "Weak MFA policy",
		Description:          "MFA not enforced for admin accounts",
		Severity:             "High",
		Recommendation:       "Enable conditional access MFA",
		Owner:                "user-2",
		CreatedFromExecution: "",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if f.ID == "" {
		t.Error("finding ID is empty")
	}
	if f.Status != engagement.FindingStatusOpen {
		t.Errorf("new finding status = %s, want open", f.Status)
	}

	// Read back.
	got, err := r.Findings.ByID(context.Background(), f.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.Title != "Weak MFA policy" {
		t.Errorf("Title = %q", got.Title)
	}

	// List by engagement.
	list, err := r.Findings.ListByEngagement(context.Background(), eng.ID)
	if err != nil {
		t.Fatalf("ListByEngagement: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 finding, got %d", len(list))
	}
}

func TestFindingStepLinking(t *testing.T) {
	r := newRepos(t)
	eng := mustCreateEngagement(t, r)
	sc := mustCreateScenario(t, r, eng.ID, 1, "Scenario 1")
	step1, _ := mustCreateStepWithExecution(t, r, sc.ID, 1, "Step 1")
	step2, _ := mustCreateStepWithExecution(t, r, sc.ID, 2, "Step 2")

	f, err := r.Findings.Create(context.Background(), engagement.NewFinding{
		EngagementID: eng.ID,
		Title:        "Test finding",
		Description:  "desc",
		Severity:     "Medium",
		Owner:        "user-1",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Link steps.
	if err := r.Findings.AddStep(context.Background(), f.ID, step1.ID); err != nil {
		t.Fatalf("AddStep: %v", err)
	}
	if err := r.Findings.AddStep(context.Background(), f.ID, step2.ID); err != nil {
		t.Fatalf("AddStep: %v", err)
	}

	// Read steps back.
	steps, err := r.Findings.Steps(context.Background(), f.ID)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(steps))
	}

	// Remove one.
	if err := r.Findings.RemoveStep(context.Background(), f.ID, step1.ID); err != nil {
		t.Fatalf("RemoveStep: %v", err)
	}
	steps, err = r.Findings.Steps(context.Background(), f.ID)
	if err != nil {
		t.Fatalf("Steps after remove: %v", err)
	}
	if len(steps) != 1 {
		t.Errorf("expected 1 step after remove, got %d", len(steps))
	}
}

func TestCommentRoundTrip(t *testing.T) {
	r := newRepos(t)
	eng := mustCreateEngagement(t, r)
	sc := mustCreateScenario(t, r, eng.ID, 1, "Scenario 1")
	_, exec := mustCreateStepWithExecution(t, r, sc.ID, 1, "Step 1")

	c, err := r.Comments.Create(context.Background(), engagement.NewComment{
		ExecutionID: exec.ID,
		AuthorID:    "user-1",
		Body:        "Initial detection notes",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if c.ID == "" {
		t.Error("comment ID is empty")
	}

	// Edit creates a revision.
	comment, rev, err := r.Comments.Edit(context.Background(), c.ID, "user-1", "Updated notes")
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if comment.Body != "Updated notes" {
		t.Errorf("Body = %q", comment.Body)
	}
	if rev.Body != "Initial detection notes" {
		t.Errorf("revision Body = %q, want previous body %q", rev.Body, "Initial detection notes")
	}

	// Read revisions.
	revs, err := r.Comments.Revisions(context.Background(), c.ID)
	if err != nil {
		t.Fatalf("Revisions: %v", err)
	}
	if len(revs) != 1 {
		t.Errorf("expected 1 revision, got %d", len(revs))
	}

	// List by execution.
	list, err := r.Comments.ListByExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("ListByExecution: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 comment, got %d", len(list))
	}
}

func TestEvidenceRoundTrip_DirectSQL(t *testing.T) {
	db := storetest.Migrated(t)
	r := repos{
		Engagements: engagement.NewEngagements(db),
		Scenarios:   engagement.NewScenarios(db),
		Steps:       engagement.NewSteps(db),
		Executions:  engagement.NewExecutions(db),
		Evidence:    engagement.NewEvidenceRepo(db),
	}
	eng := mustCreateEngagement(t, r)
	sc := mustCreateScenario(t, r, eng.ID, 1, "Scenario 1")
	_, exec := mustCreateStepWithExecution(t, r, sc.ID, 1, "Step 1")

	// Insert the blob row.
	err := writeSQL(t, db,
		`INSERT INTO app.evidence_blob (sha256, size, mime, storage_path, ref_count, created_at)
		VALUES ('abc123', 1024, 'image/png', 'abc/123.png', 1, '2026-01-01 00:00:00')`,
	)
	if err != nil {
		t.Fatalf("insert blob: %v", err)
	}

	// Create evidence linked to execution.
	ev, err := r.Evidence.Create(context.Background(), engagement.NewEvidence{
		BlobSHA256:  "abc123",
		Filename:    "screenshot.png",
		Caption:     "Login screen",
		Side:        engagement.EvidenceSideRed,
		ExecutionID: exec.ID,
		CommentID:   "",
		UploadedBy:  "user-1",
		Size:        1024,
		MIME:        "image/png",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ev.ID == "" {
		t.Error("evidence ID is empty")
	}

	// Read back.
	got, err := r.Evidence.ByID(context.Background(), ev.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.Filename != "screenshot.png" {
		t.Errorf("Filename = %q", got.Filename)
	}
	if got.ExecutionID != exec.ID {
		t.Errorf("ExecutionID = %q, want %q", got.ExecutionID, exec.ID)
	}

	// List by execution.
	list, err := r.Evidence.ListByExecution(context.Background(), exec.ID)
	if err != nil {
		t.Fatalf("ListByExecution: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 evidence, got %d", len(list))
	}
}

// ---------------------------------------------------------------------------
// References.AttackVersion
// ---------------------------------------------------------------------------

func TestReferencesAttackVersionCount(t *testing.T) {
	r := newRepos(t)

	refs := engagement.NewReferences(r.Engagements)

	// No engagements yet.
	count, err := refs.AttackVersion(context.Background(), "15.1")
	if err != nil {
		t.Fatalf("AttackVersion: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}

	// Create one with version 15.1.
	mustCreateEngagement(t, r)

	count, err = refs.AttackVersion(context.Background(), "15.1")
	if err != nil {
		t.Fatalf("AttackVersion: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	// Different version.
	count, err = refs.AttackVersion(context.Background(), "16.0")
	if err != nil {
		t.Fatalf("AttackVersion: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 for 16.0, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Structural tests
// ---------------------------------------------------------------------------

func TestNoRepositoryOwnsADatabaseHandle(t *testing.T) {
	repos := []any{
		engagement.NewEngagements(nil),
		engagement.NewScenarios(nil),
		engagement.NewSteps(nil),
		engagement.NewExecutions(nil),
		engagement.NewFindings(nil),
		engagement.NewComments(nil),
		engagement.NewEvidenceRepo(nil),
	}
	for _, repo := range repos {
		v := reflect.ValueOf(repo).Elem()
		for i := 0; i < v.NumField(); i++ {
			ft := v.Field(i).Type()
			if ft == reflect.TypeOf((*sql.DB)(nil)) ||
				ft == reflect.TypeOf((*sql.Tx)(nil)) ||
				ft == reflect.TypeOf((*sql.Conn)(nil)) {
				t.Errorf("%T has field %s of type %s", repo, v.Type().Field(i).Name, ft)
			}
		}
	}
}

func TestEveryRepositoryMethodTakesAContext(t *testing.T) {
	check := func(repo any) {
		tp := reflect.TypeOf(repo)
		for i := 0; i < tp.NumMethod(); i++ {
			m := tp.Method(i)
			if m.Type.NumIn() < 2 {
				continue
			}
			// First arg after receiver should be context.Context.
			arg0 := m.Type.In(1)
			ctxType := reflect.TypeOf((*context.Context)(nil)).Elem()
			if !arg0.Implements(ctxType) {
				t.Errorf("%s.%s: first param is %s, want context.Context",
					tp.Name(), m.Name, arg0)
			}
		}
	}
	check(engagement.NewEngagements(nil))
	check(engagement.NewScenarios(nil))
	check(engagement.NewSteps(nil))
	check(engagement.NewExecutions(nil))
	check(engagement.NewFindings(nil))
	check(engagement.NewComments(nil))
	check(engagement.NewEvidenceRepo(nil))
}

func TestTimestampsAreUTC(t *testing.T) {
	r := newRepos(t)

	e, err := r.Engagements.Create(context.Background(), engagement.NewEngagement{
		Name:              "TS Test",
		Client:            "C",
		Description:       "D",
		StartsOn:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndsOn:            time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		AttackVersion:     "15.1",
		Mode:              engagement.EngagementModeStandard,
		AutoRevealOnStart: false,
		CreatedBy:         "user-1",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if e.CreatedAt.Location() != time.UTC {
		t.Errorf("CreatedAt zone = %s, want UTC", e.CreatedAt.Location())
	}
	if e.UpdatedAt.Location() != time.UTC {
		t.Errorf("UpdatedAt zone = %s, want UTC", e.UpdatedAt.Location())
	}

	got, err := r.Engagements.ByID(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("ByID: %v", err)
	}
	if got.CreatedAt.Location() != time.UTC {
		t.Errorf("read-back CreatedAt zone = %s, want UTC", got.CreatedAt.Location())
	}
}
