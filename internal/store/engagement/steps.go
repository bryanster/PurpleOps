package engagement

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

const stepColumns = `id, scenario_id, ordinal, name, objective, technique_id, subtechnique_id, tactic_id, "procedure", template_id, target_asset, tools, controls_in_scope, attack_version, revealed_at, created_at, updated_at`

const selectStep = `SELECT ` + stepColumns + ` FROM app.step `

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
		s          Step
		revealedAt sql.NullTime
		procedure  any
		tools      any
		controls   any
	)
	if err := row.Scan(
		&s.ID, &s.ScenarioID, &s.Ordinal, &s.Name, &s.Objective,
		&s.TechniqueID, &s.SubtechniqueID, &s.TacticID,
		&procedure, &s.TemplateID, &s.TargetAsset,
		&tools, &controls,
		&s.AttackVersion, &revealedAt,
		&s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return Step{}, err
	}
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
