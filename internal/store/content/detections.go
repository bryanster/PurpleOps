package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
func (r *Detections) Create(ctx context.Context, in DetectionRuleRef) (DetectionRuleRef, error) {
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
		return uniqueOr(err, "detection_rule_ref", in.SourceID, in.Version, in.ExternalID)
	})
	if err != nil {
		return DetectionRuleRef{}, err
	}
	return r.ByID(ctx, id)
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
