package evidence

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/bryanster/blacklight/internal/config"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
)

// BlobRootName is the subdirectory under the evidence data root that holds
// content-addressed blob files, organised as {sha256[0:2]}/{sha256}.
const BlobRootName = "blobs"

// Store is the content-addressed evidence blob store. It is constructed over
// the configured evidence directory and the database-backed blob repository.
type Store struct {
	root string
	cfg  config.Evidence
	db   blobDB
}

// blobDB is the part of [storengagement.EvidenceBlobRepo] the blob store needs.
type blobDB interface {
	GetBlob(ctx context.Context, sha256 string) (storengagement.EvidenceBlob, error)
	InsertBlob(ctx context.Context, sha256, mime, storagePath string, size int64) error
	IncrementRef(ctx context.Context, sha256 string) error
	DecrementRef(ctx context.Context, sha256 string) (gc bool, _ error)
	EngagementBlobBytes(ctx context.Context, engagementID string) (int64, error)
	DeleteBlob(ctx context.Context, sha256 string) error
}

// NewStore returns a blob store over dir and the configured policy.
func NewStore(dir string, cfg config.Evidence, db blobDB) *Store {
	return &Store{root: dir, cfg: cfg, db: db}
}

// blobPath returns the stable disk path for a SHA-256 hex digest.
// Layout: {root}/blobs/{sha256[0:2]}/{sha256}
func (s *Store) blobPath(sha256hex string) string {
	if len(sha256hex) < 2 {
		return filepath.Join(s.root, BlobRootName, sha256hex)
	}
	return filepath.Join(s.root, BlobRootName, sha256hex[:2], sha256hex)
}

// Put streams an upload into the content-addressed store. It writes through a
// SHA-256 hasher to a temp file, then atomically renames into the blob tree.
// If a blob with the same hash already exists the temp file is removed, the
// ref_count is incremented, and (sha256, true, nil) is returned.
//
// sizeHint is the caller's expected size for quota accounting; the actual bytes
// written are what counts for the blob row.
//
// The caller must check MIME against the configured allowlist before calling Put.
func (s *Store) Put(ctx context.Context, src io.Reader, mime, engagementID string) (sha256hex string, existed bool, size int64, err error) {
	if err := ctx.Err(); err != nil {
		return "", false, 0, err
	}
	if src == nil {
		return "", false, 0, fmt.Errorf("evidence: put: src is nil")
	}

	// Stream to temp, hash as we go.
	dir := filepath.Join(s.root, BlobRootName)
	// Pre-create the blobs root so intermediate subdirectories are easier.
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", false, 0, fmt.Errorf("evidence: put: mkdir blobs: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return "", false, 0, fmt.Errorf("evidence: put: id: %w", err)
	}
	tmp := filepath.Join(dir, id.String()+".part")

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return "", false, 0, fmt.Errorf("evidence: put: create temp: %w", err)
	}
	cleanupTmp := true
	defer func() {
		_ = f.Close()
		if cleanupTmp {
			_ = os.Remove(tmp)
		}
	}()

	sum := sha256.New()
	limit := s.cfg.MaxUploadBytes.Int64()
	limited := io.LimitReader(src, limit+1)
	written, err := io.Copy(io.MultiWriter(f, sum), limited)
	if err != nil {
		return "", false, 0, fmt.Errorf("evidence: put: write: %w", err)
	}
	if written > limit {
		return "", false, 0, &ErrTooLarge{Limit: limit, Got: written}
	}
	if written == 0 {
		return "", false, 0, fmt.Errorf("evidence: put: empty file")
	}
	if err := f.Close(); err != nil {
		return "", false, 0, fmt.Errorf("evidence: put: close: %w", err)
	}

	digest := hex.EncodeToString(sum.Sum(nil))
	final := s.blobPath(digest)

	// Check whether this blob already exists on disk.
	_, statErr := os.Stat(final)
	if statErr == nil {
		// Blob exists on disk. Bump refcount, delete temp.
		cleanupTmp = true // remove temp
		if err := s.db.IncrementRef(ctx, digest); err != nil {
			return "", false, 0, fmt.Errorf("evidence: put: increment ref %q: %w", digest, err)
		}
		return digest, true, written, nil
	}
	if !os.IsNotExist(statErr) {
		return "", false, 0, fmt.Errorf("evidence: put: stat %q: %w", final, statErr)
	}

	// Quota check — unique blob bytes per engagement.
	if limit := s.cfg.MaxEngagementBytes.Int64(); limit > 0 {
		used, err := s.db.EngagementBlobBytes(ctx, engagementID)
		if err != nil {
			return "", false, 0, fmt.Errorf("evidence: put: quota check: %w", err)
		}
		if used+written > limit {
			return "", false, 0, &ErrEngagementQuota{Limit: limit, Used: used, Got: written}
		}
	}

	// New blob — create its directory and rename.
	blobDir := filepath.Dir(final)
	if err := os.MkdirAll(blobDir, 0o750); err != nil {
		return "", false, 0, fmt.Errorf("evidence: put: mkdir %q: %w", blobDir, err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return "", false, 0, fmt.Errorf("evidence: put: rename: %w", err)
	}
	cleanupTmp = false

	// Insert the blob row with ref_count=1.
	relPath := filepath.Join(BlobRootName, digest[:2], digest)
	if err := s.db.InsertBlob(ctx, digest, mime, relPath, written); err != nil {
		// Best-effort cleanup; the row failed but the file is on disk.
		_ = os.Remove(final)
		return "", false, 0, fmt.Errorf("evidence: put: insert blob: %w", err)
	}
	return digest, false, written, nil
}

// Open returns a reader over the blob identified by its SHA-256 hex digest.
// It returns an error if the blob does not exist on disk.
func (s *Store) Open(sha256hex string) (io.ReadCloser, error) {
	if strings.Contains(sha256hex, "..") || strings.Contains(sha256hex, "/") {
		return nil, fmt.Errorf("evidence: open: invalid sha256 %q", sha256hex)
	}
	path := s.blobPath(sha256hex)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("evidence: blob %q not found", sha256hex)
		}
		return nil, fmt.Errorf("evidence: open %q: %w", sha256hex, err)
	}
	return f, nil
}

// BlobRoot returns the absolute path to the blobs directory.
func (s *Store) BlobRoot() string {
	return filepath.Join(s.root, BlobRootName)
}

// RemoveBlobFile deletes the blob file from disk. It is called after
// [storengagement.EvidenceBlobRepo.DecrementRef] reports gc=true.
func (s *Store) RemoveBlobFile(sha256hex string) error {
	if strings.Contains(sha256hex, "..") || strings.Contains(sha256hex, "/") {
		return fmt.Errorf("evidence: remove: invalid sha256 %q", sha256hex)
	}
	path := s.blobPath(sha256hex)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("evidence: remove %q: %w", sha256hex, err)
	}
	return nil
}

// ErrTooLarge is returned when a file exceeds the configured per-file limit.
type ErrTooLarge struct {
	Limit int64
	Got   int64
}

func (e *ErrTooLarge) Error() string {
	return fmt.Sprintf("evidence: file too large: %d bytes, limit %d bytes", e.Got, e.Limit)
}

// ErrEngagementQuota is returned when an upload would exceed the per-engagement
// quota.
type ErrEngagementQuota struct {
	Limit int64
	Used  int64
	Got   int64
}

func (e *ErrEngagementQuota) Error() string {
	return fmt.Sprintf("evidence: engagement quota exceeded: %d bytes used, would add %d bytes, limit %d bytes",
		e.Used, e.Got, e.Limit)
}
