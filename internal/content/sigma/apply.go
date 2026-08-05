package sigma

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
	total := int64(len(cat.Rules))
	if prog != nil {
		msg := "staging Sigma detection rules"
		if cat.Skipped > 0 {
			msg = fmt.Sprintf("staging Sigma detection rules (%d unmapped skipped)", cat.Skipped)
		}
		prog.Report(ctx, content.PhaseApply, 0, total, msg)
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
		return storecontent.ClearDetectionVersion(ctx, tx, sourceID, stage)
	}); err != nil {
		return err
	}

	var done int64
	for i := 0; i < len(cat.Rules); i += batch {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := i + batch
		if end > len(cat.Rules) {
			end = len(cat.Rules)
		}
		chunk := cat.Rules[i:end]
		if err := w.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
			for _, rule := range chunk {
				if err := insertRule(ctx, tx, sourceID, stage, rule); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("sigma: apply batch: %w", err)
		}
		done = int64(end)
		if prog != nil {
			prog.Report(ctx, content.PhaseApply, done, total, fmt.Sprintf("staged %d/%d", end, len(cat.Rules)))
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if prog != nil {
		prog.Report(ctx, content.PhaseApply, total, total, "promoting staged catalog")
	}
	if err := w.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return storecontent.PromoteDetectionVersion(ctx, tx, sourceID, stage, target)
	}); err != nil {
		return fmt.Errorf("sigma: apply promote %s: %w", target, err)
	}
	if prog != nil {
		prog.Report(ctx, content.PhaseApply, total, total, cat.SuccessMessage())
	}
	return nil
}

func insertRule(ctx context.Context, tx *sql.Tx, sourceID, version string, r Rule) error {
	id, err := newID()
	if err != nil {
		return err
	}
	techs := r.TechniqueExternalIDs
	if techs == nil {
		techs = []string{}
	}
	techJSON, err := json.Marshal(techs)
	if err != nil {
		return fmt.Errorf("techniques %s: %w", r.ExternalID, err)
	}
	logsource := r.Logsource
	if len(logsource) == 0 {
		logsource = json.RawMessage(`{}`)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content.content_detection_rule_ref (
			id, source_id, version, external_id, name, description,
			technique_external_ids, level, rule_status, logsource, rule_yaml,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		id, sourceID, version, r.ExternalID, r.Name, r.Description,
		techJSON, r.Level, r.Status, []byte(logsource), r.RuleYAML,
	); err != nil {
		return fmt.Errorf("detection_rule_ref %s: %w", r.ExternalID, err)
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
