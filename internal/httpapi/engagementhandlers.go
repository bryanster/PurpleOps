package httpapi

import (
	"context"
	"fmt"
	"time"

	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// Engagement CRUD handlers (M3-002).

// ListEngagements returns a page of engagements the caller can see.
func (h *handlers) ListEngagements(ctx context.Context,
	request gen.ListEngagementsRequestObject) (gen.ListEngagementsResponseObject, error) {

	var filter engagement.ListFilter
	if request.Params.Status != nil {
		filter.Status = string(*request.Params.Status)
	}
	if request.Params.Cursor != nil {
		filter.After = *request.Params.Cursor
	}
	filter.Limit = limitParam(request.Params.Limit)

	actor, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	items, err := h.engagements.List(ctx, actor, filter)
	if err != nil {
		return nil, err
	}

	out := make([]gen.Engagement, 0, len(items))
	for _, e := range items {
		w, err := engagementToWire(e)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}

	page := gen.EngagementPage{Items: out}
	if len(items) >= filter.Limit {
		last := items[len(items)-1]
		page.NextCursor = nullable.NewNullableWithValue(last.ID)
	}

	return gen.ListEngagements200JSONResponse(page), nil
}

// CreateEngagement creates a new engagement with the caller as lead.
func (h *handlers) CreateEngagement(ctx context.Context,
	request gen.CreateEngagementRequestObject) (gen.CreateEngagementResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: create engagement: missing body")
	}

	input := engagement.CreateInput{
		Name:              request.Body.Name,
		Client:            stringDefault(request.Body.Client, ""),
		Description:       stringDefault(request.Body.Description, ""),
		AttackVersion:     request.Body.AttackVersion,
		AutoRevealOnStart: boolDefault(request.Body.AutoRevealOnStart, false),
	}

	if request.Body.Mode != nil {
		input.Mode = storengagement.EngagementMode(*request.Body.Mode)
	} else {
		input.Mode = storengagement.EngagementModeStandard
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	if request.Body.StartsOn != nil {
		input.StartsOn = request.Body.StartsOn.Time
	} else {
		input.StartsOn = today
	}
	if request.Body.EndsOn != nil {
		input.EndsOn = request.Body.EndsOn.Time
	} else {
		input.EndsOn = today
	}

	e, err := h.engagements.Create(ctx, subject, input)
	if err != nil {
		return nil, err
	}
	w, err := engagementToWire(e)
	if err != nil {
		return nil, err
	}
	return gen.CreateEngagement201JSONResponse(w), nil
}

// GetEngagement returns one engagement.
func (h *handlers) GetEngagement(ctx context.Context,
	request gen.GetEngagementRequestObject) (gen.GetEngagementResponseObject, error) {

	e, err := h.engagements.Get(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}
	w, err := engagementToWire(e)
	if err != nil {
		return nil, err
	}
	return gen.GetEngagement200JSONResponse(w), nil
}

// PatchEngagement patches one engagement's fields.
func (h *handlers) PatchEngagement(ctx context.Context,
	request gen.PatchEngagementRequestObject) (gen.PatchEngagementResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: patch engagement: missing body")
	}

	input := engagement.UpdateInput{
		Name:              request.Body.Name,
		Client:            request.Body.Client,
		Description:       request.Body.Description,
		AutoRevealOnStart: request.Body.AutoRevealOnStart,
	}
	if request.Body.AttackVersion != nil {
		input.AttackVersion = request.Body.AttackVersion
	}
	if request.Body.StartsOn != nil {
		input.StartsOn = &request.Body.StartsOn.Time
	}
	if request.Body.EndsOn != nil {
		input.EndsOn = &request.Body.EndsOn.Time
	}
	if request.Body.Mode != nil {
		m := storengagement.EngagementMode(*request.Body.Mode)
		input.Mode = &m
	}

	e, err := h.engagements.Update(ctx, subject, request.EngagementId.String(), input)
	if err != nil {
		return nil, err
	}
	w, err := engagementToWire(e)
	if err != nil {
		return nil, err
	}
	return gen.PatchEngagement200JSONResponse(w), nil
}

// DeleteEngagement removes an engagement and its whole graph.
func (h *handlers) DeleteEngagement(ctx context.Context,
	request gen.DeleteEngagementRequestObject) (gen.DeleteEngagementResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.engagements.Delete(ctx, subject, request.EngagementId.String()); err != nil {
		return nil, err
	}
	return gen.DeleteEngagement204Response{}, nil
}

// SetEngagementStatus transitions an engagement to a new lifecycle state.
func (h *handlers) SetEngagementStatus(ctx context.Context,
	request gen.SetEngagementStatusRequestObject) (gen.SetEngagementStatusResponseObject, error) {

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}
	if request.Body == nil {
		return nil, fmt.Errorf("httpapi: set engagement status: missing body")
	}

	e, err := h.engagements.SetStatus(ctx, subject,
		request.EngagementId.String(),
		storengagement.EngagementStatus(request.Body.Status),
	)
	if err != nil {
		return nil, err
	}
	w, err := engagementToWire(e)
	if err != nil {
		return nil, err
	}
	return gen.SetEngagementStatus200JSONResponse(w), nil
}

// engagementToWire converts a store engagement to its OpenAPI representation.
func engagementToWire(e storengagement.Engagement) (gen.Engagement, error) {
	id, err := parseUUID(e.ID)
	if err != nil {
		return gen.Engagement{}, err
	}
	createdBy, err := parseUUID(e.CreatedBy)
	if err != nil {
		return gen.Engagement{}, err
	}
	return gen.Engagement{
		Id:                id,
		Name:              e.Name,
		Client:            e.Client,
		Description:       e.Description,
		Status:            gen.EngagementStatus(e.Status),
		StartsOn:          openapi_types.Date{Time: e.StartsOn},
		EndsOn:            openapi_types.Date{Time: e.EndsOn},
		AttackVersion:     e.AttackVersion,
		Mode:              gen.EngagementMode(e.Mode),
		AutoRevealOnStart: e.AutoRevealOnStart,
		CreatedBy:         createdBy,
		CreatedAt:         e.CreatedAt,
		UpdatedAt:         e.UpdatedAt,
	}, nil
}

func boolDefault(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

func stringDefault(v *string, def string) string {
	if v == nil {
		return def
	}
	return *v
}
