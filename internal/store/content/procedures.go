package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
//
// after runs inside the same write transaction after the insert so activity
// (M2-011) shares the commit.
func (r *Procedures) Create(ctx context.Context, in ProcedureTemplate, after ...After) (ProcedureTemplate, error) {
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
		if err := uniqueOr(err, "procedure_template", in.SourceID, in.Version, in.ExternalID); err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, id), tx, after)
	})
	if err != nil {
		return ProcedureTemplate{}, err
	}
	return r.ByID(ctx, id)
}

// Update rewrites mutable fields of an existing procedure template.
//
// after runs inside the same write transaction after the update.
func (r *Procedures) Update(ctx context.Context, in ProcedureTemplate, after ...After) (ProcedureTemplate, error) {
	ts := now()
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE content.content_procedure_template SET
				name = ?, description = ?,
				platforms = ?, executor = ?, elevation_required = ?,
				command = ?, cleanup = ?, input_args = ?,
				technique_external_ids = ?, dependency_executor_name = ?, dependencies = ?,
				updated_at = ?
			WHERE id = ?`,
			in.Name, in.Description,
			bindJSON(in.Platforms), in.Executor, in.ElevationRequired,
			in.Command, in.Cleanup, bindJSONObject(in.InputArgs),
			bindJSON(in.TechniqueExternalIDs), in.DependencyExecutorName, in.Dependencies,
			ts, in.ID,
		)
		if err != nil {
			return fmt.Errorf("content: update procedure_template %s: %w", in.ID, err)
		}
		if err := requireOneRow(res, "content_procedure_template", in.ID); err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, in.ID), tx, after)
	})
	if err != nil {
		return ProcedureTemplate{}, err
	}
	return r.ByID(ctx, in.ID)
}

// Delete removes one procedure template by id.
//
// after runs inside the same write transaction after the delete.
func (r *Procedures) Delete(ctx context.Context, id string, after ...After) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`DELETE FROM content.content_procedure_template WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("content: delete procedure_template %s: %w", id, err)
		}
		if err := requireOneRow(res, "content_procedure_template", id); err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, id), tx, after)
	})
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

// ProcedureListFilter narrows procedure template listings.
//
// EnabledOnly joins content_source.enabled (library browse). Version empty
// means every non-staging version. Q is a case-insensitive substring over
// external_id, name, and description. Technique and Platform are exact
// membership matches against the JSON array columns (quoted-token scan).
type ProcedureListFilter struct {
	SourceID    string
	Version     string
	Q           string
	Technique   string
	Platform    string
	EnabledOnly bool
	Limit       int
}

// List returns procedure templates matching f, ordered by external_id then id.
func (r *Procedures) List(ctx context.Context, f ProcedureListFilter) ([]ProcedureTemplate, error) {
	var (
		b    strings.Builder
		args []any
	)
	b.WriteString(`
		SELECT p.id, p.source_id, p.version, p.external_id, p.name, p.description,
			p.platforms, p.executor, p.elevation_required, p.command, p.cleanup, p.input_args,
			p.technique_external_ids, p.dependency_executor_name, p.dependencies,
			p.created_at, p.updated_at
		FROM content.content_procedure_template p`)
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
			LOWER(p.description) LIKE ?
		)`)
		args = append(args, like, like, like)
	}
	if tech := strings.TrimSpace(f.Technique); tech != "" {
		// Quoted-token membership on the JSON array text form.
		// Exact labels only — no substring technique ids.
		b.WriteString(` AND LOWER(CAST(p.technique_external_ids AS VARCHAR)) LIKE ?`)
		args = append(args, "%\""+strings.ToLower(tech)+"\"%")
	}
	if plat := strings.TrimSpace(f.Platform); plat != "" {
		b.WriteString(` AND LOWER(CAST(p.platforms AS VARCHAR)) LIKE ?`)
		args = append(args, "%\""+strings.ToLower(plat)+"\"%")
	}
	b.WriteString(` ORDER BY p.external_id, p.id`)
	if lim := listLimit(f.Limit); lim > 0 {
		b.WriteString(` LIMIT ?`)
		args = append(args, lim)
	}
	rows, err := r.db.Read().QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("content: list procedure_template: %w", err)
	}
	defer rows.Close()
	var out []ProcedureTemplate
	for rows.Next() {
		p, err := scanProcedure(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []ProcedureTemplate{}
	}
	return out, nil
}

// ByIDEnabled returns one procedure template, or [apierr.NotFound] when the
// row is missing or (enabledOnly) its source is disabled.
func (r *Procedures) ByIDEnabled(ctx context.Context, id string, enabledOnly bool) (ProcedureTemplate, error) {
	p, err := r.ByID(ctx, id)
	if err != nil {
		return ProcedureTemplate{}, err
	}
	if enabledOnly {
		if err := requireEnabledSource(ctx, r.db, p.SourceID); err != nil {
			return ProcedureTemplate{}, err
		}
	}
	return p, nil
}

// ClearProcedureVersion deletes every procedure template for (sourceID, version).
// Exported so the Atomic adapter Apply path can share it inside a Writer
// transaction without opening a nested store.Write.
func ClearProcedureVersion(ctx context.Context, tx *sql.Tx, sourceID, version string) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM content.content_procedure_template
		WHERE source_id = ? AND version = ?`,
		sourceID, version,
	); err != nil {
		return fmt.Errorf("content: clear procedure_template %s/%s: %w", sourceID, version, err)
	}
	return nil
}

// PromoteProcedureVersion moves every procedure template row from fromVersion
// to toVersion inside tx, after deleting any existing toVersion rows. Both
// halves share one transaction so a failed re-sync never leaves a half-replaced
// rolling catalog.
func PromoteProcedureVersion(ctx context.Context, tx *sql.Tx, sourceID, fromVersion, toVersion string) error {
	if err := ClearProcedureVersion(ctx, tx, sourceID, toVersion); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE content.content_procedure_template
		SET version = ?
		WHERE source_id = ? AND version = ?`,
		toVersion, sourceID, fromVersion,
	); err != nil {
		return fmt.Errorf("content: promote procedure_template: %w", err)
	}
	return nil
}

// requireEnabledSource reports NotFound when the source is missing or disabled.
// Same contract as Objects.requireEnabledSource — disabled catalogs are hidden.
func requireEnabledSource(ctx context.Context, db DB, sourceID string) error {
	var enabled bool
	err := db.Read().QueryRowContext(ctx, `
		SELECT enabled FROM content.content_source WHERE id = ?`, sourceID,
	).Scan(&enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapObjErr(sql.ErrNoRows, "content_source", sourceID)
		}
		return fmt.Errorf("content: source enabled check: %w", err)
	}
	if !enabled {
		return wrapObjErr(sql.ErrNoRows, "content_object", sourceID)
	}
	return nil
}
