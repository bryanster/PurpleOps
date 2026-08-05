package content

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// StagingVersion is the temporary version token adapters write into before an
// atomic promote. Library list APIs never return it. Real ATT&CK release labels
// never match this string.
const StagingVersion = "__staging__"

// ObjectListFilter narrows ATT&CK object listings.
//
// EnabledOnly defaults the join against content_source.enabled when true
// (library browse). Version empty means every non-staging version. Q is a
// case-insensitive substring over external_id, name, and description.
type ObjectListFilter struct {
	Version        string
	Q              string
	Tactic         string // techniques only: join content_technique_tactic
	IsSubtechnique *bool  // techniques only
	EnabledOnly    bool
	Limit          int
}

// TechniqueDetail is a technique plus its tactic and mitigation memberships.
type TechniqueDetail struct {
	Technique
	Tactics     []string
	Mitigations []string
}

// ListTechniques returns techniques matching f, ordered by external_id then id.
func (r *Objects) ListTechniques(ctx context.Context, f ObjectListFilter) ([]Technique, error) {
	var (
		b    strings.Builder
		args []any
	)
	b.WriteString(`
		SELECT t.id, t.source_id, t.version, t.external_id, t.name, t.description,
			t.is_subtechnique, t.parent_external_id, t.created_at, t.updated_at
		FROM content.content_technique t`)
	if f.EnabledOnly {
		b.WriteString(`
		INNER JOIN content.content_source s ON s.id = t.source_id AND s.enabled = TRUE`)
	}
	if f.Tactic != "" {
		b.WriteString(`
		INNER JOIN content.content_technique_tactic tt
			ON tt.source_id = t.source_id
			AND tt.version = t.version
			AND tt.technique_external_id = t.external_id
			AND tt.tactic_external_id = ?`)
		args = append(args, f.Tactic)
	}
	b.WriteString(` WHERE t.version <> ?`)
	args = append(args, StagingVersion)
	if f.Version != "" {
		b.WriteString(` AND t.version = ?`)
		args = append(args, f.Version)
	}
	if f.IsSubtechnique != nil {
		b.WriteString(` AND t.is_subtechnique = ?`)
		args = append(args, *f.IsSubtechnique)
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		b.WriteString(` AND (
			LOWER(t.external_id) LIKE ? OR
			LOWER(t.name) LIKE ? OR
			LOWER(t.description) LIKE ?
		)`)
		args = append(args, like, like, like)
	}
	b.WriteString(` ORDER BY t.external_id, t.id`)
	if lim := listLimit(f.Limit); lim > 0 {
		b.WriteString(` LIMIT ?`)
		args = append(args, lim)
	}

	rows, err := r.db.Read().QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("content: list techniques: %w", err)
	}
	defer rows.Close()

	var out []Technique
	for rows.Next() {
		var t Technique
		if err := rows.Scan(
			&t.ID, &t.SourceID, &t.Version, &t.ExternalID, &t.Name, &t.Description,
			&t.IsSubtechnique, &t.ParentExternalID, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("content: scan technique: %w", err)
		}
		t.CreatedAt = t.CreatedAt.UTC()
		t.UpdatedAt = t.UpdatedAt.UTC()
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Technique{}
	}
	return out, nil
}

// TechniqueDetailByID returns one technique with tactic/mitigation links.
// When enabledOnly is true and the source is disabled, answers NotFound so
// library browse cannot leak disabled catalog rows by id.
func (r *Objects) TechniqueDetailByID(ctx context.Context, id string, enabledOnly bool) (TechniqueDetail, error) {
	t, err := r.TechniqueByID(ctx, id)
	if err != nil {
		return TechniqueDetail{}, err
	}
	if t.Version == StagingVersion {
		return TechniqueDetail{}, wrapObjErr(sql.ErrNoRows, "content_technique", id)
	}
	if enabledOnly {
		if err := r.requireEnabledSource(ctx, t.SourceID); err != nil {
			return TechniqueDetail{}, err
		}
	}
	tactics, err := r.TechniqueTactics(ctx, t.SourceID, t.Version, t.ExternalID)
	if err != nil {
		return TechniqueDetail{}, err
	}
	mits, err := r.TechniqueMitigations(ctx, t.SourceID, t.Version, t.ExternalID)
	if err != nil {
		return TechniqueDetail{}, err
	}
	return TechniqueDetail{Technique: t, Tactics: tactics, Mitigations: mits}, nil
}

// ListTactics returns tactics matching f.
func (r *Objects) ListTactics(ctx context.Context, f ObjectListFilter) ([]Tactic, error) {
	return listNamed[Tactic](ctx, r, f, "content_tactic", scanTactic)
}

