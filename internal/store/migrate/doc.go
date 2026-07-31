// Package migrate applies the SQL migrations embedded in the binary.
//
// A single binary that needs a separate migration tool alongside it is not a
// single binary, so the migrations ship inside it: ordered .sql files in an
// [embed.FS], applied by [Up] at startup before the server opens its listener.
// There is no third-party dependency here because the popular Go migration
// libraries do not reliably support DuckDB (PLAN.md §6).
//
// # Migrations are append-only
//
// A merged migration is never edited, renamed or removed. Every deployment that
// has already applied it would otherwise be running a schema nobody can name,
// and the next migration would be written against a definition that no longer
// matches what is on disk.
//
// This package enforces that rather than trusting it. Each applied migration's
// SHA-256 is recorded, and a startup that finds a recorded migration whose file
// has since changed refuses to run and names the file. Renaming one, or
// pointing a binary at a database migrated by a newer binary, fails the same
// way. The failure is at startup, before any handler can read a table it has
// the wrong idea about.
//
// # There are no down migrations
//
// Recovery from a bad migration is restore-from-backup, not a reverse script.
// A down migration is written before it is needed, run once under pressure, and
// is the only SQL in the tree nobody has tested against real data; when it
// matters it either fails or silently destroys the rows it was meant to save.
// See docs/migrations.md for the operator's procedure.
//
// # Where the bookkeeping lives
//
// The schema_migrations table is created unqualified, in the database's default
// schema — main on DuckDB. It has to be: migration 0001 is what creates the app
// and content schemas, and the table that records 0001 must exist before 0001
// runs.
//
// # Concurrency
//
// Migrations run one file per transaction through the store's serialized
// writer, so a failure rolls that file back whole and later migrations do not
// run. Two servers cannot race each other: DuckDB locks the database file, and
// the second process fails to open it at all. Two [DB] handles inside one
// process are a different matter — DuckDB's instance cache gives them the same
// underlying instance — but they still cannot both apply a migration: the
// second gets a write-write conflict from DuckDB, and the primary key on
// schema_migrations.version is the backstop under that.
package migrate
