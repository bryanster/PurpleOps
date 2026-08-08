package content_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/content"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

func TestCustomServiceCRUDAndExport(t *testing.T) {
	t.Parallel()
	db := storetest.Migrated(t)
	ctx := t.Context()

	svc, err := content.NewCustom(content.CustomDeps{
		Sources:      storecontent.NewSources(db),
		Procedures:   storecontent.NewProcedures(db),
		Detections:   storecontent.NewDetections(db),
		Notes:        storecontent.NewNotes(db),
		Activity:     events.New(activity.New(db)),
		NoteMaxBytes: 64,
	})
	if err != nil {
		t.Fatal(err)
	}
	actor := authn.Subject{UserID: "test-actor"}

	proc, err := svc.CreateProcedure(ctx, actor, content.ProcedureCreate{
		Name:                 "P",
		Command:              "echo hi",
		Cleanup:              "true",
		TechniqueExternalIDs: []string{"T1059.001"},
		Platforms:            []string{"linux"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if proc.SourceID != storecontent.SourceIDCustom || proc.Version != storecontent.VersionCurrent {
		t.Fatalf("proc home: %+v", proc)
	}

	_, err = svc.CreateProcedure(ctx, actor, content.ProcedureCreate{
		Name:                 "bad",
		TechniqueExternalIDs: []string{"T1"},
	})
	if err == nil {
		t.Fatal("expected invalid technique")
	}
	var ae *apierr.Error
	if !errors.As(err, &ae) || ae.Status() != 400 {
		t.Fatalf("want 400 validation, got %v", err)
	}

	// Note body over the tiny cap.
	_, err = svc.CreateNote(ctx, actor, content.NoteCreate{
		Title:        "big",
		BodyMarkdown: strings.Repeat("x", 65),
	})
	if err == nil {
		t.Fatal("expected body size rejection")
	}

	note, err := svc.CreateNote(ctx, actor, content.NoteCreate{
		Title:        "small",
		BodyMarkdown: "ok",
	})
	if err != nil {
		t.Fatal(err)
	}

	det, err := svc.CreateDetection(ctx, actor, content.DetectionCreate{
		Name:     "R",
		RuleYAML: "title: R\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	doc, err := svc.Export(ctx, content.ExportAll)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.ProcedureTemplates) != 1 || len(doc.DetectionRules) != 1 || len(doc.Notes) != 1 {
		t.Fatalf("export counts: %+v", doc)
	}
	if doc.Meta.SourceName == "" {
		t.Fatal("export meta missing source name")
	}

	if err := svc.DeleteProcedure(ctx, actor, proc.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteDetection(ctx, actor, det.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteNote(ctx, actor, note.ID); err != nil {
		t.Fatal(err)
	}
}
