package httpapi

import (
	"context"
	"database/sql"
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
// In a blind engagement, rows about unrevealed step-scoped objects are
// withheld from the blue seat (M4-008).
func (h *handlers) ListEngagementActivity(ctx context.Context,
	request gen.ListEngagementActivityRequestObject) (gen.ListEngagementActivityResponseObject, error) {
	engagementID := request.EngagementId.String()
	rows, next, err := h.activity.Entries().List(ctx, activity.ListFilter{
		ScopeEngagement: engagementID,
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

	// Filter unrevealed step-scoped rows from blue in blind engagements.
	rows, err = h.filterBlindActivity(ctx, engagementID, rows)
	if err != nil {
		return nil, err
	}

	items := make([]gen.ActivityEntry, 0, len(rows))
	for _, row := range rows {
		entry, err := activityEntry(row)
		if err != nil {
			return nil, err
		}
		items = append(items, entry)
	}
	page := gen.ActivityPage{Items: items}
	if next != "" {
		page.NextCursor = nullable.NewNullableWithValue(next)
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

// filterBlindActivity removes rows about unrevealed step-scoped objects when
// the caller is the blue seat of a blind engagement (M4-008).
func (h *handlers) filterBlindActivity(ctx context.Context, engagementID string, rows []activity.Row) ([]activity.Row, error) {
	scope, err := h.stepBlindScope(ctx, engagementID)
	if err != nil {
		return nil, err
	}
	if !scope.Withholds() {
		return rows, nil
	}

	filtered := make([]activity.Row, 0, len(rows))
	for _, row := range rows {
		stepID, err := h.resolveActivityStepID(ctx, row.ObjectType, row.ObjectID)
		if err != nil {
			filtered = append(filtered, row)
			continue
		}
		if stepID == "" {
			filtered = append(filtered, row)
			continue
		}
		revealed, err := h.IsStepRevealed(ctx, stepID)
		if err != nil {
			filtered = append(filtered, row)
			continue
		}
		if revealed {
			filtered = append(filtered, row)
		}
	}
	return filtered, nil
}
func (h *handlers) resolveActivityStepID(ctx context.Context, objectType, objectID string) (string, error) {
	switch objectType {
	case "step":
		return objectID, nil
	case "execution":
		exec, err := h.engagements.GetExecution(ctx, objectID)
		if err != nil {
			return "", err
		}
		return exec.StepID, nil
	case "comment":
		comment, err := h.engagements.GetComment(ctx, objectID)
		if err != nil {
			return "", err
		}
		exec, err := h.engagements.GetExecution(ctx, comment.ExecutionID)
		if err != nil {
			return "", err
		}
		return exec.StepID, nil
	case "evidence":
		var executionID sql.NullString
		err := h.store.Read().QueryRowContext(ctx,
			`SELECT execution_id FROM app.evidence WHERE id = ?`, objectID).Scan(&executionID)
		if err != nil {
			return "", err
		}
		if !executionID.Valid {
			return "", nil // evidence attached to a comment, not an execution — not step-scoped
		}
		exec, err := h.engagements.GetExecution(ctx, executionID.String)
		if err != nil {
			return "", err
		}
		return exec.StepID, nil
	default:
		return "", nil
	}
}
