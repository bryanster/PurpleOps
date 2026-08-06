package engagement

import (
	"context"
	"database/sql"
	"fmt"
)

const evidenceColumns = `id, blob_sha256, filename, caption, side, execution_id, comment_id, uploaded_by, uploaded_at, size, mime`

const selectEvidence = `SELECT ` + evidenceColumns + ` FROM app.evidence `

const insertEvidence = `INSERT INTO app.evidence
	(id, blob_sha256, filename, caption, side, execution_id, comment_id, uploaded_by, uploaded_at, size, mime)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// EvidenceRepo reads and writes evidence metadata rows. Construct it with
// [NewEvidenceRepo].
type EvidenceRepo struct {
	db DB
}

// NewEvidenceRepo returns a repository over db.
func NewEvidenceRepo(db DB) *EvidenceRepo { return &EvidenceRepo{db: db} }

// Create writes a new evidence metadata row and returns it as stored.
func (r *EvidenceRepo) Create(ctx context.Context, in NewEvidence, after ...After) (Evidence, error) {
	var result Evidence
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		id, err := newID()
		if err != nil {
			return fmt.Errorf("evidence: generate id: %w", err)
		}
		ts := now()
		result = Evidence{
			ID:          id,
			BlobSHA256:  in.BlobSHA256,
			Filename:    in.Filename,
			Caption:     in.Caption,
			Side:        in.Side,
			ExecutionID: in.ExecutionID,
			CommentID:   in.CommentID,
			UploadedBy:  in.UploadedBy,
			UploadedAt:  ts,
			Size:        in.Size,
			MIME:        in.MIME,
		}
		_, err = tx.ExecContext(ctx, insertEvidence,
			result.ID,
			result.BlobSHA256,
			result.Filename,
			result.Caption,
			string(result.Side),
			nullString(result.ExecutionID),
			nullString(result.CommentID),
			result.UploadedBy,
			result.UploadedAt,
			result.Size,
			result.MIME,
		)
		if err != nil {
			return fmt.Errorf("evidence: insert: %w", err)
		}
		ctx = WithAfterEntity(ctx, id)
		return runAfter(ctx, tx, after)
	})
	if err != nil {
		return Evidence{}, err
	}
	return result, nil
}

// ByID returns the evidence row with this identifier.
func (r *EvidenceRepo) ByID(ctx context.Context, id string) (Evidence, error) {
	e, err := scanEvidence(r.db.Read().QueryRowContext(ctx, selectEvidence+`WHERE id = ?`, id))
	if err != nil {
		return Evidence{}, fmt.Errorf("evidence: read %q: %w", id, err)
	}
	return e, nil
}

// ListByExecution returns evidence linked to an execution, newest first.
func (r *EvidenceRepo) ListByExecution(ctx context.Context, executionID string) ([]Evidence, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		selectEvidence+`WHERE execution_id = ? ORDER BY uploaded_at DESC`, executionID)
	if err != nil {
		return nil, fmt.Errorf("evidence: list by execution: %w", err)
	}
	defer rows.Close()
	var out []Evidence
	for rows.Next() {
		e, err := scanEvidence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListByComment returns evidence linked to a comment, newest first.
func (r *EvidenceRepo) ListByComment(ctx context.Context, commentID string) ([]Evidence, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		selectEvidence+`WHERE comment_id = ? ORDER BY uploaded_at DESC`, commentID)
	if err != nil {
		return nil, fmt.Errorf("evidence: list by comment: %w", err)
	}
	defer rows.Close()
	var out []Evidence
	for rows.Next() {
		e, err := scanEvidence(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteLink removes the evidence metadata row. The blob is not touched; GC
// handles it when ref_count reaches zero (M3-009).
func (r *EvidenceRepo) DeleteLink(ctx context.Context, id string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `DELETE FROM app.evidence WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("evidence: delete link: %w", err)
		}
		return requireOneRow(result, "evidence", id)
	})
}

func scanEvidence(row interface{ Scan(...any) error }) (Evidence, error) {
	var e Evidence
	var executionID, commentID sql.NullString
	if err := row.Scan(
		&e.ID, &e.BlobSHA256, &e.Filename, &e.Caption,
		&e.Side,
		&executionID, &commentID,
		&e.UploadedBy, &e.UploadedAt,
		&e.Size, &e.MIME,
	); err != nil {
		return Evidence{}, err
	}
	e.ExecutionID = fromNullString(executionID)
	e.CommentID = fromNullString(commentID)
	e.UploadedAt = e.UploadedAt.UTC()
	return e, nil
}
