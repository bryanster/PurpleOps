// Package storetest builds throwaway databases for tests.
//
// It is a real DuckDB file, not a mock and not :memory: — an in-memory database
// behaves differently around persistence and concurrency, so a test that used
// one would be answering a question nobody asked (M0B-003). A whole package of
// store tests still runs in well under a second.
package storetest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/store"
	"github.com/bryanster/blacklight/internal/store/migrate"
)

// New opens an empty database in a temporary directory of its own and closes it
// when the test ends. The file is removed with the directory.
//
// The database has no schema: migrations are applied by the caller that needs
// them (M0B-004).
func New(t testing.TB) *store.DB {
	t.Helper()
	return open(t, tempPath(t))
}

// tempPath is the database file a test gets: its own directory, removed with
// the test.
func tempPath(t testing.TB) string { return filepath.Join(t.TempDir(), "blacklight.duckdb") }

// open opens path and closes it when the test ends.
//
// t.TempDir has already registered its own removal by the time this is called,
// so the Close registered here runs before it. The files go away only once
// DuckDB has let go.
func open(t testing.TB, path string) *store.DB {
	t.Helper()

	db, err := store.Open(context.Background(), config.Database{Path: path})
	if err != nil {
		t.Fatalf("storetest: opening %s: %v", path, err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("storetest: closing %s: %v", path, err)
		}
	})
	return db
}

// Migrated is [New] with the shipped migrations applied — the database a
// repository test needs, and the same schema a server starts against, so a
// migration that a repository's SQL disagrees with fails here rather than in
// production.
//
// It does not run the migrator. The 21 migrations take about 110 ms of DuckDB's
// time, and running them per test made them the single largest cost in the Go
// suite — 57% of internal/store/identity's CPU, 59% of internal/engagement's,
// 18% of internal/httpapi's. So they are run once per test binary, and what
// each test gets is a byte copy of the resulting file: about 13 ms, nearly all
// of which is [store.Open] and would have been paid anyway.
//
// What a test receives is unchanged. It is the same DuckDB file the migrator
// would have produced, opened the same way, private to the test, and writable —
// the copy is made before anything opens it, so no two tests ever share one.
// The only observable difference is that schema_migrations.applied_at is the
// moment the template was built rather than the moment the test started;
// nothing asserts on it except internal/store/migrate's own tests, which build
// their databases with [New] and their own migrator calls.
//
// A migration that a repository's SQL disagrees with still fails here, and a
// migration that fails to apply still fails the first test that asks for a
// database — see [migratedTemplate].
func Migrated(t testing.TB) *store.DB {
	t.Helper()

	path := tempPath(t)
	if err := os.WriteFile(path, migratedTemplate(t), 0o600); err != nil {
		t.Fatalf("storetest.Migrated: writing %s: %v", path, err)
	}
	return open(t, path)
}

// The migrated database, built once per test binary and handed out as bytes.
//
// It is held in memory rather than left on disk because there is no such thing
// as a temp directory scoped to a test binary: t.TempDir belongs to one test,
// and anything else would outlive the run. Migrated is under 3 MiB.
var (
	templateOnce  sync.Once
	templateBytes []byte
	templateErr   error
)

// migratedTemplate returns the bytes of a freshly migrated database, building
// them on first use.
//
// Callers must not modify the slice: every test in the binary is handed this
// same one, and all any of them does is write it to a file.
//
// A failure is reported to whichever test asked first and remembered, so the
// rest of the binary fails the same way rather than each re-running a migrator
// that has already been shown not to work.
func migratedTemplate(t testing.TB) []byte {
	t.Helper()

	templateOnce.Do(func() { templateBytes, templateErr = buildTemplate() })
	if templateErr != nil {
		t.Fatalf("storetest: building the migrated template database: %v", templateErr)
	}
	return templateBytes
}

// buildTemplate migrates an empty database and reads the file back.
//
// The close is what makes the bytes worth copying: DuckDB checkpoints and
// releases the file on close, so reading it afterwards gets a complete database
// and no write-ahead log beside it. Reading it while open would get whatever
// had reached disk so far.
func buildTemplate() (b []byte, err error) {
	dir, err := os.MkdirTemp("", "storetest-template-")
	if err != nil {
		return nil, fmt.Errorf("temporary directory: %w", err)
	}
	// Nothing outlives this function: the schema leaves as bytes.
	defer func() { err = errors.Join(err, os.RemoveAll(dir)) }()

	path := filepath.Join(dir, "template.duckdb")
	db, err := store.Open(context.Background(), config.Database{Path: path})
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}

	// The migrator's progress goes nowhere: it is one line per migration, and
	// none of it is what any test is about.
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := migrate.Up(context.Background(), db, migrate.WithLogger(quiet)); err != nil {
		return nil, errors.Join(fmt.Errorf("applying migrations: %w", err), db.Close())
	}
	if err := db.Close(); err != nil {
		return nil, fmt.Errorf("closing %s: %w", path, err)
	}

	if b, err = os.ReadFile(path); err != nil {
		return nil, fmt.Errorf("reading %s back: %w", path, err)
	}
	return b, nil
}
