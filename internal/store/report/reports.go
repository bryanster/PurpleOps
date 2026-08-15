package report

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

const reportColumns = `id, engagement_id, title, client_name, logo_blob_ref, colours, created_by, created_at, updated_by, updated_at`

const reportBlockColumns = `id, report_id, ordinal, block_id, CAST(params AS VARCHAR)`

const selectReport = `SELECT ` + reportColumns + ` FROM app.report `

const selectReportBlock = `SELECT ` + reportBlockColumns + ` FROM app.report_block `

// Reports reads and writes report drafts. Construct it with [NewReports].
type Reports struct {
	db DB
}

// NewReports returns a repository over db.
func NewReports(db DB) *Reports { return &Reports{db: db} }

// Create writes a new report draft and returns it as stored.
func (r *Reports) Create(ctx context.Context, in NewReport, after ...After) (Report, error) {
	rep := Report{
		ID:           newID(),
		EngagementID: in.EngagementID,
		Title:        in.Title,
		CreatedBy:    in.CreatedBy,
		CreatedAt:    now(),
		UpdatedAt:    now(),
	}

	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO app.report
				(id, engagement_id, title, client_name, logo_blob_ref, colours, created_by, created_at, updated_by, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			rep.ID, rep.EngagementID, rep.Title,
			nil, nil, nil, // branding starts null
			rep.CreatedBy, rep.CreatedAt, nil, rep.UpdatedAt,
		)
		if err != nil {
			return fmt.Errorf("report: insert: %w", err)
		}
		for _, fn := range after {
			if fn == nil {
				continue
			}
			if err := fn(ctx, tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}
	return rep, nil
}

// ByID returns the report with this identifier, or [apierr.NotFound].
func (r *Reports) ByID(ctx context.Context, id string) (Report, error) {
	var rep Report
	var clientName, logoBlobRef, updatedBy sql.NullString
	var colours sql.NullString

	err := r.db.Read().QueryRowContext(ctx, selectReport+`WHERE id = ?`, id).Scan(
		&rep.ID, &rep.EngagementID, &rep.Title,
		&clientName, &logoBlobRef, &colours,
		&rep.CreatedBy, &rep.CreatedAt, &updatedBy, &rep.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Report{}, apierr.NotFound("report", id)
	}
	if err != nil {
		return Report{}, fmt.Errorf("report: by id %s: %w", id, err)
	}

	if clientName.Valid {
		rep.ClientName = &clientName.String
	}
	if logoBlobRef.Valid {
		rep.LogoBlobRef = &logoBlobRef.String
	}
	if colours.Valid {
		rep.Colours = json.RawMessage(colours.String)
	}
	if updatedBy.Valid {
		rep.UpdatedBy = &updatedBy.String
	}
	return rep, nil
}

// ListByEngagement returns every report in an engagement, newest first.
func (r *Reports) ListByEngagement(ctx context.Context, engagementID string) ([]Report, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		selectReport+`WHERE engagement_id = ? ORDER BY created_at DESC, id`,
		engagementID,
	)
	if err != nil {
		return nil, fmt.Errorf("report: list by engagement %s: %w", engagementID, err)
	}
	defer rows.Close()

	var reports []Report
	for rows.Next() {
		var rep Report
		var clientName, logoBlobRef, updatedBy sql.NullString
		var colours sql.NullString

		if err := rows.Scan(
			&rep.ID, &rep.EngagementID, &rep.Title,
			&clientName, &logoBlobRef, &colours,
			&rep.CreatedBy, &rep.CreatedAt, &updatedBy, &rep.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("report: scan: %w", err)
		}

		if clientName.Valid {
			rep.ClientName = &clientName.String
		}
		if logoBlobRef.Valid {
			rep.LogoBlobRef = &logoBlobRef.String
		}
		if colours.Valid {
			rep.Colours = json.RawMessage(colours.String)
		}
		if updatedBy.Valid {
			rep.UpdatedBy = &updatedBy.String
		}
		reports = append(reports, rep)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("report: rows: %w", err)
	}
	return reports, nil
}

