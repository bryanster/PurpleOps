package httpapi

import (
	"context"
	"fmt"

	"github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// Scenario CRUD + reorder handlers (M3-004).

// ListScenarios returns every scenario in an engagement.
func (h *handlers) ListScenarios(ctx context.Context,
	request gen.ListScenariosRequestObject) (gen.ListScenariosResponseObject, error) {

	scenarios, err := h.engagements.ListScenarios(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	var items []gen.Scenario
	for _, sc := range scenarios {
		w, err := scenarioToWire(sc)
		if err != nil {
			return nil, err
		}
		items = append(items, w)
	}

	return gen.ListScenarios200JSONResponse(gen.ScenarioList{Items: items}), nil
}

// CreateScenario creates a new scenario in an engagement.
func (h *handlers) CreateScenario(ctx context.Context,
	request gen.CreateScenarioRequestObject) (gen.CreateScenarioResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: create scenario: missing body")
	}

	in := engagement.CreateScenarioInput{
		EngagementID: request.EngagementId.String(),
		Name:         request.Body.Name,
	}
	if request.Body.Narrative != nil {
		in.Narrative = *request.Body.Narrative
	}
	if request.Body.ThreatActor != nil {
		in.ThreatActor = *request.Body.ThreatActor
	}
	if request.Body.Source != nil {
		in.Source = storengagement.ScenarioSource(*request.Body.Source)
	}
	if request.Body.SourceRef != nil {
		in.SourceRef = *request.Body.SourceRef
	}

	scenario, err := h.engagements.CreateScenario(ctx, subject, in)
	if err != nil {
		return nil, err
	}

	w, err := scenarioToWire(scenario)
	if err != nil {
		return nil, err
	}
	return gen.CreateScenario201JSONResponse(w), nil
}

// GetScenario returns one scenario.
func (h *handlers) GetScenario(ctx context.Context,
	request gen.GetScenarioRequestObject) (gen.GetScenarioResponseObject, error) {

	scenario, err := h.engagements.GetScenarioInEngagement(ctx, request.EngagementId.String(), request.ScenarioId.String())
	if err != nil {
		return nil, err
	}

	w, err := scenarioToWire(scenario)
	if err != nil {
		return nil, err
	}
	return gen.GetScenario200JSONResponse(w), nil
}

// PatchScenario patches one scenario's fields.
func (h *handlers) PatchScenario(ctx context.Context,
	request gen.PatchScenarioRequestObject) (gen.PatchScenarioResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: patch scenario: missing body")
	}

	in := engagement.PatchScenarioInput{
		Name:        request.Body.Name,
		Narrative:   request.Body.Narrative,
		ThreatActor: request.Body.ThreatActor,
	}

	scenario, err := h.engagements.PatchScenario(ctx, subject, request.EngagementId.String(), request.ScenarioId.String(), in)
	if err != nil {
		return nil, err
	}

	w, err := scenarioToWire(scenario)
	if err != nil {
		return nil, err
	}
	return gen.PatchScenario200JSONResponse(w), nil
}

// DeleteScenario removes a scenario and its children.
func (h *handlers) DeleteScenario(ctx context.Context,
	request gen.DeleteScenarioRequestObject) (gen.DeleteScenarioResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.engagements.DeleteScenario(ctx, subject, request.EngagementId.String(), request.ScenarioId.String()); err != nil {
		return nil, err
	}

	return gen.DeleteScenario204Response{}, nil
}

// ReorderScenarios reassigns ordinals to match the given order.
func (h *handlers) ReorderScenarios(ctx context.Context,
	request gen.ReorderScenariosRequestObject) (gen.ReorderScenariosResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: reorder scenarios: missing body")
	}

	ids := make([]string, len(request.Body.Ids))
	for i, id := range request.Body.Ids {
		ids[i] = id.String()
	}

	scenarios, err := h.engagements.ReorderScenarios(ctx, subject, request.EngagementId.String(), ids)
	if err != nil {
		return nil, err
	}

	var items []gen.Scenario
	for _, sc := range scenarios {
		w, err := scenarioToWire(sc)
		if err != nil {
			return nil, err
		}
		items = append(items, w)
	}

	return gen.ReorderScenarios200JSONResponse(gen.ScenarioList{Items: items}), nil
}

// scenarioToWire converts a store scenario to its OpenAPI representation.
func scenarioToWire(s storengagement.Scenario) (gen.Scenario, error) {
	id, err := parseUUID(s.ID)
	if err != nil {
		return gen.Scenario{}, err
	}
	engagementID, err := parseUUID(s.EngagementID)
	if err != nil {
		return gen.Scenario{}, err
	}
	out := gen.Scenario{
		Id:           id,
		EngagementId: engagementID,
		Ordinal:      s.Ordinal,
		Name:         s.Name,
		Narrative:    s.Narrative,
		Source:       gen.ScenarioSource(s.Source),
		CreatedAt:    s.CreatedAt,
		UpdatedAt:    s.UpdatedAt,
	}
	if s.ThreatActor != "" {
		out.ThreatActor = &s.ThreatActor
	}
	if s.SourceRef != "" {
		out.SourceRef = &s.SourceRef
	}
	return out, nil
}
