package content

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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

// SourceFilter narrows a source listing. Zero values mean "no filter".
//
// Enabled is a pointer so a caller can ask for disabled sources specifically
// (false) without that colliding with "I did not mention enabled".
type SourceFilter struct {
	Kind    Kind
	Enabled *bool
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

// sourceSubtreeDeletes is every content table that names a source_id, in an
// order that does not matter for integrity (there are no FKs) but keeps the
// larger join/object tables first so a partial failure is easier to read in a
// log. Jobs and versions go after objects; the registry row is last and is
// issued by the caller so it can share a transaction with activity.
var sourceSubtreeDeletes = []string{
	`DELETE FROM content.content_technique_tactic WHERE source_id = ?`,
	`DELETE FROM content.content_tactic WHERE source_id = ?`,
	`DELETE FROM content.content_technique WHERE source_id = ?`,
	`DELETE FROM content.content_mitigation WHERE source_id = ?`,
	`DELETE FROM content.content_group WHERE source_id = ?`,
	`DELETE FROM content.content_software WHERE source_id = ?`,
	`DELETE FROM content.content_data_source WHERE source_id = ?`,
	`DELETE FROM content.content_procedure_template WHERE source_id = ?`,
	`DELETE FROM content.content_detection_rule_ref WHERE source_id = ?`,
	`DELETE FROM content.content_emulation_plan_step WHERE source_id = ?`,
	`DELETE FROM content.content_emulation_plan WHERE source_id = ?`,
	`DELETE FROM content.content_note WHERE source_id = ?`,
	`DELETE FROM content.content_sync_job WHERE source_id = ?`,
	`DELETE FROM content.content_source_version WHERE source_id = ?`,
}

// Sources reads and writes registry rows. Construct it with [NewSources].
type Sources struct {
	db DB
}

// NewSources returns a repository over db.
func NewSources(db DB) *Sources { return &Sources{db: db} }

// Create stores a new source and returns it as stored.
func (r *Sources) Create(ctx context.Context, in NewSource, after ...After) (Source, error) {
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
		return runAfter(WithAfterEntity(ctx, id), tx, after)
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

// List returns every source matching f, kind then id.
func (r *Sources) List(ctx context.Context, f SourceFilter) ([]Source, error) {
	q := selectSource + `WHERE 1=1`
	args := make([]any, 0, 2)
	if f.Kind != "" {
		q += ` AND kind = ?`
		args = append(args, string(f.Kind))
	}
	if f.Enabled != nil {
		q += ` AND enabled = ?`
		args = append(args, *f.Enabled)
	}
	q += ` ORDER BY kind, id`

	rows, err := r.db.Read().QueryContext(ctx, q, args...)
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
//
// after runs inside the same transaction after the update so an activity row
// (M2-002) shares the commit.
func (r *Sources) SetEnabled(ctx context.Context, id string, enabled bool, after ...After) (Source, error) {
	ts := now()
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`UPDATE content.content_source SET enabled = ?, updated_at = ? WHERE id = ?`,
			enabled, ts, id,
		)
		if err != nil {
			return fmt.Errorf("content: set enabled on %s: %w", id, err)
		}
		if err := requireOneRow(res, "content_source", id); err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, id), tx, after)
	})
	if err != nil {
		return Source{}, err
	}
	return r.ByID(ctx, id)
}

// UpdateMeta writes name/url/ref and license fields. Kind is immutable.
//
// after runs inside the same transaction after the update.
func (r *Sources) UpdateMeta(ctx context.Context, s Source, after ...After) (Source, error) {
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
		if err := requireOneRow(res, "content_source", s.ID); err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, s.ID), tx, after)
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

// ResetSyncing moves every source left in status=syncing back to idle. Call
// once at process boot together with [Jobs.InterruptInFlight] so a crash
// cannot leave the registry looking permanently busy (M2-003).
//
// errMsg is written into the error column when it is currently empty, so an
// operator sees why the source stopped mid-flight. last_synced_at and
// item_count are left alone — the prior successful catalog still stands.
func (r *Sources) ResetSyncing(ctx context.Context, errMsg string) (int64, error) {
	if errMsg == "" {
		errMsg = "process restarted while a sync was in flight"
	}
	ts := now()
	var n int64
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE content.content_source SET
				status = ?,
				error = CASE WHEN error = '' THEN ? ELSE error END,
				updated_at = ?
			WHERE status = ?`,
			string(SourceStatusIdle),
			errMsg,
			ts,
			string(SourceStatusSyncing),
		)
		if err != nil {
			return fmt.Errorf("content: reset syncing sources: %w", err)
		}
		n, err = res.RowsAffected()
		return err
	})
	return n, err
}

// Delete removes a source row. Callers must have already cleared versions and
// objects, or use [Sources.DeleteCascade] which does that in one transaction.
//
// after runs inside the same transaction after the delete.
func (r *Sources) Delete(ctx context.Context, id string, after ...After) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM content.content_source WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("content: delete source %s: %w", id, err)
		}
		if err := requireOneRow(res, "content_source", id); err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, id), tx, after)
	})
}

// DeleteCascade removes a source and every content row that names it — object
// tables, jobs, versions — in one write transaction. There is no path into
// app, so engagement history is never touched.
//
// after runs after the source row itself is gone, still inside the transaction,
// so activity (M2-002) and the delete share a commit.
//
// Product rules that refuse a delete (custom seed, future external refs) live
// above this package; this is the storage half only.
func (r *Sources) DeleteCascade(ctx context.Context, id string, after ...After) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireSource(ctx, tx, id); err != nil {
			return err
		}
		for _, stmt := range sourceSubtreeDeletes {
			if _, err := tx.ExecContext(ctx, stmt, id); err != nil {
				return fmt.Errorf("content: cascade delete on %s (%s): %w", id, shortSQL(stmt), err)
			}
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM content.content_source WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("content: delete source %s: %w", id, err)
		}
		if err := requireOneRow(res, "content_source", id); err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, id), tx, after)
	})
}

// shortSQL keeps cascade-delete error messages readable: the table name, not
// the whole statement.
func shortSQL(stmt string) string {
	const prefix = "DELETE FROM "
	if strings.HasPrefix(stmt, prefix) {
		rest := stmt[len(prefix):]
		if i := strings.IndexByte(rest, ' '); i >= 0 {
			return rest[:i]
		}
		return rest
	}
	return stmt
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
