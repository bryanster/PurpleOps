package report_test

import (
	"context"
	"errors"
	"testing"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/report"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	storereport "github.com/bryanster/blacklight/internal/store/report"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// reportBindFixture holds two engagements, each with a report and a template,
// so cross-engagement report-family ids can be driven through the domain
// methods that must 404 (M7-012).
type reportBindFixture struct {
	svc       *report.Service
	tmplSvc   *report.TemplateService
	engA      storengagement.Engagement
	engB      storengagement.Engagement
	reportA   storereport.Report
	reportB   storereport.Report
	tmplA     storereport.Template
	tmplB     storereport.Template
	reports   *storereport.Reports
	templates *storereport.Templates
}

func newReportBindFixture(t *testing.T) *reportBindFixture {
	t.Helper()
	ctx := context.Background()

	db := storetest.Migrated(t)
	engagements := storengagement.NewEngagements(db)
	reports := storereport.NewReports(db)
	templates := storereport.NewTemplates(db)

	registry := report.NewRegistry()
	svc, err := report.New(report.Deps{Reports: reports, Registry: registry})
	if err != nil {
		t.Fatalf("report.New: %v", err)
	}
	tmplSvc, err := report.NewTemplateService(report.TemplateDeps{Templates: templates, Reports: reports, Registry: registry})
	if err != nil {
		t.Fatalf("report.NewTemplateService: %v", err)
	}

	newEngagement := func(name string) storengagement.Engagement {
		eng, err := engagements.Create(ctx, storengagement.NewEngagement{
			Name:          name,
			AttackVersion: "15.1",
			Mode:          storengagement.EngagementModeStandard,
			CreatedBy:     "0192f1a0-0000-7000-8000-000000000000",
		})
		if err != nil {
			t.Fatalf("create engagement %s: %v", name, err)
		}
		return eng
	}
	engA := newEngagement("A")
	engB := newEngagement("B")

	newReport := func(engID, title string) storereport.Report {
		r, err := reports.Create(ctx, storereport.NewReport{EngagementID: engID, Title: title, CreatedBy: "0192f1a0-0000-7000-8000-000000000000"})
		if err != nil {
			t.Fatalf("create report %s: %v", title, err)
		}
		return r
	}
	reportA := newReport(engA.ID, "report-A")
	reportB := newReport(engB.ID, "report-B")

	newTemplate := func(engID, name string) storereport.Template {
		tmpl, err := templates.Create(ctx, storereport.NewTemplate{EngagementID: engID, Name: name, CreatedBy: "0192f1a0-0000-7000-8000-000000000000"})
		if err != nil {
			t.Fatalf("create template %s: %v", name, err)
		}
		return tmpl
	}
	tmplA := newTemplate(engA.ID, "template-A")
	tmplB := newTemplate(engB.ID, "template-B")

	return &reportBindFixture{
		svc: svc, tmplSvc: tmplSvc,
		engA: engA, engB: engB,
		reportA: reportA, reportB: reportB,
		tmplA: tmplA, tmplB: tmplB,
		reports: reports, templates: templates,
	}
}

func TestReportGetBindingMismatchIsNotFound(t *testing.T) {
	fx := newReportBindFixture(t)
	ctx := context.Background()

	if _, err := fx.svc.Get(ctx, fx.engB.ID, fx.reportA.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("Get(B, A.report) error = %v, want NotFound", err)
	}
	if _, err := fx.svc.Get(ctx, fx.engA.ID, fx.reportA.ID); err != nil {
		t.Fatalf("Get(A, A.report) error = %v, want nil", err)
	}
}

func TestReportWriteBindingMismatchDoesNotMutate(t *testing.T) {
	fx := newReportBindFixture(t)
	ctx := context.Background()

	title := "patched"
	if _, err := fx.svc.Update(ctx, fx.engB.ID, fx.reportA.ID, report.UpdateInput{Title: &title, ActorID: "x"}); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("Update(B, A.report) error = %v, want NotFound", err)
	}
	if err := fx.svc.Delete(ctx, fx.engB.ID, fx.reportA.ID, "x"); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("Delete(B, A.report) error = %v, want NotFound", err)
	}
	if _, err := fx.svc.ReplaceBlocks(ctx, report.ReplaceBlocksInput{ReportID: fx.reportA.ID, EngagementID: fx.engB.ID, ActorID: "x"}); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("ReplaceBlocks(B, A.report) error = %v, want NotFound", err)
	}
	rep, err := fx.reports.ByID(ctx, fx.reportA.ID)
	if err != nil {
		t.Fatalf("report A should still exist: %v", err)
	}
	if rep.Title != fx.reportA.Title {
		t.Errorf("report A title changed to %q, want %q", rep.Title, fx.reportA.Title)
	}
}

