package archive

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/bryanster/blacklight/internal/version"
)

// WriteOptions is everything the archive writer needs to produce a complete
// engagement archive. All fields are required unless noted.
type WriteOptions struct {
	// Manifest fields
	EngagementID   string
	EngagementName string
	Client         string
	Mode           string
	AttackVersion  string
	BlindFiltered  bool

	// Data members
	Engagement EngagementArchive
	Analytics  json.RawMessage   // the frozen analytics.json blob, or nil
	Activity   []json.RawMessage // activity rows as raw JSON bytes, for JSONL
	Evidence   []EvidenceEntry

	// Evidence file opener — returns a reader for a blob identified by its
	// SHA-256 hex digest. Returns an error if the blob is missing.
	EvidenceOpener func(sha256hex string) (io.ReadCloser, error)

	// User display names used in activity rows
	ActivityActors map[string]string // actor_id -> display_name

	// Timestamp — if zero, defaults to time.Now().
	ExportedAt time.Time

	// Tool version — if empty, uses version.Get().Version.
	ToolVersion string
}

// WriteArchive writes a complete engagement archive ZIP to w. Evidence blobs
// are streamed from disk via EvidenceOpener and copied with io.Copy; the
// structured members are encoded as JSON and written directly into the ZIP
// so the whole archive is never buffered in memory.
//
// The manifest is written first so a reader can check formatVersion before
// parsing anything else.
func WriteArchive(w io.Writer, opts WriteOptions) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	exportedAt := opts.ExportedAt
	if exportedAt.IsZero() {
		exportedAt = time.Now().UTC().Truncate(time.Microsecond)
	}
	toolVersion := opts.ToolVersion
	if toolVersion == "" {
		toolVersion = version.Get().Version
	}

	// 1. manifest.json — write first so formatVersion is discoverable early.
	m := Manifest{
		FormatVersion:  FormatVersion,
		ExportedAt:     exportedAt.Format(time.RFC3339),
		ToolVersion:    toolVersion,
		EngagementID:   opts.EngagementID,
		EngagementName: opts.EngagementName,
		Client:         opts.Client,
		Mode:           opts.Mode,
		AttackVersion:  opts.AttackVersion,
		BlindFiltered:  opts.BlindFiltered,
	}
	if err := writeJSONMember(zw, "manifest.json", m); err != nil {
		return fmt.Errorf("archive: manifest: %w", err)
	}

	// 2. engagement.json — the workbook graph.
	if err := writeJSONMember(zw, "engagement.json", opts.Engagement); err != nil {
		return fmt.Errorf("archive: engagement: %w", err)
	}

	// 3. analytics.json — frozen rollups, if available.
	if opts.Analytics != nil {
		if err := writeRawJSONMember(zw, "analytics.json", opts.Analytics); err != nil {
			return fmt.Errorf("archive: analytics: %w", err)
		}
	}

	// 4. activity.jsonl — one JSON object per line, newest last (the order
	//    activity rows arrive in).
	if err := writeActivityJSONL(zw, opts.Activity, opts.ActivityActors); err != nil {
		return fmt.Errorf("archive: activity: %w", err)
	}

	// 5. evidence.json — evidence metadata.
	em := EvidenceManifest{Entries: opts.Evidence}
	if err := writeJSONMember(zw, "evidence.json", em); err != nil {
		return fmt.Errorf("archive: evidence manifest: %w", err)
	}

	// 6. evidence/<sha256> — blob files. Dedup: a sha256 seen once is written
	//    once.
	written := make(map[string]bool)
	for _, e := range opts.Evidence {
		if e.BlobSHA256 == "" {
			continue
		}
		if written[e.BlobSHA256] {
			continue
		}
		if err := writeEvidenceBlob(zw, e.BlobSHA256, opts.EvidenceOpener); err != nil {
			return fmt.Errorf("archive: evidence %s: %w", e.BlobSHA256, err)
		}
		written[e.BlobSHA256] = true
	}

	return zw.Close()
}

// writeJSONMember adds a single JSON file to the archive.
func writeJSONMember(zw *zip.Writer, name string, v any) error {
	h := &zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: time.Now().UTC(),
	}
	h.SetMode(0644)
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// writeRawJSONMember adds pre-serialised JSON bytes as a file.
func writeRawJSONMember(zw *zip.Writer, name string, raw json.RawMessage) error {
	h := &zip.FileHeader{
		Name:     name,
		Method:   zip.Deflate,
		Modified: time.Now().UTC(),
	}
	h.SetMode(0644)
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}
	_, err = w.Write(raw)
	if err != nil {
		return err
	}
	// Ensure trailing newline for readability
	_, err = w.Write([]byte("\n"))
	return err
}

// writeActivityJSONL writes activity rows as newline-delimited JSON. Each row
// gets actor display-name enrichment inline.
//
// The archive does NOT store raw activity rows — it stores the same fields
// but with user display names resolved and ActorID/ActorName instead of the
// raw actor_id.
func writeActivityJSONL(zw *zip.Writer, rows []json.RawMessage, actors map[string]string) error {
	h := &zip.FileHeader{
		Name:     "activity.jsonl",
		Method:   zip.Deflate,
		Modified: time.Now().UTC(),
	}
	h.SetMode(0644)
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}

	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)

	for _, row := range rows {
		// Enrich with actor display name if present in the map.
		// We parse, add actorName, and re-encode.
		var m map[string]any
		if err := json.Unmarshal(row, &m); err != nil {
			// If we can't parse, write the raw row as-is.
			_, _ = w.Write(row)          //nolint:errcheck
			_, _ = w.Write([]byte("\n")) //nolint:errcheck
			continue
		}

		// Add actorName field.
		if actorID, ok := m["actorId"].(string); ok {
			if name, ok := actors[actorID]; ok {
				m["actorName"] = name
			}
		}

		if err := enc.Encode(m); err != nil {
			return err
		}
	}
	return nil
}

// writeEvidenceBlob copies a blob from disk into the archive.
func writeEvidenceBlob(zw *zip.Writer, sha256hex string, opener func(string) (io.ReadCloser, error)) error {
	rc, err := opener(sha256hex)
	if err != nil {
		return fmt.Errorf("open blob %s: %w", sha256hex, err)
	}
	defer rc.Close()

	h := &zip.FileHeader{
		Name:     "evidence/" + sha256hex,
		Method:   zip.Store, // content is already compressed or not; don't re-compress
		Modified: time.Now().UTC(),
	}
	h.SetMode(0644)
	w, err := zw.CreateHeader(h)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, rc)
	return err
}
