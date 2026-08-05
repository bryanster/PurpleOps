package content

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

// RawRoot is the directory under the configured content data root that holds
// last-successful upstream snapshots. Version rows store paths relative to
// [Paths.Root], typically beginning with this segment.
const RawRoot = "raw"

// Paths resolves and validates on-disk locations under a content data root.
// Construct it with [NewPaths].
type Paths struct {
	root string
}

// NewPaths returns a path helper rooted at root. root should already be the
// configured content data directory (created and validated by config.Load).
// Relative roots are resolved against the process working directory so spool
// paths are absolute — requirePathUnderRoot and os operations need that.
func NewPaths(root string) Paths {
	clean := filepath.Clean(root)
	if abs, err := filepath.Abs(clean); err == nil {
		clean = abs
	}
	return Paths{root: clean}
}

// Root returns the configured content data directory.
func (p Paths) Root() string { return p.root }

// RawRel builds the relative path stored on a version row for a successful
// snapshot: raw/{sourceID}/{version}/{sha256}.
//
// sourceID, version and sha256 must be path-safe single segments (no slashes,
// no ".."). Invalid input is rejected rather than cleaned into something else.
func (p Paths) RawRel(sourceID, version, sha256 string) (string, error) {
	for _, part := range []struct {
		name, val string
	}{
		{"source id", sourceID},
		{"version", version},
		{"sha256", sha256},
	} {
		if err := checkPathSegment(part.name, part.val); err != nil {
			return "", err
		}
	}
	// path.Join (slash-separated) keeps the stored form portable; the absolute
	// form is produced with filepath below.
	return path.Join(RawRoot, sourceID, version, sha256), nil
}

// Abs joins a stored relative path onto the root after validating it. The
// relative path must not be absolute, must not contain "..", and must resolve
// to a location still under the root.
func (p Paths) Abs(rel string) (string, error) {
	clean, err := p.CleanRel(rel)
	if err != nil {
		return "", err
	}
	abs := filepath.Join(p.root, filepath.FromSlash(clean))
	// filepath.Join already cleaned; confirm the result still sits under root.
	// Use Rel rather than a prefix check so a root of /data/content does not
	// accept /data/content-evil.
	relToRoot, err := filepath.Rel(p.root, abs)
	if err != nil {
		return "", fmt.Errorf("content: resolve %q: %w", rel, err)
	}
	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("content: path %q escapes content data root %q", rel, p.root)
	}
	return abs, nil
}

// CleanRel validates and normalises a relative path for storage. Empty,
// absolute, volume-absolute, backslash-escaping and ".." paths are rejected.
func (p Paths) CleanRel(rel string) (string, error) {
	if rel == "" {
		return "", fmt.Errorf("content: raw path must not be empty")
	}
	if strings.Contains(rel, "\x00") {
		return "", fmt.Errorf("content: raw path must not contain NUL")
	}
	// Stored paths are slash-separated and portable. A backslash is either a
	// Windows separator somebody pasted in, or an attempt to confuse a later
	// reader — refuse both rather than normalising into something surprising.
	if strings.Contains(rel, `\`) {
		return "", fmt.Errorf("content: raw path must use '/' separators, got %q", rel)
	}
	if path.IsAbs(rel) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("content: raw path must be relative, got %q", rel)
	}
	// path.Clean turns "a/../b" into "b" and "a/../../b" into "../b". Reject
	// anything that still climbs, and anything that cleaned away a ".." only
	// because a prefix cancelled it — we refuse ".." in the input outright so
	// a stored path is exactly what an operator can read off the row.
	if containsDotDot(rel) {
		return "", fmt.Errorf("content: raw path must not contain \"..\": %q", rel)
	}
	clean := path.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("content: raw path %q is not under the content data root", rel)
	}
	if path.IsAbs(clean) {
		return "", fmt.Errorf("content: raw path must be relative, got %q", rel)
	}
	return clean, nil
}

func containsDotDot(slashPath string) bool {
	for _, seg := range strings.Split(slashPath, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

func checkPathSegment(name, val string) error {
	if val == "" {
		return fmt.Errorf("content: %s must not be empty", name)
	}
	if strings.ContainsAny(val, `/\`) || strings.Contains(val, "..") ||
		strings.Contains(val, "\x00") || val == "." || val == ".." {
		return fmt.Errorf("content: %s %q is not a safe path segment", name, val)
	}
	return nil
}
