package engagement

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

const executionColumns = `id, step_id, version, status, executed_by, started_at, ended_at, command_run, source_host, target_host, red_notes, detection_category, detection_modifiers, protection, detected_at, detecting_source, detecting_rule_ref, alert_severity, blue_notes, scored_by, scored_at, created_at, updated_at`


const executionColumnsQualified = `app.execution.id, app.execution.step_id, app.execution.version, app.execution.status, app.execution.executed_by, app.execution.started_at, app.execution.ended_at, app.execution.command_run, app.execution.source_host, app.execution.target_host, app.execution.red_notes, app.execution.detection_category, app.execution.detection_modifiers, app.execution.protection, app.execution.detected_at, app.execution.detecting_source, app.execution.detecting_rule_ref, app.execution.alert_severity, app.execution.blue_notes, app.execution.scored_by, app.execution.scored_at, app.execution.created_at, app.execution.updated_at`

const selectExecutionQualified = `SELECT ` + executionColumnsQualified + ` FROM app.execution `
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
		var vc *versionConflictError
		if errors.As(err, &vc) {
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
		if errors.Is(err, sql.ErrNoRows) {
			return Execution{}, apierr.NotFound("execution", "(id)")
		}
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

// RedPatchChanges describes the red-side fields to patch on an execution.
// All fields are optional; only non-nil fields (except ExecutedBy which uses
// the string pointer pattern) are written.
type RedPatchChanges struct {
	Status     *ExecutionStatus
	StartedAt  *time.Time
	EndedAt    *time.Time
	CommandRun *string
	SourceHost *string
	TargetHost *string
	RedNotes   *string
	ExecutedBy *string
}

// PatchRed atomically updates red-side fields on an execution with
// optimistic locking. The caller must supply the version they read;
// a mismatch returns an error wrapping [versionConflictError].
// Non-nil fields in changes are written; the version is incremented
// and updated_at is set to now on success.
func (r *Executions) PatchRed(ctx context.Context, id string, expectedVersion int, changes RedPatchChanges, after ...After) (Execution, error) {
	var e Execution
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		// Build dynamic SET clause from non-nil fields.
		var sets []string
		var args []any

		if changes.Status != nil {
			sets = append(sets, "status = ?")
			args = append(args, string(*changes.Status))
		}
		if changes.StartedAt != nil {
			sets = append(sets, "started_at = ?")
			args = append(args, toStorage(*changes.StartedAt))
		}
		if changes.EndedAt != nil {
			sets = append(sets, "ended_at = ?")
			args = append(args, toStorage(*changes.EndedAt))
		}
		if changes.CommandRun != nil {
			sets = append(sets, "command_run = ?")
			args = append(args, *changes.CommandRun)
		}
		if changes.SourceHost != nil {
			sets = append(sets, "source_host = ?")
			args = append(args, *changes.SourceHost)
		}
		if changes.TargetHost != nil {
			sets = append(sets, "target_host = ?")
			args = append(args, *changes.TargetHost)
		}
		if changes.RedNotes != nil {
			sets = append(sets, "red_notes = ?")
			args = append(args, *changes.RedNotes)
		}
		if changes.ExecutedBy != nil {
			sets = append(sets, "executed_by = ?")
			args = append(args, *changes.ExecutedBy)
		}

		sets = append(sets, "version = version + 1", "updated_at = ?")
		args = append(args, now())

		// Append version check and id at the end.
		args = append(args, id, expectedVersion)

		query := "UPDATE app.execution SET " + strings.Join(sets, ", ") + " WHERE id = ? AND version = ?" //nolint:gosec // sets built from hardcoded column names, not user input

		result, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("execution: patch red %q: %w", id, err)
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			current, err := scanExecution(tx.QueryRowContext(ctx, selectExecution+`WHERE id = ?`, id))
			if err != nil {
				return err
			}
			return &versionConflictError{current: current}
		}

		// Re-read to return the updated row.
		var scanErr error
		e, scanErr = scanExecution(tx.QueryRowContext(ctx, selectExecution+`WHERE id = ?`, id))
		if scanErr != nil {
			return scanErr
		}

		ctx := WithAfterEntity(ctx, e.ID)
		return runAfter(ctx, tx, after)
	})
	if err != nil {
		var vc *versionConflictError
		if errors.As(err, &vc) {
			return Execution{}, fmt.Errorf("execution %s: version conflict: expected %d, current %d: %w",
				id, expectedVersion, vc.current.Version, err)
		}
		return Execution{}, err
	}
	return e, nil
}

