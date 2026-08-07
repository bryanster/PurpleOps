package engagement

import (
	"context"
	"fmt"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/events"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// CreateComment writes a comment on an execution. The author is the
// authenticated caller. Authorization (comment.write) is handled by the
// authz middleware. This method validates body length and records activity.
func (s *Service) CreateComment(ctx context.Context, actor authn.Subject, engagementID, executionID, body string) (storengagement.Comment, error) {
	if body == "" {
		return storengagement.Comment{}, fmt.Errorf("engagement: comment body is empty")
	}
	if len(body) > 16384 {
		return storengagement.Comment{}, fmt.Errorf("engagement: comment body exceeds 16 KiB limit")
	}

	comment, err := s.comments.Create(ctx, storengagement.NewComment{
		ExecutionID: executionID,
		AuthorID:    actor.UserID,
		Body:        body,
	})
	if err != nil {
		return storengagement.Comment{}, err
	}

	recordCommentActivity(ctx, s.activity, actor.UserID, engagementID, executionID, comment.ID, events.VerbCommentCreated, nil)

	return comment, nil
}

// EditCommentInput is the caller's half of editing a comment.
type EditCommentInput struct {
	CommentID    string
	EngagementID string
	ExecutionID  string
	Body         string
}

// EditComment updates a comment's body, appending the previous body to the
// revision history. The caller must be the author, the engagement lead, or a
// platform admin — this is enforced by the handler layer before this method
// is called. Closed engagements are allowed.
func (s *Service) EditComment(ctx context.Context, actor authn.Subject, in EditCommentInput) (storengagement.Comment, error) {
	if in.Body == "" {
		return storengagement.Comment{}, fmt.Errorf("engagement: comment body is empty")
	}
	if len(in.Body) > 16384 {
		return storengagement.Comment{}, fmt.Errorf("engagement: comment body exceeds 16 KiB limit")
	}

	comment, _, err := s.comments.Edit(ctx, in.CommentID, actor.UserID, in.Body)
	if err != nil {
		return storengagement.Comment{}, err
	}

	delta := map[string]any{
		"body": in.Body,
	}
	recordCommentActivity(ctx, s.activity, actor.UserID, in.EngagementID, in.ExecutionID, in.CommentID, events.VerbCommentEdited, delta)

	return comment, nil
}

// ListComments returns comments on an execution, oldest first.
func (s *Service) ListComments(ctx context.Context, executionID string) ([]storengagement.Comment, error) {
	return s.comments.ListByExecution(ctx, executionID)
}

// GetComment returns a comment by id.
func (s *Service) GetComment(ctx context.Context, id string) (storengagement.Comment, error) {
	return s.comments.ByID(ctx, id)
}

// ListCommentRevisions returns the edit history for a comment, oldest first.
func (s *Service) ListCommentRevisions(ctx context.Context, commentID string) ([]storengagement.CommentRevision, error) {
	return s.comments.Revisions(ctx, commentID)
}

// recordCommentActivity writes an activity entry for a comment change,
// or is a no-op when activity recording is disabled.
func recordCommentActivity(ctx context.Context, log *events.Log, actorID, engagementID, executionID, objectID string, verb events.Verb, delta map[string]any) {
	if log == nil {
		return
	}
	//nolint:errcheck // best-effort audit trail; failure is logged by the store
	log.RecordAlone(ctx, events.Entry{
		EngagementID: engagementID,
		ActorID:      actorID,
		Verb:         verb,
		ObjectType:   events.ObjectComment,
		ObjectID:     objectID,
		ParentIDs:    map[string]string{"executionId": executionID},
		Delta:        delta,
	})
}
