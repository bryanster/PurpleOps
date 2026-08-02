// Package store owns persistence: the DuckDB connection pools, the serialized
// writer that every write goes through, the embedded SQL migrations, and the
// repositories built on top of them.
//
// # The single-writer rule
//
// DuckDB lets one process hold a database file read-write, and its transactions
// are optimistic: two transactions that touch the same rows do not queue, one of
// them fails with a conflict error (PLAN.md §1). Concurrent writers are not a
// hypothetical here — the product exists so that a room full of people can score
// an engagement at the same time — and an unserialized pool loses roughly nine
// writes in ten under that load.
//
// So this package offers exactly one way to write, [DB.Write], and it
// serializes: one transaction at a time, on one connection reserved for the
// purpose. Reads go through the ordinary pool and are never blocked by a write,
// because a DuckDB reader sees the snapshot it started with.
//
// Two rules follow, and neither is enforced by the compiler:
//
//   - Every write goes through [DB.Write]. A statement that mutates the database
//     from a connection obtained any other way reintroduces exactly the conflict
//     this package exists to prevent, and it will not fail in a test — it fails
//     in production, occasionally, when two people save at once. [DB.Read]
//     therefore returns a [Reader], which has no Exec and no Begin.
//   - [DB.Write] is not re-entrant. Calling it from inside a write callback
//     blocks until that callback's own context is cancelled.
//
// # Why a channel and not a sync.Mutex
//
// The write lock is a one-slot channel. A [sync.Mutex] cannot be acquired with a
// context: a caller whose HTTP request has already been cancelled would wait for
// the lock anyway and then perform a write nobody is waiting for. Serializing
// writes only makes queueing more likely, so giving up while queued has to work.
//
// A dedicated writer goroutine consuming from a channel would serialize equally
// well, but it puts every write on a stack that belongs to no request — panics,
// deadlines and traces all stop being the caller's. The lock keeps writes on the
// calling goroutine.
//
// # Schema
//
// The schema is owned by [github.com/bryanster/purpleops/internal/store/migrate],
// which applies the SQL migrations embedded in the binary. A server calls
// migrate.Up once at startup, after Open and before it accepts a request.
//
// # Repositories
//
// Repositories live in subpackages, one per area of the schema — the first is
// [github.com/bryanster/purpleops/internal/store/identity]. Each takes a
// database through its constructor and declares, in its own package, the
// narrow interface it needs; nothing here hands out a *sql.DB, and there is no
// package-level handle to reach for (PLAN.md §6).
package store