// TacticByIDEnabled wraps TacticByID with the enabled-source gate.
func (r *Objects) TacticByIDEnabled(ctx context.Context, id string, enabledOnly bool) (Tactic, error) {
	t, err := r.TacticByID(ctx, id)
	if err != nil {
		return Tactic{}, err
	}
	if t.Version == StagingVersion {
		return Tactic{}, wrapObjErr(sql.ErrNoRows, "content_tactic", id)
	}
	if enabledOnly {
		if err := r.requireEnabledSource(ctx, t.SourceID); err != nil {
			return Tactic{}, err
		}
	}
	return t, nil
}

// ListMitigations returns mitigations matching f.
func (r *Objects) ListMitigations(ctx context.Context, f ObjectListFilter) ([]Mitigation, error) {
	return listNamed[Mitigation](ctx, r, f, "content_mitigation", scanMitigation)
}

// MitigationByIDEnabled wraps MitigationByID with the enabled-source gate.
func (r *Objects) MitigationByIDEnabled(ctx context.Context, id string, enabledOnly bool) (Mitigation, error) {
	m, err := r.MitigationByID(ctx, id)
	if err != nil {
		return Mitigation{}, err
	}
	if m.Version == StagingVersion {
		return Mitigation{}, wrapObjErr(sql.ErrNoRows, "content_mitigation", id)
	}
	if enabledOnly {
		if err := r.requireEnabledSource(ctx, m.SourceID); err != nil {
			return Mitigation{}, err
		}
	}
	return m, nil
}

// ListGroups returns groups matching f.
func (r *Objects) ListGroups(ctx context.Context, f ObjectListFilter) ([]Group, error) {
	return listNamed[Group](ctx, r, f, "content_group", scanGroup)
}

// GroupByIDEnabled wraps GroupByID with the enabled-source gate.
func (r *Objects) GroupByIDEnabled(ctx context.Context, id string, enabledOnly bool) (Group, error) {
	g, err := r.GroupByID(ctx, id)
	if err != nil {
		return Group{}, err
	}
	if g.Version == StagingVersion {
		return Group{}, wrapObjErr(sql.ErrNoRows, "content_group", id)
	}
	if enabledOnly {
		if err := r.requireEnabledSource(ctx, g.SourceID); err != nil {
			return Group{}, err
		}
	}
	return g, nil
}

// ListSoftware returns software matching f.
func (r *Objects) ListSoftware(ctx context.Context, f ObjectListFilter) ([]Software, error) {
	var (
		b    strings.Builder
		args []any
	)
	b.WriteString(`
		SELECT o.id, o.source_id, o.version, o.external_id, o.name, o.description,
			o.software_type, o.created_at, o.updated_at
		FROM content.content_software o`)
	if f.EnabledOnly {
		b.WriteString(`
		INNER JOIN content.content_source s ON s.id = o.source_id AND s.enabled = TRUE`)
	}
	b.WriteString(` WHERE o.version <> ?`)
	args = append(args, StagingVersion)
	if f.Version != "" {
		b.WriteString(` AND o.version = ?`)
		args = append(args, f.Version)
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		b.WriteString(` AND (
			LOWER(o.external_id) LIKE ? OR
			LOWER(o.name) LIKE ? OR
			LOWER(o.description) LIKE ?
		)`)
		args = append(args, like, like, like)
	}
	b.WriteString(` ORDER BY o.external_id, o.id`)
	if lim := listLimit(f.Limit); lim > 0 {
		b.WriteString(` LIMIT ?`)
		args = append(args, lim)
	}
	rows, err := r.db.Read().QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("content: list software: %w", err)
	}
	defer rows.Close()
	var out []Software
	for rows.Next() {
		var s Software
		var st string
		if err := rows.Scan(
			&s.ID, &s.SourceID, &s.Version, &s.ExternalID, &s.Name, &s.Description,
			&st, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("content: scan software: %w", err)
		}
		s.SoftwareType = SoftwareType(st)
		s.CreatedAt = s.CreatedAt.UTC()
		s.UpdatedAt = s.UpdatedAt.UTC()
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []Software{}
	}
	return out, nil
}

// SoftwareByIDEnabled wraps SoftwareByID with the enabled-source gate.
func (r *Objects) SoftwareByIDEnabled(ctx context.Context, id string, enabledOnly bool) (Software, error) {
	s, err := r.SoftwareByID(ctx, id)
	if err != nil {
		return Software{}, err
	}
	if s.Version == StagingVersion {
		return Software{}, wrapObjErr(sql.ErrNoRows, "content_software", id)
	}
	if enabledOnly {
		if err := r.requireEnabledSource(ctx, s.SourceID); err != nil {
			return Software{}, err
		}
	}
	return s, nil
}

