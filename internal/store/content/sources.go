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

// Source is one registry row: an upstream library or the custom home.
type Source struct {
	ID           string
	Kind         Kind
	Name         string
	URL          string
	Ref          string
	Enabled      bool
	Status       SourceStatus
	LastSyncedAt time.Time
	ItemCount    int64
	Error        string
	LicenseSPDX  string
	LicenseName  string
	LicenseURL   string
	Attribution  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewSource is the caller's half of creating a source. Builtin seeds arrive via
// migration; this is for tests and any future non-seed creation path.
type NewSource struct {
	Kind        Kind
	Name        string
	URL         string
	Ref         string
	Enabled     bool
	Status      SourceStatus
	LicenseSPDX string
	LicenseName string
	LicenseURL  string
	Attribution string
}

const sourceColumns = `id, kind, name, url, ref, enabled, status,
	last_synced_at, item_count, error,
	license_spdx, license_name, license_url, attribution,
	created_at, updated_at`

const selectSource = `SELECT ` + sourceColumns + ` FROM content.content_source `

const insertSource = `INSERT INTO content.content_source (
	id, kind, name, url, ref, enabled, status,
	last_synced_at, item_count, error,
	license_spdx, license_name, license_url, attribution,
	created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, NULL, 0, '', ?, ?, ?, ?, ?, ?)`

// Sources reads and writes registry rows. Construct it with [NewSources].
type Sources struct {
	db DB
}

// NewSources returns a repository over db.
func NewSources(db DB) *Sources { return &Sources{db: db} }

// Create stores a new source and returns it as stored.
func (r *Sources) Create(ctx context.Context, in NewSource) (Source, error) {
	if in.Status == "" {
		in.Status = SourceStatusIdle
	}
	id, err := newID()
	if err != nil {
		return Source{}, err
	}
	ts := now()

	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, insertSource,
			id, string(in.Kind), in.Name, in.URL, in.Ref, in.Enabled, string(in.Status),
			in.LicenseSPDX, in.LicenseName, in.LicenseURL, in.Attribution,
			ts, ts,
		)
		if err != nil {
			if store.IsUniqueViolation(err) {
				return apierr.Conflict(fmt.Sprintf("a content source of kind %q already exists", in.Kind))
			}
			return fmt.Errorf("content: insert source: %w", err)
		}
		return nil
	})
	if err != nil {
		return Source{}, err
	}
	return r.ByID(ctx, id)
}

// ByID returns the source with this identifier, or [apierr.NotFound].
func (r *Sources) ByID(ctx context.Context, id string) (Source, error) {
	s, err := scanSource(r.db.Read().QueryRowContext(ctx, selectSource+`WHERE id = ?`, id))
	if err != nil {
		return Source{}, wrapSourceErr(err, id)
	}
	return s, nil
}

// ByKind returns the source with this kind, or [apierr.NotFound].
func (r *Sources) ByKind(ctx context.Context, kind Kind) (Source, error) {
	s, err := scanSource(r.db.Read().QueryRowContext(ctx, selectSource+`WHERE kind = ?`, string(kind)))
	if err != nil {
		return Source{}, wrapSourceErr(err, string(kind))
	}
	return s, nil
}

// List returns every source, kind then id.
func (r *Sources) List(ctx context.Context) ([]Source, error) {
	rows, err := r.db.Read().QueryContext(ctx, selectSource+`ORDER BY kind, id`)
	if err != nil {
		return nil, fmt.Errorf("content: list sources: %w", err)
	}
	defer rows.Close()

	var out []Source
	for rows.Next() {
		s, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("content: list sources: %w", err)
	}
	if out == nil {
		out = []Source{}
	}
	return out, nil
}

// SetEnabled flips the soft switch and returns the source as stored.
func (r *Sources) SetEnabled(ctx context.Context, id string, enabled bool) (Source, error) {
	ts := now()
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE content.content_source SET enabled = ?, updated_at = ? WHERE id = ?`,
			enabled, ts, id,
		)
		if err != nil {
			return fmt.Errorf("content: set enabled on %s: %w", id, err)
		}
		return requireOneRow(res, "content_source", id)
	})
	if err != nil {
		return Source{}, err
	}
	return r.ByID(ctx, id)
}

// UpdateMeta writes name/url/ref and license fields. Kind is immutable.
func (r *Sources) UpdateMeta(ctx context.Context, s Source) (Source, error) {
	ts := now()
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE content.content_source SET
				name = ?, url = ?, ref = ?,
				license_spdx = ?, license_name = ?, license_url = ?, attribution = ?,
				updated_at = ?
			WHERE id = ?`,
			s.Name, s.URL, s.Ref,
			s.LicenseSPDX, s.LicenseName, s.LicenseURL, s.Attribution,
			ts, s.ID,
		)
		if err != nil {
			return fmt.Errorf("content: update source %s: %w", s.ID, err)
		}
		return requireOneRow(res, "content_source", s.ID)
	})
	if err != nil {
		return Source{}, err
	}
	return r.ByID(ctx, s.ID)
}

// SetSyncState records the outcome of a job against the source bookkeeping
// columns. status, itemCount and errMsg are written as given; last_synced_at
// is set only when syncedAt is non-zero.
func (r *Sources) SetSyncState(ctx context.Context, id string, status SourceStatus, itemCount int64, errMsg string, syncedAt time.Time) error {
	ts := now()
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		var res sql.Result
		var err error
		if syncedAt.IsZero() {
			res, err = tx.ExecContext(ctx, `
				UPDATE content.content_source SET
					status = ?, item_count = ?, error = ?, updated_at = ?
				WHERE id = ?`,
				string(status), itemCount, errMsg, ts, id,
			)
		} else {
			res, err = tx.ExecContext(ctx, `
				UPDATE content.content_source SET
					status = ?, item_count = ?, error = ?, last_synced_at = ?, updated_at = ?
				WHERE id = ?`,
				string(status), itemCount, errMsg, toStorage(syncedAt), ts, id,
			)
		}
		if err != nil {
			return fmt.Errorf("content: set sync state on %s: %w", id, err)
		}
		return requireOneRow(res, "content_source", id)
	})
}

// Delete removes a source row. Callers must have already cleared versions and
// objects (M2-002); this is the final registry delete only.
func (r *Sources) Delete(ctx context.Context, id string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM content.content_source WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("content: delete source %s: %w", id, err)
		}
		return requireOneRow(res, "content_source", id)
	})
}

func scanSource(row interface{ Scan(...any) error }) (Source, error) {
	var (
		s          Source
		kind       string
		status     string
		lastSynced sql.NullTime
	)
	err := row.Scan(
		&s.ID, &kind, &s.Name, &s.URL, &s.Ref, &s.Enabled, &status,
		&lastSynced, &s.ItemCount, &s.Error,
		&s.LicenseSPDX, &s.LicenseName, &s.LicenseURL, &s.Attribution,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return Source{}, err
	}
	s.Kind = Kind(kind)
	s.Status = SourceStatus(status)
	s.LastSyncedAt = fromNullTime(lastSynced)
	s.CreatedAt = s.CreatedAt.UTC()
	s.UpdatedAt = s.UpdatedAt.UTC()
	return s, nil
}

func wrapSourceErr(err error, id string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return apierr.NotFound("content_source", id)
	}
	return fmt.Errorf("content: source %s: %w", id, err)
}
