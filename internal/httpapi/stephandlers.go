package httpapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/blind"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// Step CRUD, reorder and reveal handlers (M3-005).

// ListSteps returns every step in a scenario, blind-filtered.
func (h *handlers) ListSteps(ctx context.Context,
	request gen.ListStepsRequestObject) (gen.ListStepsResponseObject, error) {

	scope, err := h.stepBlindScope(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	steps, err := h.engagements.ListSteps(ctx, request.ScenarioId.String(), scope)
	if err != nil {
		return nil, err
	}

	items, err := stepsToWire(steps, scope)
	if err != nil {
		return nil, err
	}

	return gen.ListSteps200JSONResponse(gen.StepList{Items: items}), nil
}

// CreateStep creates a new step and a pending execution in one transaction.
func (h *handlers) CreateStep(ctx context.Context,
	request gen.CreateStepRequestObject) (gen.CreateStepResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: create step: missing body")
	}

	in := engagement.CreateStepInput{
		ScenarioID: request.ScenarioId.String(),
		Name:       request.Body.Name,
	}
	if request.Body.Objective != nil {
		in.Objective = *request.Body.Objective
	}
	if request.Body.TechniqueId != nil {
		in.TechniqueID = *request.Body.TechniqueId
	}
	if request.Body.SubtechniqueId != nil {
		in.SubtechniqueID = *request.Body.SubtechniqueId
	}
	if request.Body.TacticId != nil {
		in.TacticID = *request.Body.TacticId
	}
	if request.Body.TechniqueExternalId != nil {
		in.TechniqueExternalID = *request.Body.TechniqueExternalId
	}
	if request.Body.Procedure != nil {
		raw, err := json.Marshal(request.Body.Procedure)
		if err != nil {
			return nil, fmt.Errorf("httpapi: create step: procedure: %w", err)
		}
		in.Procedure = raw
	}
	if request.Body.TemplateId != nil {
		in.TemplateID = *request.Body.TemplateId
	}
	if request.Body.TargetAsset != nil {
		in.TargetAsset = *request.Body.TargetAsset
	}
	if request.Body.Tools != nil {
		raw, err := json.Marshal(request.Body.Tools)
		if err != nil {
			return nil, fmt.Errorf("httpapi: create step: tools: %w", err)
		}
		in.Tools = raw
	}
	if request.Body.ControlsInScope != nil {
		raw, err := json.Marshal(request.Body.ControlsInScope)
		if err != nil {
			return nil, fmt.Errorf("httpapi: create step: controls_in_scope: %w", err)
		}
		in.ControlsInScope = raw
	}

	step, err := h.engagements.CreateStep(ctx, subject, in)
	if err != nil {
		return nil, err
	}

	w, err := stepToWire(step)
	if err != nil {
		return nil, err
	}
	return gen.CreateStep201JSONResponse(w), nil
}

// GetStep returns one step. Blind engagements: 404-conceal for unrevealed steps to blue.
func (h *handlers) GetStep(ctx context.Context,
	request gen.GetStepRequestObject) (gen.GetStepResponseObject, error) {

	scope, err := h.stepBlindScope(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	step, err := h.engagements.GetStep(ctx, request.StepId.String())
	if err != nil {
		return nil, err
	}

	// Check blind: if scope withholds and step is not revealed, conceal as 404.
	if scope.Withholds() && step.RevealedAt == nil {
		return gen.GetStep404ApplicationProblemPlusJSONResponse{}, nil
	}

	w, err := stepToWire(step)
	if err != nil {
		return nil, err
	}
	return gen.GetStep200JSONResponse(w), nil
}

// PatchStep patches a step's always-editable fields. Soft freeze enforced.
func (h *handlers) PatchStep(ctx context.Context,
	request gen.PatchStepRequestObject) (gen.PatchStepResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: patch step: missing body")
	}

	in := engagement.PatchStepInput{
		Name:        request.Body.Name,
		Objective:   request.Body.Objective,
		TargetAsset: request.Body.TargetAsset,
	}
	if request.Body.Tools != nil {
		raw, err := json.Marshal(request.Body.Tools)
		if err != nil {
			return nil, fmt.Errorf("httpapi: patch step: tools: %w", err)
		}
		in.Tools = raw
	}
	if request.Body.ControlsInScope != nil {
		raw, err := json.Marshal(request.Body.ControlsInScope)
		if err != nil {
			return nil, fmt.Errorf("httpapi: patch step: controls_in_scope: %w", err)
		}
		in.ControlsInScope = raw
	}

	step, err := h.engagements.PatchStep(ctx, subject, request.StepId.String(), in)
	if err != nil {
		return nil, err
	}

	w, err := stepToWire(step)
	if err != nil {
		return nil, err
	}
	return gen.PatchStep200JSONResponse(w), nil
}

// DeleteStep removes a step and its child graph.
func (h *handlers) DeleteStep(ctx context.Context,
	request gen.DeleteStepRequestObject) (gen.DeleteStepResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.engagements.DeleteStep(ctx, subject, request.StepId.String()); err != nil {
		return nil, err
	}

	return gen.DeleteStep204Response{}, nil
}

