package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
)

// Tactic is one ATT&CK tactic row.
type Tactic struct {
	ID          string
	SourceID    string
	Version     string
	ExternalID  string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Technique is one ATT&CK technique or sub-technique.
type Technique struct {
	ID               string
	SourceID         string
	Version          string
	ExternalID       string
	Name             string
	Description      string
	IsSubtechnique   bool
	ParentExternalID string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Mitigation is one ATT&CK mitigation.
type Mitigation struct {
	ID          string
	SourceID    string
	Version     string
	ExternalID  string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Group is one ATT&CK intrusion set / group.
type Group struct {
	ID          string
	SourceID    string
	Version     string
	ExternalID  string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Software is one ATT&CK malware or tool.
type Software struct {
	ID           string
	SourceID     string
	Version      string
	ExternalID   string
	Name         string
	Description  string
	SoftwareType SoftwareType
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// DataSource is one ATT&CK data source / component as upstream provides it.
type DataSource struct {
	ID          string
	SourceID    string
	Version     string
	ExternalID  string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Objects writes and reads ATT&CK object families. Construct with [NewObjects].
type Objects struct {
	db DB
}

// NewObjects returns a repository over db.
func NewObjects(db DB) *Objects { return &Objects{db: db} }

// CreateTactic inserts one tactic.
func (r *Objects) CreateTactic(ctx context.Context, in Tactic) (Tactic, error) {
	id, ts, err := prepInsert(in.ID)
	if err != nil {
		return Tactic{}, err
	}
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO content.content_tactic
				(id, source_id, version, external_id, name, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.SourceID, in.Version, in.ExternalID, in.Name, in.Description, ts, ts,
		)
		return uniqueOr(err, "tactic", in.SourceID, in.Version, in.ExternalID)
	})
	if err != nil {
		return Tactic{}, err
	}
	return r.TacticByID(ctx, id)
}

// TacticByID returns one tactic or [apierr.NotFound].
func (r *Objects) TacticByID(ctx context.Context, id string) (Tactic, error) {
	row := r.db.Read().QueryRowContext(ctx, `
		SELECT id, source_id, version, external_id, name, description, created_at, updated_at
		FROM content.content_tactic WHERE id = ?`, id)
	var t Tactic
	err := row.Scan(&t.ID, &t.SourceID, &t.Version, &t.ExternalID, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return Tactic{}, wrapObjErr(err, "content_tactic", id)
	}
	t.CreatedAt = t.CreatedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	return t, nil
}

// CreateTechnique inserts one technique.
func (r *Objects) CreateTechnique(ctx context.Context, in Technique) (Technique, error) {
	id, ts, err := prepInsert(in.ID)
	if err != nil {
		return Technique{}, err
	}
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO content.content_technique
				(id, source_id, version, external_id, name, description,
				 is_subtechnique, parent_external_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.SourceID, in.Version, in.ExternalID, in.Name, in.Description,
			in.IsSubtechnique, in.ParentExternalID, ts, ts,
		)
		return uniqueOr(err, "technique", in.SourceID, in.Version, in.ExternalID)
	})
	if err != nil {
		return Technique{}, err
	}
	return r.TechniqueByID(ctx, id)
}

// TechniqueByID returns one technique or [apierr.NotFound].
func (r *Objects) TechniqueByID(ctx context.Context, id string) (Technique, error) {
	row := r.db.Read().QueryRowContext(ctx, `
		SELECT id, source_id, version, external_id, name, description,
			is_subtechnique, parent_external_id, created_at, updated_at
		FROM content.content_technique WHERE id = ?`, id)
	var t Technique
	err := row.Scan(
		&t.ID, &t.SourceID, &t.Version, &t.ExternalID, &t.Name, &t.Description,
		&t.IsSubtechnique, &t.ParentExternalID, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return Technique{}, wrapObjErr(err, "content_technique", id)
	}
	t.CreatedAt = t.CreatedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	return t, nil
}

// SetTechniqueTactics replaces the tactic membership for one technique natural
// key. Empty tactics clears the membership.
func (r *Objects) SetTechniqueTactics(ctx context.Context, sourceID, version, techniqueExternalID string, tacticExternalIDs []string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM content.content_technique_tactic
			WHERE source_id = ? AND version = ? AND technique_external_id = ?`,
			sourceID, version, techniqueExternalID,
		); err != nil {
			return fmt.Errorf("content: clear technique tactics: %w", err)
		}
		for _, tac := range tacticExternalIDs {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO content.content_technique_tactic
					(source_id, version, technique_external_id, tactic_external_id)
				VALUES (?, ?, ?, ?)`,
				sourceID, version, techniqueExternalID, tac,
			); err != nil {
				return fmt.Errorf("content: insert technique tactic: %w", err)
			}
		}
		return nil
	})
}

