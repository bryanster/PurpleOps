package engagement

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

const engagementColumns = `id, name, client, description, status, starts_on, ends_on, attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at`

const selectEngagement = `SELECT ` + engagementColumns + ` FROM app.engagement `

const insertEngagement = `INSERT INTO app.engagement
	(id, name, client, description, status, starts_on, ends_on, attack_version, mode, auto_reveal_on_start, created_by, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// Engagements reads and writes assessments. Construct it with [NewEngagements].
type Engagements struct {
	db DB
}

// NewEngagements returns a repository over db.
func NewEngagements(db DB) *Engagements { return &Engagements{db: db} }

// Create writes a new engagement and returns it as stored.
// ID, status, created_at and updated_at are assigned by the store.
func (r *Engagements) Create(ctx context.Context, in NewEngagement, after ...After) (Engagement, error) {
	var result Engagement
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		id, err := newID()
		if err != nil {
			return fmt.Errorf("engagement: generate id: %w", err)
		}
		ts := now()
		result = Engagement{
			ID:                id,
			Name:              in.Name,
			Client:            in.Client,
			Description:       in.Description,
			Status:            EngagementStatusDraft,
			StartsOn:          toStorage(in.StartsOn),
			EndsOn:            toStorage(in.EndsOn),
			AttackVersion:     in.AttackVersion,
			Mode:              in.Mode,
			AutoRevealOnStart: in.AutoRevealOnStart,
			CreatedBy:         in.CreatedBy,
			CreatedAt:         ts,
			UpdatedAt:         ts,
		}
		_, err = tx.ExecContext(ctx, insertEngagement,
			result.ID,
			result.Name,
			result.Client,
			result.Description,
			string(result.Status),
			result.StartsOn,
			result.EndsOn,
			result.AttackVersion,
			string(result.Mode),
			result.AutoRevealOnStart,
			result.CreatedBy,
			result.CreatedAt,
			result.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("engagement: insert: %w", err)
		}
		ctx = WithAfterEntity(ctx, id)
		return runAfter(ctx, tx, after)
	})
	if err != nil {
		return Engagement{}, err
	}
	return result, nil
}

// ByID returns the engagement with this identifier, or [apierr.NotFound].
func (r *Engagements) ByID(ctx context.Context, id string) (Engagement, error) {
	e, err := scanEngagement(r.db.Read().QueryRowContext(ctx, selectEngagement+`WHERE id = ?`, id))
	if err != nil {
		return Engagement{}, fmt.Errorf("engagement: read %q: %w", id, err)
	}
	return e, nil
}

// CountByAttackVersion returns how many engagements pin the given ATT&CK version.
// This is the attackpin.References implementation backing DeleteVersion refusal.
func (r *Engagements) CountByAttackVersion(ctx context.Context, version string) (int64, error) {
	var count int64
	if err := r.db.Read().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM app.engagement WHERE attack_version = ?`, version,
	).Scan(&count); err != nil {
		return 0, fmt.Errorf("engagement: count by attack version: %w", err)
	}
	return count, nil
}

// scanEngagement reads one row of engagementColumns. It takes the interface both
// *sql.Row and *sql.Rows satisfy.
func scanEngagement(row interface{ Scan(...any) error }) (Engagement, error) {
	var e Engagement
	if err := row.Scan(
		&e.ID, &e.Name, &e.Client, &e.Description,
		&e.Status, &e.StartsOn, &e.EndsOn, &e.AttackVersion,
		&e.Mode, &e.AutoRevealOnStart, &e.CreatedBy,
		&e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Engagement{}, err
		}
		return Engagement{}, err
	}
	e.CreatedAt = e.CreatedAt.UTC()
	e.UpdatedAt = e.UpdatedAt.UTC()
	return e, nil
}

const updateEngagement = `UPDATE app.engagement SET
	name = ?, client = ?, description = ?,
	starts_on = ?, ends_on = ?,
	attack_version = ?, mode = ?, auto_reveal_on_start = ?,
	updated_at = ?
	WHERE id = ?`

// UpdateChanges describes the fields to patch on an engagement.
type UpdateChanges struct {
	Name              string
	Client            string
	Description       string
	StartsOn          time.Time
	EndsOn            time.Time
	AttackVersion     string
	Mode              EngagementMode
	AutoRevealOnStart bool
}

// Update patches an engagement and returns it as stored.
// The caller is responsible for pin assertions and the soft-freeze gate.
func (r *Engagements) Update(ctx context.Context, id string, changes UpdateChanges, after ...After) (Engagement, error) {
	var result Engagement
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		ts := now()
		_, err := tx.ExecContext(ctx, updateEngagement,
			changes.Name, changes.Client, changes.Description,
			changes.StartsOn, changes.EndsOn,
			changes.AttackVersion, string(changes.Mode), changes.AutoRevealOnStart,
			ts, id,
		)
		if err != nil {
			return fmt.Errorf("engagement: update %q: %w", id, err)
		}
		result, err = scanEngagement(tx.QueryRowContext(ctx, selectEngagement+`WHERE id = ?`, id))
		if err != nil {
			return fmt.Errorf("engagement: re-read after update %q: %w", id, err)
		}
		ctx = WithAfterEntity(ctx, id)
		return runAfter(ctx, tx, after)
	})
	if err != nil {
		return Engagement{}, err
	}
	return result, nil
}

