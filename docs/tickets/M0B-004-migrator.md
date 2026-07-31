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
- `popsctl migrate status` support (the command itself lands in `M0B-014`; expose the function).

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

- [ ] Migrating an empty database applies every migration in order and records each one.
- [ ] Migrating an up-to-date database is a no-op and logs nothing alarming.
- [ ] Migrating a partially-migrated database applies only the missing ones.
- [ ] A migration containing invalid SQL aborts the run, leaves `schema_migrations` without that
      version, and returns an error naming the file and the underlying SQL error.
- [ ] Changing the contents of an applied migration causes startup to fail with a checksum error
      naming the file — not a silent skip and not a re-apply.
- [ ] Gaps or duplicate version numbers in the embedded set are detected at startup, before any SQL
      runs.
- [ ] Two servers started simultaneously against the same file do not both apply migrations
      (document the outcome: the second should fail to open the DB at all, per DuckDB's
      single-writer rule — assert that this is the observed behaviour).
- [ ] `app` and `content` schemas exist after `0001`.

## Tests

- Forward migration from empty, asserting final schema (`PLAN.md` §9).
- Idempotency: run twice, second run applies zero.
- Failure: inject a bad migration via a test-local `embed.FS` / `fs.FS` — the migrator must accept
  any `fs.FS` so tests can supply their own set. Design for this from the start.
- Checksum drift detection.
- Ordering with an out-of-order or duplicate filename.