// TechniqueTactics lists tactic external ids for one technique natural key.
func (r *Objects) TechniqueTactics(ctx context.Context, sourceID, version, techniqueExternalID string) ([]string, error) {
	rows, err := r.db.Read().QueryContext(ctx, `
		SELECT tactic_external_id FROM content.content_technique_tactic
		WHERE source_id = ? AND version = ? AND technique_external_id = ?
		ORDER BY tactic_external_id`,
		sourceID, version, techniqueExternalID,
	)
	if err != nil {
		return nil, fmt.Errorf("content: list technique tactics: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []string{}
	}
	return out, nil
}

// CreateMitigation inserts one mitigation.
func (r *Objects) CreateMitigation(ctx context.Context, in Mitigation) (Mitigation, error) {
	id, ts, err := prepInsert(in.ID)
	if err != nil {
		return Mitigation{}, err
	}
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO content.content_mitigation
				(id, source_id, version, external_id, name, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.SourceID, in.Version, in.ExternalID, in.Name, in.Description, ts, ts,
		)
		return uniqueOr(err, "mitigation", in.SourceID, in.Version, in.ExternalID)
	})
	if err != nil {
		return Mitigation{}, err
	}
	return r.MitigationByID(ctx, id)
}

// MitigationByID returns one mitigation.
func (r *Objects) MitigationByID(ctx context.Context, id string) (Mitigation, error) {
	row := r.db.Read().QueryRowContext(ctx, `
		SELECT id, source_id, version, external_id, name, description, created_at, updated_at
		FROM content.content_mitigation WHERE id = ?`, id)
	var m Mitigation
	err := row.Scan(&m.ID, &m.SourceID, &m.Version, &m.ExternalID, &m.Name, &m.Description, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return Mitigation{}, wrapObjErr(err, "content_mitigation", id)
	}
	m.CreatedAt = m.CreatedAt.UTC()
	m.UpdatedAt = m.UpdatedAt.UTC()
	return m, nil
}

// CreateGroup inserts one group.
func (r *Objects) CreateGroup(ctx context.Context, in Group) (Group, error) {
	id, ts, err := prepInsert(in.ID)
	if err != nil {
		return Group{}, err
	}
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO content.content_group
				(id, source_id, version, external_id, name, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.SourceID, in.Version, in.ExternalID, in.Name, in.Description, ts, ts,
		)
		return uniqueOr(err, "group", in.SourceID, in.Version, in.ExternalID)
	})
	if err != nil {
		return Group{}, err
	}
	return r.GroupByID(ctx, id)
}

// GroupByID returns one group.
func (r *Objects) GroupByID(ctx context.Context, id string) (Group, error) {
	row := r.db.Read().QueryRowContext(ctx, `
		SELECT id, source_id, version, external_id, name, description, created_at, updated_at
		FROM content.content_group WHERE id = ?`, id)
	var g Group
	err := row.Scan(&g.ID, &g.SourceID, &g.Version, &g.ExternalID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return Group{}, wrapObjErr(err, "content_group", id)
	}
	g.CreatedAt = g.CreatedAt.UTC()
	g.UpdatedAt = g.UpdatedAt.UTC()
	return g, nil
}