const setEngagementStatus = `UPDATE app.engagement SET status = ?, updated_at = ? WHERE id = ?`

// SetStatus updates only the status and returns the engagement as stored.
// The caller is responsible for transition validation.
func (r *Engagements) SetStatus(ctx context.Context, id string, status EngagementStatus, after ...After) (Engagement, error) {
	var result Engagement
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		ts := now()
		res, err := tx.ExecContext(ctx, setEngagementStatus, string(status), ts, id)
		if err != nil {
			return fmt.Errorf("engagement: set status %q: %w", id, err)
		}
		if err := requireOneRow(res, "engagement", id); err != nil {
			return err
		}
		result, err = scanEngagement(tx.QueryRowContext(ctx, selectEngagement+`WHERE id = ?`, id))
		if err != nil {
			return fmt.Errorf("engagement: re-read after set status %q: %w", id, err)
		}
		ctx = WithAfterEntity(ctx, id)
		return runAfter(ctx, tx, after)
	})
	if err != nil {
		return Engagement{}, err
	}
	return result, nil
}

// Sub-selects naming the rows under one engagement. Each ends with a single
// `?` bound to the engagement id, so every statement built from them takes
// exactly one parameter.
const (
	engagementExecutions = `SELECT e.id FROM app.execution e
		JOIN app.step s ON e.step_id = s.id
		JOIN app.scenario sc ON s.scenario_id = sc.id
		WHERE sc.engagement_id = ?`

	engagementComments = `SELECT id FROM app."comment" WHERE execution_id IN (` + engagementExecutions + `)`

	engagementReports = `SELECT id FROM app.report WHERE engagement_id = ?`

	engagementReportVersions = `SELECT id FROM app.report_version WHERE report_id IN (` + engagementReports + `)`

	engagementReportShares = `SELECT id FROM app.report_share WHERE version_id IN (` + engagementReportVersions + `)`

	engagementTemplates = `SELECT id FROM app.report_template WHERE engagement_id = ?`
)

// deleteEngagementGraph is the ordered list of statements that erase an
// engagement and everything hanging off it. The order respects the FK RESTRICT
// constraints — children before parents.
//
// It is a list rather than one semicolon-separated script, and each statement
// runs in a transaction of its own, for two separate DuckDB reasons:
//
// One: DuckDB cannot bind parameters across a multi-statement Exec. It prepares
// the first statement only and rejects the call with "incorrect argument count
// for command", so a script with a `?` in every statement never runs at all.
//
// Two: DuckDB enforces a RESTRICT foreign key against the child's index, which
// does not reflect the current transaction's own deletes. Removing a child and
// then its parent in one transaction therefore fails —
//
//	Violates foreign key constraint because key "report_id: …" is still
//	referenced by a foreign key in a different table
//
// — even though the referencing row was deleted moments earlier. The child's
// removal has to be committed before the parent's can be attempted, which is
// exactly what one transaction per statement gives. See "foreign key
// limitations" in the DuckDB docs.
//
// The cost is that the whole delete is not atomic. Every statement is a
// `DELETE … WHERE` keyed on the engagement, so re-running the operation after a
// failure picks up where it stopped and finishes the job; a caller that fails
// midway leaves a partly emptied engagement, not a corrupt one.
var deleteEngagementGraph = []string{
	// Reports: grants → shares → versions/blocks → report.
	`DELETE FROM app.report_share_grant WHERE share_id IN (` + engagementReportShares + `)`,
	`DELETE FROM app.report_share WHERE version_id IN (` + engagementReportVersions + `)`,
	`DELETE FROM app.report_version WHERE report_id IN (` + engagementReports + `)`,
	`DELETE FROM app.report_block WHERE report_id IN (` + engagementReports + `)`,
	`DELETE FROM app.report WHERE engagement_id = ?`,

	// Report templates: blocks → template.
	`DELETE FROM app.report_template_block WHERE template_id IN (` + engagementTemplates + `)`,
	`DELETE FROM app.report_template WHERE engagement_id = ?`,

	// Denormalized history and audit rows. Neither carries an FK, so nothing
	// forces their removal — they are dropped here so the engagement leaves no
	// orphans behind.
	`DELETE FROM app.finding_status_history WHERE engagement_id = ?`,
	`DELETE FROM app.activity WHERE engagement_id = ?`,

	// Comment revisions sit under comments, which sit under executions.
	`DELETE FROM app.comment_revision WHERE comment_id IN (` + engagementComments + `)`,

	// Evidence rows hold a ref on their blob. Release those refs before the
	// rows go, or the blobs never reach ref_count 0 and GC never reclaims the
	// files. A blob referenced by several of these rows is decremented once per
	// row, hence the correlated count rather than a flat -1.
	`UPDATE app.evidence_blob SET ref_count = ref_count - (
		SELECT COUNT(*) FROM app.evidence ev
		WHERE ev.blob_sha256 = app.evidence_blob.sha256
		  AND ev.execution_id IN (` + engagementExecutions + `)
	) WHERE sha256 IN (
		SELECT blob_sha256 FROM app.evidence WHERE execution_id IN (` + engagementExecutions + `)
	)`,
	`UPDATE app.evidence_blob SET ref_count = ref_count - (
		SELECT COUNT(*) FROM app.evidence ev
		WHERE ev.blob_sha256 = app.evidence_blob.sha256
		  AND ev.comment_id IN (` + engagementComments + `)
	) WHERE sha256 IN (
		SELECT blob_sha256 FROM app.evidence WHERE comment_id IN (` + engagementComments + `)
	)`,
	`DELETE FROM app.evidence WHERE execution_id IN (` + engagementExecutions + `)`,
	`DELETE FROM app.evidence WHERE comment_id IN (` + engagementComments + `)`,

	// The workbook itself.
	`DELETE FROM app."comment" WHERE execution_id IN (` + engagementExecutions + `)`,
	`DELETE FROM app.finding_step WHERE finding_id IN (SELECT id FROM app.finding WHERE engagement_id = ?)`,
	`DELETE FROM app.finding WHERE engagement_id = ?`,
	`DELETE FROM app.execution WHERE step_id IN (SELECT s.id FROM app.step s JOIN app.scenario sc ON s.scenario_id = sc.id WHERE sc.engagement_id = ?)`,
	`DELETE FROM app.step WHERE scenario_id IN (SELECT id FROM app.scenario WHERE engagement_id = ?)`,
	`DELETE FROM app.scenario WHERE engagement_id = ?`,
	`DELETE FROM app.engagement_member WHERE engagement_id = ?`,
}

