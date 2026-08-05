package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// DetectionRuleRef is one Sigma or custom detection reference.
type DetectionRuleRef struct {
	ID                   string
	SourceID             string
	Version              string
	ExternalID           string
	Name                 string
	Description          string
	TechniqueExternalIDs json.RawMessage // JSON array of strings
	Level                string
	RuleStatus           string
	Logsource            json.RawMessage // JSON object
	RuleYAML             string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// Detections reads and writes detection rule refs. Construct with [NewDetections].
type Detections struct {
	db DB
}

// NewDetections returns a repository over db.
func NewDetections(db DB) *Detections { return &Detections{db: db} }

const detectionColumns = `id, source_id, version, external_id, name, description,
	technique_external_ids, level, rule_status, logsource, rule_yaml,
	created_at, updated_at`

// Create inserts one detection rule ref.
//
// after runs inside the same write transaction after the insert so activity
// (M2-011) shares the commit.
func (r *Detections) Create(ctx context.Context, in DetectionRuleRef, after ...After) (DetectionRuleRef, error) {
	id, err := assignID(in.ID)
	if err != nil {
		return DetectionRuleRef{}, err
	}
	ts := now()
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO content.content_detection_rule_ref (
				id, source_id, version, external_id, name, description,
				technique_external_ids, level, rule_status, logsource, rule_yaml,
				created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.SourceID, in.Version, in.ExternalID, in.Name, in.Description,
			bindJSON(in.TechniqueExternalIDs), in.Level, in.RuleStatus,
			bindJSONObject(in.Logsource), in.RuleYAML,
			ts, ts,
		)
		if err := uniqueOr(err, "detection_rule_ref", in.SourceID, in.Version, in.ExternalID); err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, id), tx, after)
	})
	if err != nil {
		return DetectionRuleRef{}, err
	}
	return r.ByID(ctx, id)
}

// Update rewrites mutable fields of an existing detection rule ref.
//
// after runs inside the same write transaction after the update.
func (r *Detections) Update(ctx context.Context, in DetectionRuleRef, after ...After) (DetectionRuleRef, error) {
	ts := now()
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE content.content_detection_rule_ref SET
				name = ?, description = ?,
				technique_external_ids = ?, level = ?, rule_status = ?,
				logsource = ?, rule_yaml = ?,
				updated_at = ?
			WHERE id = ?`,
			in.Name, in.Description,
			bindJSON(in.TechniqueExternalIDs), in.Level, in.RuleStatus,
			bindJSONObject(in.Logsource), in.RuleYAML,
			ts, in.ID,
		)
		if err != nil {
			return fmt.Errorf("content: update detection_rule_ref %s: %w", in.ID, err)
		}
		if err := requireOneRow(res, "content_detection_rule_ref", in.ID); err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, in.ID), tx, after)
	})
	if err != nil {
		return DetectionRuleRef{}, err
	}
	return r.ByID(ctx, in.ID)
}

// Delete removes one detection rule ref by id.
//
// after runs inside the same write transaction after the delete.
func (r *Detections) Delete(ctx context.Context, id string, after ...After) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`DELETE FROM content.content_detection_rule_ref WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("content: delete detection_rule_ref %s: %w", id, err)
		}
		if err := requireOneRow(res, "content_detection_rule_ref", id); err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, id), tx, after)
	})
}

// ByID returns one detection rule ref or [apierr.NotFound].
func (r *Detections) ByID(ctx context.Context, id string) (DetectionRuleRef, error) {
	row := r.db.Read().QueryRowContext(ctx,
		`SELECT `+detectionColumns+` FROM content.content_detection_rule_ref WHERE id = ?`, id)
	d, err := scanDetection(row)
	if err != nil {
		return DetectionRuleRef{}, wrapObjErr(err, "content_detection_rule_ref", id)
	}
	return d, nil
}

func scanDetection(row interface{ Scan(...any) error }) (DetectionRuleRef, error) {
	var (
		d          DetectionRuleRef
		techniques any
		logsource  any
	)
	err := row.Scan(
		&d.ID, &d.SourceID, &d.Version, &d.ExternalID, &d.Name, &d.Description,
		&techniques, &d.Level, &d.RuleStatus, &logsource, &d.RuleYAML,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		return DetectionRuleRef{}, err
	}
	if d.TechniqueExternalIDs, err = jsonBytes(techniques); err != nil {
		return DetectionRuleRef{}, fmt.Errorf("content: detection techniques: %w", err)
	}
	if d.Logsource, err = jsonBytes(logsource); err != nil {
		return DetectionRuleRef{}, fmt.Errorf("content: detection logsource: %w", err)
	}
	d.CreatedAt = d.CreatedAt.UTC()
	d.UpdatedAt = d.UpdatedAt.UTC()
	return d, nil
}

