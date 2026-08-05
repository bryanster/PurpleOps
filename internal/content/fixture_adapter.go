package content

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// FixtureAdapter is a deterministic adapter used by runner tests and as the
// stand-in until concrete upstream adapters land (M2-006+). It is safe to
// register against any kind in tests; production wiring does not install it.
//
// Bundle format (JSON):
//
//	{"version":"current","notes":[{"externalId":"n1","title":"…","body":"…"}]}
//
// Apply writes notes via Writer batches and respects ctx cancellation between
// batches. DelayBatch, when > 0, sleeps before each batch so cancel tests can
// land mid-Apply.
type FixtureAdapter struct {
	kind storecontent.Kind

	// FetchBytes, when set, is returned by Fetch instead of hitting the network.
	// Tests use this to avoid HTTP entirely.
	FetchBytes []byte

	// FetchErr, when set, makes Fetch fail immediately.
	FetchErr error

	// DelayBatch is slept at the start of every Apply batch (after the ctx
	// check). Zero means no delay.
	DelayBatch time.Duration

	// batchesApplied counts completed Write calls — tests assert cancel stops
	// further batches.
	batchesApplied atomic.Int64
}

// NewFixtureAdapter returns a fixture bound to kind.
func NewFixtureAdapter(kind storecontent.Kind) *FixtureAdapter {
	return &FixtureAdapter{kind: kind}
}

// Kind implements [Adapter].
func (a *FixtureAdapter) Kind() storecontent.Kind { return a.kind }

// BatchesApplied reports how many Apply Write transactions finished.
func (a *FixtureAdapter) BatchesApplied() int64 { return a.batchesApplied.Load() }

// Fetch implements [Adapter]. Prefer FetchBytes; otherwise GET Source.URL.
func (a *FixtureAdapter) Fetch(ctx context.Context, req FetchRequest) (Bundle, error) {
	if a.FetchErr != nil {
		return Bundle{}, a.FetchErr
	}
	if len(a.FetchBytes) > 0 {
		version := req.Version
		if version == "" {
			version = peekVersion(a.FetchBytes)
		}
		if version == "" {
			version = storecontent.VersionCurrent
		}
		return Bundle{
			Bytes:   append([]byte(nil), a.FetchBytes...),
			Size:    int64(len(a.FetchBytes)),
			Version: version,
		}, nil
	}
	if req.Source.URL == "" {
		return Bundle{}, fmt.Errorf("fixture adapter: no FetchBytes and source URL is empty")
	}
	src := HTTPSource{
		URL:      req.Source.URL,
		MaxBytes: req.MaxBytes,
		Client:   req.HTTP,
	}
	raw, err := ReadAll(ctx, src)
	if err != nil {
		return Bundle{}, err
	}
	version := req.Version
	if version == "" {
		version = peekVersion(raw)
	}
	if version == "" {
		version = storecontent.VersionCurrent
	}
	return Bundle{Bytes: raw, Size: int64(len(raw)), Version: version}, nil
}

// Parse implements [Adapter].
func (a *FixtureAdapter) Parse(ctx context.Context, bundle Bundle) (AST, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	raw := bundle.Bytes
	if len(raw) == 0 && bundle.Path != "" {
		var err error
		raw, err = ReadAll(ctx, FileSource{Path: bundle.Path})
		if err != nil {
			return nil, err
		}
	}
	var doc fixtureDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("fixture adapter: parse: %w", err)
	}
	if doc.Version == "" {
		doc.Version = bundle.Version
	}
	if doc.Version == "" {
		doc.Version = storecontent.VersionCurrent
	}
	return doc, nil
}

// Normalize implements [Adapter].
func (a *FixtureAdapter) Normalize(ctx context.Context, ast AST) ([]Object, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	doc, ok := ast.(fixtureDoc)
	if !ok {
		return nil, fmt.Errorf("fixture adapter: unexpected AST type %T", ast)
	}
	out := make([]Object, 0, len(doc.Notes))
	for _, n := range doc.Notes {
		out = append(out, fixtureNote{
			ExternalID: n.ExternalID,
			Title:      n.Title,
			Body:       n.Body,
			Version:    doc.Version,
		})
	}
	return out, nil
}

// Apply implements [Adapter]. Rolling replace: delete existing notes for the
// version in the first batch transaction, then insert in WriteBatch-sized
// chunks. No half-visible catalog across a single batch transaction.
func (a *FixtureAdapter) Apply(ctx context.Context, w Writer, objects []Object, prog Progress) error {
	total := int64(len(objects))
	if prog != nil {
		prog.Report(ctx, PhaseApply, 0, total, "applying fixture notes")
	}

	batch := w.BatchSize()
	if batch <= 0 {
		batch = 250
	}

	// First write: clear prior rows for this version so a re-sync replaces
	// atomically from the caller's POV within each subsequent batch family.
	if err := w.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `
			DELETE FROM content.content_note
			WHERE source_id = ? AND version = ?`,
			w.SourceID(), w.Version(),
		)
		return err
	}); err != nil {
		return err
	}
	a.batchesApplied.Add(1)

	for i := 0; i < len(objects); i += batch {
		if err := ctx.Err(); err != nil {
			return err
		}
		if a.DelayBatch > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(a.DelayBatch):
			}
		}
		end := i + batch
		if end > len(objects) {
			end = len(objects)
		}
		chunk := objects[i:end]
		if err := w.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
			for _, obj := range chunk {
				n, ok := obj.(fixtureNote)
				if !ok {
					return fmt.Errorf("fixture adapter: unexpected object type %T", obj)
				}
				if err := insertFixtureNote(ctx, tx, w.SourceID(), w.Version(), n); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		a.batchesApplied.Add(1)
		if prog != nil {
			prog.Report(ctx, PhaseApply, int64(end), total, fmt.Sprintf("wrote %d/%d", end, total))
		}
	}
	return nil
}

type fixtureDoc struct {
	Version string            `json:"version"`
	Notes   []fixtureNoteJSON `json:"notes"`
}

type fixtureNoteJSON struct {
	ExternalID string `json:"externalId"`
	Title      string `json:"title"`
	Body       string `json:"body"`
}

type fixtureNote struct {
	ExternalID string
	Title      string
	Body       string
	Version    string
}

func insertFixtureNote(ctx context.Context, tx *sql.Tx, sourceID, version string, n fixtureNote) error {
	id, err := newUUIDv7()
	if err != nil {
		return err
	}
	tags := []byte("[]")
	_, err = tx.ExecContext(ctx, `
		INSERT INTO content.content_note (
			id, source_id, version, external_id, title, body_markdown,
			tags, technique_external_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		id, sourceID, version, n.ExternalID, n.Title, n.Body, tags,
	)
	if err != nil {
		return fmt.Errorf("fixture adapter: insert note %q: %w", n.ExternalID, err)
	}
	return nil
}

func peekVersion(raw []byte) string {
	var doc struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}
	return strings.TrimSpace(doc.Version)
}

// FixtureBundle builds the JSON payload the fixture adapter expects.
func FixtureBundle(version string, notes []FixtureNote) []byte {
	doc := fixtureDoc{Version: version, Notes: make([]fixtureNoteJSON, 0, len(notes))}
	for _, n := range notes {
		doc.Notes = append(doc.Notes, fixtureNoteJSON(n))
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		// Deterministic input; a marshal failure is a programming error.
		panic("content: fixture bundle: " + err.Error())
	}
	return raw
}

// FixtureNote is one note in a [FixtureBundle].
type FixtureNote struct {
	ExternalID string
	Title      string
	Body       string
}
