package engagement

import (
	"context"
	"errors"

	"database/sql"
	"fmt"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

const scenarioColumns = `id, engagement_id, ordinal, name, narrative, source, threat_actor, source_ref, plan_id, created_at, updated_at`

const selectScenario = `SELECT ` + scenarioColumns + ` FROM app.scenario `

const insertScenario = `INSERT INTO app.scenario
	(id, engagement_id, ordinal, name, narrative, source, threat_actor, source_ref, plan_id, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

// Scenarios reads and writes attack-chain sections. Construct it with [NewScenarios].
type Scenarios struct {
	db DB
}

// NewScenarios returns a repository over db.
func NewScenarios(db DB) *Scenarios { return &Scenarios{db: db} }

// Create writes a new scenario and returns it as stored.
func (r *Scenarios) Create(ctx context.Context, in NewScenario, after ...After) (Scenario, error) {
	var result Scenario
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		id, err := newID()
		if err != nil {
			return fmt.Errorf("scenario: generate id: %w", err)
		}
		ts := now()
		result = Scenario{
			ID:           id,
			EngagementID: in.EngagementID,
			Ordinal:      in.Ordinal,
			Name:         in.Name,
			Narrative:    in.Narrative,
			Source:       in.Source,
			ThreatActor:  in.ThreatActor,
			SourceRef:    in.SourceRef,
			PlanID:       in.PlanID,
			CreatedAt:    ts,
			UpdatedAt:    ts,
		}
		_, err = tx.ExecContext(ctx, insertScenario,
			result.ID,
			result.EngagementID,
			result.Ordinal,
			result.Name,
			result.Narrative,
			string(result.Source),
			result.ThreatActor,
			result.SourceRef,
			result.PlanID,
			result.CreatedAt,
			result.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("scenario: insert: %w", err)
		}
		ctx = WithAfterEntity(ctx, id)
		return runAfter(ctx, tx, after)
	})
	if err != nil {
		return Scenario{}, err
	}
	return result, nil
}

// ByID returns the scenario with this identifier, or the error from the scanner.
func (r *Scenarios) ByID(ctx context.Context, id string) (Scenario, error) {
	s, err := scanScenario(r.db.Read().QueryRowContext(ctx, selectScenario+`WHERE id = ?`, id))
	if err != nil {
		return Scenario{}, fmt.Errorf("scenario: read %q: %w", id, err)
	}
	return s, nil
}

// ListByEngagement returns every scenario in an engagement, ordered by ordinal.
func (r *Scenarios) ListByEngagement(ctx context.Context, engagementID string) ([]Scenario, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		selectScenario+`WHERE engagement_id = ? ORDER BY ordinal ASC`, engagementID)
	if err != nil {
		return nil, fmt.Errorf("scenario: list by engagement: %w", err)
	}
	defer rows.Close()
	var out []Scenario
	for rows.Next() {
		s, err := scanScenario(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// NextOrdinal returns one more than the current max ordinal for an engagement,
// or 1 if there are no scenarios yet.
func (r *Scenarios) NextOrdinal(ctx context.Context, engagementID string) (int, error) {
	var maxOrdinal sql.NullInt64
	if err := r.db.Read().QueryRowContext(ctx,
		`SELECT MAX(ordinal) FROM app.scenario WHERE engagement_id = ?`, engagementID,
	).Scan(&maxOrdinal); err != nil {
		return 0, fmt.Errorf("scenario: next ordinal: %w", err)
	}
	if maxOrdinal.Valid {
		return int(maxOrdinal.Int64) + 1, nil
	}
	return 1, nil
}

const updateScenario = `UPDATE app.scenario SET
	name = ?, narrative = ?, threat_actor = ?,
	updated_at = ?
	WHERE id = ?`

// ScenarioUpdateChanges describes the fields to patch on a scenario.
type ScenarioUpdateChanges struct {
	Name        string
	Narrative   string
	ThreatActor string
}

// Update patches a scenario and returns it as stored.
func (r *Scenarios) Update(ctx context.Context, id string, changes ScenarioUpdateChanges, after ...After) (Scenario, error) {
	var result Scenario
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		ts := now()
		_, err := tx.ExecContext(ctx, updateScenario,
			changes.Name, changes.Narrative, changes.ThreatActor,
			ts, id,
		)
		if err != nil {
			return fmt.Errorf("scenario: update %q: %w", id, err)
		}
		result, err = scanScenario(tx.QueryRowContext(ctx, selectScenario+`WHERE id = ?`, id))
		if err != nil {
			return fmt.Errorf("scenario: re-read after update %q: %w", id, err)
		}
		ctx = WithAfterEntity(ctx, id)
		return runAfter(ctx, tx, after)
	})
	if err != nil {
		return Scenario{}, err
	}
	return result, nil
}

const deleteScenarioGraph = `
	DELETE FROM app.comment_revision WHERE comment_id IN (SELECT id FROM app."comment" WHERE execution_id IN (SELECT e.id FROM app.execution e WHERE e.step_id IN (SELECT s.id FROM app.step s WHERE s.scenario_id = ?)));
	DELETE FROM app.evidence WHERE execution_id IN (SELECT e.id FROM app.execution e WHERE e.step_id IN (SELECT s.id FROM app.step s WHERE s.scenario_id = ?)) OR comment_id IN (SELECT id FROM app."comment" WHERE execution_id IN (SELECT e.id FROM app.execution e WHERE e.step_id IN (SELECT s.id FROM app.step s WHERE s.scenario_id = ?)));
	DELETE FROM app."comment" WHERE execution_id IN (SELECT e.id FROM app.execution e WHERE e.step_id IN (SELECT s.id FROM app.step s WHERE s.scenario_id = ?));
	DELETE FROM app.finding_step WHERE step_id IN (SELECT id FROM app.step WHERE scenario_id = ?);
	DELETE FROM app.execution WHERE step_id IN (SELECT id FROM app.step WHERE scenario_id = ?);
	DELETE FROM app.step WHERE scenario_id = ?;
	DELETE FROM app.scenario WHERE id = ?;
	DELETE FROM app.activity WHERE engagement_id IN (SELECT engagement_id FROM app.scenario WHERE id = ?) AND object_id = ?;
`

// Delete removes a scenario and its whole graph. The order respects FK
// RESTRICT constraints so child rows are dropped before their parents.
// Activity rows are cleaned up by object_id for the scenario itself; child
// activity rows (steps, executions) are left for a later ticket.
func (r *Scenarios) Delete(ctx context.Context, id string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, deleteScenarioGraph,
			// comment_revision
			id,
			// evidence (execution parent)
			id,
			// evidence (comment parent) — second position for second occurrence
			id,
			// comment
			id,
			// finding_step
			id,
			// execution
			id,
			// step
			id,
			// scenario
			id,
			// activity
			id, id,
		)
		if err != nil {
			return fmt.Errorf("scenario: delete %q: %w", id, err)
		}
		return nil
	})
}

const updateScenarioOrdinals = `UPDATE app.scenario SET ordinal = ?, updated_at = ? WHERE id = ?`

// Reorder assigns ordinals 1..N to match the order of ids, in one
// transaction. Every id must belong to the engagement; callers must
// validate before calling.
func (r *Scenarios) Reorder(ctx context.Context, ids []string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		ts := now()
		for i, id := range ids {
			ordinal := i + 1
			_, err := tx.ExecContext(ctx, updateScenarioOrdinals, ordinal, ts, id)
			if err != nil {
				return fmt.Errorf("scenario: reorder %q -> %d: %w", id, ordinal, err)
			}
		}
		return nil
	})
}

// Renumber dense-ordinals after a delete: shifts every ordinal above the
// gap down by one. Must run in the same transaction as the delete.
func (r *Scenarios) renumberAfterDelete(ctx context.Context, tx *sql.Tx, engagementID string, removedOrdinal int) error {
	ts := now()
	_, err := tx.ExecContext(ctx,
		`UPDATE app.scenario SET ordinal = ordinal - 1, updated_at = ? WHERE engagement_id = ? AND ordinal > ?`,
		ts, engagementID, removedOrdinal,
	)
	if err != nil {
		return fmt.Errorf("scenario: renumber after delete: %w", err)
	}
	return nil
}

func scanScenario(row interface{ Scan(...any) error }) (Scenario, error) {
	var s Scenario
	if err := row.Scan(
		&s.ID, &s.EngagementID, &s.Ordinal, &s.Name, &s.Narrative,
		&s.Source, &s.ThreatActor, &s.SourceRef, &s.PlanID,
		&s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Scenario{}, apierr.NotFound("scenario", "(id)")
		}
		return Scenario{}, err
	}
	s.CreatedAt = s.CreatedAt.UTC()
	s.UpdatedAt = s.UpdatedAt.UTC()
	return s, nil
}