// SetTechniqueMitigations replaces mitigation membership for one technique.
func (r *Objects) SetTechniqueMitigations(ctx context.Context, sourceID, version, techniqueExternalID string, mitigationExternalIDs []string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		return setTechniqueMitigationsTx(ctx, tx, sourceID, version, techniqueExternalID, mitigationExternalIDs)
	})
}

// TechniqueMitigations lists mitigation external ids for one technique.
func (r *Objects) TechniqueMitigations(ctx context.Context, sourceID, version, techniqueExternalID string) ([]string, error) {
	rows, err := r.db.Read().QueryContext(ctx, `
		SELECT mitigation_external_id FROM content.content_technique_mitigation
		WHERE source_id = ? AND version = ? AND technique_external_id = ?
		ORDER BY mitigation_external_id`,
		sourceID, version, techniqueExternalID,
	)
	if err != nil {
		return nil, fmt.Errorf("content: list technique mitigations: %w", err)
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

// ClearVersion deletes every ATT&CK object row for (sourceID, version),
// including join tables. Used by adapters that need an explicit wipe; the
// attack adapter prefers stage-and-promote and only clears staging leftovers.
func (r *Objects) ClearVersion(ctx context.Context, sourceID, version string) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		return clearAttackVersionTx(ctx, tx, sourceID, version)
	})
}

func setTechniqueMitigationsTx(ctx context.Context, tx *sql.Tx, sourceID, version, techniqueExternalID string, mitigationExternalIDs []string) error {
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM content.content_technique_mitigation
		WHERE source_id = ? AND version = ? AND technique_external_id = ?`,
		sourceID, version, techniqueExternalID,
	); err != nil {
		return fmt.Errorf("content: clear technique mitigations: %w", err)
	}
	for _, m := range mitigationExternalIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO content.content_technique_mitigation
				(source_id, version, technique_external_id, mitigation_external_id)
			VALUES (?, ?, ?, ?)`,
			sourceID, version, techniqueExternalID, m,
		); err != nil {
			return fmt.Errorf("content: insert technique mitigation: %w", err)
		}
	}
	return nil
}

// AttackVersionDeletes is the ordered DELETE list for one (source, version)
// ATT&CK catalog. Exported so the attack adapter Apply path can share it inside
// a Writer transaction without opening a nested store.Write.
func AttackVersionDeletes(sourceID, version string) (stmts []string, args []any) {
	tables := []string{
		`DELETE FROM content.content_technique_tactic WHERE source_id = ? AND version = ?`,
		`DELETE FROM content.content_technique_mitigation WHERE source_id = ? AND version = ?`,
		`DELETE FROM content.content_tactic WHERE source_id = ? AND version = ?`,
		`DELETE FROM content.content_technique WHERE source_id = ? AND version = ?`,
		`DELETE FROM content.content_mitigation WHERE source_id = ? AND version = ?`,
		`DELETE FROM content.content_group WHERE source_id = ? AND version = ?`,
		`DELETE FROM content.content_software WHERE source_id = ? AND version = ?`,
		`DELETE FROM content.content_data_source WHERE source_id = ? AND version = ?`,
	}
	args = make([]any, 0, len(tables)*2)
	for range tables {
		args = append(args, sourceID, version)
	}
	return tables, args
}

func clearAttackVersionTx(ctx context.Context, tx *sql.Tx, sourceID, version string) error {
	stmts, _ := AttackVersionDeletes(sourceID, version)
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt, sourceID, version); err != nil {
			return fmt.Errorf("content: clear version %s: %w", version, err)
		}
	}
	return nil
}

