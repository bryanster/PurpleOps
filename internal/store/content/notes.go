package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
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
func (r *Notes) Create(ctx context.Context, in Note) (Note, error) {
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
		return uniqueOr(err, "note", in.SourceID, in.Version, in.ExternalID)
	})
	if err != nil {
		return Note{}, err
	}
	return r.ByID(ctx, id)
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
