package engagement

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

const stepColumns = `id, scenario_id, ordinal, name, objective, technique_id, subtechnique_id, tactic_id, "procedure", template_id, target_asset, tools, controls_in_scope, attack_version, revealed_at, created_at, updated_at`

const selectStep = `SELECT ` + stepColumns + ` FROM app.step `

const stepColumnsQualified = `app.step.id, app.step.scenario_id, app.step.ordinal, app.step.name, app.step.objective, app.step.technique_id, app.step.subtechnique_id, app.step.tactic_id, app.step."procedure", app.step.template_id, app.step.target_asset, app.step.tools, app.step.controls_in_scope, app.step.attack_version, app.step.revealed_at, app.step.created_at, app.step.updated_at`

const selectStepQualified = `SELECT ` + stepColumnsQualified + ` FROM app.step `

const insertStep = `INSERT INTO app.step
	(id, scenario_id, ordinal, name, objective, technique_id, subtechnique_id, tactic_id, "procedure", template_id, target_asset, tools, controls_in_scope, attack_version, revealed_at, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// Steps reads and writes technique/procedure rows. Construct it with [NewSteps].
type Steps struct {
	db DB
}

// NewSteps returns a repository over db.
func NewSteps(db DB) *Steps { return &Steps{db: db} }

// Create writes a new step and returns it as stored.
func (r *Steps) Create(ctx context.Context, in NewStep, after ...After) (Step, error) {
	var result Step
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		id, err := newID()
		if err != nil {
			return fmt.Errorf("step: generate id: %w", err)
		}
		ts := now()
		result = Step{
			ID:              id,
			ScenarioID:      in.ScenarioID,
			Ordinal:         in.Ordinal,
			Name:            in.Name,
			Objective:       in.Objective,
			TechniqueID:     in.TechniqueID,
			SubtechniqueID:  in.SubtechniqueID,
			TacticID:        in.TacticID,
			Procedure:       in.Procedure,
			TemplateID:      in.TemplateID,
			TargetAsset:     in.TargetAsset,
			Tools:           in.Tools,
			ControlsInScope: in.ControlsInScope,
			AttackVersion:   in.AttackVersion,
			RevealedAt:      nil,
			CreatedAt:       ts,
			UpdatedAt:       ts,
		}
		_, err = tx.ExecContext(ctx, insertStep,
			result.ID,
			result.ScenarioID,
			result.Ordinal,
			result.Name,
			result.Objective,
			nullString(result.TechniqueID),
			nullString(result.SubtechniqueID),
			nullString(result.TacticID),
			bindJSONObject(result.Procedure),
			result.TemplateID,
			result.TargetAsset,
			bindJSON(result.Tools),
			bindJSON(result.ControlsInScope),
			result.AttackVersion,
			nullTime(result.RevealedAt),
			result.CreatedAt,
			result.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("step: insert: %w", err)
		}
		ctx = WithAfterEntity(ctx, id)
		return runAfter(ctx, tx, after)
	})
	if err != nil {
		return Step{}, err
	}
	return result, nil
}

// CreateWithExecution creates a step and its pending execution in one
// transaction. The execution version starts at 1. Both rows are returned.
func (r *Steps) CreateWithExecution(ctx context.Context, in NewStep, after ...After) (Step, Execution, error) {
	var step Step
	var exec Execution
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		stepID, err := newID()
		if err != nil {
			return fmt.Errorf("step: generate id: %w", err)
		}
		execID, err := newID()
		if err != nil {
			return fmt.Errorf("step: generate execution id: %w", err)
		}
		ts := now()
		step = Step{
			ID:              stepID,
			ScenarioID:      in.ScenarioID,
			Ordinal:         in.Ordinal,
			Name:            in.Name,
			Objective:       in.Objective,
			TechniqueID:     in.TechniqueID,
			SubtechniqueID:  in.SubtechniqueID,
			TacticID:        in.TacticID,
			Procedure:       in.Procedure,
			TemplateID:      in.TemplateID,
			TargetAsset:     in.TargetAsset,
			Tools:           in.Tools,
			ControlsInScope: in.ControlsInScope,
			AttackVersion:   in.AttackVersion,
			RevealedAt:      nil,
			CreatedAt:       ts,
			UpdatedAt:       ts,
		}
		_, err = tx.ExecContext(ctx, insertStep,
			step.ID,
			step.ScenarioID,
			step.Ordinal,
			step.Name,
			step.Objective,
			nullString(step.TechniqueID),
			nullString(step.SubtechniqueID),
			nullString(step.TacticID),
			bindJSONObject(step.Procedure),
			step.TemplateID,
			step.TargetAsset,
			bindJSON(step.Tools),
			bindJSON(step.ControlsInScope),
			step.AttackVersion,
			nullTime(step.RevealedAt),
			step.CreatedAt,
			step.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("step: insert: %w", err)
		}

		exec = Execution{
			ID:                 execID,
			StepID:             stepID,
			Version:            1,
			Status:             ExecutionStatusPending,
			ExecutedBy:         "",
			StartedAt:          nil,
			EndedAt:            nil,
			CommandRun:         "",
			SourceHost:         "",
			TargetHost:         "",
			RedNotes:           "",
			DetectionCategory:  nil,
			DetectionModifiers: json.RawMessage(`[]`),
			Protection:         nil,
			DetectedAt:         nil,
			DetectingSource:    "",
			DetectingRuleRef:   "",
			AlertSeverity:      "",
			BlueNotes:          "",
			ScoredBy:           "",
			ScoredAt:           nil,
			CreatedAt:          ts,
			UpdatedAt:          ts,
		}
		_, err = tx.ExecContext(ctx, insertExecution,
			exec.ID,
			exec.StepID,
			exec.Version,
			string(exec.Status),
			exec.ExecutedBy,
			nullTime(exec.StartedAt),
			nullTime(exec.EndedAt),
			exec.CommandRun,
			exec.SourceHost,
			exec.TargetHost,
			exec.RedNotes,
			nil, // detection_category
			bindJSON(exec.DetectionModifiers),
			nil, // protection
			nullTime(exec.DetectedAt),
			exec.DetectingSource,
			exec.DetectingRuleRef,
			exec.AlertSeverity,
			exec.BlueNotes,
			exec.ScoredBy,
			nullTime(exec.ScoredAt),
			exec.CreatedAt,
			exec.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("step: insert execution: %w", err)
		}

		ctx = WithAfterEntity(ctx, stepID)
		return runAfter(ctx, tx, after)
	})
	if err != nil {
		return Step{}, Execution{}, err
	}
	return step, exec, nil
}

// ByID returns the step with this identifier.
func (r *Steps) ByID(ctx context.Context, id string) (Step, error) {
	s, err := scanStep(r.db.Read().QueryRowContext(ctx, selectStep+`WHERE id = ?`, id))
	if err != nil {
		return Step{}, fmt.Errorf("step: read %q: %w", id, err)
	}
	return s, nil
}

// ListByScenario returns every step in a scenario, ordered by ordinal.
func (r *Steps) ListByScenario(ctx context.Context, scenarioID string) ([]Step, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		selectStep+`WHERE scenario_id = ? ORDER BY ordinal ASC`, scenarioID)
	if err != nil {
		return nil, fmt.Errorf("step: list by scenario: %w", err)
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

// NextOrdinal returns one more than the current max ordinal for a scenario.
func (r *Steps) NextOrdinal(ctx context.Context, scenarioID string) (int, error) {
	var maxOrdinal sql.NullInt64
	if err := r.db.Read().QueryRowContext(ctx,
		`SELECT MAX(ordinal) FROM app.step WHERE scenario_id = ?`, scenarioID,
	).Scan(&maxOrdinal); err != nil {
		return 0, fmt.Errorf("step: next ordinal: %w", err)
	}
	if maxOrdinal.Valid {
		return int(maxOrdinal.Int64) + 1, nil
	}
	return 1, nil
}

func scanStep(row interface{ Scan(...any) error }) (Step, error) {
	var (
		s              Step
		revealedAt     sql.NullTime
		techniqueID    sql.NullString
		subtechniqueID sql.NullString
		tacticID       sql.NullString
		templateID     sql.NullString
		procedure      any
		tools          any
		controls       any
	)
	if err := row.Scan(
		&s.ID, &s.ScenarioID, &s.Ordinal, &s.Name, &s.Objective,
		&techniqueID, &subtechniqueID, &tacticID,
		&procedure, &templateID, &s.TargetAsset,
		&tools, &controls,
		&s.AttackVersion, &revealedAt,
		&s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return Step{}, err
	}
	s.TechniqueID = fromNullString(techniqueID)
	s.SubtechniqueID = fromNullString(subtechniqueID)
	s.TacticID = fromNullString(tacticID)
	s.TemplateID = fromNullString(templateID)
	var err error
	if s.Procedure, err = jsonBytes(procedure); err != nil {
		return Step{}, fmt.Errorf("step: procedure: %w", err)
	}
	if s.Tools, err = jsonBytes(tools); err != nil {
		return Step{}, fmt.Errorf("step: tools: %w", err)
	}
	if s.ControlsInScope, err = jsonBytes(controls); err != nil {
		return Step{}, fmt.Errorf("step: controls_in_scope: %w", err)
	}
	s.RevealedAt = fromNullTime(revealedAt)
	s.CreatedAt = s.CreatedAt.UTC()
	s.UpdatedAt = s.UpdatedAt.UTC()
	return s, nil
}

const updateStep = `UPDATE app.step SET
	name = ?, objective = ?, target_asset = ?, tools = ?, controls_in_scope = ?, updated_at = ?
	WHERE id = ?`

// StepUpdateChanges describes the fields to patch on a step.
type StepUpdateChanges struct {
	Name            string
	Objective       string
	TargetAsset     string
	Tools           json.RawMessage
	ControlsInScope json.RawMessage
}

// Update patches a step's always-editable fields and returns it as stored.
// Identity fields (technique, procedure, template, attack_version) are NOT
// patchable here; callers enforce soft-freeze before reaching this method.
func (r *Steps) Update(ctx context.Context, id string, changes StepUpdateChanges, after ...After) (Step, error) {
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		ts := now()
		_, err := tx.ExecContext(ctx, updateStep,
			changes.Name,
			changes.Objective,
			changes.TargetAsset,
			bindJSON(changes.Tools),
			bindJSON(changes.ControlsInScope),
			ts,
			id,
		)
		if err != nil {
			return fmt.Errorf("step: update %q: %w", id, err)
		}
		ctx = WithAfterEntity(ctx, id)
		return runAfter(ctx, tx, after)
	})
	if err != nil {
		return Step{}, err
	}
	return r.ByID(ctx, id)
}

const deleteStepGraph = `
	DELETE FROM app.comment_revision WHERE comment_id IN (SELECT id FROM app."comment" WHERE execution_id IN (SELECT id FROM app.execution WHERE step_id = ?));
	DELETE FROM app.evidence WHERE execution_id IN (SELECT id FROM app.execution WHERE step_id = ?) OR comment_id IN (SELECT id FROM app."comment" WHERE execution_id IN (SELECT id FROM app.execution WHERE step_id = ?));
	DELETE FROM app."comment" WHERE execution_id IN (SELECT id FROM app.execution WHERE step_id = ?);
	DELETE FROM app.finding_step WHERE step_id = ?;
	DELETE FROM app.execution WHERE step_id = ?;
	DELETE FROM app.step WHERE id = ?;
	DELETE FROM app.activity WHERE object_id = ?;
