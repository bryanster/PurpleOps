package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

// Job is one content_sync_job row.
type Job struct {
	ID              string
	SourceID        string
	Version         string // empty when NULL in the database
	Kind            JobKind
	Status          JobStatus
	Phase           string
	ProgressCurrent int64
	ProgressTotal   int64
	Message         string
	Error           string
	CreatedBy       string
	CreatedAt       time.Time
	StartedAt       time.Time
	FinishedAt      time.Time
	Checkpoint      json.RawMessage
}

// NewJob is the caller's half of enqueueing a job.
type NewJob struct {
	SourceID  string
	Version   string // optional
	Kind      JobKind
	CreatedBy string
}

// Jobs reads and writes sync job rows. Construct with [NewJobs].
type Jobs struct {
	db DB
}

// NewJobs returns a repository over db.
func NewJobs(db DB) *Jobs { return &Jobs{db: db} }

const jobColumns = `id, source_id, version, kind, status, phase,
	progress_current, progress_total, message, error, created_by,
	created_at, started_at, finished_at, checkpoint`

const selectJob = `SELECT ` + jobColumns + ` FROM content.content_sync_job `

const insertJob = `INSERT INTO content.content_sync_job (
	id, source_id, version, kind, status, phase,
	progress_current, progress_total, message, error, created_by,
	created_at, started_at, finished_at, checkpoint
) VALUES (?, ?, ?, ?, ?, '', 0, 0, '', '', ?, ?, NULL, NULL, ?)`

// Create enqueues a job in status queued and returns it as stored.
func (r *Jobs) Create(ctx context.Context, in NewJob) (Job, error) {
	id, err := newID()
	if err != nil {
		return Job{}, err
	}
	ts := now()
	checkpoint := []byte(`{}`)

	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, insertJob,
			id, in.SourceID, nullString(in.Version), string(in.Kind), string(JobStatusQueued),
			in.CreatedBy, ts, checkpoint,
		)
		if err != nil {
			return fmt.Errorf("content: insert job: %w", err)
		}
		return nil
	})
	if err != nil {
		return Job{}, err
	}
	return r.ByID(ctx, id)
}

// ByID returns the job with this identifier, or [apierr.NotFound].
func (r *Jobs) ByID(ctx context.Context, id string) (Job, error) {
	j, err := scanJob(r.db.Read().QueryRowContext(ctx, selectJob+`WHERE id = ?`, id))
	if err != nil {
		return Job{}, wrapJobErr(err, id)
	}
	return j, nil
}

// ListFilter narrows job listings. Zero values mean "no filter".
type ListFilter struct {
	SourceID string
	Status   JobStatus
	Limit    int
}

