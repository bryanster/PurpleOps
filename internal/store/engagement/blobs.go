package engagement

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// EvidenceBlobRepo manages the content-addressed blob rows in
// app.evidence_blob. Construct it with [NewEvidenceBlobRepo].
type EvidenceBlobRepo struct {
	db DB
}

// NewEvidenceBlobRepo returns a repository over db.
func NewEvidenceBlobRepo(db DB) *EvidenceBlobRepo { return &EvidenceBlobRepo{db: db} }

const blobColumns = `sha256, size, mime, storage_path, ref_count, created_at`

const selectBlob = `SELECT ` + blobColumns + ` FROM app.evidence_blob `

// GetBlob returns the blob row for this SHA-256, or an error.
func (r *EvidenceBlobRepo) GetBlob(ctx context.Context, sha256 string) (EvidenceBlob, error) {
	b, err := scanBlob(r.db.Read().QueryRowContext(ctx, selectBlob+`WHERE sha256 = ?`, sha256))
	if err != nil {
		return EvidenceBlob{}, fmt.Errorf("evidence_blob: read %q: %w", sha256, err)
	}
	return b, nil
}

// InsertBlob writes a new blob row with ref_count=1.
func (r *EvidenceBlobRepo) InsertBlob(ctx context.Context, sha256, mime, storagePath string, size int64) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO app.evidence_blob (sha256, size, mime, storage_path, ref_count, created_at)
			 VALUES (?, ?, ?, ?, 1, ?)`,
			sha256, size, mime, storagePath, toStorage(time.Now()),
		)
		if err != nil {
			return fmt.Errorf("evidence_blob: insert %q: %w", sha256, err)
		}
		return nil
	})
}

// IncrementRef bumps the blob's ref_count by one and returns the new count.
// Returns an error if the blob does not exist.
func (r *EvidenceBlobRepo) IncrementRef(ctx context.Context, sha256 string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE app.evidence_blob SET ref_count = ref_count + 1 WHERE sha256 = ?`, sha256)
		if err != nil {
			return fmt.Errorf("evidence_blob: increment ref %q: %w", sha256, err)
		}
		return requireOneRow(result, "evidence_blob", sha256)
	})
}

// DecrementRef drops the blob's ref_count by one. It reports (gc, true) when
// ref_count reached zero and the blob file should be removed.
func (r *EvidenceBlobRepo) DecrementRef(ctx context.Context, sha256 string) (gc bool, _ error) {
	var refCount int
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE app.evidence_blob SET ref_count = ref_count - 1 WHERE sha256 = ?`, sha256)
		if err != nil {
			return fmt.Errorf("evidence_blob: decrement ref %q: %w", sha256, err)
		}
		if err := requireOneRow(result, "evidence_blob", sha256); err != nil {
			return err
		}
		return tx.QueryRowContext(ctx,
			`SELECT ref_count FROM app.evidence_blob WHERE sha256 = ?`, sha256).Scan(&refCount)
	})
	if err != nil {
		return false, err
	}
	return refCount == 0, nil
}

// EngagementBlobBytes returns the sum of unique evidence blob sizes linked to
// an engagement. Blobs shared across executions within the engagement count
// once.
func (r *EvidenceBlobRepo) EngagementBlobBytes(ctx context.Context, engagementID string) (int64, error) {
	query := `
		SELECT COALESCE(SUM(b.size), 0)
		FROM app.evidence_blob b
		WHERE b.sha256 IN (
			SELECT DISTINCT e.blob_sha256
			FROM app.evidence e
			JOIN app.execution ex ON e.execution_id = ex.id
			JOIN app.step s ON ex.step_id = s.id
			JOIN app.scenario sc ON s.scenario_id = sc.id
			WHERE sc.engagement_id = ?
		)`
	var total int64
	if err := r.db.Read().QueryRowContext(ctx, query, engagementID).Scan(&total); err != nil {
		return 0, fmt.Errorf("evidence_blob: quota query for engagement %q: %w", engagementID, err)
	}
	return total, nil
}

// DeleteBlob removes the blob row entirely. Only call after DecrementRef
// reports gc=true.
func (r *EvidenceBlobRepo) DeleteBlob(ctx context.Context, sha256 string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`DELETE FROM app.evidence_blob WHERE sha256 = ?`, sha256)
		if err != nil {
			return fmt.Errorf("evidence_blob: delete %q: %w", sha256, err)
		}
		return requireOneRow(result, "evidence_blob", sha256)
	})
}

func scanBlob(row interface{ Scan(...any) error }) (EvidenceBlob, error) {
	var b EvidenceBlob
	if err := row.Scan(&b.SHA256, &b.Size, &b.MIME, &b.StoragePath, &b.RefCount, &b.CreatedAt); err != nil {
		return EvidenceBlob{}, err
	}
	b.CreatedAt = b.CreatedAt.UTC()
	return b, nil
}