// CreateSoftware inserts one software row.
func (r *Objects) CreateSoftware(ctx context.Context, in Software) (Software, error) {
	id, ts, err := prepInsert(in.ID)
	if err != nil {
		return Software{}, err
	}
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO content.content_software
				(id, source_id, version, external_id, name, description, software_type, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.SourceID, in.Version, in.ExternalID, in.Name, in.Description, string(in.SoftwareType), ts, ts,
		)
		return uniqueOr(err, "software", in.SourceID, in.Version, in.ExternalID)
	})
	if err != nil {
		return Software{}, err
	}
	return r.SoftwareByID(ctx, id)
}

// SoftwareByID returns one software row.
func (r *Objects) SoftwareByID(ctx context.Context, id string) (Software, error) {
	row := r.db.Read().QueryRowContext(ctx, `
		SELECT id, source_id, version, external_id, name, description, software_type, created_at, updated_at
		FROM content.content_software WHERE id = ?`, id)
	var s Software
	var st string
	err := row.Scan(&s.ID, &s.SourceID, &s.Version, &s.ExternalID, &s.Name, &s.Description, &st, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return Software{}, wrapObjErr(err, "content_software", id)
	}
	s.SoftwareType = SoftwareType(st)
	s.CreatedAt = s.CreatedAt.UTC()
	s.UpdatedAt = s.UpdatedAt.UTC()
	return s, nil
}

// CreateDataSource inserts one data source.
func (r *Objects) CreateDataSource(ctx context.Context, in DataSource) (DataSource, error) {
	id, ts, err := prepInsert(in.ID)
	if err != nil {
		return DataSource{}, err
	}
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO content.content_data_source
				(id, source_id, version, external_id, name, description, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.SourceID, in.Version, in.ExternalID, in.Name, in.Description, ts, ts,
		)
		return uniqueOr(err, "data_source", in.SourceID, in.Version, in.ExternalID)
	})
	if err != nil {
		return DataSource{}, err
	}
	return r.DataSourceByID(ctx, id)
}

// DataSourceByID returns one data source.
func (r *Objects) DataSourceByID(ctx context.Context, id string) (DataSource, error) {
	row := r.db.Read().QueryRowContext(ctx, `
		SELECT id, source_id, version, external_id, name, description, created_at, updated_at
		FROM content.content_data_source WHERE id = ?`, id)
	var d DataSource
	err := row.Scan(&d.ID, &d.SourceID, &d.Version, &d.ExternalID, &d.Name, &d.Description, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return DataSource{}, wrapObjErr(err, "content_data_source", id)
	}
	d.CreatedAt = d.CreatedAt.UTC()
	d.UpdatedAt = d.UpdatedAt.UTC()
	return d, nil
}

func prepInsert(id string) (string, time.Time, error) {
	out, err := assignID(id)
	if err != nil {
		return "", time.Time{}, err
	}
	return out, now(), nil
}

func assignID(id string) (string, error) {
	if id != "" {
		return id, nil
	}
	return newID()
}

func uniqueOr(err error, label, sourceID, version, externalID string) error {
	if err == nil {
		return nil
	}
	if store.IsUniqueViolation(err) {
		return apierr.Conflict(fmt.Sprintf(
			"%s %q already exists for source %s version %s",
			label, externalID, sourceID, version,
		))
	}
	return fmt.Errorf("content: insert %s: %w", label, err)
}

func wrapObjErr(err error, resource, id string) error {
	if errors.Is(err, sql.ErrNoRows) {
		return apierr.NotFound(resource, id)
	}
	return fmt.Errorf("content: %s %s: %w", resource, id, err)
}

// bindJSON returns a driver-friendly value for a JSON array column.
func bindJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return []byte(`[]`)
	}
	buf := make([]byte, len(raw))
	copy(buf, raw)
	return buf
}

// bindJSONObject is bindJSON for object-shaped defaults.
func bindJSONObject(raw json.RawMessage) any {
	if len(raw) == 0 {
		return []byte(`{}`)
	}
	buf := make([]byte, len(raw))
	copy(buf, raw)
	return buf
}
