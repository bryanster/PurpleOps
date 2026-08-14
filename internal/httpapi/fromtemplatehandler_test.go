package httpapi

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

// The from-template import over the whole chain (M3-013).
//
// The service-level test in internal/engagement covers the snapshot itself.
// What is left to this one is the part that only shows up with a real server
// on the other end of the socket: the route, the authorization mapping, and —
// the reason it exists — the write actually committing. Every import answered
// 500 after a 30-second stall, because the activity hook running inside the
// step's write transaction opened a second one against a writer that
// serializes and is not re-entrant. See
// TestCreateStepFromTemplate_RecordsActivityOnTheCallersTransaction.
func TestCreateStepFromTemplateOverHTTP(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	lead := server.seedUser(t)
	leadCookie := server.signIn(t)

	engID := uuid.NewString()
	seedEngagementPlumbing(t, server.db, engID, lead.ID, string(authz.EngagementRoleLead))

	scenarioPath := BasePath + "/engagements/" + engID + "/scenarios"
	rec := server.post(scenarioPath, `{"name":"Initial Access"}`, leadCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create scenario: %d\n%s", rec.Code, rec.Body)
	}
	scenario := decodeJSON[gen.Scenario](t, rec)

	// A custom template carries no technique ids, so the import needs no
	// pinned ATT&CK install — this test is about the write, not resolution.
	rec = server.post(customProceduresPath, `{
		"name":"LSASS dump",
		"description":"Dumps LSASS memory with procdump.",
		"platforms":["windows"],
		"executor":"command_prompt",
		"elevationRequired":true,
		"command":"procdump -ma lsass.exe #{path}",
		"cleanup":"del #{path}",
		"inputArgs":[{"name":"path","description":"dump path","type":"path","default":"C:\\\\temp\\\\lsass.dmp"}]
	}`, leadCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create template: %d\n%s", rec.Code, rec.Body)
	}
	tmpl := decodeJSON[gen.ContentProcedureTemplate](t, rec)

	stepsPath := scenarioPath + "/" + scenario.Id.String() + "/steps"
	body := `{
		"templateId":"` + tmpl.Id.String() + `",
		"targetAsset":"DC01",
		"argValues":{"path":"C:\\temp\\dc01.dmp"}
	}`

	// A deadlocked write does not fail, it hangs, and the server answers only
	// once the request deadline fires. Timing the call is what tells the two
	// apart when this regresses.
	started := time.Now()
	rec = server.post(stepsPath+"/from-template", body, leadCookie)
	elapsed := time.Since(started)

	if rec.Code != http.StatusCreated {
		t.Fatalf("from-template: %d after %s\n%s", rec.Code, elapsed, rec.Body)
	}
	if elapsed > 10*time.Second {
		t.Errorf("from-template took %s; a single insert taking this long means the write stalled", elapsed)
	}

	step := decodeJSON[gen.Step](t, rec)
	if step.Name != "LSASS dump" {
		t.Errorf("name = %q, want the template name", step.Name)
	}
	if step.Objective != "Dumps LSASS memory with procdump." {
		t.Errorf("objective = %q, want the template description", step.Objective)
	}
	if step.TargetAsset != "DC01" {
		t.Errorf("targetAsset = %q, want DC01", step.TargetAsset)
	}
	if step.Ordinal != 1 {
		t.Errorf("ordinal = %d, want 1", step.Ordinal)
	}
	if step.TemplateId != tmpl.Id.String() {
		t.Errorf("templateId = %q, want %s", step.TemplateId, tmpl.Id)
	}
	if step.Procedure == nil {
		t.Fatal("procedure missing from the created step")
	}
	proc := *step.Procedure
	if got, want := proc["command"], `procdump -ma lsass.exe C:\temp\dc01.dmp`; got != want {
		t.Errorf("command = %v, want %q — argValues were not substituted", got, want)
	}
	if got, want := proc["cleanup"], `del C:\temp\dc01.dmp`; got != want {
		t.Errorf("cleanup = %v, want %q", got, want)
	}

	// The step is a real row, not just a response body.
	rec = server.get(stepsPath, leadCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list steps: %d\n%s", rec.Code, rec.Body)
	}
	steps := decodeJSON[gen.StepList](t, rec).Items
	if len(steps) != 1 || steps[0].Id != step.Id {
		t.Fatalf("steps list = %+v, want the imported step", steps)
	}

	// And so is its activity row: the hook shares the step's commit, so an
	// imported step without one would mean the two came apart.
	rec = server.get(BasePath+"/engagements/"+engID+"/activity", leadCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list activity: %d\n%s", rec.Code, rec.Body)
	}
	page := decodeJSON[gen.ActivityPage](t, rec)
	var found *gen.ActivityEntry
	for i, entry := range page.Items {
		if entry.ObjectId == step.Id.String() && entry.Verb == "step.created" {
			found = &page.Items[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no step.created activity entry for %s\nentries: %+v", step.Id, page.Items)
	}
	if found.Delta == nil {
		t.Fatal("activity entry has no delta; the template it was imported from is unrecorded")
	}
	if got := (*found.Delta)["template_id"]; got != tmpl.Id.String() {
		t.Errorf("delta.template_id = %v, want %s", got, tmpl.Id)
	}
}

// The sibling After hooks, which recorded activity the same broken way and
// stalled on the same serialized writer. Patching a step or a scenario is a
// short request whose whole substance is the write, so a status other than 200
// here is the same defect wearing a different verb.
func TestPatchThroughAfterHooksCommits(t *testing.T) {
	t.Parallel()

	server := newAuthServer(t)
	lead := server.seedUser(t)
	leadCookie := server.signIn(t)

	engID := uuid.NewString()
	seedEngagementPlumbing(t, server.db, engID, lead.ID, string(authz.EngagementRoleLead))

	scenarioPath := BasePath + "/engagements/" + engID + "/scenarios"
	rec := server.post(scenarioPath, `{"name":"Execution"}`, leadCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create scenario: %d\n%s", rec.Code, rec.Body)
	}
	scenario := decodeJSON[gen.Scenario](t, rec)

	stepsPath := scenarioPath + "/" + scenario.Id.String() + "/steps"
	rec = server.post(stepsPath, `{"name":"Run payload"}`, leadCookie)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create step: %d\n%s", rec.Code, rec.Body)
	}
	step := decodeJSON[gen.Step](t, rec)

	rec = server.send(http.MethodPatch, stepsPath+"/"+step.Id.String(),
		`{"name":"Run payload (revised)"}`, leadCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch step: %d\n%s", rec.Code, rec.Body)
	}
	if got := decodeJSON[gen.Step](t, rec).Name; got != "Run payload (revised)" {
		t.Errorf("patched step name = %q", got)
	}

	rec = server.send(http.MethodPatch, scenarioPath+"/"+scenario.Id.String(),
		`{"name":"Execution (revised)"}`, leadCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch scenario: %d\n%s", rec.Code, rec.Body)
	}
	if got := decodeJSON[gen.Scenario](t, rec).Name; got != "Execution (revised)" {
		t.Errorf("patched scenario name = %q", got)
	}

	// Both patches logged, on the same commits as the changes themselves.
	rec = server.get(BasePath+"/engagements/"+engID+"/activity", leadCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("list activity: %d\n%s", rec.Code, rec.Body)
	}
	verbs := map[string]bool{}
	for _, entry := range decodeJSON[gen.ActivityPage](t, rec).Items {
		verbs[entry.Verb] = true
	}
	for _, want := range []string{"step.updated", "scenario.updated"} {
		if !verbs[want] {
			t.Errorf("no %s activity entry; recorded verbs: %v", want, verbs)
		}
	}
}