func TestTemplateBindingMismatchIsNotFound(t *testing.T) {
	fx := newReportBindFixture(t)
	ctx := context.Background()

	if _, err := fx.tmplSvc.GetTemplate(ctx, fx.engB.ID, fx.tmplA.ID); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("GetTemplate(B, A.template) error = %v, want NotFound", err)
	}
	if _, err := fx.tmplSvc.GetTemplate(ctx, fx.engA.ID, fx.tmplA.ID); err != nil {
		t.Fatalf("GetTemplate(A, A.template) error = %v, want nil", err)
	}

	name := "patched"
	if _, err := fx.tmplSvc.UpdateTemplate(ctx, fx.engB.ID, fx.tmplA.ID, report.UpdateTemplateInput{Name: &name, ActorID: "x"}); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("UpdateTemplate(B, A.template) error = %v, want NotFound", err)
	}
	if err := fx.tmplSvc.DeleteTemplate(ctx, fx.engB.ID, fx.tmplA.ID, "x"); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("DeleteTemplate(B, A.template) error = %v, want NotFound", err)
	}
	tmpl, err := fx.templates.ByID(ctx, fx.tmplA.ID)
	if err != nil {
		t.Fatalf("template A should still exist: %v", err)
	}
	if tmpl.Name != fx.tmplA.Name {
		t.Errorf("template A name changed to %q, want %q", tmpl.Name, fx.tmplA.Name)
	}
}

func TestApplyTemplateCrossEngagementIsNotFound(t *testing.T) {
	fx := newReportBindFixture(t)
	ctx := context.Background()

	// Template from A onto report in B: 404, no blocks copied.
	if _, err := fx.tmplSvc.ApplyTemplate(ctx, report.ApplyTemplateInput{
		ReportID: fx.reportB.ID, TemplateID: fx.tmplA.ID, EngagementID: fx.engB.ID, ActorID: "x",
	}); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("ApplyTemplate(B.report, A.template) error = %v, want NotFound", err)
	}
	// Report from A with a path engagement of B: 404 too.
	if _, err := fx.tmplSvc.ApplyTemplate(ctx, report.ApplyTemplateInput{
		ReportID: fx.reportA.ID, TemplateID: fx.tmplB.ID, EngagementID: fx.engB.ID, ActorID: "x",
	}); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("ApplyTemplate(A.report, path B) error = %v, want NotFound", err)
	}
}

func TestCreateFromReportCrossEngagementIsNotFound(t *testing.T) {
	fx := newReportBindFixture(t)
	ctx := context.Background()

	// Source report from A, but the new template would live in B.
	if _, err := fx.tmplSvc.CreateFromReport(ctx, report.CreateFromReportInput{
		EngagementID: fx.engB.ID, ReportID: fx.reportA.ID, Name: "stolen", ActorID: "x",
	}); !errors.Is(err, apierr.ErrNotFound) {
		t.Fatalf("CreateFromReport(B, A.report) error = %v, want NotFound", err)
	}
}
