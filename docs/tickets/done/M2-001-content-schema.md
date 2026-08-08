# M2-001 — Content schema: sources, versions, jobs, raw snapshots

**Milestone:** M2 · **Size:** L · **Depends on:** M1 complete (esp. M0B-004, M1-001 patterns)

## Why

`PLAN.md` §2 splits the database into `content` (reference, replaceable) and `app` (engagements,
precious). That split is what makes "reinstall ATT&CK v17" safe. Nothing in M2 can land without the
tables, and every later adapter ticket should only *add columns it truly owns* — not redesign the
registry.

This ticket is storage only: shape, constraints, repositories, seed rows. No HTTP, no adapters.

## Scope

**In**

- New SQL schema `content` (alongside `app`), migration(s) under the existing migrator (`M0B-004`).
- Tables (column lists indicative; names may tighten, properties may not):

| Table | Purpose |
|---|---|
| `content.content_source` | Registry row per upstream or custom library |
| `content.content_source_version` | ATT&CK: many versions; rolling sources: one current head |
| `content.content_sync_job` | DB-backed job row for sync / reprocess / bundle import |
| `content.content_tactic` | ATT&CK tactics, versioned |
| `content.content_technique` | Techniques + sub-techniques (parent link), versioned |
| `content.content_mitigation` | Mitigations, versioned |
| `content.content_group` | Intrusion sets / groups, versioned |
| `content.content_software` | Malware + tools, versioned |
| `content.content_data_source` | Data sources / components as upstream provides, versioned |
| `content.content_procedure_template` | Atomic + custom structured procedures |
| `content.content_detection_rule_ref` | Sigma + custom detection references |
| `content.content_emulation_plan` | CTID plan catalog |
| `content.content_emulation_plan_step` | Ordered steps under a plan |
| `content.content_note` | Freeform KB notes (custom / imported) |

- `content_source` columns at minimum: `id`, `kind` (`attack`\|`atomic`\|`sigma`\|`ctid`\|`custom`),
  `name`, `url`, `ref`, `enabled`, `status`, `last_synced_at`, `item_count`, `error`,
  `license_spdx`, `license_name`, `license_url`, `attribution`, `created_at`, `updated_at`.
- `content_source_version`: `id`, `source_id`, `version` (text — e.g. `15.1` or `current` for
  rolling), `status`, `item_count`, `synced_at`, `error`, `raw_sha256`, `raw_path`, `raw_bytes`,
  unique `(source_id, version)`.
- `content_sync_job`: `id`, `source_id`, `version` (nullable), `kind` (`sync`\|`reprocess`\|
  `bundle_import`\|`v1_import`), `status` (`queued`\|`running`\|`cancelling`\|`cancelled`\|
  `succeeded`\|`failed`\|`interrupted`), `phase`, `progress_current`, `progress_total`, `message`,
  `error`, `created_by`, `created_at`, `started_at`, `finished_at`, `checkpoint` JSON.
- Every content object table: UUIDv7 `id`, `source_id`, `version` (text, matching the version row),
  `external_id`, `name`, `description` (where applicable), `created_at`, `updated_at`, and
  **unique `(source_id, version, external_id)`**. No STIX ID as primary key.
- `content_technique` carries `is_subtechnique`, `parent_external_id` (nullable), `tactics` (as a
  child table or canonical JSON — pick one, document why; prefer a join table if M5 heatmaps will
  SQL over it).
- `content_procedure_template` preserves structure: `platforms`, `executor`, `command`, `cleanup`,
  `input_args` JSON, `technique_external_ids`, `dependency_executor_name`, etc. **Do not** flatten
  to one `actions` string.
- `content_detection_rule_ref`: `technique_external_ids`, `level`, `status`, `logsource` JSON,
  `rule_yaml` (or equivalent body), title/description.
- `content_note`: `title`, `body_markdown`, `tags`, optional `technique_external_id`.
- Raw snapshot files live on disk under a configurable data dir (e.g. `content/raw/{source_id}/{version}/{sha256}`) —
  DB holds path + hash + size, not the blob bytes.
- **Seed** four disabled upstream sources (ATT&CK Enterprise, Atomic Red Team, SigmaHQ core-mapped,
  CTID adversary emulation) with default HTTPS archive base URLs, default `ref` pattern, and
  license/attribution strings filled in. One enabled `custom` source for user-authored rows
  (always present; not "synced" from upstream).
