# M0B-003 — DuckDB connection, pools, serialized writer

**Milestone:** M0b · **Size:** L · **Depends on:** M0B-002

## Why

`PLAN.md` §1 names the one real constraint of this architecture: **DuckDB allows one read-write
process, and concurrent write transactions can conflict.** If every repository opens its own
transaction whenever it likes, that constraint surfaces as intermittent, unreproducible write
failures under exactly the conditions the product is designed for — a war room with twenty people
scoring at once. Solve it once, structurally, at the bottom of the stack.

This ticket is the foundation everything persistent sits on. Get it reviewed carefully.

## Scope

**In**

- `internal/store` — package owning the database handle. Exposes:
  - a **read** path: the normal `*sql.DB` pool, any number of concurrent readers.
  - a **write** path: all writes serialized through a single connection, one at a time.
- Transaction helper that gives a caller a write transaction with automatic rollback on error/panic.
- Connection lifecycle: open, ping, apply DuckDB settings, close cleanly.
- A `store.Store` interface small enough that a future Postgres implementation is plausible
  (`PLAN.md` §1, "escape hatch").
- Test helper `storetest.New(t)` returning a fresh temp-file database, closed and removed via
  `t.Cleanup`.

**Out**

- Migrations (`M0B-004`).
- Any domain table or repository (M1+).

## Design constraints

- **Serialization mechanism.** Either a dedicated `*sql.Conn` guarded by a mutex, or a write-queue
  goroutine consuming write functions from a channel. Pick one, document why in a package comment.
  The mutex approach is simpler and sufficient at this write volume; prefer it unless you can show
  it isn't.
- **Context cancellation must work.** A caller whose request is cancelled while queued for the write
  lock must return promptly with `ctx.Err()` rather than acquiring the lock and doing the write
  anyway. This is the subtle part of the ticket.
- **Use a file, not `:memory:`.** In-memory DuckDB behaves differently around persistence and
  concurrency; tests that use it will lie to you.
- **No package-level global.** `store.Open(cfg)` returns a `*store.DB`; everything else takes it as
  a constructor argument (`PLAN.md` §6).
- A write path that is accidentally used for a read is a bug, but a read path used for a write is a
  *dangerous* bug. Make the write path the only one exposing `Exec`/`Begin`.

## Suggested API shape

Adapt as needed; the properties matter more than the names.

- `Open(ctx, cfg) (*DB, error)` / `(*DB).Close() error`
- `(*DB).Read() *sql.DB` — pooled, read-only by convention
- `(*DB).Write(ctx, func(tx *sql.Tx) error) error` — serialized, transactional, auto-rollback
- `(*DB).Health(ctx) error` — used by `/healthz`

## Acceptance criteria

- [x] `Open` fails with a clear error when the DB path's directory doesn't exist or isn't writable,
      naming the path.
- [x] `Write` runs its callback inside a transaction; returning an error rolls back and no partial
      state is visible to readers afterwards.
- [x] A panic inside the `Write` callback rolls back and re-panics — it must not leave the write
      lock held. (A deadlocked writer is worse than a crash.)
- [x] 100 goroutines calling `Write` concurrently all succeed, and the resulting row count is exactly
      100. Zero conflict errors.
- [x] A `Write` whose context is cancelled while waiting for the lock returns `context.Canceled` and
      **does not** apply its mutation.
- [x] Readers are not blocked by an in-flight write (assert a read completes while a slow write is
      running).
- [x] `Close` is safe to call twice and waits for in-flight writes.
- [x] `go test -race ./internal/store/...` is clean.
- [x] Package doc comment explains the single-writer rule and tells future contributors that all
      writes must go through `Write`.

## Tests

Integration tests against a temp file database — no container, no mocks. Target: the whole package's
tests run in well under a second (`PLAN.md` §9).

Required cases: concurrent writers, rollback on error, rollback on panic, context cancellation while
queued, read during write, double close, reopen an existing file.

## Notes for the implementer

- Driver is `github.com/duckdb/duckdb-go/v2` — the *official* driver as of v2.5.0, migrated from
  `marcboeker/go-duckdb`. Versions encode the DuckDB version (`v2.MAJOR_MINOR_PATCH.x`). Do not
  add the old module.
- This needs `CGO_ENABLED=1`. If your editor or `go test` suddenly can't build, check that first.
- Set `db.SetMaxOpenConns` deliberately on the read pool and write a comment justifying the number.
- Before starting, skim the DuckDB Go driver's concurrency documentation — the failure mode you are
  designing around is real and cheap to reproduce.

---

## Implementation notes (added on completion)

### The constraint is real, and it was measured first

Before designing anything, 50 goroutines wrote to one row through an ordinary pool on a temp-file
database: **4 writes landed, 46 failed** with `TransactionContext Error: Conflict on update!`. That
number is quoted in the package comment because "DuckDB transactions are optimistic" is abstract and
"you lose nine writes in ten" is not. Readers, in the same spike, were never blocked by an
uncommitted write and saw the pre-write snapshot.

Driver is `github.com/duckdb/duckdb-go/v2 v2.10505.0` (DuckDB 1.5.5). The version scheme in the
ticket has since moved on: the current stable releases are `v2.1MMPP.x`, so the ticket's
`v2.MAJOR_MINOR_PATCH.x` now reads as `v2.10505.0` = DuckDB 1.05.05. `v2.5.x` is the *older*
numbering, not a newer release.

### Shape

