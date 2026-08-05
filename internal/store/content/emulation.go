package content

import (
	"context"
	"database/sql"
	"time"
)

// EmulationPlan is one CTID catalog plan.
type EmulationPlan struct {
	ID          string
	SourceID    string
	Version     string
	ExternalID  string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// EmulationPlanStep is one ordered step under a plan.
type EmulationPlanStep struct {
	ID                  string
	SourceID            string
	Version             string
	PlanID              string
	Position            int
	ExternalID          string
	Name                string
	Description         string
	TechniqueExternalID string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// EmulationPlans reads and writes CTID catalog rows. Construct with
// [NewEmulationPlans].
type EmulationPlans struct {
	db DB
}

// NewEmulationPlans returns a repository over db.
func NewEmulationPlans(db DB) *EmulationPlans { return &EmulationPlans{db: db} }

// Create inserts one plan.
func (r *EmulationPlans) Create(ctx context.Context, in EmulationPlan) (EmulationPlan, error) {
	id, err := assignID(in.ID)
	if err != nil {
		return EmulationPlan{}, err
	}
	ts := now()
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO content.content_emulation_plan
				(id, source_id, version, external_id, name, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.SourceID, in.Version, in.ExternalID, in.Name, in.Description, ts, ts,
		)
		return uniqueOr(err, "emulation_plan", in.SourceID, in.Version, in.ExternalID)
	})
	if err != nil {
		return EmulationPlan{}, err
	}
	return r.ByID(ctx, id)
}

// ByID returns one plan or [apierr.NotFound].
func (r *EmulationPlans) ByID(ctx context.Context, id string) (EmulationPlan, error) {
	row := r.db.Read().QueryRowContext(ctx, `
		SELECT id, source_id, version, external_id, name, description, created_at, updated_at
		FROM content.content_emulation_plan WHERE id = ?`, id)
	var p EmulationPlan
	err := row.Scan(&p.ID, &p.SourceID, &p.Version, &p.ExternalID, &p.Name, &p.Description, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return EmulationPlan{}, wrapObjErr(err, "content_emulation_plan", id)
	}
	p.CreatedAt = p.CreatedAt.UTC()
	p.UpdatedAt = p.UpdatedAt.UTC()
	return p, nil
}

// CreateStep inserts one ordered step under a plan.
func (r *EmulationPlans) CreateStep(ctx context.Context, in EmulationPlanStep) (EmulationPlanStep, error) {
	id, err := assignID(in.ID)
	if err != nil {
		return EmulationPlanStep{}, err
	}
	ts := now()
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO content.content_emulation_plan_step (
				id, source_id, version, plan_id, position, external_id,
				name, description, technique_external_id, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.SourceID, in.Version, in.PlanID, in.Position, in.ExternalID,
			in.Name, in.Description, in.TechniqueExternalID, ts, ts,
		)
		return uniqueOr(err, "emulation_plan_step", in.SourceID, in.Version, in.ExternalID)
	})
	if err != nil {
		return EmulationPlanStep{}, err
	}
	return r.StepByID(ctx, id)
}

// StepByID returns one step or [apierr.NotFound].
func (r *EmulationPlans) StepByID(ctx context.Context, id string) (EmulationPlanStep, error) {
	row := r.db.Read().QueryRowContext(ctx, `
		SELECT id, source_id, version, plan_id, position, external_id,
			name, description, technique_external_id, created_at, updated_at
		FROM content.content_emulation_plan_step WHERE id = ?`, id)
	var s EmulationPlanStep
	err := row.Scan(
		&s.ID, &s.SourceID, &s.Version, &s.PlanID, &s.Position, &s.ExternalID,
		&s.Name, &s.Description, &s.TechniqueExternalID, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return EmulationPlanStep{}, wrapObjErr(err, "content_emulation_plan_step", id)
	}
	s.CreatedAt = s.CreatedAt.UTC()
	s.UpdatedAt = s.UpdatedAt.UTC()
	return s, nil
}

// StepsByPlan returns steps for a plan in position order.
func (r *EmulationPlans) StepsByPlan(ctx context.Context, planID string) ([]EmulationPlanStep, error) {
	rows, err := r.db.Read().QueryContext(ctx, `
		SELECT id, source_id, version, plan_id, position, external_id,
			name, description, technique_external_id, created_at, updated_at
		FROM content.content_emulation_plan_step
		WHERE plan_id = ?
		ORDER BY position, id`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EmulationPlanStep
	for rows.Next() {
		var s EmulationPlanStep
		if err := rows.Scan(
			&s.ID, &s.SourceID, &s.Version, &s.PlanID, &s.Position, &s.ExternalID,
			&s.Name, &s.Description, &s.TechniqueExternalID, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, err
		}
		s.CreatedAt = s.CreatedAt.UTC()
		s.UpdatedAt = s.UpdatedAt.UTC()
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []EmulationPlanStep{}
	}
	return out, nil
}
