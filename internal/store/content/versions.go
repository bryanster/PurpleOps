package content

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
)

// SourceVersion is one version snapshot under a source.
type SourceVersion struct {
	ID        string
	SourceID  string
	Version   string
	Status    VersionStatus
	ItemCount int64
	SyncedAt  time.Time
	Error     string
	RawSHA256 string
	RawPath   string
	RawBytes  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewSourceVersion is the caller's half of creating a version row.
type NewSourceVersion struct {
	SourceID string
	Version  string
	Status   VersionStatus
}

// Versions reads and writes version snapshot rows. Construct with [NewVersions].
//
// rawPath validation needs a [Paths]; pass the same root the process was
// configured with. A zero Paths rejects every non-empty raw path — tests that
// never touch raw snapshots can pass Paths{}.
type Versions struct {
	db    DB
	paths Paths
}

// NewVersions returns a repository over db. paths validates raw_path values.
func NewVersions(db DB, paths Paths) *Versions {
	return &Versions{db: db, paths: paths}
}

const versionColumns = `id, source_id, version, status, item_count, synced_at, error,
	raw_sha256, raw_path, raw_bytes, created_at, updated_at`

const selectVersion = `SELECT ` + versionColumns + ` FROM content.content_source_version `

const insertVersion = `INSERT INTO content.content_source_version (
	id, source_id, version, status, item_count, synced_at, error,
	raw_sha256, raw_path, raw_bytes, created_at, updated_at
) VALUES (?, ?, ?, ?, 0, NULL, '', '', '', 0, ?, ?)`

// Create stores a new version row. Duplicate (source_id, version) is
// [apierr.Conflict].
func (r *Versions) Create(ctx context.Context, in NewSourceVersion) (SourceVersion, error) {
	if in.Status == "" {
		in.Status = VersionStatusPending
	}
	id, err := newID()
	if err != nil {
		return SourceVersion{}, err
	}
	ts := now()

	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, insertVersion,
			id, in.SourceID, in.Version, string(in.Status), ts, ts,
		)
		if err != nil {
			if store.IsUniqueViolation(err) {
				return apierr.Conflict(fmt.Sprintf(
					"version %q already exists for source %s", in.Version, in.SourceID))
			}
			return fmt.Errorf("content: insert version: %w", err)
		}
		return nil
	})
	if err != nil {
		return SourceVersion{}, err
	}
	return r.ByID(ctx, id)
}

// ByID returns the version with this identifier, or [apierr.NotFound].
func (r *Versions) ByID(ctx context.Context, id string) (SourceVersion, error) {
	v, err := scanVersion(r.db.Read().QueryRowContext(ctx, selectVersion+`WHERE id = ?`, id))
	if err != nil {
		return SourceVersion{}, wrapVersionErr(err, id)
	}
	return v, nil
}

// BySourceVersion returns the row for (sourceID, version), or [apierr.NotFound].
func (r *Versions) BySourceVersion(ctx context.Context, sourceID, version string) (SourceVersion, error) {
	v, err := scanVersion(r.db.Read().QueryRowContext(ctx,
		selectVersion+`WHERE source_id = ? AND version = ?`, sourceID, version,
	))
	if err != nil {
		return SourceVersion{}, wrapVersionErr(err, sourceID+"/"+version)
	}
	return v, nil
}

// ListBySource returns every version under a source, version then id.
func (r *Versions) ListBySource(ctx context.Context, sourceID string) ([]SourceVersion, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		selectVersion+`WHERE source_id = ? ORDER BY version, id`, sourceID,
	)
	if err != nil {
		return nil, fmt.Errorf("content: list versions for %s: %w", sourceID, err)
	}
	defer rows.Close()

	var out []SourceVersion
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("content: list versions for %s: %w", sourceID, err)
	}
	if out == nil {
		out = []SourceVersion{}
	}
	return out, nil
}

// SetRaw records a successful raw snapshot on the version row. relPath is
// validated against the configured content data root; absolute paths and
// anything containing ".." are rejected.
func (r *Versions) SetRaw(ctx context.Context, id, relPath, sha256 string, bytes int64) (SourceVersion, error) {
	clean, err := r.paths.CleanRel(relPath)
	if err != nil {
		return SourceVersion{}, err
	}
	// Also confirm it resolves under root — defence in depth if root is empty
	// in a miswired test.
	if r.paths.Root() != "" {
		if _, err := r.paths.Abs(clean); err != nil {
			return SourceVersion{}, err
		}
	}
	if bytes < 0 {
		return SourceVersion{}, fmt.Errorf("content: raw_bytes must be non-negative")
	}
	ts := now()
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE content.content_source_version SET
				raw_path = ?, raw_sha256 = ?, raw_bytes = ?, updated_at = ?
			WHERE id = ?`,
			clean, sha256, bytes, ts, id,
		)
		if err != nil {
			return fmt.Errorf("content: set raw on version %s: %w", id, err)
		}
		return requireOneRow(res, "content_source_version", id)
	})
	if err != nil {
		return SourceVersion{}, err
	}
	return r.ByID(ctx, id)
}

// SetState updates status, item_count, error and optionally synced_at.
func (r *Versions) SetState(ctx context.Context, id string, status VersionStatus, itemCount int64, errMsg string, syncedAt time.Time) error {
	ts := now()
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		var res sql.Result
		var err error
		if syncedAt.IsZero() {
			res, err = tx.ExecContext(ctx, `
				UPDATE content.content_source_version SET
					status = ?, item_count = ?, error = ?, updated_at = ?
				WHERE id = ?`,
				string(status), itemCount, errMsg, ts, id,
			)
		} else {
			res, err = tx.ExecContext(ctx, `
				UPDATE content.content_source_version SET
					status = ?, item_count = ?, error = ?, synced_at = ?, updated_at = ?
				WHERE id = ?`,
				string(status), itemCount, errMsg, toStorage(syncedAt), ts, id,
			)
		}
		if err != nil {
			return fmt.Errorf("content: set state on version %s: %w", id, err)
		}
		return requireOneRow(res, "content_source_version", id)
	})
}

// Delete removes one version row. Callers clear objects first.
func (r *Versions) Delete(ctx context.Context, id string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`DELETE FROM content.content_source_version WHERE id = ?`, id,
		)
		if err != nil {
			return fmt.Errorf("content: delete version %s: %w", id, err)
		}
		return requireOneRow(res, "content_source_version", id)
	})
}

func scanVersion(row interface{ Scan(...any) error }) (SourceVersion, error) {
	var (
		v        SourceVersion
		status   string
		syncedAt sql.NullTime
	)
	err := row.Scan(
		&v.ID, &v.SourceID, &v.Version, &status, &v.ItemCount, &syncedAt, &v.Error,
		&v.RawSHA256, &v.RawPath, &v.RawBytes, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return SourceVersion{}, err
	}
	v.Status = VersionStatus(status)
	v.SyncedAt = fromNullTime(syncedAt)
	v.CreatedAt = v.CreatedAt.UTC()
	v.UpdatedAt = v.UpdatedAt.UTC()
	return v, nil
}

func wrapVersionErr(err error, id string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return apierr.NotFound("content_source_version", id)
	}
	return fmt.Errorf("content: version %s: %w", id, err)
}
