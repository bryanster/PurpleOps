package httpapi

import (
	"context"
	"encoding/json"

	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/activity"
)

// The activity endpoints (M1-015). Authorization is decided by the middleware
// from api/openapi.yaml — activity.read for the platform feed, engagement.read
// for an engagement's. This file lists rows and nothing else.

// ListActivity returns a page of the installation-wide activity log.
func (h *handlers) ListActivity(ctx context.Context,
	request gen.ListActivityRequestObject) (gen.ListActivityResponseObject, error) {
	page, err := h.listActivity(ctx, activity.ListFilter{
		ScopePlatform: true,
		ActorID:       uuidParam(request.Params.Actor),
		Verb:          stringParam(request.Params.Verb),
		ObjectType:    stringParam(request.Params.ObjectType),
		ObjectID:      stringParam(request.Params.ObjectId),
		Cursor:        stringParam(request.Params.Cursor),
		Limit:         limitParam(request.Params.Limit),
	})
	if err != nil {
		return nil, err
	}
	return gen.ListActivity200JSONResponse(page), nil
}

// ListEngagementActivity returns a page of one engagement's activity log.
func (h *handlers) ListEngagementActivity(ctx context.Context,
	request gen.ListEngagementActivityRequestObject) (gen.ListEngagementActivityResponseObject, error) {
	page, err := h.listActivity(ctx, activity.ListFilter{
		ScopeEngagement: request.EngagementId.String(),
		ActorID:         uuidParam(request.Params.Actor),
		Verb:            stringParam(request.Params.Verb),
		ObjectType:      stringParam(request.Params.ObjectType),
		ObjectID:        stringParam(request.Params.ObjectId),
		Cursor:          stringParam(request.Params.Cursor),
		Limit:           limitParam(request.Params.Limit),
	})
	if err != nil {
		return nil, err
	}
	return gen.ListEngagementActivity200JSONResponse(page), nil
}

func (h *handlers) listActivity(ctx context.Context, filter activity.ListFilter) (gen.ActivityPage, error) {
	if h.activity == nil {
		return gen.ActivityPage{Items: []gen.ActivityEntry{}}, nil
	}
	rows, next, err := h.activity.Entries().List(ctx, filter)
	if err != nil {
		return gen.ActivityPage{}, err
	}
	items := make([]gen.ActivityEntry, 0, len(rows))
	for _, row := range rows {
		entry, err := activityEntry(row)
		if err != nil {
			return gen.ActivityPage{}, err
		}
		items = append(items, entry)
	}
	page := gen.ActivityPage{Items: items}
	if next != "" {
		page.NextCursor = nullable.NewNullableWithValue(next)
	}
	return page, nil
}

func activityEntry(row activity.Row) (gen.ActivityEntry, error) {
	id, err := parseUUID(row.ID)
	if err != nil {
		return gen.ActivityEntry{}, err
	}
	entry := gen.ActivityEntry{
		Id:         id,
		Verb:       row.Verb,
		ObjectType: row.ObjectType,
		ObjectId:   row.ObjectID,
		At:         row.At,
	}
	if row.ActorID != "" {
		actor, err := parseUUID(row.ActorID)
		if err != nil {
			return gen.ActivityEntry{}, err
		}
		entry.ActorId = &actor
	}
	if row.EngagementID != "" {
		eng, err := parseUUID(row.EngagementID)
		if err != nil {
			return gen.ActivityEntry{}, err
		}
		entry.EngagementId = &eng
	}
	if len(row.Delta) > 0 {
		var delta map[string]interface{}
		if err := json.Unmarshal(row.Delta, &delta); err != nil {
			return gen.ActivityEntry{}, err
		}
		entry.Delta = &delta
	}
	return entry, nil
}

func parseUUID(s string) (openapi_types.UUID, error) {
	var id openapi_types.UUID
	if err := id.UnmarshalText([]byte(s)); err != nil {
		return openapi_types.UUID{}, err
	}
	return id, nil
}

func limitParam(limit *gen.Limit) int {
	if limit == nil {
		return 0
	}
	return *limit
}

func stringParam[T ~string](p *T) string {
	if p == nil {
		return ""
	}
	return string(*p)
}

func uuidParam(p *openapi_types.UUID) string {
	if p == nil {
		return ""
	}
	return p.String()
}
