# M0B-004 — Embedded SQL migrator and `schema_migrations`

**Milestone:** M0b · **Size:** M · **Depends on:** M0B-003

## Why

Schema changes must be ordered, recorded and shipped inside the binary — a single binary that needs
a separate migration tool alongside it isn't a single binary. `PLAN.md` §6 specifies an in-house
migrator rather than a third-party one, because the popular Go migrate libraries don't reliably
support DuckDB.

`PLAN.md` §2 also requires **two schemas in one database**: `content` (reference data, safe to drop
and reinstall) and `app` (engagement data, precious). That separation starts here.

## Scope

**In**

- `internal/store/migrate` — reads ordered `.sql` files from an `embed.FS`, applies unapplied ones
  in a single transaction each, records them in `schema_migrations`.
- `schema_migrations` table: `version` (int, PK), `name`, `checksum`, `applied_at`.
- Migration `0001_init.sql` creating the `app` and `content` schemas and nothing else.
- Migrations run automatically at server startup, before the HTTP listener opens.
- `blctl migrate status` support (the command itself lands in `M0B-014`; expose the function).

**Out**

- Down migrations. We do not support them — recovery is restore-from-backup. Say so in the docs.
- Any actual domain table.

## Rules

- File naming: `NNNN_snake_case_name.sql`, zero-padded to 4, strictly increasing, no gaps.
- Each file applies in **one** transaction through the serialized writer. A failure rolls that file
  back entirely and aborts the run — later migrations do not run.
- **Checksums.** Store a hash of each migration's contents. On startup, if an already-applied
  migration's file has changed, refuse to start with an error naming the file. This catches the
  single most common junior-engineer mistake in this area: editing a merged migration.
- Migrations are append-only. Never edit a merged one.

## Acceptance criteria

- [x] Migrating an empty database applies every migration in order and records each one.
- [x] Migrating an up-to-date database is a no-op and logs nothing alarming.
- [x] Migrating a partially-migrated database applies only the missing ones.
- [x] A migration containing invalid SQL aborts the run, leaves `schema_migrations` without that
      version, and returns an error naming the file and the underlying SQL error.
- [x] Changing the contents of an applied migration causes startup to fail with a checksum error
      naming the file — not a silent skip and not a re-apply.
- [x] Gaps or duplicate version numbers in the embedded set are detected at startup, before any SQL
      runs.
- [x] Two servers started simultaneously against the same file do not both apply migrations
      (document the outcome: the second should fail to open the DB at all, per DuckDB's
      single-writer rule — assert that this is the observed behaviour).
- [x] `app` and `content` schemas exist after `0001`.

## Tests

- Forward migration from empty, asserting final schema (`PLAN.md` §9).
- Idempotency: run twice, second run applies zero.
- Failure: inject a bad migration via a test-local `embed.FS` / `fs.FS` — the migrator must accept
  any `fs.FS` so tests can supply their own set. Design for this from the start.
- Checksum drift detection.
- Ordering with an out-of-order or duplicate filename.

---

## Implementation notes (added on completion)

### The DuckDB behaviour was measured, not assumed

Four properties this design leans on were verified in a spike before any of it was written:

| Question | Answer |
|---|---|
| Is DDL transactional? | **Yes.** A multi-statement `Exec` whose third statement fails leaves nothing from the first two |
| Do multiple statements work in one `Exec`? | **Yes**, so a migration file goes to the driver whole |
| Where does an unqualified `CREATE TABLE` land? | `main` — so `schema_migrations` can exist before `0001` creates `app` and `content` |
| Does a file containing `BEGIN;` escape the wrapping transaction? | **No.** `cannot start a transaction within a transaction`, and the file rolls back |

The last one is why there is no lint against transaction control in a migration: DuckDB already
makes that mistake loud.

### The "two servers" criterion is only true across processes

The ticket predicts the second server "should fail to open the DB at all". That is exactly what
happens **between processes** — `IO Error: Could not set lock on file … Conflicting lock is held in
… (PID n)` — and `TestASecondProcessCannotMigrateTheSameFile` asserts it by building
`testdata/lockprobe` and running it against a held database.

Inside **one** process it is not true: DuckDB's instance cache hands two `store.Open` calls on the
same path the same underlying instance, so the second succeeds. Two migrators racing there still
cannot double-apply — the loser gets `Catalog write-write conflict on create with "app"`, and the
primary key on `schema_migrations.version` is the backstop under that. Both were verified. This is
why the lock test needs a real subprocess: the behaviour is invisible from inside one.

The subprocess test compiles a CGO binary and costs ~2s, which is the whole package's runtime. It
has a control — after the first handle closes, the same probe succeeds and applies zero — so it
cannot pass with a broken probe.

### Decisions the ticket did not fix

1. **`schema_migrations` is unqualified**, not `main.schema_migrations`. It cannot live in `app` or
   `content`, because it records the migration that creates them. `main` is DuckDB's name for the
   default schema and PostgreSQL's is `public`, so naming it would break the portability rule in
   the backlog conventions for no gain.