`Open` takes `config.Database`, not the whole config, and returns a `*store.DB`. One
`duckdb.Connector` backs one `*sql.DB`; the writer's connection is checked out of that pool at
startup with `db.Conn(ctx)` and kept for the life of the process, so `SetMaxOpenConns` is
`readerConns + 1`. A second connector on the same path would also have worked — DuckDB's instance
cache shares one instance per file, verified in the spike — but two `*sql.DB`s sharing one connector
double-close it, and two connectors is more moving parts for nothing.

**`Read()` returns a `Reader` interface, not `*sql.DB`.** The ticket asks for `Read() *sql.DB` in the
suggested shape but also for the write path to be the only thing exposing `Exec`/`Begin`; those
cannot both hold, and the ticket says the properties matter more than the names. `Reader` has
`QueryContext` and `QueryRowContext` only. `*sql.Tx` satisfies it too, so a repository's read helpers
work unchanged inside a write transaction.

### The subtle part: cancellation

The write lock is a one-slot channel, not a `sync.Mutex`, because a `Mutex` cannot be acquired with a
context. `acquireWrite` checks `ctx.Err()` and `closing` **before** the blocking select as well as
inside it: a `select` whose cases are all ready picks one at random, so a cancelled caller would
otherwise win a free lock about half the time.

That belt-and-braces check is deliberately not load-bearing, and the mutation testing below shows
why it can't be tested from outside.

### Mutation testing: what the tests actually catch

Each guarantee was broken on purpose and the suite re-run:

| Broken | Caught by |
|---|---|
| No serialization (write through the pool) | `TestConcurrentWritersAllSucceed` — "7 write callbacks ran at once" |
| Queued writer ignores cancellation | `TestWriteCancelledWhileQueuedAppliesNothing` |
| No rollback when the callback panics | `TestWriteRollsBackOnPanicAndReleasesTheLock` |
| `Close` takes the lock before closing the door | `TestWriteQueuedAtCloseReturnsErrClosed` |
| Connection settings not applied | `TestEveryConnectionIsConfigured` — `TimeZone="Etc/UTC"` |
| Pre-check on an already-cancelled context | **nothing** — `BeginTx` rejects a cancelled context itself |
| `Close` does not wait for in-flight writes | **nothing** — `sql.Conn.Close` blocks on an open transaction |

The last two survive because `database/sql` already implements the guarantee. Both are kept anyway:
the acceptance criteria are about this package's behaviour, and it should not silently depend on
stdlib internals that no test would notice changing. Anyone tempted to delete them as dead code
should read this table first.

### Decisions the ticket did not fix

1. **`Health` checks the read pool only.** It runs `SELECT 1`, not `Ping` (a query proves a statement
   can run, which is what the caller needs next). It deliberately does *not* take the write lock: a
   `/healthz` that queues behind a long write reports a busy server as a dead one, and an
   orchestrator killing the process mid-write is the outage the endpoint exists to prevent. Readers
   and the writer share one DuckDB instance, so a dead instance still fails here.
2. **A path containing `?` is rejected.** It is a legal filename, but the driver reads a DSN: the
   tail becomes DuckDB settings and the error names an "unrecognised option" after part of the
   filename. `checkPath` also rejects a missing or non-directory parent with a sentence an operator
   can act on, rather than letting a DuckDB `IO Error` speak for it.
3. **Three connection settings, applied per connection** via the connector's init hook.
   `TimeZone='UTC'` keeps the repo's UTC rule true for `now()` and for casts;
   `autoinstall_known_extensions=false` and `autoload_known_extensions=false` stop a query reaching
   the network mid-request, which a single-binary, possibly air-gapped deployment must never do.
   Everything used (icu, json, parquet) is linked into the driver and already loaded — checked with
   `duckdb_extensions()` after disabling autoload, not assumed. `enable_external_access` was left
   alone: it would also block `COPY … TO`, which M6's exports may want.
4. **A failed rollback on the panic path logs through `slog.Default()` rather than panicking.** The
   backlog forbids `panic()` on a reachable path, and a second panic would replace the handler's
   panic value with this package's. It is not a silent failure: `database/sql` discards a connection
   whose rollback failed, so the next write reports the broken connection for itself.
   The panic is **not** recovered — recovering to re-panic would also turn a `runtime.Goexit`
   (what `t.Fatal` does inside a callback) into a panic on nil.
5. **`readerConns = 8`.** A connection buys concurrent *statements*, not throughput: DuckDB
   parallelises each query across its own thread pool, so more connections mostly add memory and
   contend for the same threads. As a limit it also stops a traffic spike from opening one DuckDB
   connection per in-flight HTTP request.
6. **`TestOpenRejectsAnUnwritableDirectory` skips when `euid == 0`**, which is the case in the
   devcontainer. It was verified for real by running the compiled test binary as `nobody`, where it
   passes; CI does not run as root, so it executes there.

### For the tickets that consume this

- `storetest.New(t)` gives a closed-on-cleanup database on a real file, with **no schema** —
  M0B-004 owns migrating it, and the store tests deliberately create their own table.
- `Store` is the interface to depend on (`Read`/`Write`/`Health`/`Close`); `*DB` is the DuckDB
  implementation of it. Repositories should take `store.Store` or a narrower interface, never a
  `*sql.DB`.
- `Write` is **not re-entrant**: calling it from inside a write callback blocks until that callback's
  context is cancelled.
- `Close` blocks until the in-flight write finishes. M0B-006's shutdown path should close the store
  *after* the HTTP server has drained, not alongside it.
- Nothing calls `store.Open` yet — wiring it into `cmd/blacklight` belongs to M0B-006, which owns
  process startup.
