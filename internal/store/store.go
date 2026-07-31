package store

import (
	"context"
	"database/sql"
	"errors"
)

// ErrClosed is returned by [DB.Write] and [DB.Health] once [DB.Close] has been
// called. It is a distinct error so that a request racing a shutdown can be
// answered with "try again" rather than reported as a database fault.
var ErrClosed = errors.New("store: database is closed")

// Reader is the read surface of the database: statements that return rows, and
// nothing else. It has no Exec, no Begin and no Prepare, so that the only way to
// write is [DB.Write] — see the package comment for why that matters.
//
// Both *sql.DB and *sql.Tx satisfy it, so a repository's read helpers can be
// reused unchanged inside a write transaction.
type Reader interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Store is the persistence port: the whole surface the rest of the application
// may depend on. Everything DuckDB-specific stays inside this package, so that
// a future HA requirement means writing a second implementation rather than
// rewriting the callers (PLAN.md §1, "escape hatch").
//
// The transaction type is database/sql's, which is portable across drivers;
// keeping the schema portable ANSI SQL is what makes the rest of the swap
// plausible, and that is a convention rather than something this interface can
// enforce.
type Store interface {
	// Read returns the pooled read handle. Any number of goroutines may use it
	// at once.
	Read() Reader

	// Write runs fn inside a transaction, one caller at a time.
	Write(ctx context.Context, fn func(tx *sql.Tx) error) error

	// Health reports whether the database is answering.
	Health(ctx context.Context) error

	// Close releases the database. It is safe to call more than once.
	Close() error
}

var _ Store = (*DB)(nil)
