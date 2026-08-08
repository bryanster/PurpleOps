package atomic

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/bryanster/blacklight/internal/content"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

func applyCatalog(ctx context.Context, w content.Writer, cat *Catalog, prog content.Progress) error {
	total := int64(len(cat.Templates))
	if prog != nil {
		prog.Report(ctx, content.PhaseApply, 0, total, "staging Atomic procedures")
	}

	sourceID := w.SourceID()
	target := w.Version()
	if target == "" {
		target = storecontent.VersionCurrent
	}
	stage := storecontent.StagingVersion
	batch := w.BatchSize()
	if batch <= 0 {
		batch = 250
	}

	// Drop any leftover staging rows from a prior interrupted apply.
	if err := w.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return storecontent.ClearProcedureVersion(ctx, tx, sourceID, stage)
	}); err != nil {
		return err
	}

	var done int64
	for i := 0; i < len(cat.Templates); i += batch {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := i + batch
		if end > len(cat.Templates) {
			end = len(cat.Templates)
		}
		chunk := cat.Templates[i:end]
		if err := w.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
			for _, t := range chunk {
				if err := insertTemplate(ctx, tx, sourceID, stage, t); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("atomic: apply batch: %w", err)
		}
		done = int64(end)
		if prog != nil {
			prog.Report(ctx, content.PhaseApply, done, total, fmt.Sprintf("staged %d/%d", end, len(cat.Templates)))
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if prog != nil {
		prog.Report(ctx, content.PhaseApply, total, total, "promoting staged catalog")
	}
	if err := w.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return storecontent.PromoteProcedureVersion(ctx, tx, sourceID, stage, target)
	}); err != nil {
		return fmt.Errorf("atomic: apply promote %s: %w", target, err)
	}
	if prog != nil {
		prog.Report(ctx, content.PhaseApply, total, total, "applied Atomic current")
	}
	return nil
}

func insertTemplate(ctx context.Context, tx *sql.Tx, sourceID, version string, t Template) error {
	id, err := newID()
	if err != nil {
		return err
	}
	platforms, err := json.Marshal(t.Platforms)
	if err != nil {
		return fmt.Errorf("platforms %s: %w", t.ExternalID, err)
	}
	techs, err := json.Marshal(t.TechniqueExternalIDs)
	if err != nil {
		return fmt.Errorf("techniques %s: %w", t.ExternalID, err)
	}
	args := t.InputArgs
	if args == nil {
		args = []InputArg{}
	}
	inputArgs, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("input_args %s: %w", t.ExternalID, err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content.content_procedure_template (
			id, source_id, version, external_id, name, description,
			platforms, executor, elevation_required, command, cleanup, input_args,
			technique_external_ids, dependency_executor_name, dependencies,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		id, sourceID, version, t.ExternalID, t.Name, t.Description,
		platforms, t.Executor, t.ElevationRequired, t.Command, t.Cleanup, inputArgs,
		techs, t.DependencyExecutorName, t.Dependencies,
	); err != nil {
		return fmt.Errorf("procedure %s: %w", t.ExternalID, err)
	}
	return nil
}

func newID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return id.String(), nil
}
