# M7-005 — Upgrade path, backup procedure, optional `blctl backup`

**Milestone:** M7 · **Size:** M · **Depends on:** M7-003 (docs contract), M0B-014

## Why

Single-file DuckDB + `evidence/` is simple until an operator upgrades without a backup and a
migration fails. `docs/migrations.md` already says recovery is restore-not-reverse. Cutover needs a
**repeatable operator procedure** and, if cheap, a `blctl` wrapper so the steps are harder to get
wrong. `docs/cli.md` already reserves `blctl backup` for M7.

## Scope

**In**

- **Canonical upgrade procedure** (must match M7-003 docs spine; implement gaps here):
  1. Read the release changelog upgrade notes.
  2. Stop the server (one writer process rule).
  3. Back up: DuckDB file + `evidence/` (+ encryption/session material required to use the backup).
  4. Replace image/binary with the new release.
  5. Apply migrations (startup auto-migrate and/or `blctl` migrate — document the supported order).
  6. Start; verify `/api/v1/healthz` and login.
  7. On failure: stop, restore backup files, run **previous** release.
- **`blctl backup`** (recommended in this ticket):
  - Creates a single archive (tar.gz) of the data dir contents needed for restore: database file,
    `evidence/`, and any entrypoint-managed secrets on that volume that are required to decrypt/use
    the backup (document exactly what is included).
  - Refuses to run while another process holds the DB **or** documents that the server must be
    stopped and enforces a lock/probe.
  - `blctl restore` is **optional**; a documented manual extract + replace is acceptable for v1.0.0
    if restore automation is risky. Prefer symmetric backup/restore if it stays small.
- Wire help text and `docs/cli.md` row to the real command.
- Cross-link deploy backup section to `blctl backup` as the preferred path, keep manual `docker run
  tar` as fallback.
- State that **engagement archive export (M5-012) is not a backup** of the install.

**Out**

- Online/hot backup without stopping writes.
- PITR, replication, remote object-storage backends.
- Migrating data from Mongo v1.
- Schema down-migrations.

## Files

- `internal/cli/` (`backup.go`, tests), `cmd/blctl` wiring if needed
- `docs/cli.md`, `docs/deploy.md`, `docs/migrations.md`
- Possibly `deploy/entrypoint.sh` only if backup needs a documented data-dir layout flag

## Acceptance criteria

- [ ] Documented upgrade procedure is complete end-to-end for compose operators.
- [ ] Backup artifact, when restored per docs onto a stopped data volume and started with the
      **same** release that took the backup, yields a healthy login and intact engagement data
      (manual or automated smoke in completion notes).
- [ ] Backup includes enough key material **or** docs scream that `BLACKLIGHT_ENCRYPTION_KEY` /
      session secrets must be recorded separately — no silent unrestorable backup.
- [ ] `blctl backup --help` exists and matches docs (if automation shipped); otherwise docs only
      describe manual backup and the cli.md placeholder is removed/replaced honestly.
- [ ] Failure path "restore previous release" is explicit.
- [ ] Does not require network.

## Tests

- CLI unit/integration: backup writes a non-empty archive with expected members on a temp data dir.
- Refuse or fail loudly when DB path missing.
- Prefer a small smoke that restore round-trips a tiny DuckDB + one evidence file.

## Notes for the implementer

- Data paths come from config (`BLACKLIGHT_DATA_DIR` / deploy defaults) — do not hardcode only
  `/var/lib/blacklight` without reading config.
- Stopping the server is a feature, not a bug; do not pretend hot backup is safe on DuckDB single
  writer without evidence.
- Coordinate copy with M7-003 so the two PRs do not thrash the same sections — land procedure once.