const deleteEngagementRow = `DELETE FROM app.engagement WHERE id = ?`

// Delete removes an engagement and every row in its workbook graph. It reports
// a not-found error when no engagement has that id.
//
// The graph is emptied one committed statement at a time rather than in a
// single transaction; see [deleteEngagementGraph] for why DuckDB leaves no
// choice, and what that means if a statement fails partway through.
func (r *Engagements) Delete(ctx context.Context, id string) error {
	for _, stmt := range deleteEngagementGraph {
		args := make([]any, strings.Count(stmt, "?"))
		for i := range args {
			args[i] = id
		}
		err := r.db.Write(ctx, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, stmt, args...)
			return err
		})
		if err != nil {
			return fmt.Errorf("engagement: delete %q: %w", id, err)
		}
	}

	// Last, so that a missing engagement is reported only after the graph
	// statements have proved they run clean.
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, deleteEngagementRow, id)
		if err != nil {
			return fmt.Errorf("engagement: delete %q: %w", id, err)
		}
		return requireOneRow(result, "engagement", id)
	})
}

const countEngagementSteps = `
	SELECT COUNT(*)
	FROM app.step s
	JOIN app.scenario sc ON s.scenario_id = sc.id
	WHERE sc.engagement_id = ?
`

// CountSteps returns how many steps exist under an engagement.
func (r *Engagements) CountSteps(ctx context.Context, engagementID string) (int64, error) {
	var count int64
	if err := r.db.Read().QueryRowContext(ctx, countEngagementSteps,
		engagementID).Scan(&count); err != nil {
		return 0, fmt.Errorf("engagement: count steps %q: %w", engagementID, err)
	}
	return count, nil
}

const listEngagementsBase = selectEngagement + `WHERE 1=1 `

// ListFilter narrows the engagement list.
type ListFilter struct {
	Status   string // empty means no filter
	After    string // cursor: id of the last row from the previous page
	Limit    int    // max rows; 0 means the caller's default
	MemberID string // when set, only engagements this user belongs to are returned
}

// List returns a page of engagements, newest first by created_at descending,
// with id as the stable tiebreaker.
func (r *Engagements) List(ctx context.Context, filter ListFilter) ([]Engagement, error) {
	args := make([]any, 0, 4)
	query := listEngagementsBase

	if filter.MemberID != "" {
		query += `AND id IN (SELECT engagement_id FROM app.engagement_member WHERE user_id = ?) `
		args = append(args, filter.MemberID)
	}

	if filter.After != "" {
		// DuckDB rejects a two-column scalar subquery in a row comparison, so
		// the cursor is expanded into an OR over created_at with id as the
		// tiebreaker — the same shape activity.List uses.
		query += `AND (created_at < (SELECT created_at FROM app.engagement WHERE id = ?) OR (created_at = (SELECT created_at FROM app.engagement WHERE id = ?) AND id < ?)) `
		args = append(args, filter.After, filter.After, filter.After)
	}
	if filter.Status != "" {
		query += `AND status = ? `
		args = append(args, filter.Status)
	}
	query += `ORDER BY created_at DESC, id DESC `

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	query += `LIMIT ?`
	args = append(args, limit)

	rows, err := r.db.Read().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("engagement: list: %w", err)
	}
	defer rows.Close()

	var out []Engagement
	for rows.Next() {
		e, err := scanEngagement(rows)
		if err != nil {
			return nil, fmt.Errorf("engagement: list: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("engagement: list: %w", err)
	}
	return out, nil
}
