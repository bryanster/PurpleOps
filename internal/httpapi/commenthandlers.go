package httpapi

import (
	"context"
	"fmt"

	"github.com/oapi-codegen/nullable"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/bryanster/blacklight/internal/authn"
	engagement "github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// Comment handlers (M3-010).

// ListComments returns every comment on an execution, oldest first.
// In blind mode, comments on unrevealed steps are 404-concealed.
func (h *handlers) ListComments(ctx context.Context,
	request gen.ListCommentsRequestObject) (gen.ListCommentsResponseObject, error) {

	// Blind check: load the execution's step to check revealed status.
	exec, err := h.engagements.GetExecution(ctx, request.ExecutionId.String())
	if err != nil {
		return nil, err
	}
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

	comments, err := h.engagements.ListComments(ctx, request.ExecutionId.String())
	if err != nil {
		return nil, err
	}

	wire := make([]gen.Comment, len(comments))
	for i, c := range comments {
		wire[i] = commentToWire(c)
	}
	return gen.ListComments200JSONResponse(wire), nil
}

// CreateComment writes a comment on an execution. The author is the
// authenticated caller. Blind conceal applies: cannot comment on an
// unrevealed execution.
func (h *handlers) CreateComment(ctx context.Context,
	request gen.CreateCommentRequestObject) (gen.CreateCommentResponseObject, error) {

	actor, ok := authn.SubjectFrom(ctx)
	if !ok {
		return nil, apierr.Unauthenticated("no authenticated subject")
	}

	// Blind check.
	exec, err := h.engagements.GetExecution(ctx, request.ExecutionId.String())
	if err != nil {
		return nil, err
	}
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

	comment, err := h.engagements.CreateComment(ctx, actor, request.EngagementId.String(), request.ExecutionId.String(), request.Body.Body)
	if err != nil {
		return nil, err
	}

	return gen.CreateComment201JSONResponse(commentToWire(comment)), nil
}

// PatchComment edits a comment's body. Only the author, the engagement
// lead, or a platform administrator may edit.
func (h *handlers) PatchComment(ctx context.Context,
	request gen.PatchCommentRequestObject) (gen.PatchCommentResponseObject, error) {

	actor, ok := authn.SubjectFrom(ctx)
	if !ok {
		return nil, apierr.Unauthenticated("no authenticated subject")
	}

	comment, err := h.engagements.GetComment(ctx, request.CommentId.String())
	if err != nil {
		return nil, err
	}
	// Permission check: author, lead, or admin.
	if actor.UserID != comment.AuthorID {
		if err := canEditComment(ctx, request.EngagementId.String()); err != nil {
			return nil, err
		}
	}
	edited, err := h.engagements.EditComment(ctx, actor, engagement.EditCommentInput{
		CommentID:    request.CommentId.String(),
		EngagementID: request.EngagementId.String(),
		ExecutionID:  comment.ExecutionID,
		Body:         request.Body.Body,
	})
	if err != nil {
		return nil, err
	}

	return gen.PatchComment200JSONResponse(commentToWire(edited)), nil
}

// ListCommentRevisions returns the edit history for a comment, oldest first.
func (h *handlers) ListCommentRevisions(ctx context.Context,
	request gen.ListCommentRevisionsRequestObject) (gen.ListCommentRevisionsResponseObject, error) {

	revisions, err := h.engagements.ListCommentRevisions(ctx, request.CommentId.String())
	if err != nil {
		return nil, err
	}

	wire := make([]gen.CommentRevision, len(revisions))
	for i, r := range revisions {
		id, err := parseUUID(r.ID)
		if err != nil {
			return nil, err
		}
		cid, err := parseUUID(r.CommentID)
		if err != nil {
			return nil, err
		}
		wire[i] = gen.CommentRevision{
			Id:        id,
			CommentId: cid,
			Body:      r.Body,
			EditedBy:  r.EditedBy,
			EditedAt:  r.EditedAt,
		}
	}
	return gen.ListCommentRevisions200JSONResponse(wire), nil
}

// commentToWire converts a store comment to its OpenAPI representation.
func commentToWire(c storengagement.Comment) gen.Comment {
	id := mustParseUUID(c.ID)
	execID := mustParseUUID(c.ExecutionID)
	w := gen.Comment{
		Id:          id,
		ExecutionId: execID,
		AuthorId:    c.AuthorID,
		Body:        c.Body,
		CreatedAt:   c.CreatedAt,
	}
	if c.EditedAt != nil {
		w.EditedAt = nullable.NewNullableWithValue(*c.EditedAt)
	}
	return w
}

// mustParseUUID parses a UUID string or panics. Use only where the UUID is
// guaranteed valid from the database.
func mustParseUUID(s string) openapi_types.UUID {
	var id openapi_types.UUID
	if err := id.UnmarshalText([]byte(s)); err != nil {
		panic(fmt.Sprintf("invalid UUID from database: %q: %v", s, err))
	}
	return id
}
