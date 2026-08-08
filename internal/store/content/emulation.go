package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// EmulationPlan is one CTID catalog plan.
//
// AdversaryName is the upstream threat-actor / group label (for example
// "FIN6"). Metadata holds source-side bookkeeping (attack_version,
// format_version, archive path) as a JSON object. M3-013 snapshots these
// fields onto a Scenario — they are not live FKs into app.
type EmulationPlan struct {
	ID            string
	SourceID      string
	Version       string
	ExternalID    string
	Name          string
	Description   string
	AdversaryName string
	Metadata      json.RawMessage // JSON object
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// EmulationPlanStep is one ordered step under a plan.
//
// Position is 1-based document order within the upstream plan YAML (dense,
// stable across re-sync for an unchanged file). TechniqueExternalID is a
// single ATT&CK id string (empty when upstream omits it). Procedure is a JSON
// object carrying platforms, executors/commands, input args, and other
// procedure-ish fields when present.
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
	Procedure           json.RawMessage // JSON object
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// EmulationPlanDetail is a plan with its steps ordered by position ascending.
type EmulationPlanDetail struct {
	EmulationPlan
	Steps []EmulationPlanStep
}

// EmulationPlans reads and writes CTID catalog rows. Construct with
// [NewEmulationPlans].
type EmulationPlans struct {
	db DB
}

// NewEmulationPlans returns a repository over db.
func NewEmulationPlans(db DB) *EmulationPlans { return &EmulationPlans{db: db} }

const planColumns = `id, source_id, version, external_id, name, description,
	adversary_name, metadata, created_at, updated_at`

const stepColumns = `id, source_id, version, plan_id, position, external_id,
	name, description, technique_external_id, procedure, created_at, updated_at`

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
				(id, source_id, version, external_id, name, description,
				 adversary_name, metadata, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.SourceID, in.Version, in.ExternalID, in.Name, in.Description,
			in.AdversaryName, bindJSONObject(in.Metadata), ts, ts,
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
		SELECT `+planColumns+`
		FROM content.content_emulation_plan WHERE id = ?`, id)
	p, err := scanPlan(row)
	if err != nil {
		return EmulationPlan{}, wrapObjErr(err, "content_emulation_plan", id)
	}
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
				name, description, technique_external_id, procedure,
				created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.SourceID, in.Version, in.PlanID, in.Position, in.ExternalID,
			in.Name, in.Description, in.TechniqueExternalID, bindJSONObject(in.Procedure), ts, ts,
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
		SELECT `+stepColumns+`
		FROM content.content_emulation_plan_step WHERE id = ?`, id)
	s, err := scanStep(row)
	if err != nil {
		return EmulationPlanStep{}, wrapObjErr(err, "content_emulation_plan_step", id)
	}
	return s, nil
}

