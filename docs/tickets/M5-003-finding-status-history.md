# M5-003 — `finding_status_history` migration + write path

**Milestone:** M5 · **Size:** M · **Depends on:** M3-011

## Why

A burndown needs history, and there is none. `app.finding` stores current `status` and `updated_at`
only, so "how many findings were open on 12 June" is unanswerable from the workbook tables.

The activity log almost works — `finding.updated` deltas do carry status — but `0009_activity.sql`
already commits to a `blctl` retention/prune command, and a burndown chart that quietly changes shape
after an operator runs retention is precisely the class of analytics error the M5 epic's risk section
exists to prevent. History that a report depends on belongs in a table that reports own.

This is the only schema change in M5.

## Scope

**In**

- Migration `0017_finding_status_history.sql`, append-only, `app` schema:

  | Column | Notes |
  |---|---|
  | `id` | UUIDv7 text PK |
  | `finding_id` | FK → `app.finding(id)`, RESTRICT like every other FK here |
  | `engagement_id` | Denormalized so a burndown scans one engagement without joining `finding` |
  | `from_status` | Nullable — NULL is the creation row |
  | `to_status` | One of `open`, `in_progress`, `resolved`, `accepted_risk`; CHECK constraint |
  | `changed_by` | User id text, identity-style (no hard FK — see `0009_activity.sql`) |
  | `changed_at` | UTC `TIMESTAMP` |

- Index on `(engagement_id, changed_at)` — the burndown's only access path.
- **A creation row for every finding.** `CreateFinding` writes `(NULL → open)` in the same
  transaction as the insert. A finding with no history row is a hole in the chart.
- `UpdateFinding` writes a history row **only when status actually changes**, in the same transaction
  as the update. Editing a title is not a status transition and must not appear as one.
- Backfill in the migration for findings that already exist: one `(NULL → current_status)` row at the
  finding's `created_at`. Document that pre-M5 transitions are unrecoverable and the backfill row is
  the honest approximation, not a reconstruction.
- Repository in `internal/store/engagement/findings.go` (or a sibling file): append and
  list-by-engagement. **No update path, no delete path** — same discipline as `app.activity`.
- Deleting a finding removes its history rows (application-enforced cascade, as `M3-002` does for
  engagement delete). A history row pointing at nothing is not evidence.

**Out**

- The burndown query itself (`M5-007`).
- Any endpoint exposing raw history — it is analytics input, not a resource.
- Status history for anything other than findings.
- Retention/pruning of history. It is small and it is report input; if that changes, it is a decision
  with its own ticket.

## Files

- `internal/store/migrate/sql/0017_finding_status_history.sql`
- `internal/store/engagement/findings.go`
- `internal/engagement/findings.go`

## Acceptance criteria

- [ ] `blctl db info` lists `app.finding_status_history` after migrate.
- [ ] Creating a finding writes exactly one history row, `from_status` NULL, `to_status` `open`,
      `changed_at` equal to the finding's `created_at`.
- [ ] Patching title/description/severity/recommendation/owner **without** a status change writes
      **no** history row. Table-driven over each field.
- [ ] A status change writes exactly one row with the correct `from_status`, and a same-value status
      patch (`open` → `open`) writes none.
- [ ] History write and finding write share one transaction: a forced failure on the second leaves
      neither. Tested, not asserted by inspection.
- [ ] Migration-from-empty is green; the backfill produces one row per pre-existing finding, tested
      by seeding findings under migration `0016` and migrating forward.
- [ ] Invalid `to_status` fails the CHECK.
- [ ] Deleting a finding leaves no orphan history rows.
- [ ] Timestamps UTC; ids UUIDv7 text.

## Tests

- Migration forward from empty and from a seeded `0016` database (the backfill path).
- Repository round-trip; append-only enforced by the absence of any other method.
- Service-level: create, five patch shapes, status change, same-value status change, delete.
- Transaction atomicity under an induced failure.

## Notes for the implementer

- Migrations are append-only (README § Conventions) — `0017` is new, `0016` is untouched.
- Do not name a column `at`; DuckDB reserves it (`M1-001` note). `changed_at`, not `at`.
- The activity row for the same transition stays exactly as it is. This table is not a replacement
  for the audit trail; it is a report input with a different retention contract, and the M5 epic says
  why.
- Write through `store.Write` like every other writer — `PLAN.md` §1's serialized writer is not
  optional for a second write path.
