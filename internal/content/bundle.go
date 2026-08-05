package content

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// UploadRoot is the directory under the content data root that holds spooled
// offline bundle uploads until the job finishes and they are cleaned up.
const UploadRoot = "uploads"

// StartBundleImportRequest is the caller half of an offline bundle_import job.
type StartBundleImportRequest struct {
	SourceID string
	// Version is optional. Same semantics as [StartSyncRequest.Version].
	Version string
	// BundlePath is an absolute path under the content data root. Typically the
	// return value of [Runner.SpoolUpload].
	BundlePath string
	// BundleSHA256 is the hex digest of BundlePath. Empty recomputes at apply.
	BundleSHA256 string
}

// StartReprocessRequest is the caller half of a reprocess-from-raw job.
type StartReprocessRequest struct {
	SourceID string
	// Version selects which raw snapshot. Empty means "current" for rolling
	// sources; ATT&CK requires an explicit pin.
	Version string
}

// MaxBytes returns the configured upload / fetch ceiling in bytes.
func (r *Runner) MaxBytes() int64 { return r.maxBytes }

// Paths returns the content data-root helper the runner was built with.
func (r *Runner) Paths() storecontent.Paths { return r.paths }

// SpoolUpload streams src into a new file under {contentDir}/uploads/,
// enforcing the configured max-bytes ceiling. It returns the absolute path,
// hex SHA-256, and byte size. On any error no file is left behind.
//
// filename is retained only for diagnostics (extension hints); it is not used
// as the on-disk name. Callers that hold untrusted input must use this rather
// than writing the bytes themselves.
func (r *Runner) SpoolUpload(ctx context.Context, src io.Reader, filename string) (absPath, shaHex string, size int64, err error) {
	if err := ctx.Err(); err != nil {
		return "", "", 0, err
	}
	if src == nil {
		return "", "", 0, apierr.Validation(apierr.Field("file", "is required"))
	}

	root := r.paths.Root()
	if root == "" {
		return "", "", 0, errors.New("content: spool: content data root is empty")
	}
	dir := filepath.Join(root, UploadRoot)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", "", 0, fmt.Errorf("content: spool: mkdir uploads: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return "", "", 0, fmt.Errorf("content: spool: id: %w", err)
	}
	// Keep a short, path-safe hint from the original name for operators grepping
	// the uploads dir; the UUID is the real identity.
	hint := sanitizeUploadHint(filename)
	base := id.String()
	if hint != "" {
		base = id.String() + "-" + hint
	}
	tmp := filepath.Join(dir, base+".part")
	final := filepath.Join(dir, base)

	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return "", "", 0, fmt.Errorf("content: spool: create: %w", err)
	}
	// Always remove the temp on the error path; rename succeeds → final stays.
	cleanupTmp := true
	defer func() {
		_ = f.Close()
		if cleanupTmp {
			_ = os.Remove(tmp)
		}
	}()

	sum := sha256.New()
	// Read one past the limit so we can distinguish "exactly at limit" from
	// "over". LimitReader stops after maxBytes+1 without error.
	limited := io.LimitReader(src, r.maxBytes+1)
	written, err := io.Copy(io.MultiWriter(f, sum), limited)
	if err != nil {
		return "", "", 0, fmt.Errorf("content: spool: write: %w", err)
	}
	if written > r.maxBytes {
		return "", "", 0, &ErrTooLarge{Limit: r.maxBytes, Got: written}
	}
	if written == 0 {
		return "", "", 0, apierr.Validation(apierr.Field("file", "must not be empty"))
	}
	if err := f.Close(); err != nil {
		return "", "", 0, fmt.Errorf("content: spool: close: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		return "", "", 0, fmt.Errorf("content: spool: rename: %w", err)
	}
	cleanupTmp = false
	return final, hex.EncodeToString(sum.Sum(nil)), written, nil
}

// StartBundleImport enqueues a bundle_import job over a pre-spooled archive.
// The file at BundlePath is removed when the job reaches any terminal status.
// If enqueue fails the file is removed immediately.
func (r *Runner) StartBundleImport(ctx context.Context, actor authn.Subject, req StartBundleImportRequest) (storecontent.Job, error) {
	if req.BundlePath == "" {
		return storecontent.Job{}, apierr.Validation(apierr.Field("file", "is required"))
	}
	if err := r.requirePathUnderRoot(req.BundlePath); err != nil {
		// Untrusted path — drop the file if we created it under uploads.
		r.removeUpload(req.BundlePath)
		return storecontent.Job{}, err
	}
	job, err := r.StartSync(ctx, actor, StartSyncRequest{
		SourceID:      req.SourceID,
		Version:       req.Version,
		Kind:          storecontent.JobKindBundleImport,
		BundlePath:    req.BundlePath,
		BundleSHA256:  req.BundleSHA256,
		CleanupUpload: true,
	})
	if err != nil {
		r.removeUpload(req.BundlePath)
		return storecontent.Job{}, err
	}
	return job, nil
}