// StepsByPlan returns steps for a plan in position order (ordinal ascending).
func (r *EmulationPlans) StepsByPlan(ctx context.Context, planID string) ([]EmulationPlanStep, error) {
	rows, err := r.db.Read().QueryContext(ctx, `
		SELECT `+stepColumns+`
		FROM content.content_emulation_plan_step
		WHERE plan_id = ?
		ORDER BY position, id`, planID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EmulationPlanStep
	for rows.Next() {
		s, err := scanStep(rows)
		if err != nil {
			return nil, err
		}
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

// EmulationPlanListFilter narrows plan listings.
//
// EnabledOnly joins content_source.enabled (library browse). Version empty
// means every non-staging version. Q is a case-insensitive substring over
// external_id, name, description, and adversary_name. Technique matches plans
// that have at least one step with that technique_external_id (exact,
// case-insensitive).
type EmulationPlanListFilter struct {
	SourceID    string
	Version     string
	Q           string
	Technique   string
	EnabledOnly bool
	Limit       int
}

// List returns plans matching f, ordered by external_id then id.
func (r *EmulationPlans) List(ctx context.Context, f EmulationPlanListFilter) ([]EmulationPlan, error) {
	var (
		b    strings.Builder
		args []any
	)
	b.WriteString(`
		SELECT p.id, p.source_id, p.version, p.external_id, p.name, p.description,
			p.adversary_name, p.metadata, p.created_at, p.updated_at
		FROM content.content_emulation_plan p`)
	if f.EnabledOnly {
		b.WriteString(`
		INNER JOIN content.content_source s ON s.id = p.source_id AND s.enabled = TRUE`)
	}
	b.WriteString(` WHERE p.version <> ?`)
	args = append(args, StagingVersion)
	if f.SourceID != "" {
		b.WriteString(` AND p.source_id = ?`)
		args = append(args, f.SourceID)
	}
	if f.Version != "" {
		b.WriteString(` AND p.version = ?`)
		args = append(args, f.Version)
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		b.WriteString(` AND (
			LOWER(p.external_id) LIKE ? OR
			LOWER(p.name) LIKE ? OR
			LOWER(p.description) LIKE ? OR
			LOWER(p.adversary_name) LIKE ?
		)`)
		args = append(args, like, like, like, like)
	}
	if tech := strings.TrimSpace(f.Technique); tech != "" {
		b.WriteString(` AND EXISTS (
			SELECT 1 FROM content.content_emulation_plan_step st
			WHERE st.plan_id = p.id
			  AND LOWER(st.technique_external_id) = ?
		)`)
		args = append(args, strings.ToLower(tech))
	}
	b.WriteString(` ORDER BY p.external_id, p.id`)
	if lim := listLimit(f.Limit); lim > 0 {
		b.WriteString(` LIMIT ?`)
		args = append(args, lim)
	}
	rows, err := r.db.Read().QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("content: list emulation_plan: %w", err)
	}
	defer rows.Close()
	var out []EmulationPlan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []EmulationPlan{}
	}
	return out, nil
}

// ByIDEnabled returns one plan, or [apierr.NotFound] when the row is missing
// or (enabledOnly) its source is disabled. Staging rows are hidden.
func (r *EmulationPlans) ByIDEnabled(ctx context.Context, id string, enabledOnly bool) (EmulationPlan, error) {
	p, err := r.ByID(ctx, id)
	if err != nil {
		return EmulationPlan{}, err
	}
	if p.Version == StagingVersion {
		return EmulationPlan{}, wrapObjErr(sql.ErrNoRows, "content_emulation_plan", id)
	}
	if enabledOnly {
		if err := requireEnabledSource(ctx, r.db, p.SourceID); err != nil {
			return EmulationPlan{}, err
		}
	}
	return p, nil
}

// DetailByIDEnabled returns a plan with steps ordered by position, applying the
// same enabled/staging visibility as [EmulationPlans.ByIDEnabled].
func (r *EmulationPlans) DetailByIDEnabled(ctx context.Context, id string, enabledOnly bool) (EmulationPlanDetail, error) {
	p, err := r.ByIDEnabled(ctx, id, enabledOnly)
	if err != nil {
		return EmulationPlanDetail{}, err
	}
	steps, err := r.StepsByPlan(ctx, p.ID)
	if err != nil {
		return EmulationPlanDetail{}, err
	}
	return EmulationPlanDetail{EmulationPlan: p, Steps: steps}, nil
}

// ClearEmulationVersion deletes every plan and step for (sourceID, version).
// Steps first so a partial failure never leaves orphan step rows pointing at a
// deleted plan id. Exported so the CTID adapter Apply path can share it inside
// a Writer transaction without opening a nested store.Write.
func ClearEmulationVersion(ctx context.Context, tx *sql.Tx, sourceID, version string) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM content.content_emulation_plan_step
		WHERE source_id = ? AND version = ?`,
		sourceID, version,
	); err != nil {
		return fmt.Errorf("content: clear emulation_plan_step %s/%s: %w", sourceID, version, err)
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM content.content_emulation_plan
		WHERE source_id = ? AND version = ?`,
		sourceID, version,
	); err != nil {
		return fmt.Errorf("content: clear emulation_plan %s/%s: %w", sourceID, version, err)
	}
	return nil
}

// PromoteEmulationVersion moves every plan and step row from fromVersion to
// toVersion inside tx, after deleting any existing toVersion rows. Both halves
// share one transaction so a failed re-sync never leaves a half-replaced
// rolling catalog. Steps are updated with their plans so plan_id stays valid
// (surrogate ids do not change on promote).
func PromoteEmulationVersion(ctx context.Context, tx *sql.Tx, sourceID, fromVersion, toVersion string) error {
	if err := ClearEmulationVersion(ctx, tx, sourceID, toVersion); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE content.content_emulation_plan
		SET version = ?
		WHERE source_id = ? AND version = ?`,
		toVersion, sourceID, fromVersion,
	); err != nil {
		return fmt.Errorf("content: promote emulation_plan: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE content.content_emulation_plan_step
		SET version = ?
		WHERE source_id = ? AND version = ?`,
		toVersion, sourceID, fromVersion,
	); err != nil {
		return fmt.Errorf("content: promote emulation_plan_step: %w", err)
	}
	return nil
}

func scanPlan(row interface{ Scan(...any) error }) (EmulationPlan, error) {
	var (
		p             EmulationPlan
		meta          any
		adversaryName sql.NullString
	)
	err := row.Scan(
		&p.ID, &p.SourceID, &p.Version, &p.ExternalID, &p.Name, &p.Description,
		&adversaryName, &meta, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return EmulationPlan{}, err
	}
	if adversaryName.Valid {
		p.AdversaryName = adversaryName.String
	}
	if p.Metadata, err = jsonBytes(meta); err != nil {
		return EmulationPlan{}, fmt.Errorf("content: emulation_plan metadata: %w", err)
	}
	p.CreatedAt = p.CreatedAt.UTC()
	p.UpdatedAt = p.UpdatedAt.UTC()
	return p, nil
}

func scanStep(row interface{ Scan(...any) error }) (EmulationPlanStep, error) {
	var (
		s    EmulationPlanStep
		proc any
	)
	err := row.Scan(
		&s.ID, &s.SourceID, &s.Version, &s.PlanID, &s.Position, &s.ExternalID,
		&s.Name, &s.Description, &s.TechniqueExternalID, &proc,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return EmulationPlanStep{}, err
	}
	if s.Procedure, err = jsonBytes(proc); err != nil {
		return EmulationPlanStep{}, fmt.Errorf("content: emulation_plan_step procedure: %w", err)
	}
	s.CreatedAt = s.CreatedAt.UTC()
	s.UpdatedAt = s.UpdatedAt.UTC()
	return s, nil
}
