package attack

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"

	"github.com/bryanster/blacklight/internal/content"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

func applyCatalog(ctx context.Context, w content.Writer, cat *Catalog, prog content.Progress) error {
	total := int64(len(cat.Tactics) + len(cat.Techniques) + len(cat.Mitigations) +
		len(cat.Groups) + len(cat.Software) + len(cat.DataSources))
	if prog != nil {
		prog.Report(ctx, content.PhaseApply, 0, total, "staging ATT&CK catalog")
	}

	sourceID := w.SourceID()
	target := cat.Version
	stage := storecontent.StagingVersion
	batch := w.BatchSize()
	if batch <= 0 {
		batch = 250
	}

	// Drop any leftover staging rows from a prior interrupted apply.
	if err := w.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return clearVersion(ctx, tx, sourceID, stage)
	}); err != nil {
		return err
	}

	var done int64
	report := func(msg string) {
		if prog != nil {
			prog.Report(ctx, content.PhaseApply, done, total, msg)
		}
	}

	type inserter func(ctx context.Context, tx *sql.Tx, sourceID, version string) error

	flush := func(items []inserter, label string) error {
		for i := 0; i < len(items); i += batch {
			if err := ctx.Err(); err != nil {
				return err
			}
			end := i + batch
			if end > len(items) {
				end = len(items)
			}
			chunk := items[i:end]
			if err := w.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
				for _, fn := range chunk {
					if err := fn(ctx, tx, sourceID, stage); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				return fmt.Errorf("attack: apply %s batch: %w", label, err)
			}
			done += int64(len(chunk))
			report(fmt.Sprintf("staged %s %d/%d", label, end, len(items)))
		}
		return nil
	}

	tacticIns := make([]inserter, 0, len(cat.Tactics))
	for i := range cat.Tactics {
		t := cat.Tactics[i]
		tacticIns = append(tacticIns, func(ctx context.Context, tx *sql.Tx, sourceID, version string) error {
			return insertTactic(ctx, tx, sourceID, version, t)
		})
	}
	if err := flush(tacticIns, "tactics"); err != nil {
		return err
	}

	techIns := make([]inserter, 0, len(cat.Techniques))
	for i := range cat.Techniques {
		t := cat.Techniques[i]
		techIns = append(techIns, func(ctx context.Context, tx *sql.Tx, sourceID, version string) error {
			return insertTechnique(ctx, tx, sourceID, version, t)
		})
	}
	if err := flush(techIns, "techniques"); err != nil {
		return err
	}

	linkIns := make([]inserter, 0, len(cat.TechTactics)+len(cat.TechMitigations))
	for techExt, tactics := range cat.TechTactics {
		tactics := append([]string(nil), tactics...)
		linkIns = append(linkIns, func(ctx context.Context, tx *sql.Tx, sourceID, version string) error {
			for _, tac := range tactics {
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO content.content_technique_tactic
						(source_id, version, technique_external_id, tactic_external_id)
					VALUES (?, ?, ?, ?)`,
					sourceID, version, techExt, tac,
				); err != nil {
					return fmt.Errorf("technique_tactic %s→%s: %w", techExt, tac, err)
				}
			}
			return nil
		})
	}
	for techExt, mits := range cat.TechMitigations {
		mits := append([]string(nil), mits...)
		linkIns = append(linkIns, func(ctx context.Context, tx *sql.Tx, sourceID, version string) error {
			for _, m := range mits {
				if _, err := tx.ExecContext(ctx, `
					INSERT INTO content.content_technique_mitigation
						(source_id, version, technique_external_id, mitigation_external_id)
					VALUES (?, ?, ?, ?)`,
					sourceID, version, techExt, m,
				); err != nil {
					return fmt.Errorf("technique_mitigation %s→%s: %w", techExt, m, err)
				}
			}
			return nil
		})
	}
	if err := flush(linkIns, "links"); err != nil {
		return err
	}
	if done > total {
		done = total
	}

	mitIns := make([]inserter, 0, len(cat.Mitigations))
	for i := range cat.Mitigations {
		m := cat.Mitigations[i]
		mitIns = append(mitIns, func(ctx context.Context, tx *sql.Tx, sourceID, version string) error {
			return insertMitigation(ctx, tx, sourceID, version, m)
		})
	}
	if err := flush(mitIns, "mitigations"); err != nil {
		return err
	}

	groupIns := make([]inserter, 0, len(cat.Groups))
	for i := range cat.Groups {
		g := cat.Groups[i]
		groupIns = append(groupIns, func(ctx context.Context, tx *sql.Tx, sourceID, version string) error {
			return insertGroup(ctx, tx, sourceID, version, g)
		})
	}
	if err := flush(groupIns, "groups"); err != nil {
		return err
	}

	softIns := make([]inserter, 0, len(cat.Software))
	for i := range cat.Software {
		s := cat.Software[i]
		softIns = append(softIns, func(ctx context.Context, tx *sql.Tx, sourceID, version string) error {
			return insertSoftware(ctx, tx, sourceID, version, s)
		})
	}
	if err := flush(softIns, "software"); err != nil {
		return err
	}

	dsIns := make([]inserter, 0, len(cat.DataSources))
	for i := range cat.DataSources {
		d := cat.DataSources[i]
		dsIns = append(dsIns, func(ctx context.Context, tx *sql.Tx, sourceID, version string) error {
			return insertDataSource(ctx, tx, sourceID, version, d)
		})
	}
	if err := flush(dsIns, "data_sources"); err != nil {
		return err
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	if prog != nil {
		prog.Report(ctx, content.PhaseApply, total, total, "promoting staged catalog")
	}
	if err := w.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		return storecontent.PromoteAttackVersion(ctx, tx, sourceID, stage, target)
	}); err != nil {
		return fmt.Errorf("attack: apply promote %s: %w", target, err)
	}
	if prog != nil {
		prog.Report(ctx, content.PhaseApply, total, total, fmt.Sprintf("applied ATT&CK %s", target))
	}
	return nil
}

func clearVersion(ctx context.Context, tx *sql.Tx, sourceID, version string) error {
	stmts, _ := storecontent.AttackVersionDeletes(sourceID, version)
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt, sourceID, version); err != nil {
			return err
		}
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

func insertTactic(ctx context.Context, tx *sql.Tx, sourceID, version string, t Tactic) error {
	id, err := newID()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content.content_tactic
			(id, source_id, version, external_id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		id, sourceID, version, t.ExternalID, t.Name, t.Description,
	); err != nil {
		return fmt.Errorf("tactic %s: %w", t.ExternalID, err)
	}
	return nil
}

func insertMitigation(ctx context.Context, tx *sql.Tx, sourceID, version string, m Mitigation) error {
	id, err := newID()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content.content_mitigation
			(id, source_id, version, external_id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		id, sourceID, version, m.ExternalID, m.Name, m.Description,
	); err != nil {
		return fmt.Errorf("mitigation %s: %w", m.ExternalID, err)
	}
	return nil
}

func insertGroup(ctx context.Context, tx *sql.Tx, sourceID, version string, g Group) error {
	id, err := newID()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content.content_group
			(id, source_id, version, external_id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		id, sourceID, version, g.ExternalID, g.Name, g.Description,
	); err != nil {
		return fmt.Errorf("group %s: %w", g.ExternalID, err)
	}
	return nil
}

func insertDataSource(ctx context.Context, tx *sql.Tx, sourceID, version string, d DataSource) error {
	id, err := newID()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content.content_data_source
			(id, source_id, version, external_id, name, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		id, sourceID, version, d.ExternalID, d.Name, d.Description,
	); err != nil {
		return fmt.Errorf("data_source %s: %w", d.ExternalID, err)
	}
	return nil
}

func insertTechnique(ctx context.Context, tx *sql.Tx, sourceID, version string, t Technique) error {
	id, err := newID()
	if err != nil {
		return err
	}
	parent := t.ParentExternalID
	if !t.IsSubtechnique {
		parent = ""
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content.content_technique
			(id, source_id, version, external_id, name, description,
			 is_subtechnique, parent_external_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		id, sourceID, version, t.ExternalID, t.Name, t.Description,
		t.IsSubtechnique, parent,
	); err != nil {
		return fmt.Errorf("technique %s: %w", t.ExternalID, err)
	}
	return nil
}

func insertSoftware(ctx context.Context, tx *sql.Tx, sourceID, version string, s Software) error {
	id, err := newID()
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO content.content_software
			(id, source_id, version, external_id, name, description, software_type,
			 created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		id, sourceID, version, s.ExternalID, s.Name, s.Description, string(s.SoftwareType),
	); err != nil {
		return fmt.Errorf("software %s: %w", s.ExternalID, err)
	}
	return nil
}
