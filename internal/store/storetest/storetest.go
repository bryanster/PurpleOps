// Package storetest builds throwaway databases for tests.
//
// It is a real DuckDB file, not a mock and not :memory: — an in-memory database
// behaves differently around persistence and concurrency, so a test that used
// one would be answering a question nobody asked (M0B-003). A whole package of
// store tests still runs in well under a second.
package storetest

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/bryanster/purpleops/internal/config"
	"github.com/bryanster/purpleops/internal/store"
	"github.com/bryanster/purpleops/internal/store/migrate"
)

// New opens an empty database in a temporary directory of its own and closes it
// when the test ends. The file is removed with the directory.
//
// The database has no schema: migrations are applied by the caller that needs
// them (M0B-004).
func New(t testing.TB) *store.DB {
	t.Helper()

	// t.TempDir registers its own removal now, so the Close below — registered
	// after it — runs before it. The files go away only once DuckDB has let go.
	path := filepath.Join(t.TempDir(), "purpleops.duckdb")

	db, err := store.Open(context.Background(), config.Database{Path: path})
	if err != nil {
		t.Fatalf("storetest.New: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("storetest.New: closing %s: %v", path, err)
		}
	})
	return db
}

// Migrated is [New] with the shipped migrations applied — the database a
// repository test needs, and the same schema a server starts against, so a
// migration that a repository's SQL disagrees with fails here rather than in
// production.
//
// The migrator's progress goes nowhere: it is one line per migration per test,
// and none of it is what the test is about.
func Migrated(t testing.TB) *store.DB {
	t.Helper()

	db := New(t)
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := migrate.Up(context.Background(), db, migrate.WithLogger(quiet)); err != nil {
		t.Fatalf("storetest.Migrated: %v", err)
	}
	return db
}