// List returns jobs newest first.
func (r *Jobs) List(ctx context.Context, f ListFilter) ([]Job, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	q := selectJob + `WHERE 1=1`
	args := make([]any, 0, 3)
	if f.SourceID != "" {
		q += ` AND source_id = ?`
		args = append(args, f.SourceID)
	}
	if f.Status != "" {
		q += ` AND status = ?`
		args = append(args, string(f.Status))
	}
	q += ` ORDER BY created_at DESC, id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := r.db.Read().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("content: list jobs: %w", err)
	}
	defer rows.Close()

	var out []Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("content: list jobs: %w", err)
	}
	if out == nil {
		out = []Job{}
	}
	return out, nil
}

// SetStatus writes a new status and optional timestamps/message/error.
type JobUpdate struct {
	Status          JobStatus
	Phase           string
	ProgressCurrent int64
	ProgressTotal   int64
	Message         string
	Error           string
	Checkpoint      json.RawMessage // nil = leave unchanged
	StartedAt       time.Time       // non-zero sets started_at
	FinishedAt      time.Time       // non-zero sets finished_at
}

// Update applies a partial progress/status write.
func (r *Jobs) Update(ctx context.Context, id string, u JobUpdate) (Job, error) {
	err := r.db.Write(ctx, func(tx *sql.Tx) error {

		// Read-modify would be racy without the serialized writer; with it,
		// a single UPDATE of the columns the caller cares about is enough.
		// Checkpoint is optional so a progress tick need not re-send it.
		var (
			res sql.Result
			err error
		)
		if u.Checkpoint != nil {
			buf := make([]byte, len(u.Checkpoint))
			copy(buf, u.Checkpoint)
			if len(buf) == 0 {
				buf = []byte(`{}`)
			}
			res, err = tx.ExecContext(ctx, `
				UPDATE content.content_sync_job SET
					status = ?, phase = ?, progress_current = ?, progress_total = ?,
					message = ?, error = ?, checkpoint = ?,
					started_at = COALESCE(?, started_at),
					finished_at = COALESCE(?, finished_at)
				WHERE id = ?`,
				string(u.Status), u.Phase, u.ProgressCurrent, u.ProgressTotal,
				u.Message, u.Error, buf,
				nullTime(u.StartedAt), nullTime(u.FinishedAt),
				id,
			)
		} else {
			res, err = tx.ExecContext(ctx, `
				UPDATE content.content_sync_job SET
					status = ?, phase = ?, progress_current = ?, progress_total = ?,
					message = ?, error = ?,
					started_at = COALESCE(?, started_at),
					finished_at = COALESCE(?, finished_at)
				WHERE id = ?`,
				string(u.Status), u.Phase, u.ProgressCurrent, u.ProgressTotal,
				u.Message, u.Error,
				nullTime(u.StartedAt), nullTime(u.FinishedAt),
				id,
			)
		}
		if err != nil {
			return fmt.Errorf("content: update job %s: %w", id, err)
		}
		return requireOneRow(res, "content_sync_job", id)
	})
	if err != nil {
		return Job{}, err
	}
	return r.ByID(ctx, id)
}

// InterruptInFlight marks every job left in running or cancelling as
// interrupted. Call once at process boot before the job runner starts; do not
// silently resume. Returns the number of rows changed.
func (r *Jobs) InterruptInFlight(ctx context.Context, message string) (int64, error) {
	if message == "" {
		message = "process restarted while job was in flight"
	}
	ts := now()
	var n int64
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE content.content_sync_job SET
				status = ?,
				error = CASE WHEN error = '' THEN ? ELSE error END,
				message = ?,
				finished_at = COALESCE(finished_at, ?)
			WHERE status IN (?, ?)`,
			string(JobStatusInterrupted),
			message,
			message,
			ts,
			string(JobStatusRunning),
			string(JobStatusCancelling),
		)
		if err != nil {
			return fmt.Errorf("content: interrupt in-flight jobs: %w", err)
		}
		n, err = res.RowsAffected()
		return err
	})
	return n, err
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return toStorage(t)
}

func scanJob(row interface{ Scan(...any) error }) (Job, error) {
	var (
		j          Job
		version    sql.NullString
		kind       string
		status     string
		startedAt  sql.NullTime
		finishedAt sql.NullTime
		checkpoint any
	)
	err := row.Scan(
		&j.ID, &j.SourceID, &version, &kind, &status, &j.Phase,
		&j.ProgressCurrent, &j.ProgressTotal, &j.Message, &j.Error, &j.CreatedBy,
		&j.CreatedAt, &startedAt, &finishedAt, &checkpoint,
	)
	if err != nil {
		return Job{}, err
	}
	j.Version = fromNullString(version)
	j.Kind = JobKind(kind)
	j.Status = JobStatus(status)
	j.CreatedAt = j.CreatedAt.UTC()
	j.StartedAt = fromNullTime(startedAt)
	j.FinishedAt = fromNullTime(finishedAt)
	raw, err := jsonBytes(checkpoint)
	if err != nil {
		return Job{}, fmt.Errorf("content: job %s checkpoint: %w", j.ID, err)
	}
	j.Checkpoint = raw
	return j, nil
}

// jsonBytes normalises whatever the driver handed back for a JSON column.
func jsonBytes(v any) (json.RawMessage, error) {
	switch d := v.(type) {
	case nil:
		return json.RawMessage(`{}`), nil
	case []byte:
		if len(d) == 0 {
			return json.RawMessage(`{}`), nil
		}
		return append(json.RawMessage(nil), d...), nil
	case string:
		if d == "" {
			return json.RawMessage(`{}`), nil
		}
		return json.RawMessage(d), nil
	default:
		raw, err := json.Marshal(d)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(raw), nil
	}
}

func wrapJobErr(err error, id string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return apierr.NotFound("content_sync_job", id)
	}
	return fmt.Errorf("content: job %s: %w", id, err)
}
