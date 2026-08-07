package httpapi

import (
	"context"
	"fmt"

	"github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// CreateStepFromTemplate creates a step from a content procedure template (M3-013).
func (h *handlers) CreateStepFromTemplate(ctx context.Context,
	request gen.CreateStepFromTemplateRequestObject) (gen.CreateStepFromTemplateResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: create step from template: missing body")
	}

	body := request.Body

	// Resolve the template from the content catalog.
	tmpl, err := h.resolveTemplate(ctx, body.TemplateId.String())
	if err != nil {
		return nil, err
	}

	// Build arg values from the request.
	argValues := make(map[string]string)
	if body.ArgValues != nil {
		for k, v := range *body.ArgValues {
			argValues[k] = v
		}
	}

	in := engagement.CreateStepFromTemplateInput{
		EngagementID: request.EngagementId.String(),
		ScenarioID:   request.ScenarioId.String(),
		Template:     tmpl,
		ArgValues:    argValues,
	}
	if body.Name != nil {
		in.Name = *body.Name
	}
	if body.Objective != nil {
		in.Objective = *body.Objective
	}
	if body.TargetAsset != nil {
		in.TargetAsset = *body.TargetAsset
	}

	step, err := h.engagements.CreateStepFromTemplate(ctx, subject, in)
	if err != nil {
		return nil, err
	}

	w, err := stepToWire(step)
	if err != nil {
		return nil, err
	}
	return gen.CreateStepFromTemplate201JSONResponse(w), nil
}

// resolveTemplate loads a procedure template by UUID from the content catalog.
// When the template's source is disabled, ByIDEnabled returns not-found so the
// caller gets 404 — matching the contract that disabled sources refuse new
// references at the content layer (M2-EPIC).
func (h *handlers) resolveTemplate(ctx context.Context, templateID string) (storecontent.ProcedureTemplate, error) {
	tmpl, err := h.procedures.ByIDEnabled(ctx, templateID, true)
	if err != nil {
		return storecontent.ProcedureTemplate{}, fmt.Errorf("httpapi: resolve template %s: %w", templateID, err)
	}
	return tmpl, nil
}
