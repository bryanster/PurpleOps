package httpapi

import (
	"net/http"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/identity"
)

func TestReportTemplateCRUD(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	user := s.seedUser(t)
	cookie := s.signIn(t)

	engID := "019385a2-9000-7000-8cf0-ef0123456789"
	seedEngagementPlumbing(t, s.db, engID, user.ID, "lead")

	// Create a report.
	s.post(BasePath+"/engagements/"+engID+"/reports", `{"title":"Test Report"}`, cookie)

	// ── Create template ──
	rec := s.post(BasePath+"/engagements/"+engID+"/report-templates", `{"name":"My Template"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create template: %d\nbody: %s", rec.Code, rec.Body)
	}
	tmpl := decodeJSON[gen.ReportTemplate](t, rec)
	if tmpl.Name != "My Template" {
		t.Errorf("template name = %q, want %q", tmpl.Name, "My Template")
	}

	// ── List templates ──
	rec = s.get(BasePath+"/engagements/"+engID+"/report-templates", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list templates: %d\nbody: %s", rec.Code, rec.Body)
	}
	list := decodeJSON[[]gen.ReportTemplate](t, rec)
	if len(list) != 1 {
		t.Errorf("list templates len = %d, want 1", len(list))
	}

	// ── Get template (blocks omitted from list, included in get) ──
	rec = s.get(BasePath+"/engagements/"+engID+"/report-templates/"+tmpl.Id.String(), cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("get template: %d\nbody: %s", rec.Code, rec.Body)
	}
	tmplFull := decodeJSON[gen.ReportTemplate](t, rec)
	if tmplFull.Blocks == nil || len(*tmplFull.Blocks) != 0 {
		t.Errorf("template blocks = %v, want empty", tmplFull.Blocks)
	}

	// ── Patch template ──
	rec = s.send(http.MethodPatch, BasePath+"/engagements/"+engID+"/report-templates/"+tmpl.Id.String(),
		`{"name":"Renamed"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch template: %d\nbody: %s", rec.Code, rec.Body)
	}
	patched := decodeJSON[gen.ReportTemplate](t, rec)
	if patched.Name != "Renamed" {
		t.Errorf("patched name = %q, want %q", patched.Name, "Renamed")
	}

	// ── Delete template ──
	rec = s.send(http.MethodDelete, BasePath+"/engagements/"+engID+"/report-templates/"+tmpl.Id.String(), "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete template: %d\nbody: %s", rec.Code, rec.Body)
	}
	rec = s.get(BasePath+"/engagements/"+engID+"/report-templates/"+tmpl.Id.String(), cookie)
	if rec.Code != http.StatusNotFound {
		t.Errorf("get deleted template = %d, want 404", rec.Code)
	}
}

func TestReportTemplateNonMemberRejected(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	s.seedUser(t)
	// Create a member (not admin) who is NOT in the engagement.
	outsiderUser := s.seedUser(t, func(in *identity.NewUser) {
		in.Email = "outsider@example.com"
		in.PlatformRole = authz.PlatformRoleMember
	})
	rec := s.login(outsiderUser.Email, testPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("outsider login: %d", rec.Code)
	}
	outsiderCookie := sessionCookie(t, rec)

	engID := "019385a2-9001-7000-8cf0-ef0123456789"
	seedEngagementOnly(t, s.db, engID)

	rec = s.post(BasePath+"/engagements/"+engID+"/report-templates", `{"name":"Nope"}`, outsiderCookie)
	if rec.Code != http.StatusNotFound {
		t.Errorf("non-member create template = %d, want 404", rec.Code)
	}
}

func TestReportTemplateApplyAndFromReport(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	user := s.seedUser(t)
	cookie := s.signIn(t)

	engID := "019385a2-9002-7000-8cf0-ef0123456789"
	seedEngagementPlumbing(t, s.db, engID, user.ID, "lead")

	// Create a report.
	rec := s.post(BasePath+"/engagements/"+engID+"/reports", `{"title":"Source"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create report: %d", rec.Code)
	}
	rep := decodeJSON[gen.Report](t, rec)

	// ── from-report: snapshot empty blocks ──
	rec = s.post(BasePath+"/engagements/"+engID+"/report-templates/from-report",
		`{"reportId":"`+rep.Id.String()+`","name":"From Empty"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("from-report: %d\nbody: %s", rec.Code, rec.Body)
	}
	tmpl := decodeJSON[gen.ReportTemplate](t, rec)
	if tmpl.Blocks == nil || len(*tmpl.Blocks) != 0 {
		t.Errorf("from-report blocks len = %v, want 0", tmpl.Blocks)
	}

	// ── Create target report, apply template ──
	rec = s.post(BasePath+"/engagements/"+engID+"/reports", `{"title":"Target"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create target: %d", rec.Code)
	}
	target := decodeJSON[gen.Report](t, rec)

	rec = s.post(BasePath+"/engagements/"+engID+"/reports/"+target.Id.String()+"/apply-template",
		`{"templateId":"`+tmpl.Id.String()+`"}`, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply template: %d\nbody: %s", rec.Code, rec.Body)
	}
	applied := decodeJSON[gen.Report](t, rec)
	if applied.Blocks != nil && len(*applied.Blocks) != 0 {
		t.Errorf("applied blocks = %v, want 0", applied.Blocks)
	}

	// ── Delete template, verify report survives ──
	rec = s.send(http.MethodDelete, BasePath+"/engagements/"+engID+"/report-templates/"+tmpl.Id.String(), "", cookie)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete template: %d", rec.Code)
	}
	rec = s.get(BasePath+"/engagements/"+engID+"/reports/"+target.Id.String(), cookie)
	if rec.Code != http.StatusOK {
		t.Errorf("report after template delete = %d, want 200", rec.Code)
	}
}