- Repositories in `internal/store/content/` (mirror `internal/store/identity/`): constructor-injected,
  writes via `store.Write`, reads via read pool.
- On process boot helper (called from server startup in a later ticket, defined here): mark any job
  left in `running`/`cancelling` as `interrupted` with a clear message. Do not silently resume.

**Out**

- HTTP API (`M2-002`), adapters (`M2-006`…), job runner logic (`M2-003`), UI.
- Engagement FKs — content must not reference `app` engagement tables.
- Full-text search indexes.

## Acceptance criteria

- [x] `blctl db info` lists the `content` schema tables after migrate.
- [x] Seed sources exist on a fresh DB: four upstream `enabled=false`, one `custom` ready for use.
- [x] Inserting two techniques with the same `(source_id, version, external_id)` fails on the unique
      constraint.
- [x] Role/kind/status columns are CHECK-constrained so invalid enums cannot be written.
- [x] Deleting a `content_source` that still has version or object rows is either restricted or
      cascades **within `content` only** — document the choice in the migration. (Product delete
      rules that inspect engagement refs land in `M2-002` / M3; schema must not cascade into `app`.)
- [x] Raw path columns never hold a relative path that escapes the configured content data root —
      repository rejects `..` and absolute paths outside the root.
- [x] Every repository method takes `context.Context` first; no repository holds `*sql.DB`.
- [x] Timestamps are UTC.
- [x] Package doc states: rolling sources use version token `current` (or the agreed single token)
      and a re-sync replaces objects for that token inside one transaction.


## Tests

- Migration-from-empty via `storetest.Migrated`.
- Constraint tests: duplicate natural key, invalid `kind`, invalid job `status`.
- Seed test: exact set of builtin sources and license fields non-empty for upstream kinds.
- Repository round-trips for source, version, job, and one object type per family (technique,
  procedure_template, detection_rule_ref, emulation_plan+steps, note).

## Notes for the implementer

- SQL stays portable ANSI (`PLAN.md` §1 escape hatch). No DuckDB-only types outside an allowed
  store corner.
- Prefer `content` as the schema name exactly as `PLAN.md` §2 — do not invent `ref` / `library`.
- ATT&CK multi-version means **rows duplicated per version**, not a slowly-changing dimension on
  one row. Rolling sources delete-and-replace (or stage-and-swap) within `version = 'current'`.
- Leave hooks/comments for M3 copy-on-use: engagement steps will snapshot fields and may store
  `template_id` as weak lineage with **no FK** from `app` to `content` required in M2.
- Config keys for data dir and size limits can be stubbed in `M0B-002` style here or in `M2-003`;
  if you add them, validate on load.

## Implementation notes

- Migration: `internal/store/migrate/sql/0011_content.sql`.
- Repositories: `internal/store/content/` (sources, versions, jobs, ATT&CK objects, procedures,
  detections, emulation plans/steps, notes). Boot helper: `Jobs.InterruptInFlight`.
- **No FKs** from children to `content_source` / `content_source_version`. DuckDB UPDATE-as-delete
  would otherwise make sync bookkeeping updates fail once any object exists (same lesson as
  `0003_user_updatable`). Delete is application-enforced in M2-002: clear the content subtree in
  one write transaction, then remove the source. Schema never cascades into `app`.
- Technique↔tactic is join table `content_technique_tactic` keyed by natural ids
  `(source_id, version, technique_external_id, tactic_external_id)` so M5 heatmaps can SQL over it
  and version-scoped replace need not resolve UUIDs.
- Rolling version token: `content.VersionCurrent` = `"current"`.
- Stable seed UUIDs: `content.SourceIDAttack` … `SourceIDCustom`.
- Config: `BLACKLIGHT_CONTENT_DIR` (default `./content`), created+writability-checked on load.
  Max-bytes / job-timeout / write-batch deferred to M2-003.
- Raw paths stored relative under `raw/{source_id}/{version}/{sha256}`; `content.Paths` rejects
  `..`, absolute paths, and backslashes.
- Detection rule status column is `rule_status` (not `status`) to avoid clashing with job/source
  status vocabulary in prose and greps.
- `detection_rule_ref.level` / `rule_status` are free text in M2; adapters own any tighter checks.