// BluePatchChanges describes the blue-side fields to patch on an execution.
// All fields are optional; only non-nil fields are written.
type BluePatchChanges struct {
	DetectionCategory  *DetectionCategory
	DetectionModifiers json.RawMessage
	Protection         *Protection
	DetectedAt         *time.Time
	DetectingSource    *string
	DetectingRuleRef   *string
	AlertSeverity      *string
	BlueNotes          *string
	ScoredBy           *string
	ScoredAt           *time.Time
}

// PatchBlue atomically updates blue-side fields on an execution with
// optimistic locking. The caller must supply the version they read;
// a mismatch returns an error wrapping [versionConflictError].
// Non-nil fields in changes are written; the version is incremented
// and updated_at is set to now on success.
func (r *Executions) PatchBlue(ctx context.Context, id string, expectedVersion int, changes BluePatchChanges, after ...After) (Execution, error) {
	var e Execution
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		var sets []string
		var args []any

		if changes.DetectionCategory != nil {
			sets = append(sets, "detection_category = ?")
			args = append(args, string(*changes.DetectionCategory))
		}
		// detection_modifiers is JSON — always write when provided (even empty array).
		if changes.DetectionModifiers != nil {
			sets = append(sets, "detection_modifiers = ?")
			args = append(args, bindJSON(changes.DetectionModifiers))
		}
		if changes.Protection != nil {
			sets = append(sets, "protection = ?")
			args = append(args, string(*changes.Protection))
		}
		if changes.DetectedAt != nil {
			sets = append(sets, "detected_at = ?")
			args = append(args, toStorage(*changes.DetectedAt))
		}
		if changes.DetectingSource != nil {
			sets = append(sets, "detecting_source = ?")
			args = append(args, *changes.DetectingSource)
		}
		if changes.DetectingRuleRef != nil {
			sets = append(sets, "detecting_rule_ref = ?")
			args = append(args, *changes.DetectingRuleRef)
		}
		if changes.AlertSeverity != nil {
			sets = append(sets, "alert_severity = ?")
			args = append(args, *changes.AlertSeverity)
		}
		if changes.BlueNotes != nil {
			sets = append(sets, "blue_notes = ?")
			args = append(args, *changes.BlueNotes)
		}
		if changes.ScoredBy != nil {
			sets = append(sets, "scored_by = ?")
			args = append(args, *changes.ScoredBy)
		}
		if changes.ScoredAt != nil {
			sets = append(sets, "scored_at = ?")
			args = append(args, toStorage(*changes.ScoredAt))
		}

		sets = append(sets, "version = version + 1", "updated_at = ?")
		args = append(args, now())

		args = append(args, id, expectedVersion)

		query := "UPDATE app.execution SET " + strings.Join(sets, ", ") + " WHERE id = ? AND version = ?" //nolint:gosec // sets built from hardcoded column names, not user input

		result, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("execution: patch blue %q: %w", id, err)
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			current, err := scanExecution(tx.QueryRowContext(ctx, selectExecution+`WHERE id = ?`, id))
			if err != nil {
				return err
			}
			return &versionConflictError{current: current}
		}

		var scanErr error
		e, scanErr = scanExecution(tx.QueryRowContext(ctx, selectExecution+`WHERE id = ?`, id))
		if scanErr != nil {
			return scanErr
		}

		ctx := WithAfterEntity(ctx, e.ID)
		return runAfter(ctx, tx, after)
	})
	if err != nil {
		var vc *versionConflictError
		if errors.As(err, &vc) {
			return Execution{}, fmt.Errorf("execution %s: version conflict: expected %d, current %d: %w",
				id, expectedVersion, vc.current.Version, err)
		}
		return Execution{}, err
	}
	return e, nil
}

// ListByEngagement returns executions for an engagement, optionally
// filtered by scenario and/or status. Ordered by scenario ordinal,
// then step ordinal so the workbook order is stable.
func (r *Executions) ListByEngagement(ctx context.Context, engagementID string, scenarioID *string, status *ExecutionStatus) ([]Execution, error) {
	query := selectExecutionQualified + `
		JOIN app.step ON app.step.id = app.execution.step_id
		JOIN app.scenario ON app.scenario.id = app.step.scenario_id
		WHERE app.scenario.engagement_id = ?`
	args := []any{engagementID}

	if scenarioID != nil {
		query += ` AND app.scenario.id = ?`
		args = append(args, *scenarioID)
	}
	if status != nil {
		query += ` AND app.execution.status = ?`
		args = append(args, string(*status))
	}

	query += ` ORDER BY app.scenario.ordinal ASC, app.step.ordinal ASC`

	rows, err := r.db.Read().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("execution: list by engagement %q: %w", engagementID, err)
	}
	defer rows.Close()

	var executions []Execution
	for rows.Next() {
		e, err := scanExecution(rows)
		if err != nil {
			return nil, err
		}
		executions = append(executions, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("execution: list by engagement %q: %w", engagementID, err)
	}
	return executions, nil
}
