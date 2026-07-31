package migrate

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strconv"
)

// filePattern is the only filename this package accepts: a version zero-padded
// to four digits, an underscore, and a lower_snake_case description.
//
// The padding is not decoration. Migrations are read in filename order by every
// tool that is not this one — an editor's file tree, `ls`, a diff — and without
// it 10 sorts before 9.
var filePattern = regexp.MustCompile(`^([0-9]{4})_([a-z0-9]+(?:_[a-z0-9]+)*)\.sql$`)

// Migration is one migration file: an ordered, immutable unit of schema change.
type Migration struct {
	// Version orders the set. Versions run from 1 upwards with no gaps.
	Version int

	// Name is the descriptive part of the filename, without the version or the
	// extension: "init" for 0001_init.sql.
	Name string

	// Checksum is the hex SHA-256 of the file's contents, and is what makes an
	// edited migration a startup failure rather than a silent skip.
	Checksum string

	// sql is the file's contents. It is unexported because a caller has no
	// business running a migration outside [Migrator.Up].
	sql string
}

// Filename is the migration's file, for error messages and status output. It is
// derived rather than stored so that it cannot disagree with the fields it is
// built from.
func (m Migration) Filename() string {
	return fmt.Sprintf("%04d_%s.sql", m.Version, m.Name)
}

// parseSet reads and validates every migration in fsys, which must contain
// migration files and nothing else.
//
// It is strict about things that could instead be ignored — a stray file, an
// unpadded version, a gap in the sequence — because each of them is a migration
// somebody expected to run. Silently skipping one produces a database that is
// wrong in a way nothing reports until a query fails against it much later.
func parseSet(fsys fs.FS) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("migrate: reading the migration directory: %w", err)
	}

	set := make([]Migration, 0, len(entries))
	seen := make(map[int]string, len(entries))
	for _, entry := range entries {
		filename := entry.Name()
		if entry.IsDir() {
			return nil, fmt.Errorf("migrate: %q is a directory; the migration directory "+
				"holds migration files and nothing else", filename)
		}

		match := filePattern.FindStringSubmatch(filename)
		if match == nil {
			return nil, fmt.Errorf("migrate: %q is not a migration filename; "+
				"expected NNNN_lower_snake_case.sql, with the version padded to four digits", filename)
		}
		version, err := strconv.Atoi(match[1])
		if err != nil {
			// Unreachable: the pattern matched exactly four digits, and 9999
			// fits in an int everywhere Go runs.
			return nil, fmt.Errorf("migrate: %q has an unreadable version: %w", filename, err)
		}

		if first, duplicate := seen[version]; duplicate {
			return nil, fmt.Errorf("migrate: %q and %q are both version %d; "+
				"two migrations cannot share a version", first, filename, version)
		}
		seen[version] = filename

		body, err := fs.ReadFile(fsys, filename)
		if err != nil {
			return nil, fmt.Errorf("migrate: reading %q: %w", filename, err)
		}

		set = append(set, Migration{
			Version:  version,
			Name:     match[2],
			Checksum: checksum(body),
			sql:      string(body),
		})
	}

	if len(set) == 0 {
		return nil, errors.New("migrate: the migration directory is empty; " +
			"a migrator with nothing to apply is a wiring mistake, not an empty schema")
	}

	slices.SortFunc(set, func(a, b Migration) int { return cmp.Compare(a.Version, b.Version) })

	// Versions are contiguous from 1. Two branches that each add "the next
	// migration" merge into a duplicate, caught above; one that is reverted
	// leaves a gap, caught here. Either way the sequence in the tree no longer
	// matches the sequence some database has already applied.
	for i, migration := range set {
		if want := i + 1; migration.Version != want {
			return nil, fmt.Errorf("migrate: %q is version %d where %04d was expected; "+
				"versions run from 0001 upwards with no gaps", migration.Filename(), migration.Version, want)
		}
	}
	return set, nil
}

// checksum hashes a migration's contents, having first normalised CRLF to LF.
//
// Git can be configured to rewrite line endings on checkout (core.autocrlf), so
// the same committed migration can reach the compiler as different bytes on
// different machines. That is not an edited migration, and refusing to start
// over it would be an alarm about a file the operator can see is unchanged.
func checksum(body []byte) string {
	sum := sha256.Sum256(bytes.ReplaceAll(body, []byte("\r\n"), []byte("\n")))
	return hex.EncodeToString(sum[:])
}

// shortSum abbreviates a checksum for an error message. Twelve hex characters
// is plenty to tell two migrations apart by eye, and a full pair of SHA-256s
// buries the sentence explaining what to do about them.
func shortSum(sum string) string {
	const shown = 12
	if len(sum) <= shown {
		return sum
	}
	return sum[:shown]
}
