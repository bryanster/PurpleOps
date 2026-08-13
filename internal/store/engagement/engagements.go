package engagement

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

const deleteEngagementGraph = `
	DELETE FROM app.activity WHERE engagement_id = ?;
	DELETE FROM app.comment_revision WHERE comment_id IN (SELECT id FROM app."comment" WHERE execution_id IN (SELECT e.id FROM app.execution e JOIN app.step s ON e.step_id = s.id JOIN app.scenario sc ON s.scenario_id = sc.id WHERE sc.engagement_id = ?));
	DELETE FROM app.evidence WHERE execution_id IN (SELECT e.id FROM app.execution e JOIN app.step s ON e.step_id = s.id JOIN app.scenario sc ON s.scenario_id = sc.id WHERE sc.engagement_id = ?) OR comment_id IN (SELECT id FROM app."comment" WHERE execution_id IN (SELECT e.id FROM app.execution e JOIN app.step s ON e.step_id = s.id JOIN app.scenario sc ON s.scenario_id = sc.id WHERE sc.engagement_id = ?));
	DELETE FROM app."comment" WHERE execution_id IN (SELECT e.id FROM app.execution e JOIN app.step s ON e.step_id = s.id JOIN app.scenario sc ON s.scenario_id = sc.id WHERE sc.engagement_id = ?);
	DELETE FROM app.finding_step WHERE finding_id IN (SELECT id FROM app.finding WHERE engagement_id = ?);
	DELETE FROM app.finding WHERE engagement_id = ?;
	DELETE FROM app.execution WHERE step_id IN (SELECT s.id FROM app.step s JOIN app.scenario sc ON s.scenario_id = sc.id WHERE sc.engagement_id = ?);
	DELETE FROM app.step WHERE scenario_id IN (SELECT id FROM app.scenario WHERE engagement_id = ?);
	DELETE FROM app.scenario WHERE engagement_id = ?;
	DELETE FROM app.engagement_member WHERE engagement_id = ?;
	DELETE FROM app.engagement WHERE id = ?;
`

// Delete removes an engagement and every row in its workbook graph.
// The order in deleteEngagementGraph respects FK RESTRICT constraints so that
// child rows are dropped before their parents.
func (r *Engagements) Delete(ctx context.Context, id string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, deleteEngagementGraph,
			// activity
			id,
			// comment_revision
			id,
			// evidence (execution parent)
			id,
			// evidence (comment parent) — second occurrence in third position
			id,
			// comment
			id,
			// finding_step
			id,
			// finding
			id,
			// execution
			id,
			// step
			id,
			// scenario
			id,
			// engagement_member
			id,
			// engagement
			id,
		)
		if err != nil {
			return fmt.Errorf("engagement: delete %q: %w", id, err)
		}
		return nil
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