// StartReprocess enqueues a reprocess job that re-parses the last successful
// raw snapshot for the source/version. No network I/O. Answers 409 when no
// raw snapshot exists.
func (r *Runner) StartReprocess(ctx context.Context, actor authn.Subject, req StartReprocessRequest) (storecontent.Job, error) {
	if req.SourceID == "" {
		return storecontent.Job{}, apierr.Validation(apierr.Field("sourceId", "is required"))
	}
	src, err := r.sources.ByID(ctx, req.SourceID)
	if err != nil {
		return storecontent.Job{}, err
	}

	version := strings.TrimSpace(req.Version)
	if version == "" {
		if src.Kind == storecontent.KindAttack {
			return storecontent.Job{}, apierr.Validation(apierr.Field(
				"version", "is required to reprocess an ATT&CK source",
			))
		}
		version = storecontent.VersionCurrent
	}

	ver, err := r.versions.BySourceVersion(ctx, src.ID, version)
	if err != nil {
		if errors.Is(err, apierr.ErrNotFound) {
			return storecontent.Job{}, apierr.Conflict(
				fmt.Sprintf("no raw snapshot exists for source %s version %q; sync or upload a bundle first",
					src.ID, version))
		}
		return storecontent.Job{}, err
	}
	if ver.RawPath == "" || ver.RawSHA256 == "" {
		return storecontent.Job{}, apierr.Conflict(
			fmt.Sprintf("no raw snapshot exists for source %s version %q; sync or upload a bundle first",
				src.ID, version))
	}
	abs, err := r.paths.Abs(ver.RawPath)
	if err != nil {
		return storecontent.Job{}, apierr.Conflict(
			fmt.Sprintf("raw snapshot path for source %s version %q is invalid: %v", src.ID, version, err))
	}
	if _, err := os.Stat(abs); err != nil {
		if os.IsNotExist(err) {
			return storecontent.Job{}, apierr.Conflict(
				fmt.Sprintf("raw snapshot file for source %s version %q is missing on disk; sync or upload a bundle first",
					src.ID, version))
		}
		return storecontent.Job{}, fmt.Errorf("content: stat raw snapshot: %w", err)
	}

	return r.StartSync(ctx, actor, StartSyncRequest{
		SourceID:     src.ID,
		Version:      version,
		Kind:         storecontent.JobKindReprocess,
		BundlePath:   abs,
		BundleSHA256: ver.RawSHA256,
	})
}

// ReadBundleMultipart walks a multipart body looking for the `file` part (and
// optional `version` text part), spooling the file under the content data root.
// Unknown parts are skipped. Missing `file` is a validation error.
func (r *Runner) ReadBundleMultipart(ctx context.Context, mr *multipart.Reader) (path, sha, version string, size int64, err error) {
	if mr == nil {
		return "", "", "", 0, apierr.Validation(apierr.Field("file", "is required"))
	}
	var (
		gotFile bool
		ver     string
	)
	for {
		if err := ctx.Err(); err != nil {
			return "", "", "", 0, err
		}
		part, nextErr := mr.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return "", "", "", 0, apierr.Validation(apierr.Field("file", "could not be read: "+nextErr.Error()))
		}
		name := part.FormName()
		switch name {
		case "file":
			if gotFile {
				_ = part.Close()
				return "", "", "", 0, apierr.Validation(apierr.Field("file", "must appear only once"))
			}
			p, s, n, spoolErr := r.SpoolUpload(ctx, part, part.FileName())
			_ = part.Close()
			if spoolErr != nil {
				return "", "", "", 0, mapUploadErr(spoolErr)
			}
			path, sha, size = p, s, n
			gotFile = true
		case "version":
			raw, readErr := io.ReadAll(io.LimitReader(part, 256))
			_ = part.Close()
			if readErr != nil {
				return "", "", "", 0, apierr.Validation(apierr.Field("version", "could not be read"))
			}
			ver = strings.TrimSpace(string(raw))
			if ver == "" {
				return "", "", "", 0, apierr.Validation(apierr.Field("version", "must not be empty when provided"))
			}
			if len(ver) > 64 {
				return "", "", "", 0, apierr.Validation(apierr.Field("version", "must be at most 64 characters"))
			}
		default:
			// Drain and ignore unknown parts so the reader can advance.
			if _, copyErr := io.Copy(io.Discard, io.LimitReader(part, 1<<20)); copyErr != nil {
				_ = part.Close()
				return "", "", "", 0, apierr.Validation(apierr.Field(name, "could not be read"))
			}
			_ = part.Close()
		}
	}
	if !gotFile {
		return "", "", "", 0, apierr.Validation(apierr.Field("file", "is required"))
	}
	return path, sha, ver, size, nil
}

func mapUploadErr(err error) error {
	var tooLarge *ErrTooLarge
	if errors.As(err, &tooLarge) {
		// Surface as validation so the client gets 400 with the limit named
		// before any job row exists (M2-005 acceptance).
		return apierr.Validation(apierr.Field("file", tooLarge.Error()))
	}
	return err
}

func (r *Runner) requirePathUnderRoot(abs string) error {
	root := r.paths.Root()
	if root == "" {
		return errors.New("content: content data root is empty")
	}
	clean := filepath.Clean(abs)
	if !filepath.IsAbs(clean) {
		return apierr.Validation(apierr.Field("file", "bundle path must be absolute"))
	}
	rel, err := filepath.Rel(root, clean)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return apierr.Validation(apierr.Field("file", "bundle path must sit under the content data directory"))
	}
	return nil
}

func (r *Runner) removeUpload(abs string) {
	if abs == "" {
		return
	}
	// Only touch files we created under uploads/.
	root := r.paths.Root()
	if root == "" {
		return
	}
	rel, err := filepath.Rel(filepath.Join(root, UploadRoot), filepath.Clean(abs))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return
	}
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		r.log.Warn("content: remove upload", "path", abs, "error", err)
	}
}

func sanitizeUploadHint(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return ""
	}
	// Keep a short alnum/dash/dot/underscore suffix so the dir stays greppable.
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
		if b.Len() >= 48 {
			break
		}
	}
	return b.String()
}
