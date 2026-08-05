package ctid

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
	total := int64(len(cat.Plans))
	if prog != nil {
		prog.Report(ctx, content.PhaseApply, 0, total, "staging CTID emulation plans")
	}

	sourceID := w.SourceID()
	target := w.Version()
	if target == "" {
		target = storecontent.VersionCurrent
	}
	stage := storecontent.StagingVersion
	batch := w.BatchSize()
	if batch <= 0 {
		batch = 50
	}

	// Drop any leftover staging rows from a prior interrupted apply.
	if err := w.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return storecontent.ClearEmulationVersion(ctx, tx, sourceID, stage)
	}); err != nil {
		return err
	}

	var done int64
	for i := 0; i < len(cat.Plans); i += batch {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := i + batch
		if end > len(cat.Plans) {
			end = len(cat.Plans)
		}
		chunk := cat.Plans[i:end]
		if err := w.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
			for _, p := range chunk {
				if err := insertPlan(ctx, tx, sourceID, stage, p); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		done += int64(len(chunk))
		if prog != nil {
			prog.Report(ctx, content.PhaseApply, done, total,
				fmt.Sprintf("staged %d/%d plans", done, total))
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if prog != nil {
		prog.Report(ctx, content.PhaseApply, total, total, "promoting staged catalog")
	}
	if err := w.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return storecontent.PromoteEmulationVersion(ctx, tx, sourceID, stage, target)
	}); err != nil {
		return fmt.Errorf("ctid: apply promote %s: %w", target, err)
	}
	if prog != nil {
		prog.Report(ctx, content.PhaseApply, total, total, cat.SuccessMessage())
	}
	return nil
}

func insertPlan(ctx context.Context, tx *sql.Tx, sourceID, version string, p Plan) error {
	planID, err := newID()
	if err != nil {
		return err
	}
	meta := p.Metadata
	if len(meta) == 0 {
		meta = json.RawMessage(`{}`)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content.content_emulation_plan (
			id, source_id, version, external_id, name, description,
			adversary_name, metadata, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		planID, sourceID, version, p.ExternalID, p.Name, p.Description,
		p.AdversaryName, []byte(meta),
	); err != nil {
		return fmt.Errorf("emulation_plan %s: %w", p.ExternalID, err)
	}

	for _, s := range p.Steps {
		if err := insertStep(ctx, tx, sourceID, version, planID, s); err != nil {
			return err
		}
	}
	return nil
}

func insertStep(ctx context.Context, tx *sql.Tx, sourceID, version, planID string, s Step) error {
	id, err := newID()
	if err != nil {
		return err
	}
	proc := s.Procedure
	if len(proc) == 0 {
		proc = json.RawMessage(`{}`)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content.content_emulation_plan_step (
			id, source_id, version, plan_id, position, external_id,
			name, description, technique_external_id, procedure,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		id, sourceID, version, planID, s.Position, s.ExternalID,
		s.Name, s.Description, s.TechniqueExternalID, []byte(proc),
	); err != nil {
		return fmt.Errorf("emulation_plan_step %s: %w", s.ExternalID, err)
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
