package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/report"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// The builder UI's two loops, end to end: add blocks and save them
// (PUT .../blocks), then render the draft (POST .../preview). Both are what
// the report builder page does on every edit, and neither had a handler-level
// test — the store round-trip tests cover the layer underneath, and the render
// tests cover the layer above.

func TestReportBuilderSaveAndPreview(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	user := s.seedUser(t)
	cookie := s.signIn(t)

	engID := "019385a2-9100-7000-8cf0-ef0123456789"
	seedEngagementPlumbing(t, s.db, engID, user.ID, "lead")

	rec := s.post(BasePath+"/engagements/"+engID+"/reports", `{"title":"Builder"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create report: %d\nbody: %s", rec.Code, rec.Body)
	}
	rep := decodeJSON[gen.Report](t, rec)
	base := BasePath + "/engagements/" + engID + "/reports/" + rep.Id.String()

	// Save the block arrangement the builder produces for a report with a
	// cover, a rich-text section, and a findings backlog.
	body := `{"blocks":[
		{"blockId":"cover","params":{}},
		{"blockId":"rich_text","params":{"html":"<p>Narrative.</p>"}},
		{"blockId":"findings_backlog","params":{}}
	]}`
	rec = s.send(http.MethodPut, base+"/blocks", body, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("put blocks: %d\nbody: %s", rec.Code, rec.Body)
	}
	saved := decodeJSON[gen.Report](t, rec)
	if saved.Blocks == nil || len(*saved.Blocks) != 3 {
		t.Fatalf("saved blocks = %v, want 3", saved.Blocks)
	}

	// Reload: the params must survive the round trip, which is what the
	// builder reads back into its editors.
	rec = s.get(base, cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("get report: %d\nbody: %s", rec.Code, rec.Body)
	}
	got := decodeJSON[gen.Report](t, rec)
	if got.Blocks == nil || len(*got.Blocks) != 3 {
		t.Fatalf("reloaded blocks = %v, want 3", got.Blocks)
	}
	savedHTML, ok := (*got.Blocks)[1].Params["html"].(string)
	if !ok || !strings.Contains(savedHTML, "Narrative.") {
		t.Errorf("rich_text html param = %v, want the saved narrative", (*got.Blocks)[1].Params["html"])
	}

	// Preview renders the saved draft.
	rec = s.post(base+"/preview", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview: %d\nbody: %s", rec.Code, rec.Body)
	}
	html := rec.Body.String()
	if !strings.HasPrefix(html, "<!DOCTYPE html>") {
		t.Errorf("preview body is not an HTML document:\n%s", truncate(html))
	}
	if !strings.Contains(html, "Narrative.") {
		t.Errorf("preview is missing the rich_text content:\n%s", truncate(html))
	}
}

// TestReportBuilderPreviewEmptyDraft pins that a report with no blocks still
// previews. It is the first thing a user sees after creating a report, and an
// error there reads as "preview is broken" rather than "add a block".
func TestReportBuilderPreviewEmptyDraft(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	user := s.seedUser(t)
	cookie := s.signIn(t)

	engID := "019385a2-9101-7000-8cf0-ef0123456789"
	seedEngagementPlumbing(t, s.db, engID, user.ID, "lead")

	rec := s.post(BasePath+"/engagements/"+engID+"/reports", `{"title":"Empty"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create report: %d\nbody: %s", rec.Code, rec.Body)
	}
	rep := decodeJSON[gen.Report](t, rec)

	rec = s.post(BasePath+"/engagements/"+engID+"/reports/"+rep.Id.String()+"/preview", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview empty draft: %d\nbody: %s", rec.Code, rec.Body)
	}
	if !strings.HasPrefix(rec.Body.String(), "<!DOCTYPE html>") {
		t.Errorf("preview body is not an HTML document:\n%s", truncate(rec.Body.String()))
	}
}

// TestReportBuilderMemberCanSave pins that saving blocks works from an
// ordinary member session and not only an admin one: putReportBlocks names both
// `engagement: engagementId` and `param: reportId` in its `x-authz-resource`,
// and getting those two the wrong way round turns every member's save into a
// 404 on an engagement they belong to.
func TestReportBuilderMemberCanSave(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	s.seedUser(t)

	member := s.seedUser(t, func(in *identity.NewUser) {
		in.Email = "builder-member@example.com"
		in.PlatformRole = authz.PlatformRoleMember
	})
	rec := s.login(member.Email, testPassword)
	if rec.Code != http.StatusOK {
		t.Fatalf("member login: %d", rec.Code)
	}
	memberCookie := sessionCookie(t, rec)

	engID := "019385a2-9103-7000-8cf0-ef0123456789"
	seedEngagementPlumbing(t, s.db, engID, member.ID, "red")

	rec = s.post(BasePath+"/engagements/"+engID+"/reports", `{"title":"Member"}`, memberCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("member create report: %d\nbody: %s", rec.Code, rec.Body)
	}
	rep := decodeJSON[gen.Report](t, rec)

	rec = s.send(http.MethodPut,
		BasePath+"/engagements/"+engID+"/reports/"+rep.Id.String()+"/blocks",
		`{"blocks":[{"blockId":"cover","params":{}}]}`, memberCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("member put blocks: %d\nbody: %s", rec.Code, rec.Body)
	}

	rec = s.post(BasePath+"/engagements/"+engID+"/reports/"+rep.Id.String()+"/preview", "",
		memberCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("member preview: %d\nbody: %s", rec.Code, rec.Body)
	}
}

// TestReportBlockParamsSurviveResave saves each block once, reads back the
// params the server stored, and saves those same params again — which is
// exactly what the builder does on every edit after the first, since it holds
// the params it last read.
//
// It is a round trip rather than a fixed table because the breakage came from
// the server's own defaults: applyDefaults writes evidence_appendix's
// `limit: 50` into the stored params, and ValidateParams then refused the type
// it had just written. The report could be saved once and never again.
func TestReportBlockParamsSurviveResave(t *testing.T) {
	t.Parallel()

	s := newAuthServer(t)
	user := s.seedUser(t)
	cookie := s.signIn(t)

	engID := "019385a2-9102-7000-8cf0-ef0123456789"
	seedEngagementPlumbing(t, s.db, engID, user.ID, "lead")

	rec := s.post(BasePath+"/engagements/"+engID+"/reports", `{"title":"Resave"}`, cookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create report: %d\nbody: %s", rec.Code, rec.Body)
	}
	rep := decodeJSON[gen.Report](t, rec)
	blocksURL := BasePath + "/engagements/" + engID + "/reports/" + rep.Id.String() + "/blocks"

	for _, id := range report.AllBlockIDs() {
		t.Run(string(id), func(t *testing.T) {
			rec := s.send(http.MethodPut, blocksURL,
				`{"blocks":[{"blockId":"`+string(id)+`","params":{}}]}`, cookie)
			if rec.Code != http.StatusOK {
				t.Fatalf("first save: %d\nbody: %s", rec.Code, rec.Body)
			}
			saved := decodeJSON[gen.Report](t, rec)
			if saved.Blocks == nil || len(*saved.Blocks) != 1 {
				t.Fatalf("first save returned %v blocks, want 1", saved.Blocks)
			}

			echoed, err := json.Marshal(map[string]any{"blocks": []any{map[string]any{
				"blockId": string(id),
				"params":  (*saved.Blocks)[0].Params,
			}}})
			if err != nil {
				t.Fatalf("marshal echo body: %v", err)
			}

			rec = s.send(http.MethodPut, blocksURL, string(echoed), cookie)
			if rec.Code != http.StatusOK {
				t.Fatalf("second save of the params the server just returned: %d\nbody: %s\nsent: %s",
					rec.Code, rec.Body, echoed)
			}
		})
	}
}

func truncate(s string) string {
	if len(s) > 800 {
		return s[:800] + "…"
	}
	return s
}
