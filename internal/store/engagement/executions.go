package engagement

import (
	"context"
	"database/sql"
	"fmt"
)

const executionColumns = `id, step_id, version, status, executed_by, started_at, ended_at, command_run, source_host, target_host, red_notes, detection_category, detection_modifiers, protection, detected_at, detecting_source, detecting_rule_ref, alert_severity, blue_notes, scored_by, scored_at, created_at, updated_at`

const selectExecution = `SELECT ` + executionColumns + ` FROM app.execution `

const insertExecution = `INSERT INTO app.execution
	(id, step_id, version, status, executed_by, started_at, ended_at, command_run, source_host, target_host, red_notes, detection_category, detection_modifiers, protection, detected_at, detecting_source, detecting_rule_ref, alert_severity, blue_notes, scored_by, scored_at, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// Executions reads and writes red + blue fill-in rows. Construct it with
// [NewExecutions].
type Executions struct {
	db DB
}

// NewExecutions returns a repository over db.
func NewExecutions(db DB) *Executions { return &Executions{db: db} }

// ByStepID returns the execution for a step, or the error from the scanner.
func (r *Executions) ByStepID(ctx context.Context, stepID string) (Execution, error) {
	e, err := scanExecution(r.db.Read().QueryRowContext(ctx, selectExecution+`WHERE step_id = ?`, stepID))
	if err != nil {
		return Execution{}, fmt.Errorf("execution: read by step %q: %w", stepID, err)
	}
	return e, nil
}

// ByID returns the execution with this identifier.
func (r *Executions) ByID(ctx context.Context, id string) (Execution, error) {
	e, err := scanExecution(r.db.Read().QueryRowContext(ctx, selectExecution+`WHERE id = ?`, id))
	if err != nil {
		return Execution{}, fmt.Errorf("execution: read %q: %w", id, err)
	}
	return e, nil
}

// IncrementVersion atomically bumps the version and returns the new value.
// Used by red/blue PATCH handlers for optimistic locking. The caller must
// supply the version they read; a mismatch is [apierr.Conflict].
func (r *Executions) IncrementVersion(ctx context.Context, id string, expectedVersion int) (int, error) {
	var newVersion int
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE app.execution SET version = version + 1, updated_at = ? WHERE id = ? AND version = ?`,
			now(), id, expectedVersion,
		)
		if err != nil {
			return fmt.Errorf("execution: increment version: %w", err)
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			// Version conflict — read the current row to report it.
			current, err := scanExecution(tx.QueryRowContext(ctx, selectExecution+`WHERE id = ?`, id))
			if err != nil {
				return err
			}
			return &versionConflictError{current: current}
		}
		newVersion = expectedVersion + 1
		return nil
	})
	if err != nil {
		if vc, ok := err.(*versionConflictError); ok {
			return 0, fmt.Errorf("execution %s: version conflict: expected %d, current %d: %w",
				id, expectedVersion, vc.current.Version, err)
		}
		return 0, err
	}
	return newVersion, nil
}

// versionConflictError is a sentinel for optimistic-lock failures.
type versionConflictError struct {
	current Execution
}

func (e *versionConflictError) Error() string {
	return "version conflict"
}

func scanExecution(row interface{ Scan(...any) error }) (Execution, error) {
	var e Execution
	var startedAt, endedAt sql.NullTime
	var detectionCategory sql.NullString
	var protection sql.NullString
	var detectedAt, scoredAt sql.NullTime
	var modifiers any

	if err := row.Scan(
		&e.ID, &e.StepID, &e.Version,
		&e.Status, &e.ExecutedBy,
		&startedAt, &endedAt,
		&e.CommandRun, &e.SourceHost, &e.TargetHost, &e.RedNotes,
		&detectionCategory, &modifiers, &protection,
		&detectedAt,
		&e.DetectingSource, &e.DetectingRuleRef, &e.AlertSeverity,
		&e.BlueNotes, &e.ScoredBy, &scoredAt,
		&e.CreatedAt, &e.UpdatedAt,
	); err != nil {
		return Execution{}, err
	}

	e.StartedAt = fromNullTime(startedAt)
	e.EndedAt = fromNullTime(endedAt)
	e.DetectedAt = fromNullTime(detectedAt)
	e.ScoredAt = fromNullTime(scoredAt)

	if detectionCategory.Valid {
		dc := DetectionCategory(detectionCategory.String)
		e.DetectionCategory = &dc
	}
	if protection.Valid {
		p := Protection(protection.String)
		e.Protection = &p
	}

	var err error
	if e.DetectionModifiers, err = jsonBytes(modifiers); err != nil {
		return Execution{}, fmt.Errorf("execution: detection_modifiers: %w", err)
	}

	e.CreatedAt = e.CreatedAt.UTC()
	e.UpdatedAt = e.UpdatedAt.UTC()
	return e, nil
}
