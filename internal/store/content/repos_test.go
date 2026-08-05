package content_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

func TestSourceRoundTrip(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	sources := content.NewSources(db)

	got, err := sources.ByID(t.Context(), content.SourceIDAttack)
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != content.KindAttack || got.Name == "" {
		t.Fatalf("unexpected seed: %+v", got)
	}

	// Updating bookkeeping must work even with object children present later —
	// this is the DuckDB FK hazard the migration documents.
	if err := sources.SetSyncState(t.Context(), got.ID, content.SourceStatusSyncing, 0, "", time.Time{}); err != nil {
		t.Fatal(err)
	}
	got, err = sources.ByID(t.Context(), got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != content.SourceStatusSyncing {
		t.Fatalf("status = %q", got.Status)
	}

	disabled, err := sources.SetEnabled(t.Context(), content.SourceIDCustom, false)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Enabled {
		t.Fatal("custom still enabled")
	}
	_, err = sources.SetEnabled(t.Context(), content.SourceIDCustom, true)
	if err != nil {
		t.Fatal(err)
	}
}

func TestVersionAndRawPathRoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	db := storetest.Migrated(t)
	paths := content.NewPaths(root)
	versions := content.NewVersions(db, paths)

	v, err := versions.Create(t.Context(), content.NewSourceVersion{
		SourceID: content.SourceIDAttack,
		Version:  "15.1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if v.Status != content.VersionStatusPending {
		t.Fatalf("status = %q", v.Status)
	}

	rel, err := paths.RawRel(content.SourceIDAttack, "15.1", "deadbeef")
	if err != nil {
		t.Fatal(err)
	}
	v, err = versions.SetRaw(t.Context(), v.ID, rel, "deadbeef", 42)
	if err != nil {
		t.Fatal(err)
	}
	if v.RawPath != rel || v.RawSHA256 != "deadbeef" || v.RawBytes != 42 {
		t.Fatalf("raw fields: %+v", v)
	}

	// Escape attempts refused at the repository boundary.
	if _, err := versions.SetRaw(t.Context(), v.ID, "../etc/passwd", "x", 1); err == nil {
		t.Fatal("accepted escaping raw path")
	}
	if _, err := versions.SetRaw(t.Context(), v.ID, "/abs/path", "x", 1); err == nil {
		t.Fatal("accepted absolute raw path")
	}

	// Duplicate natural key.
	_, err = versions.Create(t.Context(), content.NewSourceVersion{
		SourceID: content.SourceIDAttack,
		Version:  "15.1",
	})
	if !isConflict(err) {
		t.Fatalf("duplicate version: %v", err)
	}

	// Source bookkeeping still updatable with a version child (no FK hazard).
	sources := content.NewSources(db)
	if err := sources.SetSyncState(t.Context(), content.SourceIDAttack, content.SourceStatusIdle, 1, "", time.Now()); err != nil {
		t.Fatalf("update source with version child: %v", err)
	}

	listed, err := versions.ListBySource(t.Context(), content.SourceIDAttack)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Version != "15.1" {
		t.Fatalf("list = %+v", listed)
	}
}

func TestJobRoundTripAndInterrupt(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	jobs := content.NewJobs(db)

	j, err := jobs.Create(t.Context(), content.NewJob{
		SourceID:  content.SourceIDAtomic,
		Version:   content.VersionCurrent,
		Kind:      content.JobKindSync,
		CreatedBy: "user-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if j.Status != content.JobStatusQueued {
		t.Fatalf("status = %q", j.Status)
	}
	if string(j.Checkpoint) != "{}" && string(j.Checkpoint) != "null" {
		// DuckDB may normalise; accept empty object forms.
		var m map[string]any
		if err := json.Unmarshal(j.Checkpoint, &m); err != nil || len(m) != 0 {
			t.Fatalf("checkpoint = %s", j.Checkpoint)
		}
	}

	started := time.Now().UTC()
	j, err = jobs.Update(t.Context(), j.ID, content.JobUpdate{
		Status:          content.JobStatusRunning,
		Phase:           "fetch",
		ProgressCurrent: 1,
		ProgressTotal:   10,
		Message:         "downloading",
		StartedAt:       started,
		Checkpoint:      json.RawMessage(`{"page":1}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if j.Status != content.JobStatusRunning || j.Phase != "fetch" || j.ProgressCurrent != 1 {
		t.Fatalf("updated job: %+v", j)
	}
	if j.StartedAt.IsZero() {
		t.Fatal("started_at not set")
	}

	// A second in-flight job, then boot reconciliation.
	j2, err := jobs.Create(t.Context(), content.NewJob{
		SourceID: content.SourceIDSigma,
		Kind:     content.JobKindReprocess,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := jobs.Update(t.Context(), j2.ID, content.JobUpdate{
		Status:    content.JobStatusCancelling,
		Phase:     "apply",
		StartedAt: started,
	}); err != nil {
		t.Fatal(err)
	}

	n, err := jobs.InterruptInFlight(t.Context(), "process restarted while job was in flight")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("interrupted %d, want 2", n)
	}
	for _, id := range []string{j.ID, j2.ID} {
		got, err := jobs.ByID(t.Context(), id)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != content.JobStatusInterrupted {
			t.Errorf("%s status = %q", id, got.Status)
		}
		if got.FinishedAt.IsZero() {
			t.Errorf("%s finished_at not set", id)
		}
	}

	// Idempotent on a second boot: already interrupted rows are not running.
	n, err = jobs.InterruptInFlight(t.Context(), "again")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("second interrupt changed %d rows", n)
	}
}

func TestObjectFamilyRoundTrips(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	objs := content.NewObjects(db)
	ctx := t.Context()
	src := content.SourceIDAttack
	ver := "15.1"

	tac, err := objs.CreateTactic(ctx, content.Tactic{
		SourceID: src, Version: ver, ExternalID: "TA0002",
		Name: "Execution", Description: "exec",
	})
	if err != nil {
		t.Fatal(err)
	}
	if tac.ID == "" || tac.Name != "Execution" {
		t.Fatalf("tactic: %+v", tac)
	}

	tech, err := objs.CreateTechnique(ctx, content.Technique{
		SourceID: src, Version: ver, ExternalID: "T1059.001",
		Name: "PowerShell", Description: "ps",
		IsSubtechnique: true, ParentExternalID: "T1059",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !tech.IsSubtechnique || tech.ParentExternalID != "T1059" {
		t.Fatalf("technique: %+v", tech)
	}
	if err := objs.SetTechniqueTactics(ctx, src, ver, tech.ExternalID, []string{"TA0002"}); err != nil {
		t.Fatal(err)
	}
	tactics, err := objs.TechniqueTactics(ctx, src, ver, tech.ExternalID)
	if err != nil {
		t.Fatal(err)
	}
	if len(tactics) != 1 || tactics[0] != "TA0002" {
		t.Fatalf("tactics = %v", tactics)
	}

	// Duplicate natural key via repository → conflict.
	_, err = objs.CreateTechnique(ctx, content.Technique{
		SourceID: src, Version: ver, ExternalID: "T1059.001", Name: "dup",
	})
	if !isConflict(err) {
		t.Fatalf("dup technique: %v", err)
	}

	if _, err := objs.CreateMitigation(ctx, content.Mitigation{
		SourceID: src, Version: ver, ExternalID: "M1049", Name: "AV",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := objs.CreateGroup(ctx, content.Group{
		SourceID: src, Version: ver, ExternalID: "G0016", Name: "APT29",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := objs.CreateSoftware(ctx, content.Software{
		SourceID: src, Version: ver, ExternalID: "S0154",
		Name: "Cobalt Strike", SoftwareType: content.SoftwareTool,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := objs.CreateDataSource(ctx, content.DataSource{
		SourceID: src, Version: ver, ExternalID: "DS0017", Name: "Command",
	}); err != nil {
		t.Fatal(err)
	}

	// Source still updatable with object children.
	if err := content.NewSources(db).SetSyncState(ctx, src, content.SourceStatusIdle, 6, "", time.Now()); err != nil {
		t.Fatalf("update source with objects: %v", err)
	}
}

func TestProcedureDetectionEmulationNoteRoundTrips(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	ctx := t.Context()

	proc, err := content.NewProcedures(db).Create(ctx, content.ProcedureTemplate{
		SourceID:             content.SourceIDAtomic,
		Version:              content.VersionCurrent,
		ExternalID:           "T1059.001-1",
		Name:                 "PowerShell",
		Platforms:            json.RawMessage(`["windows"]`),
		Executor:             "powershell",
		Command:              "Write-Host hi",
		Cleanup:              "Remove-Item x",
		InputArgs:            json.RawMessage(`{"path":{"default":"C:\\"}}`),
		TechniqueExternalIDs: json.RawMessage(`["T1059.001"]`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if proc.Executor != "powershell" || string(proc.Platforms) == "" {
		t.Fatalf("procedure: %+v", proc)
	}
	// Structure preserved — not a single actions blob.
	if proc.Command == "" || proc.Cleanup == "" {
		t.Fatal("command/cleanup flattened away")
	}

	det, err := content.NewDetections(db).Create(ctx, content.DetectionRuleRef{
		SourceID:             content.SourceIDSigma,
		Version:              content.VersionCurrent,
		ExternalID:           "proc_creation_powershell",
		Name:                 "PowerShell",
		TechniqueExternalIDs: json.RawMessage(`["T1059.001"]`),
		Level:                "high",
		RuleStatus:           "stable",
		Logsource:            json.RawMessage(`{"product":"windows","category":"process_creation"}`),
		RuleYAML:             "title: PowerShell\ndetection: {}\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if det.RuleYAML == "" || det.Level != "high" {
		t.Fatalf("detection: %+v", det)
	}

	plans := content.NewEmulationPlans(db)
	plan, err := plans.Create(ctx, content.EmulationPlan{
		SourceID:   content.SourceIDCTID,
		Version:    content.VersionCurrent,
		ExternalID: "apt29",
		Name:       "APT29",
	})
	if err != nil {
		t.Fatal(err)
	}
	step, err := plans.CreateStep(ctx, content.EmulationPlanStep{
		SourceID:            content.SourceIDCTID,
		Version:             content.VersionCurrent,
		PlanID:              plan.ID,
		Position:            1,
		ExternalID:          "apt29-step-1",
		Name:                "Initial Access",
		TechniqueExternalID: "T1566",
	})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := plans.StepsByPlan(ctx, plan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 1 || steps[0].ID != step.ID || steps[0].Position != 1 {
		t.Fatalf("steps = %+v", steps)
	}

	note, err := content.NewNotes(db).Create(ctx, content.Note{
		SourceID:            content.SourceIDCustom,
		Version:             content.VersionCurrent,
		ExternalID:          "kb-1",
		Title:               "Notes on PowerShell",
		BodyMarkdown:        "Be careful.",
		Tags:                json.RawMessage(`["windows","powershell"]`),
		TechniqueExternalID: "T1059.001",
	})
	if err != nil {
		t.Fatal(err)
	}
	if note.Title == "" || note.TechniqueExternalID != "T1059.001" {
		t.Fatalf("note: %+v", note)
	}

	// Rolling token is the agreed constant.
	if proc.Version != content.VersionCurrent {
		t.Fatalf("rolling version = %q", proc.Version)
	}

}

func isConflict(err error) bool {
	var e *apierr.Error
	return errors.As(err, &e) && e.Status() == 409
}
