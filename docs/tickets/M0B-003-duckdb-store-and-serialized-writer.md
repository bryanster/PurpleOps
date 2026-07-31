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

- [ ] `Open` fails with a clear error when the DB path's directory doesn't exist or isn't writable,
      naming the path.
- [ ] `Write` runs its callback inside a transaction; returning an error rolls back and no partial
      state is visible to readers afterwards.
- [ ] A panic inside the `Write` callback rolls back and re-panics — it must not leave the write
      lock held. (A deadlocked writer is worse than a crash.)
- [ ] 100 goroutines calling `Write` concurrently all succeed, and the resulting row count is exactly
      100. Zero conflict errors.
- [ ] A `Write` whose context is cancelled while waiting for the lock returns `context.Canceled` and
      **does not** apply its mutation.
- [ ] Readers are not blocked by an in-flight write (assert a read completes while a slow write is
      running).
- [ ] `Close` is safe to call twice and waits for in-flight writes.
- [ ] `go test -race ./internal/store/...` is clean.
- [ ] Package doc comment explains the single-writer rule and tells future contributors that all
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
