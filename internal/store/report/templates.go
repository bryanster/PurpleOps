package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

const templateColumns = `id, engagement_id, name, created_by, created_at, updated_at`

const templateBlockColumns = `template_id, ordinal, block_id, params`

const selectTemplate = `SELECT ` + templateColumns + ` FROM app.report_template `

const selectTemplateBlock = `SELECT ` + templateBlockColumns + ` FROM app.report_template_block `

// Templates reads and writes report templates. Construct it with [NewTemplates].
type Templates struct {
	db DB
}

// NewTemplates returns a repository over db.
func NewTemplates(db DB) *Templates { return &Templates{db: db} }

// ---------------------------------------------------------------------------
// Template CRUD
// ---------------------------------------------------------------------------

// Create writes a new template and returns it as stored.
func (r *Templates) Create(ctx context.Context, in NewTemplate, after ...After) (Template, error) {
	tmpl := Template{
		ID:           newID(),
		EngagementID: in.EngagementID,
		Name:         in.Name,
		CreatedBy:    in.CreatedBy,
		CreatedAt:    now(),
		UpdatedAt:    now(),
	}

	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO app.report_template (id, engagement_id, name, created_by, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			tmpl.ID, tmpl.EngagementID, tmpl.Name, tmpl.CreatedBy, tmpl.CreatedAt, tmpl.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("templates: insert: %w", err)
		}
		for _, a := range after {
			if err := a(ctx, tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Template{}, err
	}
	return tmpl, nil
}

// ByID returns the template with this identifier, or [apierr.NotFound].
func (r *Templates) ByID(ctx context.Context, id string) (Template, error) {
	row := r.db.Read().QueryRowContext(ctx, selectTemplate+`WHERE id = $1`, id)
	var tmpl Template
	if err := row.Scan(&tmpl.ID, &tmpl.EngagementID, &tmpl.Name, &tmpl.CreatedBy, &tmpl.CreatedAt, &tmpl.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return Template{}, apierr.NotFound("template", id)
		}
		return Template{}, fmt.Errorf("templates: by id %s: %w", id, err)
	}
	return tmpl, nil
}

// ListByEngagement returns every template in an engagement, newest first.
func (r *Templates) ListByEngagement(ctx context.Context, engagementID string) ([]Template, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		selectTemplate+`WHERE engagement_id = $1 ORDER BY created_at DESC`,
		engagementID,
	)
	if err != nil {
		return nil, fmt.Errorf("templates: list by engagement %s: %w", engagementID, err)
	}
	defer rows.Close()

	var out []Template
	for rows.Next() {
		var tmpl Template
		if err := rows.Scan(&tmpl.ID, &tmpl.EngagementID, &tmpl.Name, &tmpl.CreatedBy, &tmpl.CreatedAt, &tmpl.UpdatedAt); err != nil {
			return nil, fmt.Errorf("templates: scan: %w", err)
		}
		out = append(out, tmpl)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("templates: rows: %w", err)
	}
	if out == nil {
		out = []Template{}
	}
	return out, nil
}

// Update patches a template and returns it as stored.
func (r *Templates) Update(ctx context.Context, id string, changes TemplateUpdate, after ...After) (Template, error) {
	var tmpl Template
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, selectTemplate+`WHERE id = $1`, id)
		if err := row.Scan(&tmpl.ID, &tmpl.EngagementID, &tmpl.Name, &tmpl.CreatedBy, &tmpl.CreatedAt, &tmpl.UpdatedAt); err != nil {
			if err == sql.ErrNoRows {
				return apierr.NotFound("template", id)
			}
			return fmt.Errorf("templates: update scan %s: %w", id, err)
		}

		if changes.Name != nil {
			tmpl.Name = *changes.Name
		}
		tmpl.UpdatedAt = now()

		result, err := tx.ExecContext(ctx,
			`UPDATE app.report_template SET name = $1, updated_at = $2 WHERE id = $3`,
			tmpl.Name, tmpl.UpdatedAt, id,
		)
		if err != nil {
			return fmt.Errorf("templates: update: %w", err)
		}
		if err := requireOneRow(result, "template", id); err != nil {
			return err
		}
		for _, a := range after {
			if err := a(ctx, tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Template{}, err
	}
	return tmpl, nil
}

// Delete removes a template and cascades to its blocks.
// Blocks are deleted first to respect FK RESTRICT constraints.
func (r *Templates) Delete(ctx context.Context, id string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		var found string
		if err := tx.QueryRowContext(ctx, `SELECT id FROM app.report_template WHERE id = $1`, id).Scan(&found); err != nil {
			if err == sql.ErrNoRows {
				return apierr.NotFound("template", id)
			}
			return fmt.Errorf("templates: delete check %s: %w", id, err)
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM app.report_template_block WHERE template_id = $1`, id); err != nil {
			return fmt.Errorf("templates: delete blocks for %s: %w", id, err)
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM app.report_template WHERE id = $1`, id); err != nil {
			return fmt.Errorf("templates: delete %s: %w", id, err)
		}
		return nil
	})
}

// ---------------------------------------------------------------------------
// Template blocks
// ---------------------------------------------------------------------------

// BlocksByTemplate returns every block in a template, ordered by ordinal.
func (r *Templates) BlocksByTemplate(ctx context.Context, templateID string) ([]TemplateBlock, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		selectTemplateBlock+`WHERE template_id = $1 ORDER BY ordinal`,
		templateID,
	)
	if err != nil {
		return nil, fmt.Errorf("templates: blocks by template %s: %w", templateID, err)
	}
	defer rows.Close()

	var out []TemplateBlock
	for rows.Next() {
		var b TemplateBlock
		var paramsStr string
		if err := rows.Scan(&b.TemplateID, &b.Ordinal, &b.BlockID, &paramsStr); err != nil {
			return nil, fmt.Errorf("templates: scan block: %w", err)
		}
		if len(paramsStr) > 0 {
			b.Params = json.RawMessage(paramsStr)
		} else {
			b.Params = json.RawMessage(`{}`)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("templates: block rows: %w", err)
	}
	if out == nil {
		out = []TemplateBlock{}
	}
	return out, nil
}

// ReplaceBlocks atomically replaces all blocks in a template.
// The input slice defines the new ordered set; ordinals are assigned from 0.
func (r *Templates) ReplaceBlocks(ctx context.Context, templateID string, in []NewTemplateBlock) ([]TemplateBlock, error) {
	var blocks []TemplateBlock
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		var found string
		if err := tx.QueryRowContext(ctx, `SELECT id FROM app.report_template WHERE id = $1`, templateID).Scan(&found); err != nil {
			if err == sql.ErrNoRows {
				return apierr.NotFound("template", templateID)
			}
			return fmt.Errorf("templates: replace blocks check %s: %w", templateID, err)
		}

		if _, err := tx.ExecContext(ctx, `DELETE FROM app.report_template_block WHERE template_id = $1`, templateID); err != nil {
			return fmt.Errorf("templates: replace blocks delete: %w", err)
		}

		blocks = make([]TemplateBlock, len(in))
		for i, nb := range in {
			b := TemplateBlock{
				TemplateID: templateID,
				Ordinal:    i,
				BlockID:    nb.BlockID,
				Params:     nb.Params,
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO app.report_template_block (template_id, ordinal, block_id, params)
				 VALUES ($1, $2, $3, $4)`,
				b.TemplateID, b.Ordinal, b.BlockID, nullJSON(b.Params),
			); err != nil {
				return fmt.Errorf("templates: replace blocks insert[%d]: %w", i, err)
			}
			blocks[i] = b
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return blocks, nil
}