// PromoteAttackVersion moves every ATT&CK row from fromVersion to toVersion
// inside tx, after deleting any existing toVersion rows. Both halves share one
// transaction so a failed re-sync never leaves a half-replaced catalog.
func PromoteAttackVersion(ctx context.Context, tx *sql.Tx, sourceID, fromVersion, toVersion string) error {
	if err := clearAttackVersionTx(ctx, tx, sourceID, toVersion); err != nil {
		return err
	}
	// Fixed statement list — table names are not caller input (gosec G201).
	moves := []struct {
		name string
		stmt string
	}{
		{"content_technique_tactic", `UPDATE content.content_technique_tactic SET version = ? WHERE source_id = ? AND version = ?`},
		{"content_technique_mitigation", `UPDATE content.content_technique_mitigation SET version = ? WHERE source_id = ? AND version = ?`},
		{"content_tactic", `UPDATE content.content_tactic SET version = ? WHERE source_id = ? AND version = ?`},
		{"content_technique", `UPDATE content.content_technique SET version = ? WHERE source_id = ? AND version = ?`},
		{"content_mitigation", `UPDATE content.content_mitigation SET version = ? WHERE source_id = ? AND version = ?`},
		{"content_group", `UPDATE content.content_group SET version = ? WHERE source_id = ? AND version = ?`},
		{"content_software", `UPDATE content.content_software SET version = ? WHERE source_id = ? AND version = ?`},
		{"content_data_source", `UPDATE content.content_data_source SET version = ? WHERE source_id = ? AND version = ?`},
	}
	for _, m := range moves {
		if _, err := tx.ExecContext(ctx, m.stmt, toVersion, sourceID, fromVersion); err != nil {
			return fmt.Errorf("content: promote %s: %w", m.name, err)
		}
	}
	return nil
}

func (r *Objects) requireEnabledSource(ctx context.Context, sourceID string) error {
	var enabled bool
	err := r.db.Read().QueryRowContext(ctx, `
		SELECT enabled FROM content.content_source WHERE id = ?`, sourceID,
	).Scan(&enabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return wrapObjErr(sql.ErrNoRows, "content_source", sourceID)
		}
		return fmt.Errorf("content: source enabled check: %w", err)
	}
	if !enabled {
		// Same as missing from the caller's POV: disabled catalogs are hidden.
		return wrapObjErr(sql.ErrNoRows, "content_object", sourceID)
	}
	return nil
}

func listLimit(n int) int {
	if n <= 0 {
		return 500
	}
	if n > 2000 {
		return 2000
	}
	return n
}

type namedScanner[T any] func(rows *sql.Rows) (T, error)

func listNamed[T any](ctx context.Context, r *Objects, f ObjectListFilter, table string, scan namedScanner[T]) ([]T, error) {
	var (
		b    strings.Builder
		args []any
	)
	b.WriteString(`
		SELECT o.id, o.source_id, o.version, o.external_id, o.name, o.description,
			o.created_at, o.updated_at
		FROM content.`)
	b.WriteString(table)
	b.WriteString(` o`)
	if f.EnabledOnly {
		b.WriteString(`
		INNER JOIN content.content_source s ON s.id = o.source_id AND s.enabled = TRUE`)
	}
	b.WriteString(` WHERE o.version <> ?`)
	args = append(args, StagingVersion)
	if f.Version != "" {
		b.WriteString(` AND o.version = ?`)
		args = append(args, f.Version)
	}
	if q := strings.TrimSpace(f.Q); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		b.WriteString(` AND (
			LOWER(o.external_id) LIKE ? OR
			LOWER(o.name) LIKE ? OR
			LOWER(o.description) LIKE ?
		)`)
		args = append(args, like, like, like)
	}
	b.WriteString(` ORDER BY o.external_id, o.id`)
	if lim := listLimit(f.Limit); lim > 0 {
		b.WriteString(` LIMIT ?`)
		args = append(args, lim)
	}
	rows, err := r.db.Read().QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("content: list %s: %w", table, err)
	}
	defer rows.Close()
	var out []T
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []T{}
	}
	return out, nil
}

func scanTactic(rows *sql.Rows) (Tactic, error) {
	var t Tactic
	if err := rows.Scan(&t.ID, &t.SourceID, &t.Version, &t.ExternalID, &t.Name, &t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return Tactic{}, err
	}
	t.CreatedAt = t.CreatedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	return t, nil
}

func scanMitigation(rows *sql.Rows) (Mitigation, error) {
	var m Mitigation
	if err := rows.Scan(&m.ID, &m.SourceID, &m.Version, &m.ExternalID, &m.Name, &m.Description, &m.CreatedAt, &m.UpdatedAt); err != nil {
		return Mitigation{}, err
	}
	m.CreatedAt = m.CreatedAt.UTC()
	m.UpdatedAt = m.UpdatedAt.UTC()
	return m, nil
}

func scanGroup(rows *sql.Rows) (Group, error) {
	var g Group
	if err := rows.Scan(&g.ID, &g.SourceID, &g.Version, &g.ExternalID, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return Group{}, err
	}
	g.CreatedAt = g.CreatedAt.UTC()
	g.UpdatedAt = g.UpdatedAt.UTC()
	return g, nil
}
