package httpapi

import (
	"context"
	"fmt"

	"github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// ImportPlan imports a CTID emulation plan from the content catalog into a
// new Scenario under an engagement (M3-012).
func (h *handlers) ImportPlan(ctx context.Context,
	request gen.ImportPlanRequestObject) (gen.ImportPlanResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: import plan: missing body")
	}

	body := request.Body

	// Resolve the plan from the content catalog.
	plan, steps, err := h.resolveImportPlan(ctx, body)
	if err != nil {
		return nil, err
	}

	// Build the service input.
	var stepInputs []engagement.ImportPlanStepInput
	for _, s := range steps {
		stepInputs = append(stepInputs, engagement.ImportPlanStepInput{
			Name:                s.Name,
			Description:         s.Description,
			TechniqueExternalID: s.TechniqueExternalID,
			Procedure:           s.Procedure,
		})
	}

	startOrdinal := 1
	if body.StartingOrdinal != nil {
		startOrdinal = *body.StartingOrdinal
	}

	nameOverride := ""
	if body.Name != nil {
		nameOverride = *body.Name
	}

	in := engagement.ImportPlanInput{
		EngagementID:    request.EngagementId.String(),
		PlanName:        plan.Name,
		PlanNarrative:   plan.Description,
		AdversaryName:   plan.AdversaryName,
		PlanExternalID:  plan.ExternalID,
		PlanID:          plan.ID,
		NameOverride:    nameOverride,
		StartingOrdinal: startOrdinal,
		Steps:           stepInputs,
	}

	result, err := h.engagements.ImportPlan(ctx, subject, in)
	if err != nil {
		return nil, err
	}

	// Build the response.
	scenarioWire, err := scenarioToWire(result.Scenario)
	if err != nil {
		return nil, err
	}
	var stepWires []gen.Step
	for _, s := range result.Steps {
		sw, err := stepToWire(s)
		if err != nil {
			return nil, err
		}
		stepWires = append(stepWires, sw)
	}
	var warnWires []gen.ImportPlanWarning
	for _, w := range result.Warnings {
		warnWires = append(warnWires, gen.ImportPlanWarning{
			StepOrdinal:         w.StepOrdinal,
			StepName:            w.StepName,
			TechniqueExternalId: w.TechniqueExternalID,
			Message:             w.Message,
		})
	}

	return gen.ImportPlan201JSONResponse(gen.ImportPlanResponse{
		Scenario:  scenarioWire,
		Steps:     stepWires,
		StepCount: len(result.Steps),
		Warnings:  warnWires,
	}), nil
}

// resolveImportPlan loads a plan from the content catalog by planId or
// (planExternalId + sourceId). At least one form must be provided.
func (h *handlers) resolveImportPlan(ctx context.Context, body *gen.ImportPlanRequest) (storecontent.EmulationPlan, []storecontent.EmulationPlanStep, error) {
	if body.PlanId != nil {
		planID := body.PlanId.String()
		detail, err := h.emulationPlans.DetailByIDEnabled(ctx, planID, true)
		if err != nil {
			return storecontent.EmulationPlan{}, nil, err
		}
		return detail.EmulationPlan, detail.Steps, nil
	}

	if body.PlanExternalId != nil {
		// Look up by external id + source.
		// The source must be provided or default to the CTID source.
		sourceID := storecontent.SourceIDCTID
		if body.SourceId != nil {
			sourceID = body.SourceId.String()
		}

		// List plans matching the external id for this source.
		plans, err := h.emulationPlans.List(ctx, storecontent.EmulationPlanListFilter{
			SourceID: sourceID,
			Version:  storecontent.VersionCurrent,
		})
		if err != nil {
			return storecontent.EmulationPlan{}, nil, fmt.Errorf("import plan: list plans: %w", err)
		}

		for _, p := range plans {
			if p.ExternalID == *body.PlanExternalId {
				detail, err := h.emulationPlans.DetailByIDEnabled(ctx, p.ID, true)
				if err != nil {
					return storecontent.EmulationPlan{}, nil, err
				}
				return detail.EmulationPlan, detail.Steps, nil
			}
		}
		return storecontent.EmulationPlan{}, nil, apierr.NotFound("plan", *body.PlanExternalId)
	}

	return storecontent.EmulationPlan{}, nil, fmt.Errorf("httpapi: import plan: planId or planExternalId is required")
}