`

// Delete removes a step and its whole graph, then renumbers remaining
// ordinals in the scenario to keep them dense. The order respects FK
// RESTRICT constraints so child rows are dropped before their parents.
func (r *Steps) Delete(ctx context.Context, id string, scenarioID string, ordinal int) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, deleteStepGraph,
			// comment_revision
			id,
			// evidence (execution parent)
			id,
			// evidence (comment parent)
			id,
			// comment
			id,
			// finding_step
			id,
			// execution
			id,
			// step
			id,
			// activity
			id,
		)
		if err != nil {
			return fmt.Errorf("step: delete %q: %w", id, err)
		}
		// Renumber ordinals after the gap to keep them dense and unique.
		ts := now()
		_, err = tx.ExecContext(ctx,
			`UPDATE app.step SET ordinal = ordinal - 1, updated_at = ? WHERE scenario_id = ? AND ordinal > ?`,
			ts, scenarioID, ordinal,
		)
		if err != nil {
			return fmt.Errorf("step: renumber after delete: %w", err)
		}
		return nil
	})
}

const updateStepOrdinals = `UPDATE app.step SET ordinal = ?, updated_at = ? WHERE id = ?`

// Reorder assigns ordinals 1..N to match the order of ids, in one
// transaction. Every id must belong to the scenario; callers must
// validate before calling.
func (r *Steps) Reorder(ctx context.Context, ids []string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		ts := now()
		for i, id := range ids {
			ordinal := i + 1
			_, err := tx.ExecContext(ctx, updateStepOrdinals, ordinal, ts, id)
			if err != nil {
				return fmt.Errorf("step: reorder %q -> %d: %w", id, ordinal, err)
			}
		}
		return nil
	})
}

const revealStep = `UPDATE app.step SET revealed_at = ?, updated_at = ? WHERE id = ? AND revealed_at IS NULL`

// Reveal sets revealed_at to now if it is still NULL. Returns the step as
// stored. Idempotent: an already-revealed step is a no-op and returns the
// current row.
func (r *Steps) Reveal(ctx context.Context, id string) (Step, error) {
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		ts := now()
		_, err := tx.ExecContext(ctx, revealStep, ts, ts, id)
		if err != nil {
			return fmt.Errorf("step: reveal %q: %w", id, err)
		}
		return nil
	})
	if err != nil {
		return Step{}, err
	}
	return r.ByID(ctx, id)
}

const listStepsByEngagement = selectStepQualified + `
	JOIN app.scenario ON app.step.scenario_id = app.scenario.id
	WHERE app.scenario.engagement_id = ?
	ORDER BY app.scenario.ordinal ASC, app.step.ordinal ASC`

// ListByEngagement returns every step across all scenarios in an engagement,
// ordered by scenario ordinal then step ordinal. Callers apply blind filtering
// via [Scope.Where] by concatenating it into the WHERE clause; this method
// returns all steps unfiltered so the caller can control what to hide.
func (r *Steps) ListByEngagement(ctx context.Context, engagementID string) ([]Step, error) {
	rows, err := r.db.Read().QueryContext(ctx, listStepsByEngagement, engagementID)
	if err != nil {
		return nil, fmt.Errorf("step: list by engagement: %w", err)
	}
	defer rows.Close()
	var out = make([]Step, 0)
	for rows.Next() {
		s, err := scanStep(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