// Update patches a report and returns it as stored.
func (r *Reports) Update(ctx context.Context, id string, changes ReportUpdate, after ...After) (Report, error) {
	rep, err := r.ByID(ctx, id)
	if err != nil {
		return Report{}, err
	}

	if changes.Title != nil {
		rep.Title = *changes.Title
	}
	if changes.ClientName != nil {
		rep.ClientName = *changes.ClientName
	}
	if changes.LogoBlobRef != nil {
		rep.LogoBlobRef = *changes.LogoBlobRef
	}
	if changes.Colours != nil {
		rep.Colours = *changes.Colours
	}
	rep.UpdatedBy = &changes.UpdatedBy
	ts := now()
	rep.UpdatedAt = ts

	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE app.report SET
				title = ?, client_name = ?, logo_blob_ref = ?, colours = ?,
				updated_by = ?, updated_at = ?
				WHERE id = ?`,
			rep.Title,
			rep.ClientName,
			rep.LogoBlobRef,
			nullJSON(rep.Colours),
			rep.UpdatedBy,
			rep.UpdatedAt,
			id,
		)
		if err != nil {
			return fmt.Errorf("report: update %s: %w", id, err)
		}
		if err := requireOneRow(result, "report", id); err != nil {
			return err
		}
		for _, fn := range after {
			if fn == nil {
				continue
			}
			if err := fn(ctx, tx); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}
	return rep, nil
}

// Delete removes a report and cascades to its draft blocks.
// The order respects FK RESTRICT constraints: blocks first, then report.
func (r *Reports) Delete(ctx context.Context, id string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM app.report_block WHERE report_id = ?`, id); err != nil {
			return fmt.Errorf("report: delete blocks %s: %w", id, err)
		}
		result, err := tx.ExecContext(ctx, `DELETE FROM app.report WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("report: delete %s: %w", id, err)
		}
		return requireOneRow(result, "report", id)
	})
}

// ---------------------------------------------------------------------------
// Blocks
// ---------------------------------------------------------------------------

// BlocksByReport returns every block in a report, ordered by ordinal.
func (r *Reports) BlocksByReport(ctx context.Context, reportID string) ([]ReportBlock, error) {
	rows, err := r.db.Read().QueryContext(ctx,
		selectReportBlock+`WHERE report_id = ? ORDER BY ordinal`,
		reportID,
	)
	if err != nil {
		return nil, fmt.Errorf("report block: list %s: %w", reportID, err)
	}
	defer rows.Close()

	var blocks []ReportBlock
	for rows.Next() {
		var b ReportBlock
		var paramsStr string
		if err := rows.Scan(&b.ID, &b.ReportID, &b.Ordinal, &b.BlockID, &paramsStr); err != nil {
			return nil, fmt.Errorf("report block: scan: %w", err)
		}
		b.Params = json.RawMessage(paramsStr)
		blocks = append(blocks, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("report block: rows: %w", err)
	}
	return blocks, nil
}

// ReplaceBlocks atomically replaces all blocks in a report.
// The input slice defines the new ordered set; ordinals are assigned from 0.
func (r *Reports) ReplaceBlocks(ctx context.Context, reportID string, in []NewBlock) ([]ReportBlock, error) {
	blocks := make([]ReportBlock, len(in))
	for i, nb := range in {
		blocks[i] = ReportBlock{
			ID:       newID(),
			ReportID: reportID,
			Ordinal:  i,
			BlockID:  nb.BlockID,
			Params:   nb.Params,
		}
	}
	// Sort for deterministic output regardless of input order.
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].Ordinal < blocks[j].Ordinal })

	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM app.report_block WHERE report_id = ?`, reportID); err != nil {
			return fmt.Errorf("report block: clear %s: %w", reportID, err)
		}
		for _, b := range blocks {
			_, err := tx.ExecContext(ctx,
				`INSERT INTO app.report_block (id, report_id, ordinal, block_id, params) VALUES (?, ?, ?, ?, ?)`,
				b.ID, b.ReportID, b.Ordinal, b.BlockID, string(b.Params),
			)
			if err != nil {
				return fmt.Errorf("report block: insert %s: %w", reportID, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return blocks, nil
}

// NewBlock describes the caller's half of creating a block.
type NewBlock struct {
	BlockID string
	Params  json.RawMessage
}

// nullJSON returns nil for a nil or empty JSON message, for DuckDB NULL binding.
func nullJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return string(raw)
}