// DetectionListFilter narrows detection rule listings.
//
// EnabledOnly joins content_source.enabled (library browse). Version empty
// means every non-staging version. Q is a case-insensitive substring over
// external_id, name, and description. Technique is an exact membership match
// against the JSON array column (quoted-token scan). Level is a
// case-insensitive exact match on rule_status's sibling level column.
type DetectionListFilter struct {
	SourceID    string
	Version     string
	Q           string
	Technique   string
	Level       string
	EnabledOnly bool
	Limit       int
}

// List returns detection rule refs matching f, ordered by external_id then id.
func (r *Detections) List(ctx context.Context, f DetectionListFilter) ([]DetectionRuleRef, error) {
	var (
		b    strings.Builder
		args []any
	)
	b.WriteString(`
		SELECT d.id, d.source_id, d.version, d.external_id, d.name, d.description,
			d.technique_external_ids, d.level, d.rule_status, d.logsource, d.rule_yaml,
			d.created_at, d.updated_at
		FROM content.content_detection_rule_ref d`)
	if f.EnabledOnly {
		b.WriteString(`
		INNER JOIN content.content_source s ON s.id = d.source_id AND s.enabled = TRUE`)
	}
	b.WriteString(` WHERE d.version <> ?`)
	args = append(args, StagingVersion)
	if f.SourceID != "" {
		b.WriteString(` AND d.source_id = ?`)
		args = append(args, f.SourceID)
	}
	if f.Version != "" {
		b.WriteString(` AND d.version = ?`)
		args = append(args, f.Version)
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		b.WriteString(` AND (
			LOWER(d.external_id) LIKE ? OR
			LOWER(d.name) LIKE ? OR
			LOWER(d.description) LIKE ?
		)`)
		args = append(args, like, like, like)
	}
	if tech := strings.TrimSpace(f.Technique); tech != "" {
		// Quoted-token membership on the JSON array text form.
		// Exact labels only — no substring technique ids.
		b.WriteString(` AND LOWER(CAST(d.technique_external_ids AS VARCHAR)) LIKE ?`)
		args = append(args, "%\""+strings.ToLower(tech)+"\"%")
	}
	if level := strings.TrimSpace(f.Level); level != "" {
		b.WriteString(` AND LOWER(d.level) = ?`)
		args = append(args, strings.ToLower(level))
	}
	b.WriteString(` ORDER BY d.external_id, d.id`)
	if lim := listLimit(f.Limit); lim > 0 {
		b.WriteString(` LIMIT ?`)
		args = append(args, lim)
	}
	rows, err := r.db.Read().QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("content: list detection_rule_ref: %w", err)
	}
	defer rows.Close()
	var out []DetectionRuleRef
	for rows.Next() {
		d, err := scanDetection(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []DetectionRuleRef{}
	}
	return out, nil
}

// ByIDEnabled returns one detection rule ref, or [apierr.NotFound] when the
// row is missing or (enabledOnly) its source is disabled.
func (r *Detections) ByIDEnabled(ctx context.Context, id string, enabledOnly bool) (DetectionRuleRef, error) {
	d, err := r.ByID(ctx, id)
	if err != nil {
		return DetectionRuleRef{}, err
	}
	if d.Version == StagingVersion {
		return DetectionRuleRef{}, wrapObjErr(sql.ErrNoRows, "content_detection_rule_ref", id)
	}
	if enabledOnly {
		if err := requireEnabledSource(ctx, r.db, d.SourceID); err != nil {
			return DetectionRuleRef{}, err
		}
	}
	return d, nil
}

// ClearDetectionVersion deletes every detection rule ref for (sourceID, version).
// Exported so the Sigma adapter Apply path can share it inside a Writer
// transaction without opening a nested store.Write.
func ClearDetectionVersion(ctx context.Context, tx *sql.Tx, sourceID, version string) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM content.content_detection_rule_ref
		WHERE source_id = ? AND version = ?`,
		sourceID, version,
	); err != nil {
		return fmt.Errorf("content: clear detection_rule_ref %s/%s: %w", sourceID, version, err)
	}
	return nil
}

// PromoteDetectionVersion moves every detection rule ref row from fromVersion
// to toVersion inside tx, after deleting any existing toVersion rows. Both
// halves share one transaction so a failed re-sync never leaves a half-replaced
// rolling catalog.
func PromoteDetectionVersion(ctx context.Context, tx *sql.Tx, sourceID, fromVersion, toVersion string) error {
	if err := ClearDetectionVersion(ctx, tx, sourceID, toVersion); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE content.content_detection_rule_ref
		SET version = ?
		WHERE source_id = ? AND version = ?`,
		toVersion, sourceID, fromVersion,
	); err != nil {
		return fmt.Errorf("content: promote detection_rule_ref: %w", err)
	}
	return nil
}
