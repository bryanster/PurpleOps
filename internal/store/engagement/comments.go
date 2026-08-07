package engagement

import (
	"context"
	"database/sql"
	"fmt"
)

const commentColumns = `id, execution_id, author_id, body, created_at, edited_at`

const selectComment = `SELECT ` + commentColumns + ` FROM app."comment" `

const insertComment = `INSERT INTO app."comment"
	(id, execution_id, author_id, body, created_at, edited_at)
	VALUES (?, ?, ?, ?, ?, ?)`

// Comments reads and writes threaded notes on executions. Construct it with
// [NewComments].
type Comments struct {
	db DB
}

// NewComments returns a repository over db.
func NewComments(db DB) *Comments { return &Comments{db: db} }

// Create writes a new comment and returns it as stored.
func (r *Comments) Create(ctx context.Context, in NewComment, after ...After) (Comment, error) {
	var result Comment
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		id, err := newID()
		if err != nil {
			return fmt.Errorf("comment: generate id: %w", err)
		}
		ts := now()
		result = Comment{
			ID:          id,
			ExecutionID: in.ExecutionID,
			AuthorID:    in.AuthorID,
			Body:        in.Body,
			CreatedAt:   ts,
			EditedAt:    nil,
		}
		_, err = tx.ExecContext(ctx, insertComment,
			result.ID,
			result.ExecutionID,
			result.AuthorID,
			result.Body,
			result.CreatedAt,
			nullTime(result.EditedAt),
		)
		if err != nil {
			return fmt.Errorf("comment: insert: %w", err)
		}
		ctx = WithAfterEntity(ctx, id)
		return runAfter(ctx, tx, after)
	})
	if err != nil {
		return Comment{}, err
	}
	return result, nil
}

// ByID returns the comment with this identifier.
func (r *Comments) ByID(ctx context.Context, id string) (Comment, error) {
	c, err := scanComment(r.db.Read().QueryRowContext(ctx, selectComment+`WHERE id = ?`, id))
	if err != nil {
		return Comment{}, fmt.Errorf("comment: read %q: %w", id, err)
	}
	return c, nil
}

// ListByExecution returns every comment on an execution, oldest first.
func (r *Comments) ListByExecution(ctx context.Context, executionID string) ([]Comment, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		selectComment+`WHERE execution_id = ? ORDER BY created_at ASC`, executionID)
	if err != nil {
		return nil, fmt.Errorf("comment: list by execution: %w", err)
	}
	defer rows.Close()
	var out []Comment
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Edit changes a comment's body, appending a revision row with the *previous*
// body and updating edited_at. Returns the comment as stored and the new revision.
func (r *Comments) Edit(ctx context.Context, commentID, editedBy, newBody string) (Comment, CommentRevision, error) {
	var comment Comment
	var rev CommentRevision
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		// Read the current body to store as the revision.
		current, err := scanComment(tx.QueryRowContext(ctx, selectComment+`WHERE id = ?`, commentID))
		if err != nil {
			return err
		}

		revID, err := newID()
		if err != nil {
			return fmt.Errorf("comment: generate revision id: %w", err)
		}
		ts := now()

		// Update the comment row.
		result, err := tx.ExecContext(ctx,
			`UPDATE app."comment" SET body = ?, edited_at = ? WHERE id = ?`,
			newBody, ts, commentID,
		)
		if err != nil {
			return fmt.Errorf("comment: edit: %w", err)
		}
		if err := requireOneRow(result, "comment", commentID); err != nil {
			return err
		}

		// Append the revision with the *previous* body.
		rev = CommentRevision{
			ID:        revID,
			CommentID: commentID,
			Body:      current.Body,
			EditedBy:  editedBy,
			EditedAt:  ts,
		}
		_, err = tx.ExecContext(ctx,
			`INSERT INTO app.comment_revision (id, comment_id, body, edited_by, edited_at) VALUES (?, ?, ?, ?, ?)`,
			rev.ID, rev.CommentID, rev.Body, rev.EditedBy, rev.EditedAt,
		)
		if err != nil {
			return fmt.Errorf("comment: insert revision: %w", err)
		}

		// Read the updated comment back.
		comment, err = scanComment(tx.QueryRowContext(ctx, selectComment+`WHERE id = ?`, commentID))
		return err
	})
	if err != nil {
		return Comment{}, CommentRevision{}, err
	}
	return comment, rev, nil
}

// Revisions returns the edit history for a comment, oldest first.
func (r *Comments) Revisions(ctx context.Context, commentID string) ([]CommentRevision, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		`SELECT id, comment_id, body, edited_by, edited_at FROM app.comment_revision
			WHERE comment_id = ? ORDER BY edited_at ASC`, commentID)
	if err != nil {
		return nil, fmt.Errorf("comment: revisions: %w", err)
	}
	defer rows.Close()
	var out []CommentRevision
	for rows.Next() {
		var cr CommentRevision
		if err := rows.Scan(&cr.ID, &cr.CommentID, &cr.Body, &cr.EditedBy, &cr.EditedAt); err != nil {
			return nil, err
		}
		cr.EditedAt = cr.EditedAt.UTC()
		out = append(out, cr)
	}
	return out, rows.Err()
}

func scanComment(row interface{ Scan(...any) error }) (Comment, error) {
	var c Comment
	var editedAt sql.NullTime
	if err := row.Scan(
		&c.ID, &c.ExecutionID, &c.AuthorID, &c.Body,
		&c.CreatedAt, &editedAt,
	); err != nil {
		return Comment{}, err
	}
	c.EditedAt = fromNullTime(editedAt)
	c.CreatedAt = c.CreatedAt.UTC()
	return c, nil
}
