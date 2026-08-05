package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Note is one freeform KB note (custom / imported).
type Note struct {
	ID                  string
	SourceID            string
	Version             string
	ExternalID          string
	Title               string
	BodyMarkdown        string
	Tags                json.RawMessage // JSON array of strings
	TechniqueExternalID string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// Notes reads and writes content notes. Construct with [NewNotes].
type Notes struct {
	db DB
}

// NewNotes returns a repository over db.
func NewNotes(db DB) *Notes { return &Notes{db: db} }

const noteColumns = `id, source_id, version, external_id, title, body_markdown,
	tags, technique_external_id, created_at, updated_at`

// Create inserts one note.
//
// after runs inside the same write transaction after the insert so activity
// (M2-011) shares the commit.
func (r *Notes) Create(ctx context.Context, in Note, after ...After) (Note, error) {
	id, err := assignID(in.ID)
	if err != nil {
		return Note{}, err
	}
	ts := now()
	err = r.db.Write(ctx, func(tx *sql.Tx) error {
		if err := requireSource(ctx, tx, in.SourceID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO content.content_note (
				id, source_id, version, external_id, title, body_markdown,
				tags, technique_external_id, created_at, updated_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, in.SourceID, in.Version, in.ExternalID, in.Title, in.BodyMarkdown,
			bindJSON(in.Tags), in.TechniqueExternalID, ts, ts,
		)
		if err := uniqueOr(err, "note", in.SourceID, in.Version, in.ExternalID); err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, id), tx, after)
	})
	if err != nil {
		return Note{}, err
	}
	return r.ByID(ctx, id)
}

// Update rewrites mutable fields of an existing note.
//
// after runs inside the same write transaction after the update.
func (r *Notes) Update(ctx context.Context, in Note, after ...After) (Note, error) {
	ts := now()
	err := r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `
			UPDATE content.content_note SET
				title = ?, body_markdown = ?, tags = ?, technique_external_id = ?,
				updated_at = ?
			WHERE id = ?`,
			in.Title, in.BodyMarkdown, bindJSON(in.Tags), in.TechniqueExternalID,
			ts, in.ID,
		)
		if err != nil {
			return fmt.Errorf("content: update note %s: %w", in.ID, err)
		}
		if err := requireOneRow(res, "content_note", in.ID); err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, in.ID), tx, after)
	})
	if err != nil {
		return Note{}, err
	}
	return r.ByID(ctx, in.ID)
}

// Delete removes one note by id.
//
// after runs inside the same write transaction after the delete.
func (r *Notes) Delete(ctx context.Context, id string, after ...After) error {
	return r.db.Write(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM content.content_note WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("content: delete note %s: %w", id, err)
		}
		if err := requireOneRow(res, "content_note", id); err != nil {
			return err
		}
		return runAfter(WithAfterEntity(ctx, id), tx, after)
	})
}

// ByID returns one note or [apierr.NotFound].
func (r *Notes) ByID(ctx context.Context, id string) (Note, error) {
	row := r.db.Read().QueryRowContext(ctx,
		`SELECT `+noteColumns+` FROM content.content_note WHERE id = ?`, id)
	n, err := scanNote(row)
	if err != nil {
		return Note{}, wrapObjErr(err, "content_note", id)
	}
	return n, nil
}

// NoteListFilter narrows note listings.
//
// EnabledOnly joins content_source.enabled. Version empty means every
// non-staging version. Q is a case-insensitive substring over external_id,
// title, and body_markdown. Technique is an exact match on
// technique_external_id.
type NoteListFilter struct {
	SourceID    string
	Version     string
	Q           string
	Technique   string
	EnabledOnly bool
	Limit       int
}

// List returns notes matching f, ordered by external_id then id.
func (r *Notes) List(ctx context.Context, f NoteListFilter) ([]Note, error) {
	var (
		b    strings.Builder
		args []any
	)
	b.WriteString(`
		SELECT n.id, n.source_id, n.version, n.external_id, n.title, n.body_markdown,
			n.tags, n.technique_external_id, n.created_at, n.updated_at
		FROM content.content_note n`)
	if f.EnabledOnly {
		b.WriteString(`
		INNER JOIN content.content_source s ON s.id = n.source_id AND s.enabled = TRUE`)
	}
	b.WriteString(` WHERE n.version <> ?`)
	args = append(args, StagingVersion)
	if f.SourceID != "" {
		b.WriteString(` AND n.source_id = ?`)
		args = append(args, f.SourceID)
	}
	if f.Version != "" {
		b.WriteString(` AND n.version = ?`)
		args = append(args, f.Version)
	}
	if f.Q != "" {
		like := "%" + strings.ToLower(f.Q) + "%"
		b.WriteString(` AND (
			LOWER(n.external_id) LIKE ? OR
			LOWER(n.title) LIKE ? OR
			LOWER(n.body_markdown) LIKE ?
		)`)
		args = append(args, like, like, like)
	}
	if f.Technique != "" {
		b.WriteString(` AND n.technique_external_id = ?`)
		args = append(args, f.Technique)
	}
	b.WriteString(` ORDER BY n.external_id ASC, n.id ASC`)
	if f.Limit > 0 {
		b.WriteString(` LIMIT ?`)
		args = append(args, f.Limit)
	}

	rows, err := r.db.Read().QueryContext(ctx, b.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("content: list notes: %w", err)
	}
	defer rows.Close()

	out := make([]Note, 0)
	for rows.Next() {
		n, err := scanNote(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("content: list notes: %w", err)
	}
	return out, nil
}

func scanNote(row interface{ Scan(...any) error }) (Note, error) {
	var (
		n    Note
		tags any
	)
	err := row.Scan(
		&n.ID, &n.SourceID, &n.Version, &n.ExternalID, &n.Title, &n.BodyMarkdown,
		&tags, &n.TechniqueExternalID, &n.CreatedAt, &n.UpdatedAt,
	)
	if err != nil {
		return Note{}, err
	}
	if n.Tags, err = jsonBytes(tags); err != nil {
		return Note{}, fmt.Errorf("content: note tags: %w", err)
	}
	n.CreatedAt = n.CreatedAt.UTC()
	n.UpdatedAt = n.UpdatedAt.UTC()
	return n, nil
}
