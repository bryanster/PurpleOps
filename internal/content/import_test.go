package content_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

func v1Fixture(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Join(append([]string{filepath.Dir(file), "testdata", "v1import"}, parts...)...)
}

func newCustom(t *testing.T) *content.Custom {
	t.Helper()
	db := storetest.Migrated(t)
	svc, err := content.NewCustom(content.CustomDeps{
		Sources:    storecontent.NewSources(db),
		Procedures: storecontent.NewProcedures(db),
		Detections: storecontent.NewDetections(db),
		Notes:      storecontent.NewNotes(db),
		Activity:   events.New(activity.New(db)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestImportJSONFixtureCreatesTemplates(t *testing.T) {
	t.Parallel()
	svc := newCustom(t)
	ctx := t.Context()
	actor := authn.Subject{UserID: "importer"}

	raw, err := os.ReadFile(v1Fixture(t, "testcases.json"))
	if err != nil {
		t.Fatal(err)
	}
	report, err := svc.Import(ctx, actor, content.ImportRequest{
		Format:   "testcases_json",
		Filename: "testcases.json",
		Data:     raw,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ProceduresCreated != 2 {
		t.Fatalf("created=%d report=%+v", report.ProceduresCreated, report)
	}
	procs, err := svc.ListProcedures(ctx, storecontent.ProcedureListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(procs) != 2 {
		t.Fatalf("list=%d", len(procs))
	}
	for _, p := range procs {
		if p.Name == "" || p.Command == "" {
			t.Fatalf("incomplete: %+v", p)
		}
	}

	// Re-import upserts, does not duplicate.
	report2, err := svc.Import(ctx, actor, content.ImportRequest{
		Format:   "testcases_json",
		Filename: "testcases.json",
		Data:     raw,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report2.ProceduresCreated != 0 || report2.ProceduresUpdated != 2 {
		t.Fatalf("reimport: %+v", report2)
	}
	procs, err = svc.ListProcedures(ctx, storecontent.ProcedureListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(procs) != 2 {
		t.Fatalf("after reimport list=%d", len(procs))
	}
}

func TestImportYAMLDirAndDryRun(t *testing.T) {
	t.Parallel()
	svc := newCustom(t)
	ctx := t.Context()
	actor := authn.Subject{UserID: "importer"}
	dir := v1Fixture(t, "testcases")

	dry, err := svc.Import(ctx, actor, content.ImportRequest{
		Format: "testcases_yaml",
		DryRun: true,
		Path:   dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if dry.ProceduresCreated != 2 {
		t.Fatalf("dry created=%d errors=%v", dry.ProceduresCreated, dry.Errors)
	}
	if len(dry.Errors) == 0 {
		t.Fatal("expected broken.yaml error on dry-run")
	}
	procs, err := svc.ListProcedures(ctx, storecontent.ProcedureListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(procs) != 0 {
		t.Fatalf("dry-run wrote %d rows", len(procs))
	}

	live, err := svc.Import(ctx, actor, content.ImportRequest{
		Format: "testcases_yaml",
		Path:   dir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if live.ProceduresCreated != 2 {
		t.Fatalf("live created=%d", live.ProceduresCreated)
	}
	// Counts shape matches dry-run for the valid files.
	if dry.ProceduresCreated != live.ProceduresCreated {
		t.Fatalf("dry/live mismatch: dry=%+v live=%+v", dry, live)
	}
}

func TestImportKnowledgebaseNotes(t *testing.T) {
	t.Parallel()
	svc := newCustom(t)
	ctx := t.Context()
	actor := authn.Subject{UserID: "importer"}

	report, err := svc.Import(ctx, actor, content.ImportRequest{
		Format: "knowledgebase_yaml",
		Path:   v1Fixture(t, "knowledgebase"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.NotesCreated != 2 {
		t.Fatalf("notes created=%d errors=%v", report.NotesCreated, report.Errors)
	}
	notes, err := svc.ListNotes(ctx, storecontent.NoteListFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 {
		t.Fatalf("list=%d", len(notes))
	}
	for _, n := range notes {
		if n.BodyMarkdown == "" {
			t.Fatalf("empty body: %+v", n)
		}
	}
}

func TestImportHybridZipAuto(t *testing.T) {
	t.Parallel()
	svc := newCustom(t)
	ctx := t.Context()
	actor := authn.Subject{UserID: "importer"}
	raw, err := os.ReadFile(v1Fixture(t, "hybrid.zip"))
	if err != nil {
		t.Fatal(err)
	}
	report, err := svc.Import(ctx, actor, content.ImportRequest{
		Format:   "auto",
		Filename: "hybrid.zip",
		Data:     raw,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.ProceduresCreated < 3 || report.NotesCreated != 2 {
		t.Fatalf("report=%+v", report)
	}
}
