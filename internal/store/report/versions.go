package report

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

// ---------------------------------------------------------------------------
// Column constants
// ---------------------------------------------------------------------------
const versionColumns = `id, report_id, ordinal, title, published_by, published_at,
	include_evidence, blind_scope, blocks_json, branding_json, html,
	content_sha256, pdf_sha256`

// versionSelectColumns is versionColumns with the two JSON columns cast to
// VARCHAR. DuckDB scans a JSON column as a parsed value ([]interface{} /
// map[string]interface{}), which database/sql cannot store in json.RawMessage;
// casting to VARCHAR returns the raw JSON text.
const versionSelectColumns = `id, report_id, ordinal, title, published_by, published_at,
	include_evidence, blind_scope, CAST(blocks_json AS VARCHAR), CAST(branding_json AS VARCHAR), html,
	content_sha256, pdf_sha256`

// ---------------------------------------------------------------------------
// Versions repository
// ---------------------------------------------------------------------------

// Versions reads and writes published report versions. Construct with [NewVersions].
// Content columns are insert-only — no Update method exists.
type Versions struct {
	db DB
}

// NewVersions returns a repository over db.
func NewVersions(db DB) *Versions { return &Versions{db: db} }

// Insert creates a new published version. It is the only write path for
// version content — there is no Update, so content is immutable.
func (v *Versions) Insert(ctx context.Context, in NewVersion, after ...After) (ReportVersion, error) {
	id := newID()
	now := now()

	var contentSHA256Arg any
	if in.ContentSHA256 != "" {
		contentSHA256Arg = in.ContentSHA256
	}

	row := ReportVersion{
		ID:              id,
		ReportID:        in.ReportID,
		Ordinal:         in.Ordinal,
		Title:           in.Title,
		PublishedBy:     in.PublishedBy,
		PublishedAt:     now,
		IncludeEvidence: in.IncludeEvidence,
		BlindScope:      in.BlindScope,
		BlocksJSON:      in.BlocksJSON,
		BrandingJSON:    in.BrandingJSON,
		HTML:            in.HTML,
		ContentSHA256:   strPtr(in.ContentSHA256),
	}

	err := v.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO app.report_version (`+versionColumns+`)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.ReportID, in.Ordinal, in.Title, in.PublishedBy,
			now, in.IncludeEvidence, in.BlindScope,
			jsonOrDefault(in.BlocksJSON, `[]`), jsonOrDefault(in.BrandingJSON, `{}`),
			in.HTML, contentSHA256Arg, nil,
		)
		if err != nil {
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
		return ReportVersion{}, err
	}

	row.PublishedAt = now
	return row, nil
}

// ByID returns the version with this identifier, or [apierr.NotFound].
func (v *Versions) ByID(ctx context.Context, id string) (ReportVersion, error) {
	var ver ReportVersion
	err := v.db.Read().QueryRowContext(ctx,
		`SELECT `+versionSelectColumns+` FROM app.report_version WHERE id = ?`, id,
	).Scan(
		&ver.ID, &ver.ReportID, &ver.Ordinal, &ver.Title, &ver.PublishedBy,
		&ver.PublishedAt, &ver.IncludeEvidence, &ver.BlindScope,
		&ver.BlocksJSON, &ver.BrandingJSON, &ver.HTML,
		&ver.ContentSHA256, &ver.PDFSHA256,
	)
	if err == sql.ErrNoRows {
		return ReportVersion{}, apierr.NotFound("report_version", id)
	}
	if err != nil {
		return ReportVersion{}, err
	}
	return ver, nil
}

// ListByReport returns every version of a report, newest first.
func (v *Versions) ListByReport(ctx context.Context, reportID string) ([]ReportVersion, error) {
	rows, err := v.db.Read().QueryContext(ctx,
		`SELECT `+versionSelectColumns+`
		 FROM app.report_version
		 WHERE report_id = ?
		 ORDER BY ordinal DESC`, reportID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []ReportVersion
	for rows.Next() {
		var ver ReportVersion
		if err := rows.Scan(
			&ver.ID, &ver.ReportID, &ver.Ordinal, &ver.Title, &ver.PublishedBy,
			&ver.PublishedAt, &ver.IncludeEvidence, &ver.BlindScope,
			&ver.BlocksJSON, &ver.BrandingJSON, &ver.HTML,
			&ver.ContentSHA256, &ver.PDFSHA256,
		); err != nil {
			return nil, err
		}
		versions = append(versions, ver)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if versions == nil {
		versions = []ReportVersion{}
	}
	return versions, nil
}

// SetPDFSHA256 stores the PDF hash after first generation. This is the only
// mutable column on a version row — the PDF is generated lazily.
func (v *Versions) SetPDFSHA256(ctx context.Context, id string, sha256Hex string) error {
	return v.db.Write(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE app.report_version SET pdf_sha256 = ? WHERE id = ?`,
			sha256Hex, id,
		)
		if err != nil {
			return err
		}
		return requireOneRow(result, "report_version", id)
	})
}

// NextOrdinal returns the next ordinal for a report (1 if no versions exist).
func (v *Versions) NextOrdinal(ctx context.Context, reportID string) (int, error) {
	var ordinal sql.NullInt64
	err := v.db.Read().QueryRowContext(ctx,
		`SELECT MAX(ordinal) FROM app.report_version WHERE report_id = ?`,
		reportID,
	).Scan(&ordinal)
	if err != nil {
		return 0, err
	}
	if !ordinal.Valid {
		return 1, nil
	}
	return int(ordinal.Int64) + 1, nil
}

// CountByReport returns the number of published versions for a report.
func (v *Versions) CountByReport(ctx context.Context, reportID string) (int, error) {
	var count int
	err := v.db.Read().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM app.report_version WHERE report_id = ?`,
		reportID,
	).Scan(&count)
	return count, err
}

// DeleteByReport removes all versions of a report (cascade before deleting
// the report itself, since DuckDB only supports RESTRICT).
func (v *Versions) DeleteByReport(ctx context.Context, reportID string) error {
	return v.db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx,
			`DELETE FROM app.report_version WHERE report_id = ?`,
			reportID,
		)
		return err
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// HashBytes returns the hex-encoded SHA-256 of data.
func HashBytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// jsonOrDefault returns the JSON message as a string, or fallback when empty.
// blocks_json and branding_json are NOT NULL columns; an empty draft must store
// valid JSON ('[]' / '{}') rather than NULL, or the read-back scan into json.RawMessage fails.
func jsonOrDefault(raw json.RawMessage, fallback string) any {
	if len(raw) == 0 || string(raw) == "null" {
		return fallback
	}
	return string(raw)
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// scanVersion scans a row into a ReportVersion. Exported for tests.
func ScanVersion(scanner interface {
	Scan(dest ...any) error
}) (ReportVersion, error) {
	var ver ReportVersion
	err := scanner.Scan(
		&ver.ID, &ver.ReportID, &ver.Ordinal, &ver.Title, &ver.PublishedBy,
		&ver.PublishedAt, &ver.IncludeEvidence, &ver.BlindScope,
		&ver.BlocksJSON, &ver.BrandingJSON, &ver.HTML,
		&ver.ContentSHA256, &ver.PDFSHA256,
	)
	return ver, err
}