2. **The checksum ignores CRLF/LF.** A checkout with `core.autocrlf` delivers the same committed
   migration as different bytes; refusing to start over that would be an alarm about a file the
   operator can see is unchanged. `TestChecksumDistinguishesRealEdits` is the counterweight —
   normalising must not make the checksum blind to an edit that matters.
3. **Two failures beyond the checksum drift the ticket asks for**, because both mean the same thing
   — the tree and the database disagree about a migration that has already run:
   - a *renamed* migration (`0001_init.sql` → `0001_start.sql`), which the checksum alone misses;
   - a version the database has applied that the binary does not contain, i.e. an older build
     deployed over a newer database. Its queries were written against a schema it is not looking at.
4. **`Status` verifies and fails like `Up` does.** A status command that prints a reassuring table
   about a database the server refuses to open is worse than one that says why. There is one
   definition of "the database and the binary agree" (`reconcile`) and every entry point uses it.
   `Status` creates the bookkeeping table if absent, which is the only thing it writes.
5. **A stray file in `sql/` is a startup error.** The whole directory is embedded (`//go:embed sql`,
   not `sql/*.sql`) precisely so that `0004_wip.sql.bak` is caught rather than silently not embedded.
6. **The migrator logs.** At startup the log is the only UI, and only the migrator knows when each
   file started. `WithLogger` defaults to `slog.Default()`; `Up` also returns what it applied, which
   is what `blctl` and the tests use.

### Mutation testing: what the tests actually catch

Every guarantee was broken on purpose and the suite re-run.

| Broken | Caught by |
|---|---|
| Checksum drift not detected | `TestUpRejectsAnEditedMigration`, `TestChecksumDistinguishesRealEdits`, `TestStatusRefusesToReassureAboutADatabaseItCannotUse` |
| Rename not detected | `TestUpRejectsARenamedMigration` |
| Applied version missing from the set tolerated | `TestUpRejectsADatabaseMigratedByANewerBinary` |
| Migrations applied newest-first | 11 tests, including `TestUpFromEmptyAppliesEveryMigrationInOrder` |
| Run continues past a failed migration | `TestUpAbortsAtTheFirstFailingMigration` |
| Gaps allowed | `TestNewRejectsAMalformedSet` (3 cases) and `TestABrokenSetIsRejectedBeforeAnySQLRuns` |
| Duplicate versions allowed | `TestNewRejectsAMalformedSet/duplicate_versions` |
| Checksum blind to contents | the three drift tests above |
| Set validated only as far as it parses (so `Up` reaches the database) | `TestABrokenSetIsRejectedBeforeAnySQLRuns` |
| Migration and its record in **separate** transactions | **nothing, at first** — `TestAMigrationAndItsRecordCommitTogether` was written for it |
| `applied_at` not normalised to UTC on read-back | **nothing** — the column is a naked `TIMESTAMP` and the connection is `SET TimeZone='UTC'`, so the driver already returns UTC |

The atomicity gap was real: a migration committed without the row recording it is re-applied on the
next startup, against a schema that already has it, and the server then fails to start over a
migration that worked. The window is too narrow to hit by racing a crash, so the test makes the
insert fail instead — the migration drops the table its own record is about to go into. The `.UTC()`
normalisation survives because the driver already does it; it is kept because `State.AppliedAt` is
documented as UTC and should not depend on that quietly continuing to be true.

### Deviations from the ticket

- **`cmd/blacklight` is unchanged.** The scope says migrations run at server startup before the
  listener opens; there is no listener until M0B-006, which owns process startup (M0B-003's notes
  say the same about `store.Open`). Wiring a temporary `main()` now would be rewritten there.
  Confirmed with the ticket owner. **M0B-006 must call, in this order:**

  ```go
  db, err := store.Open(ctx, cfg.Database)   // then defer db.Close()
  applied, err := migrate.Up(ctx, db)        // <- before the listener opens
  srv.ListenAndServe()
  ```

  A migration failure must abort startup. Do not serve on an unmigrated database.

- **Down migrations**: not implemented, as specified. `docs/migrations.md` says so, and gives the
  backup and restore procedure that replaces them.

### For the tickets that consume this

- `migrate.Up(ctx, db)` and `migrate.Status(ctx, db)` work on the embedded set. `migrate.New(fsys)`
  takes any `fs.FS` and is how tests supply their own.
- `Status` returns `[]State` in version order, each carrying `Applied` and a UTC `AppliedAt` —
  that is the function **M0B-014**'s `blctl migrate status` renders.
- `migrate.DB` is `Read()` + `Write()`; `store.Store` satisfies it.
- **Adding a table**: a new `internal/store/migrate/sql/NNNN_*.sql`, never an edit to an existing
  one. `docs/migrations.md` has the rules; the set is validated by the package's own tests, so a
  malformed filename fails locally rather than at deploy time.
- `0001_init.sql` creates the `app` and `content` schemas and nothing else. Domain tables belong to
  the milestone that needs them.
