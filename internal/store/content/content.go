package content

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store"
)

// DB is the part of the store these repositories need: pooled reads, and writes
// serialized into one transaction at a time. [store.DB] satisfies it.
//
// It is declared here, in the package that consumes it, rather than exported by
// the store — so a test can substitute one, and so this package's dependency is
// the two methods it calls rather than everything a database happens to offer.
type DB interface {
	Read() store.Reader
	Write(ctx context.Context, fn func(tx *sql.Tx) error) error
}

// VersionCurrent is the single version token used by rolling sources (Atomic,
// Sigma, CTID, and custom). A re-sync replaces objects for this token inside
// one transaction. ATT&CK never uses it; ATT&CK versions are release labels
// such as "15.1".
const VersionCurrent = "current"

// Kind is a content_source.kind value. The vocabulary is closed: new kinds are
// a migration, not a string somebody passed.
type Kind string

const (
	KindAttack Kind = "attack"
	KindAtomic Kind = "atomic"
	KindSigma  Kind = "sigma"
	KindCTID   Kind = "ctid"
	KindCustom Kind = "custom"
)

// SourceStatus is the operational state of a registry row.
type SourceStatus string

const (
	SourceStatusIdle    SourceStatus = "idle"
	SourceStatusSyncing SourceStatus = "syncing"
	SourceStatusError   SourceStatus = "error"
)

// VersionStatus is the state of one version snapshot under a source.
type VersionStatus string

const (
	VersionStatusPending VersionStatus = "pending"
	VersionStatusReady   VersionStatus = "ready"
	VersionStatusError   VersionStatus = "error"
)

// JobKind is what a content_sync_job is doing.
type JobKind string

const (
	JobKindSync         JobKind = "sync"
	JobKindReprocess    JobKind = "reprocess"
	JobKindBundleImport JobKind = "bundle_import"
	JobKindV1Import     JobKind = "v1_import"
)

// JobStatus is the lifecycle state of a content_sync_job.
type JobStatus string

const (
	JobStatusQueued      JobStatus = "queued"
	JobStatusRunning     JobStatus = "running"
	JobStatusCancelling  JobStatus = "cancelling"
	JobStatusCancelled   JobStatus = "cancelled"
	JobStatusSucceeded   JobStatus = "succeeded"
	JobStatusFailed      JobStatus = "failed"
	JobStatusInterrupted JobStatus = "interrupted"
)

// SoftwareType distinguishes malware from tool on content_software.
type SoftwareType string

const (
	SoftwareMalware SoftwareType = "malware"
	SoftwareTool    SoftwareType = "tool"
)

// Builtin source identifiers — stable across installs, seeded by 0011_content.
const (
	SourceIDAttack = "01900000-0000-7000-8000-000000000001"
	SourceIDAtomic = "01900000-0000-7000-8000-000000000002"
	SourceIDSigma  = "01900000-0000-7000-8000-000000000003"
	SourceIDCTID   = "01900000-0000-7000-8000-000000000004"
	SourceIDCustom = "01900000-0000-7000-8000-000000000005"
)

// newID mints a UUIDv7: sortable by creation time, so ORDER BY id is a stable
// tiebreaker and rows arrive in the order they were made.
func newID() (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("content: generating an identifier: %w", err)
	}
	return id.String(), nil
}

// now is the timestamp written to a row: UTC, truncated to the microsecond
// DuckDB's TIMESTAMP stores.
func now() time.Time { return toStorage(time.Now()) }

// toStorage prepares a caller's timestamp the same way the store does.
func toStorage(t time.Time) time.Time { return t.UTC().Truncate(time.Microsecond) }

// nullString maps Go's "" to SQL NULL for nullable text columns.
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// fromNullString maps SQL NULL to "".
func fromNullString(s sql.NullString) string {
	if !s.Valid {
		return ""
	}
	return s.String
}

// fromNullTime maps SQL NULL to the zero time.
func fromNullTime(t sql.NullTime) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time.UTC()
}

// requireOneRow turns "the statement matched nothing" into [apierr.NotFound].
func requireOneRow(result sql.Result, resource, id string) error {
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("content: rows affected for %s %s: %w", resource, id, err)
	}
	if n == 0 {
		return apierr.NotFound(resource, id)
	}
	if n != 1 {
		return fmt.Errorf("content: %s %s: expected 1 row, got %d", resource, id, n)
	}
	return nil
}

// requireSource reports [apierr.NotFound] unless the source exists. It runs
// inside the caller's write transaction so the answer cannot change between
// the check and the insert — the substitute for a foreign key that DuckDB
// cannot host on an updatable parent (migration 0011_content.sql).
func requireSource(ctx context.Context, tx *sql.Tx, sourceID string) error {
	var one int
	err := tx.QueryRowContext(ctx,
		`SELECT 1 FROM content.content_source WHERE id = ?`, sourceID,
	).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return apierr.NotFound("content_source", sourceID)
	}
	if err != nil {
		return fmt.Errorf("content: lookup source %s: %w", sourceID, err)
	}
	return nil
}
