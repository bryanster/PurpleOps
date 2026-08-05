package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ProcedureTemplate is one Atomic or custom structured procedure.
//
// Platforms, input args and technique ids stay structured JSON — they are not
// flattened to a single actions string (PLAN.md §3). M3 engagement steps will
// snapshot these fields; template_id on an engagement step is weak lineage
// only, with no FK from app to content.
type ProcedureTemplate struct {
	ID                     string
	SourceID               string
	Version                string
	ExternalID             string
	Name                   string
	Description            string
	Platforms              json.RawMessage // JSON array of strings
	Executor               string
	ElevationRequired      bool
	Command                string
	Cleanup                string
	InputArgs              json.RawMessage // JSON object/array
	TechniqueExternalIDs   json.RawMessage // JSON array of strings
	DependencyExecutorName string
	Dependencies           string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// Procedures reads and writes procedure templates. Construct with [NewProcedures].
type Procedures struct {
	db DB
}

// NewProcedures returns a repository over db.
func NewProcedures(db DB) *Procedures { return &Procedures{db: db} }

const procedureColumns = `id, source_id, version, external_id, name, description,
	platforms, executor, elevation_required, command, cleanup, input_args,
	technique_external_ids, dependency_executor_name, dependencies,
	created_at, updated_at`

// Create inserts one procedure template.
func (r *Procedures) Create(ctx context.Context, in ProcedureTemplate) (ProcedureTemplate, error) {
	id, err := assignID(in.ID)
	if err != nil {
		return ProcedureTemplate{}, err
	}
	ts := now()
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO content.content_procedure_template (
				id, source_id, version, external_id, name, description,
				platforms, executor, elevation_required, command, cleanup, input_args,
				technique_external_ids, dependency_executor_name, dependencies,
				created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.SourceID, in.Version, in.ExternalID, in.Name, in.Description,
			bindJSON(in.Platforms), in.Executor, in.ElevationRequired, in.Command, in.Cleanup,
			bindJSONObject(in.InputArgs), bindJSON(in.TechniqueExternalIDs),
			in.DependencyExecutorName, in.Dependencies,
			ts, ts,
		)
		return uniqueOr(err, "procedure_template", in.SourceID, in.Version, in.ExternalID)
	})
	if err != nil {
		return ProcedureTemplate{}, err
	}
	return r.ByID(ctx, id)
}

// ByID returns one procedure template or [apierr.NotFound].
func (r *Procedures) ByID(ctx context.Context, id string) (ProcedureTemplate, error) {
	row := r.db.Read().QueryRowContext(ctx,
		`SELECT `+procedureColumns+` FROM content.content_procedure_template WHERE id = ?`, id)
	p, err := scanProcedure(row)
	if err != nil {
		return ProcedureTemplate{}, wrapObjErr(err, "content_procedure_template", id)
	}
	return p, nil
}

func scanProcedure(row interface{ Scan(...any) error }) (ProcedureTemplate, error) {
	var (
		p          ProcedureTemplate
		platforms  any
		inputArgs  any
		techniques any
	)
	err := row.Scan(
		&p.ID, &p.SourceID, &p.Version, &p.ExternalID, &p.Name, &p.Description,
		&platforms, &p.Executor, &p.ElevationRequired, &p.Command, &p.Cleanup, &inputArgs,
		&techniques, &p.DependencyExecutorName, &p.Dependencies,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return ProcedureTemplate{}, err
	}
	if p.Platforms, err = jsonBytes(platforms); err != nil {
		return ProcedureTemplate{}, fmt.Errorf("content: procedure platforms: %w", err)
	}
	if p.InputArgs, err = jsonBytes(inputArgs); err != nil {
		return ProcedureTemplate{}, fmt.Errorf("content: procedure input_args: %w", err)
	}
	if p.TechniqueExternalIDs, err = jsonBytes(techniques); err != nil {
		return ProcedureTemplate{}, fmt.Errorf("content: procedure techniques: %w", err)
	}
	p.CreatedAt = p.CreatedAt.UTC()
	p.UpdatedAt = p.UpdatedAt.UTC()
	return p, nil
}
