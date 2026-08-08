package engagement

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/content/attackpin"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// ImportPlanStepInput is one step to import from a content catalog plan.
type ImportPlanStepInput struct {
	Name                string
	Description         string
	TechniqueExternalID string
	Procedure           json.RawMessage
}

// ImportPlanInput is the caller's half of importing a CTID plan as a Scenario.
type ImportPlanInput struct {
	EngagementID    string
	PlanName        string
	PlanNarrative   string
	AdversaryName   string
	PlanExternalID  string
	PlanID          string // content catalog surrogate id for lineage
	NameOverride    string
	StartingOrdinal int // 0 means default to 1
	Steps           []ImportPlanStepInput
}

// ImportStepWarning describes a step whose technique did not resolve.
type ImportStepWarning struct {
	StepOrdinal         int
	StepName            string
	TechniqueExternalID string
	Message             string
}

// ImportPlanResult is the outcome of a plan import.
type ImportPlanResult struct {
	Scenario storengagement.Scenario
	Steps    []storengagement.Step
	Warnings []ImportStepWarning
}

// ImportPlan imports a CTID emulation plan as a new Scenario with Steps and
// pending Executions. Closed/archived engagements are refused. If any step
// has a technique_external_id, the engagement's ATT&CK pin is asserted first.
// Technique ids that do not resolve in the pinned version produce warnings
// but do not block the import — those steps are created with empty technique
// fields.
func (s *Service) ImportPlan(ctx context.Context, actor authn.Subject, in ImportPlanInput) (ImportPlanResult, error) {
	eng, err := s.engagements.ByID(ctx, in.EngagementID)
	if err != nil {
		return ImportPlanResult{}, err
	}
	if eng.Status == storengagement.EngagementStatusClosed || eng.Status == storengagement.EngagementStatusArchived {
		return ImportPlanResult{}, apierr.Conflict(fmt.Sprintf("engagement is %s", eng.Status))
	}

	attackVersion := eng.AttackVersion
	hasTechnique := false
	for _, step := range in.Steps {
		if step.TechniqueExternalID != "" {
			hasTechnique = true
			break
		}
	}
	if hasTechnique {
		if s.attackpin != nil {
			if err := s.attackpin.AssertPinned(ctx, attackVersion); err != nil {
				return ImportPlanResult{}, attackpin.MapError(err)
			}
		}
	}

	// Sanitise inputs.
	name := in.PlanName
	if in.NameOverride != "" {
		name = in.NameOverride
	}
	narrative := in.PlanNarrative
	startingOrdinal := in.StartingOrdinal
	if startingOrdinal < 1 {
		startingOrdinal = 1
	}

	// Build source_ref: plan external_id plus weak plan_id lineage.
	sourceRef := in.PlanExternalID

	// Create the scenario.
	scenario, err := s.CreateScenario(ctx, actor, CreateScenarioInput{
		EngagementID: in.EngagementID,
		Name:         name,
		Narrative:    narrative,
		ThreatActor:  in.AdversaryName,
		Source:       storengagement.ScenarioSourceCTID,
		SourceRef:    sourceRef,
	})
	if err != nil {
		return ImportPlanResult{}, fmt.Errorf("import: create scenario: %w", err)
	}

	// Record the import activity on the scenario.
	s.recordActivity(ctx, actor.UserID, in.EngagementID,
		events.VerbScenarioImported, events.ObjectScenario, scenario.ID,
		map[string]any{
			"plan_id":          in.PlanID,
			"plan_external_id": in.PlanExternalID,
			"step_count":       len(in.Steps),
			"name":             name,
		},
	)

	// Create steps with pending executions.
	var steps []storengagement.Step
	var warnings []ImportStepWarning
	for i, stepIn := range in.Steps {
		ord := startingOrdinal + i

		// Try to resolve technique if present.
		var techID, subtechID, tacticID string
		var proc json.RawMessage
		var warn *ImportStepWarning
		// Try to resolve technique if present.
		if stepIn.TechniqueExternalID != "" && s.attackpin != nil {
			tech, err := s.attackpin.ResolveTechnique(ctx, attackVersion, stepIn.TechniqueExternalID)
			if err != nil {
				warn = &ImportStepWarning{
					StepOrdinal:         ord,
					StepName:            stepIn.Name,
					TechniqueExternalID: stepIn.TechniqueExternalID,
					Message:             fmt.Sprintf("technique %s not found in ATT&CK version %s", stepIn.TechniqueExternalID, attackVersion),
				}
			} else {
				techID = tech.ExternalID
			}
		}

		// Build the step procedure: merge catalog description into procedure JSON.
		proc = stepIn.Procedure
		if proc == nil || string(proc) == "null" {
			proc = json.RawMessage(`{}`)
		}
		// If there's a description, add it to the procedure payload.
		if stepIn.Description != "" {
			var procMap map[string]any
			if err := json.Unmarshal(proc, &procMap); err != nil {
				procMap = make(map[string]any)
			}
			procMap["description"] = stepIn.Description
			if stepIn.Name != "" {
				procMap["name"] = stepIn.Name
			}
			raw, err := json.Marshal(procMap)
			if err != nil {
				return ImportPlanResult{}, fmt.Errorf("import: marshal procedure: %w", err)
			}
			proc = raw
		}

		step, _, err := s.steps.CreateWithExecution(ctx, storengagement.NewStep{
			ScenarioID:      scenario.ID,
			Ordinal:         ord,
			Name:            stepIn.Name,
			Objective:       "",
			TechniqueID:     techID,
			SubtechniqueID:  subtechID,
			TacticID:        tacticID,
			Procedure:       proc,
			TemplateID:      "",
			TargetAsset:     "",
			Tools:           json.RawMessage(`[]`),
			ControlsInScope: json.RawMessage(`[]`),
			AttackVersion:   attackVersion,
		})
		if err != nil {
			return ImportPlanResult{}, fmt.Errorf("import: create step %d (%s): %w", ord, stepIn.Name, err)
		}
		steps = append(steps, step)

		if warn != nil {
			warnings = append(warnings, *warn)
		}
	}

	return ImportPlanResult{
		Scenario: scenario,
		Steps:    steps,
		Warnings: warnings,
	}, nil
}
