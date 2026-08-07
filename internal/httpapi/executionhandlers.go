package httpapi

import (
	"context"
	"encoding/json"

	"github.com/oapi-codegen/nullable"

	"github.com/bryanster/blacklight/internal/authn"
	engagement "github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// Execution read and red-side write handlers (M3-006).

// GetExecution returns one execution by id. In blind mode, blue members
// receive 404-conceal for executions belonging to unrevealed steps.
func (h *handlers) GetExecution(ctx context.Context,
	request gen.GetExecutionRequestObject) (gen.GetExecutionResponseObject, error) {

	exec, err := h.engagements.GetExecution(ctx, request.ExecutionId.String())
	if err != nil {
		return nil, err
	}

	// Blind check: load the step to check revealed status for blind mode.
	step, err := h.engagements.GetStep(ctx, exec.StepID)
	if err != nil {
		return nil, err
	}
	scope, err := h.stepBlindScope(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}
	if !scope.Permits(step.RevealedAt != nil) {
		return nil, apierr.NotFound("execution", request.ExecutionId.String())
	}

	wire, err := executionToWire(exec)
	if err != nil {
		return nil, err
	}
	return gen.GetExecution200JSONResponse(wire), nil
}

// ListEngagementExecutions returns executions in an engagement, filtered
// by scenario and/or status. Blind-filtered for blue members.
func (h *handlers) ListEngagementExecutions(ctx context.Context,
	request gen.ListEngagementExecutionsRequestObject) (gen.ListEngagementExecutionsResponseObject, error) {

	var scenarioID *string
	if request.Params.ScenarioId != nil {
		s := request.Params.ScenarioId.String()
		scenarioID = &s
	}

	var status *storengagement.ExecutionStatus
	if request.Params.Status != nil {
		s := storengagement.ExecutionStatus(string(*request.Params.Status))
		status = &s
	}

	execs, err := h.engagements.ListEngagementExecutions(ctx, request.EngagementId.String(), scenarioID, status)
	if err != nil {
		return nil, err
	}

	// Apply blind filtering.
	scope, err := h.stepBlindScope(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	var items []gen.Execution
	for _, exec := range execs {
		step, err := h.engagements.GetStep(ctx, exec.StepID)
		if err != nil {
			return nil, err
		}
		if !scope.Permits(step.RevealedAt != nil) {
			continue
		}
		wire, err := executionToWire(exec)
		if err != nil {
			return nil, err
		}
		items = append(items, wire)
	}

	return gen.ListEngagementExecutions200JSONResponse(gen.ExecutionList{Items: items}), nil
}

// PatchRedExecution writes the red side of an execution. Optimistic locking,
// status transition validation, auto-reveal and activity are handled by the
// domain layer.
func (h *handlers) PatchRedExecution(ctx context.Context,
	request gen.PatchRedExecutionRequestObject) (gen.PatchRedExecutionResponseObject, error) {

	actor, ok := authn.SubjectFrom(ctx)
	if !ok {
		return nil, apierr.Unauthenticated("no subject")
	}

	in := engagement.PatchRedExecutionInput{
		Version: request.Body.Version,
	}

	if request.Body.Status != nil {
		s := storengagement.ExecutionStatus(string(*request.Body.Status))
		in.Status = &s
	}
	if request.Body.StartedAt != nil {
		in.StartedAt = request.Body.StartedAt
	}
	if request.Body.EndedAt != nil {
		in.EndedAt = request.Body.EndedAt
	}
	if request.Body.CommandRun != nil {
		in.CommandRun = request.Body.CommandRun
	}
	if request.Body.SourceHost != nil {
		in.SourceHost = request.Body.SourceHost
	}
	if request.Body.TargetHost != nil {
		in.TargetHost = request.Body.TargetHost
	}
	if request.Body.RedNotes != nil {
		in.RedNotes = request.Body.RedNotes
	}

	exec, err := h.engagements.PatchRedExecution(ctx, actor, request.ExecutionId.String(), in)
	if err != nil {
		return nil, err
	}

	wire, err := executionToWire(exec)
	if err != nil {
		return nil, err
	}
	return gen.PatchRedExecution200JSONResponse(wire), nil
}

// executionToWire converts a store execution to its OpenAPI representation.
func executionToWire(e storengagement.Execution) (gen.Execution, error) {
	mods := make([]string, 0)
	if len(e.DetectionModifiers) > 0 {
		if err := json.Unmarshal(e.DetectionModifiers, &mods); err != nil {
			mods = []string{}
		}
	}

	id, err := parseUUID(e.ID)
	if err != nil {
		return gen.Execution{}, err
	}
	stepID, err := parseUUID(e.StepID)
	if err != nil {
		return gen.Execution{}, err
	}

	wire := gen.Execution{
		Id:                 id,
		StepId:             stepID,
		Version:            e.Version,
		Status:             gen.ExecutionStatus(e.Status),
		ExecutedBy:         e.ExecutedBy,
		CommandRun:         e.CommandRun,
		SourceHost:         e.SourceHost,
		TargetHost:         e.TargetHost,
		RedNotes:           e.RedNotes,
		DetectionModifiers: mods,
		DetectingSource:    e.DetectingSource,
		DetectingRuleRef:   e.DetectingRuleRef,
		AlertSeverity:      e.AlertSeverity,
		BlueNotes:          e.BlueNotes,
		ScoredBy:           e.ScoredBy,
		CreatedAt:          e.CreatedAt,
		UpdatedAt:          e.UpdatedAt,
	}

	if e.StartedAt != nil {
		wire.StartedAt = e.StartedAt
	}
	if e.EndedAt != nil {
		wire.EndedAt = e.EndedAt
	}
	if e.DetectionCategory != nil {
		wire.DetectionCategory = nullable.NewNullableWithValue(gen.ExecutionDetectionCategory(string(*e.DetectionCategory)))
	}
	if e.Protection != nil {
		wire.Protection = nullable.NewNullableWithValue(gen.ExecutionProtection(string(*e.Protection)))
	}
	if e.DetectedAt != nil {
		wire.DetectedAt = e.DetectedAt
	}
	if e.ScoredAt != nil {
		wire.ScoredAt = e.ScoredAt
	}

	return wire, nil
}
