package engagement

import (
	"context"
	"database/sql"
	"fmt"
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

func scanScenario(row interface{ Scan(...any) error }) (Scenario, error) {
	var s Scenario
	if err := row.Scan(
		&s.ID, &s.EngagementID, &s.Ordinal, &s.Name, &s.Narrative,
		&s.Source, &s.ThreatActor, &s.SourceRef, &s.PlanID,
		&s.CreatedAt, &s.UpdatedAt,
	); err != nil {
		return Scenario{}, err
	}
	s.CreatedAt = s.CreatedAt.UTC()
	s.UpdatedAt = s.UpdatedAt.UTC()
	return s, nil
}
