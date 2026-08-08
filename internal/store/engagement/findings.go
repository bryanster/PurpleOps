package engagement

import (
	"context"
	"database/sql"
	"fmt"
)

const findingColumns = `id, engagement_id, title, description, severity, recommendation, "owner", status, created_from_execution, created_at, updated_at`

const selectFinding = `SELECT ` + findingColumns + ` FROM app.finding `

const insertFinding = `INSERT INTO app.finding
	(id, engagement_id, title, description, severity, recommendation, "owner", status, created_from_execution, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// Findings reads and writes remediation items. Construct it with [NewFindings].
type Findings struct {
	db DB
}

// NewFindings returns a repository over db.
func NewFindings(db DB) *Findings { return &Findings{db: db} }

// Create writes a new finding and returns it as stored.
func (r *Findings) Create(ctx context.Context, in NewFinding, after ...After) (Finding, error) {
	var result Finding
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		id, err := newID()
		if err != nil {
			return fmt.Errorf("finding: generate id: %w", err)
		}
		ts := now()
		result = Finding{
			ID:                   id,
			EngagementID:         in.EngagementID,
			Title:                in.Title,
			Description:          in.Description,
			Severity:             in.Severity,
			Recommendation:       in.Recommendation,
			Owner:                in.Owner,
			Status:               FindingStatusOpen,
			CreatedFromExecution: in.CreatedFromExecution,
			CreatedAt:            ts,
			UpdatedAt:            ts,
		}
		_, err = tx.ExecContext(ctx, insertFinding,
			result.ID,
			result.EngagementID,
			result.Title,
			result.Description,
			result.Severity,
			result.Recommendation,
			result.Owner,
			string(result.Status),
			nullString(result.CreatedFromExecution),
			result.CreatedAt,
			result.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("finding: insert: %w", err)
		}
		ctx = WithAfterEntity(ctx, id)
		return runAfter(ctx, tx, after)
	})
	if err != nil {
		return Finding{}, err
	}
	return result, nil
}

// ByID returns the finding with this identifier.
func (r *Findings) ByID(ctx context.Context, id string) (Finding, error) {
	f, err := scanFinding(r.db.Read().QueryRowContext(ctx, selectFinding+`WHERE id = ?`, id))
	if err != nil {
		return Finding{}, fmt.Errorf("finding: read %q: %w", id, err)
	}
	return f, nil
}

// Update patches a finding. Only non-nil/non-empty fields from [PatchFinding]
// are applied. The updated_at timestamp is always bumped.
func (r *Findings) Update(ctx context.Context, id string, in PatchFinding, after ...After) (Finding, error) {
	var result Finding
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		ts := now()
		_, err := tx.ExecContext(ctx,
			`UPDATE app.finding SET
				title = COALESCE(NULLIF(?, ''), title),
				description = COALESCE(NULLIF(?, ''), description),
				severity = COALESCE(NULLIF(?, ''), severity),
				recommendation = COALESCE(NULLIF(?, ''), recommendation),
				"owner" = COALESCE(NULLIF(?, ''), "owner"),
				status = COALESCE(NULLIF(?, ''), status),
				updated_at = ?
			WHERE id = ?`,
			in.Title, in.Description, in.Severity, in.Recommendation, in.Owner, in.Status,
			ts, id,
		)
		if err != nil {
			return fmt.Errorf("finding: update: %w", err)
		}
		ctx = WithAfterEntity(ctx, id)
		if err := runAfter(ctx, tx, after); err != nil {
			return err
		}
		// Re-read to return the current row.
		result, err = scanFinding(tx.QueryRowContext(ctx, selectFinding+`WHERE id = ?`, id))
		return err
	})
	if err != nil {
		return Finding{}, err
	}
	return result, nil
}

// Delete removes a finding and its finding_step rows.
func (r *Findings) Delete(ctx context.Context, id string, after ...After) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM app.finding_step WHERE finding_id = ?`, id)
		if err != nil {
			return fmt.Errorf("finding: delete steps: %w", err)
		}
		res, err := tx.ExecContext(ctx, `DELETE FROM app.finding WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("finding: delete: %w", err)
		}
		if err := requireOneRow(res, "finding", id); err != nil {
			return err
		}
		ctx = WithAfterEntity(ctx, id)
		return runAfter(ctx, tx, after)
	})
}

// SetSteps replaces the entire step set for a finding inside one transaction.
func (r *Findings) SetSteps(ctx context.Context, findingID string, stepIDs []string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM app.finding_step WHERE finding_id = ?`, findingID)
		if err != nil {
			return fmt.Errorf("finding: set steps delete: %w", err)
		}
		for _, stepID := range stepIDs {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO app.finding_step (finding_id, step_id) VALUES (?, ?)`,
				findingID, stepID,
			)
			if err != nil {
				return fmt.Errorf("finding: set steps insert: %w", err)
			}
		}
		return nil
	})
}

// ListByEngagement returns every finding in an engagement, newest first.
func (r *Findings) ListByEngagement(ctx context.Context, engagementID string) ([]Finding, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		selectFinding+`WHERE engagement_id = ? ORDER BY created_at DESC`, engagementID)
	if err != nil {
		return nil, fmt.Errorf("finding: list by engagement: %w", err)
	}
	defer rows.Close()
	var out []Finding
	for rows.Next() {
		f, err := scanFinding(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// AddStep links a step to a finding. Idempotent: a duplicate (finding_id, step_id)
// is a no-op.
func (r *Findings) AddStep(ctx context.Context, findingID, stepID string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO app.finding_step (finding_id, step_id) VALUES (?, ?)`,
			findingID, stepID,
		)
		if err != nil {
			return fmt.Errorf("finding: add step: %w", err)
		}
		return nil
	})
}

// RemoveStep unlinks a step from a finding.
func (r *Findings) RemoveStep(ctx context.Context, findingID, stepID string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`DELETE FROM app.finding_step WHERE finding_id = ? AND step_id = ?`,
			findingID, stepID,
		)
		if err != nil {
			return fmt.Errorf("finding: remove step: %w", err)
		}
		return nil
	})
}

// Steps returns every step linked to a finding, ordered by step ordinal within
// their scenario.
func (r *Findings) Steps(ctx context.Context, findingID string) ([]Step, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		`SELECT `+stepColumns+` FROM app.step
			INNER JOIN app.finding_step ON app.step.id = app.finding_step.step_id
			WHERE app.finding_step.finding_id = ?
			ORDER BY app.step.scenario_id, app.step.ordinal ASC`,
		findingID,
	)
	if err != nil {
		return nil, fmt.Errorf("finding: steps: %w", err)
	}
	defer rows.Close()
	var out []Step
	for rows.Next() {
		s, err := scanStep(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func scanFinding(row interface{ Scan(...any) error }) (Finding, error) {
	var f Finding
	var createdFromExecution sql.NullString
	if err := row.Scan(
		&f.ID, &f.EngagementID, &f.Title, &f.Description,
		&f.Severity, &f.Recommendation, &f.Owner, &f.Status,
		&createdFromExecution,
		&f.CreatedAt, &f.UpdatedAt,
	); err != nil {
		return Finding{}, err
	}
	f.CreatedFromExecution = fromNullString(createdFromExecution)
	f.CreatedAt = f.CreatedAt.UTC()
	f.UpdatedAt = f.UpdatedAt.UTC()
	return f, nil
}
