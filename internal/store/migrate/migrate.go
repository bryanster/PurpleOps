package migrate

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/bryanster/purpleops/internal/store"
)

// sqlDir holds the shipped migrations and nothing else — the whole directory is
// embedded, so a file that does not belong in it is a startup error rather than
// a migration that quietly never runs.
const sqlDir = "sql"

//go:embed sql
var embedded embed.FS

// tableName is deliberately unqualified. The package comment covers why it
// cannot live in app or content; it is not spelled main.schema_migrations
// either, because "main" is DuckDB's name for the default schema and the
// backlog keeps this SQL portable (PostgreSQL would call it "public").
const tableName = "schema_migrations"

const createTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version    INTEGER   NOT NULL PRIMARY KEY,
	name       TEXT      NOT NULL,
	checksum   TEXT      NOT NULL,
	applied_at TIMESTAMP NOT NULL
)`

const selectApplied = `SELECT version, name, checksum, applied_at
	FROM schema_migrations
	ORDER BY version`

const insertApplied = `INSERT INTO schema_migrations (version, name, checksum, applied_at)
	VALUES (?, ?, ?, ?)`

// DB is the part of the store this package needs: pooled reads, and writes
// serialized into one transaction at a time. [store.Store] satisfies it.
type DB interface {
	Read() store.Reader
	Write(ctx context.Context, fn func(tx *sql.Tx) error) error
}

// State is a migration and what the database knows about it, as reported by
// [Migrator.Status].
type State struct {
	Migration

	// Applied is false for a migration this database has not run yet.
	Applied bool

	// AppliedAt is UTC, and zero unless Applied.
	AppliedAt time.Time
}

// Migrator applies one validated set of migrations. Construct it with [New] or
// [Default]; both validate the set, so a Migrator that exists is one whose
// filenames and ordering are already known to be sound.
type Migrator struct {
	set    []Migration
	logger *slog.Logger
}

// Option configures a [Migrator].
type Option func(*Migrator)

// WithLogger sends progress somewhere other than [slog.Default]. A nil logger
// is ignored rather than installed, so a caller threading an optional logger
// through does not have to check first.
func WithLogger(logger *slog.Logger) Option {
	return func(m *Migrator) {
		if logger != nil {
			m.logger = logger
		}
	}
}

// New returns a Migrator over the migrations in fsys, which are read and
// validated now: an unparseable filename, a duplicate version or a gap in the
// sequence is an error here, before anything has touched a database.
//
// fsys is rooted at the migrations themselves, not at a parent directory. Tests
// supply their own — an [fstest.MapFS] is enough — which is the only supported
// way to run a set other than the shipped one.
func New(fsys fs.FS, opts ...Option) (*Migrator, error) {
	set, err := parseSet(fsys)
	if err != nil {
		return nil, err
	}

	m := &Migrator{set: set, logger: slog.Default()}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

// Default returns a Migrator over the migrations embedded in this binary.
func Default(opts ...Option) (*Migrator, error) {
	fsys, err := fs.Sub(embedded, sqlDir)
	if err != nil {
		return nil, fmt.Errorf("migrate: reading the embedded migrations: %w", err)
	}
	return New(fsys, opts...)
}

// Up applies the migrations embedded in this binary that db has not run yet.
// It is what a server calls at startup, after opening the store and before
// opening its listener.
func Up(ctx context.Context, db DB, opts ...Option) ([]Migration, error) {
	m, err := Default(opts...)
	if err != nil {
		return nil, err
	}
	return m.Up(ctx, db)
}

// Status reports the embedded migrations against what db has applied. It backs
// `popsctl migrate status`.
func Status(ctx context.Context, db DB, opts ...Option) ([]State, error) {
	m, err := Default(opts...)
	if err != nil {
		return nil, err
	}
	return m.Status(ctx, db)
}

// Up applies every migration db has not run yet, in version order, and returns
// those it applied — empty, and nil error, when the database was already up to
// date.
//
// Each migration is one transaction: a failure rolls that file back entirely,
// leaves it unrecorded, and aborts the run without attempting any later
// migration. The returned error names the file.
//
// It refuses to do anything at all if the database and this binary disagree
// about a migration that has already been applied. See the package comment.
func (m *Migrator) Up(ctx context.Context, db DB) ([]Migration, error) {
	applied, err := m.reconcile(ctx, db)
	if err != nil {
		return nil, err
	}

	var done []Migration
	for _, migration := range m.set {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		started := time.Now()
		if err := m.apply(ctx, db, migration); err != nil {
			return done, err
		}
		m.logger.InfoContext(ctx, "applied migration",
			"version", migration.Version,
			"file", migration.Filename(),
			"duration", time.Since(started).Round(time.Millisecond))
		done = append(done, migration)
	}

	if len(done) == 0 {
		// The set is never empty — parseSet rejects that — so the last entry is
		// the schema version this binary expects.
		m.logger.InfoContext(ctx, "database schema is up to date",
			"version", m.set[len(m.set)-1].Version)
	}
	return done, nil
}

// Status reports every migration in the set and whether this database has run
// it, in version order.
//
// It applies the same consistency checks as [Migrator.Up] and fails the same
// way, so that a status command on a database this binary must not touch says
// why instead of printing a reassuring table. It creates the bookkeeping table
// if it is absent, which is the only thing it writes.
func (m *Migrator) Status(ctx context.Context, db DB) ([]State, error) {
	applied, err := m.reconcile(ctx, db)
	if err != nil {
		return nil, err
	}

	states := make([]State, len(m.set))
	for i, migration := range m.set {
		states[i] = State{Migration: migration}
		if record, ok := applied[migration.Version]; ok {
			states[i].Applied = true
			states[i].AppliedAt = record.appliedAt
		}
	}
	return states, nil
}

// record is one row of schema_migrations.
type record struct {
	version   int
	name      string
	checksum  string
	appliedAt time.Time
}

// reconcile makes sure the bookkeeping table exists, reads it, and checks it
// against the set. Every entry point goes through it, so "the database and the
// binary agree" has one definition.
func (m *Migrator) reconcile(ctx context.Context, db DB) (map[int]record, error) {
	if err := ensureTable(ctx, db); err != nil {
		return nil, err
	}
	applied, err := readApplied(ctx, db)
	if err != nil {
		return nil, err
	}
	if err := m.verify(applied); err != nil {
		return nil, err
	}
	return applied, nil
}

// verify reports the first way in which the applied migrations and the set
// disagree. Each of these means the tree has been changed under a database that
// already ran the old version of it, and none of them is safe to continue past:
// the schema in front of us is not the one this binary's queries were written
// against.
func (m *Migrator) verify(applied map[int]record) error {
	inSet := make(map[int]Migration, len(m.set))
	for _, migration := range m.set {
		inSet[migration.Version] = migration
	}

	// Sorted, so a database several migrations out of step reports its earliest
	// divergence rather than whichever one the map happened to yield first.
	for _, version := range slices.Sorted(maps.Keys(applied)) {
		record := applied[version]
		migration, ok := inSet[version]
		switch {
		case !ok:
			return fmt.Errorf("migrate: this database has migration %04d (%q) applied and "+
				"this binary does not contain it: the binary is older than the database. "+
				"Deploy a build that includes it, or restore a backup taken before it ran",
				version, record.name)

		case record.name != migration.Name:
			return fmt.Errorf("migrate: migration %04d was applied as %q and this binary calls it %q: "+
				"migrations are append-only. Restore the original filename and add a new migration instead",
				version, record.name, migration.Name)

		case record.checksum != migration.Checksum:
			return fmt.Errorf("migrate: %s has changed since it was applied "+
				"(recorded %s, now %s): migrations are append-only. "+
				"Revert the file and add a new migration instead",
				migration.Filename(), shortSum(record.checksum), shortSum(migration.Checksum))
		}
	}
	return nil
}

// apply runs one migration and records it in the same transaction, so a
// database can never hold a migration's effects without the row saying so.
func (m *Migrator) apply(ctx context.Context, db DB, migration Migration) error {
	err := db.Write(ctx, func(tx *sql.Tx) error {
		// The whole file goes to the driver in one call. DuckDB's DDL is
		// transactional, so a statement failing part-way through rolls back the
		// statements before it too — verified, not assumed. A file containing
		// its own BEGIN or COMMIT fails loudly here rather than escaping the
		// transaction, which is the outcome we want from that mistake.
		if _, err := tx.ExecContext(ctx, migration.sql); err != nil {
			return fmt.Errorf("running it: %w", err)
		}
		if _, err := tx.ExecContext(ctx, insertApplied,
			migration.Version, migration.Name, migration.Checksum, time.Now().UTC()); err != nil {
			return fmt.Errorf("recording it in %s: %w", tableName, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("migrate: %s: %w", migration.Filename(), err)
	}
	return nil
}

// ensureTable creates the bookkeeping table if this is a database's first
// migration. It cannot be a migration itself: the row recording 0001 has to
// have somewhere to go before 0001 runs.
func ensureTable(ctx context.Context, db DB) error {
	err := db.Write(ctx, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, createTable)
		return err
	})
	if err != nil {
		return fmt.Errorf("migrate: creating %s: %w", tableName, err)
	}
	return nil
}

// readApplied returns what the database says it has already run, by version.
func readApplied(ctx context.Context, db DB) (map[int]record, error) {
	rows, err := db.Read().QueryContext(ctx, selectApplied)
	if err != nil {
		return nil, fmt.Errorf("migrate: reading %s: %w", tableName, err)
	}
	defer rows.Close()

	applied := make(map[int]record)
	for rows.Next() {
		var r record
		if err := rows.Scan(&r.version, &r.name, &r.checksum, &r.appliedAt); err != nil {
			return nil, fmt.Errorf("migrate: reading %s: %w", tableName, err)
		}
		// Stored UTC, but a driver is free to hand back a zone; the callers of
		// this compare and format it, and neither should depend on which.
		r.appliedAt = r.appliedAt.UTC()
		applied[r.version] = r
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrate: reading %s: %w", tableName, err)
	}
	return applied, nil
}