// ReorderSteps reassigns ordinals to match the given order.
func (h *handlers) ReorderSteps(ctx context.Context,
	request gen.ReorderStepsRequestObject) (gen.ReorderStepsResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: reorder steps: missing body")
	}

	ids := make([]string, len(request.Body.Ids))
	for i, id := range request.Body.Ids {
		ids[i] = id.String()
	}

	steps, err := h.engagements.ReorderSteps(ctx, subject, request.ScenarioId.String(), ids)
	if err != nil {
		return nil, err
	}

	scope, err := h.stepBlindScope(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	items, err := stepsToWire(steps, scope)
	if err != nil {
		return nil, err
	}

	return gen.ReorderSteps200JSONResponse(gen.StepList{Items: items}), nil
}

// RevealStep sets revealed_at to now. Idempotent.
func (h *handlers) RevealStep(ctx context.Context,
	request gen.RevealStepRequestObject) (gen.RevealStepResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	step, err := h.engagements.RevealStep(ctx, subject, request.StepId.String())
	if err != nil {
		return nil, err
	}

	w, err := stepToWire(step)
	if err != nil {
		return nil, err
	}
	return gen.RevealStep200JSONResponse(w), nil
}

// ListEngagementSteps returns every step across all scenarios in an engagement, blind-filtered.
func (h *handlers) ListEngagementSteps(ctx context.Context,
	request gen.ListEngagementStepsRequestObject) (gen.ListEngagementStepsResponseObject, error) {

	scope, err := h.stepBlindScope(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	steps, err := h.engagements.ListEngagementSteps(ctx, request.EngagementId.String(), scope)
	if err != nil {
		return nil, err
	}

	items, err := stepsToWire(steps, scope)
	if err != nil {
		return nil, err
	}

	return gen.ListEngagementSteps200JSONResponse(gen.StepList{Items: items}), nil
}

// stepBlindScope builds a blind.Scope for the given engagement from the
// authorization context. It reads the engagement to determine its mode and
// matches the caller's seat to decide whether they are held to blind mode.
//
// For Self operations (e.g. SSE subscribe), the authorization Subject may not
// carry cached memberships. This function falls back to the ownership store
// when the cached map is empty (M4-009).
func (h *handlers) stepBlindScope(ctx context.Context, engagementID string) (blind.Scope, error) {
	eng, err := h.engagements.Get(ctx, engagementID)
	if err != nil {
		return blind.Scope{}, err
	}

	auth, ok := authorizationFrom(ctx)
	if !ok {
		// No auth context -- treat as not blind (admin/internal path).
		return blind.Scope{}, nil
	}

	seat, member := auth.Subject.MembershipIn(engagementID)
	if !member && h.ownership != nil {
		// Self operations (like SSE subscribe) don't carry cached
		// memberships in the authorization Subject. Load from store.
		if s, m, err := h.ownership.Seat(ctx, engagementID, auth.Subject.UserID); err == nil && m {
			seat = s
		}
	}

	return blind.Scope{
		Blind: eng.Mode == storengagement.EngagementModeBlind,
		Seat:  seat,
	}, nil
}

// stepToWire converts a store step to its OpenAPI representation.
func stepToWire(s storengagement.Step) (gen.Step, error) {
	id, err := parseUUID(s.ID)
	if err != nil {
		return gen.Step{}, err
	}
	scenarioID, err := parseUUID(s.ScenarioID)
	if err != nil {
		return gen.Step{}, err
	}

	out := gen.Step{
		Id:            id,
		ScenarioId:    scenarioID,
		Ordinal:       s.Ordinal,
		Name:          s.Name,
		Objective:     s.Objective,
		AttackVersion: s.AttackVersion,
		CreatedAt:     s.CreatedAt,
		UpdatedAt:     s.UpdatedAt,
		TemplateId:    s.TemplateID,
	}

	if s.TechniqueID != "" {
		out.TechniqueId = &s.TechniqueID
	}
	if s.SubtechniqueID != "" {
		out.SubtechniqueId = &s.SubtechniqueID
	}
	if s.TacticID != "" {
		out.TacticId = &s.TacticID
	}
	if len(s.Procedure) > 0 {
		var proc map[string]interface{}
		if err := json.Unmarshal(s.Procedure, &proc); err == nil {
			out.Procedure = &proc
		}
	}
	if s.TargetAsset != "" {
		out.TargetAsset = s.TargetAsset
	}
	if s.RevealedAt != nil {
		out.RevealedAt = s.RevealedAt
	}
	if tools := string(s.Tools); tools != "[]" && tools != "" && tools != "null" {
		var t []string
		if err := json.Unmarshal(s.Tools, &t); err == nil {
			out.Tools = &t
		}
	}
	if controls := string(s.ControlsInScope); controls != "[]" && controls != "" && controls != "null" {
		var c []string
		if err := json.Unmarshal(s.ControlsInScope, &c); err == nil {
			out.ControlsInScope = &c
		}
	}

	return out, nil
}

// stepsToWire converts a slice of store steps, filtering by blind scope.
func stepsToWire(steps []storengagement.Step, scope blind.Scope) ([]gen.Step, error) {
	var items = make([]gen.Step, 0)
	for _, s := range steps {
		// Filter out unrevealed steps for blue in blind mode.
		if scope.Withholds() && s.RevealedAt == nil {
			continue
		}
		w, err := stepToWire(s)
		if err != nil {
			return nil, err
		}
		items = append(items, w)
	}
	return items, nil
}
