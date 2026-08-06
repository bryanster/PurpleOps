# M3-001 — App domain schema: engagement through comment

**Milestone:** M3 · **Size:** L · **Depends on:** M1 complete (esp. M1-001), M2 complete (content stays separate)

## Why

Nothing in the product workbook can land without tables. M1 left `engagement_member.engagement_id`
**without** an FK on purpose (`M1-001` implementation notes); this ticket adds the real `engagement`
row and the rest of the `app` workbook graph. Storage only — no HTTP.

**No `round` table.** Per `M3-EPIC` decisions, retest rounds are out of v1; execution is 1:1 with
step.

## Scope

**In**

- Migration(s) under the existing migrator (`M0B-004`), `app` schema. Column lists indicative; names
  may tighten, properties may not.

| Table | Purpose |
|---|---|
| `app.engagement` | Assessment header: pin, mode, status, dates |
| `app.scenario` | Ordered attack-chain section inside an engagement |
| `app.step` | One technique/procedure row under a scenario (snapshots) |
| `app.execution` | Red + blue fill-in for one step (`UNIQUE(step_id)`) |
| `app.finding` | Remediation item on an engagement |
| `app.finding_step` | Which steps a finding points at |
| `app.evidence` | Metadata row for a blob linked to execution or comment |
| `app.evidence_blob` | Content-addressed blob index (sha256 PK / unique) |
| `app.comment` | Comment on an execution |
| `app.comment_revision` | Edit history bodies |

- **`engagement`** at minimum: `id`, `name`, `client`, `description`, `status`
  (`draft`\|`active`\|`closed`\|`archived`), `starts_on`, `ends_on`, `attack_version` (text, pinned),
  `mode` (`standard`\|`blind`), `auto_reveal_on_start` (bool, default false), `created_by`,
  `created_at`, `updated_at`.
- **`scenario`:** `id`, `engagement_id`, `ordinal`, `name`, `narrative`,
  `source` (`manual`\|`ctid`\|`imported`), `threat_actor`, `source_ref`, optional lineage
  `plan_id` (text, **no FK** to content), `created_at`, `updated_at`. Unique
  `(engagement_id, ordinal)`.
- **`step`:** `id`, `scenario_id`, `ordinal`, `name`, `objective`,
  `technique_id`, `subtechnique_id`, `tactic_id` (ATT&CK external ids as text, nullable where
  unknown), `procedure` JSON (`platform`, `executor`, `command`, `cleanup`, `args`, …),
  `template_id` (lineage text, no FK), `target_asset`, `tools` (JSON array), `controls_in_scope`
  (JSON array), `attack_version` (snapshot of engagement pin at create), `revealed_at` (nullable
  timestamptz — NULL = hidden from blue when engagement is blind), `created_at`, `updated_at`.
  Unique `(scenario_id, ordinal)`.
- **`execution`:** `id`, `step_id` **UNIQUE**, `version` (INT NOT NULL DEFAULT 1) for optimistic
  locking,
  - red: `status` (`pending`\|`running`\|`complete`\|`blocked`\|`skipped`), `executed_by`,
    `started_at`, `ended_at`, `command_run`, `source_host`, `target_host`, `red_notes`,
  - blue: `detection_category` (`none`\|`telemetry`\|`general`\|`tactic`\|`technique`, nullable until
    scored), `detection_modifiers` (JSON array of enum strings), `protection`
    (`blocked`\|`partial`\|`not_blocked`\|`n/a`, nullable), `detected_at`, `detecting_source`,
    `detecting_rule_ref`, `alert_severity`, `blue_notes`, `scored_by`, `scored_at`,
  - `created_at`, `updated_at`.
  - **Do not** store derived outcome.
- **`finding`:** `id`, `engagement_id`, `title`, `description`, `severity`, `recommendation`,
  `owner` (user id text, no hard FK required if consistent with identity style), `status`
  (`open`\|`in_progress`\|`resolved`\|`accepted_risk`), `created_from_execution` (nullable),
  `created_at`, `updated_at`.
- **`finding_step`:** PK `(finding_id, step_id)`.
- **`evidence_blob`:** `sha256` (unique), `size`, `mime`, `storage_path`, `ref_count`, `created_at`.
- **`evidence`:** `id`, `blob_sha256`, `filename`, `caption`, `side` (`red`\|`blue`),
  `execution_id` XOR link pattern with `comment_id` (exactly one parent — enforce in domain if
  CHECK is awkward), `uploaded_by`, `uploaded_at`, `size`, `mime`.
- **`comment`:** `id`, `execution_id`, `author_id`, `body`, `created_at`, `edited_at`.
- **`comment_revision`:** `id`, `comment_id`, `body`, `edited_by`, `edited_at` — append on each edit.
- Add **FK** from `app.engagement_member.engagement_id` → `app.engagement(id)` now that the parent
  exists. DuckDB: expect **RESTRICT** only (see M1-001 notes) — document; app-enforced cascades on
  engagement delete belong in `M3-002`.
- Wire `attackpin.References.AttackVersion` implementation that counts engagements with that pin
  (replace M2 `NopReferences` in server wiring).
- Repositories in `internal/store/engagement/` (or split `engagement`, `scenario`, … — mirror
  `identity/` / `content/`): constructor-injected, writes via `store.Write`, reads via read pool.
- CHECK constraints on every enum column.
- Indexes: `engagement(status)`, `scenario(engagement_id, ordinal)`, `step(scenario_id, ordinal)`,
  `execution(step_id)` (unique), `finding(engagement_id, status)`, `evidence(execution_id)`,
  `evidence(blob_sha256)`, `comment(execution_id)`, `engagement_member` already indexed.

**Out**

- HTTP, domain scoring helpers (`M3-008`), blob bytes on disk (`M3-009` may own disk layout — schema
  still has blob metadata table here or split path documented).
- UI.
- Any `round` / `round_id` column.

## Acceptance criteria

- [ ] `blctl db info` lists the new tables after migrate.
- [ ] Inserting two executions for the same `step_id` fails on the unique constraint.
- [ ] Invalid `detection_category` / `status` / `mode` / `side` fail CHECK.
- [ ] `engagement_member` insert for unknown `engagement_id` fails FK (or documented equivalent).
- [ ] `attackpin.References.AttackVersion` returns non-zero when an engagement pins that version.
- [ ] No repository holds `*sql.DB`; every method takes `context.Context` first.
- [ ] Timestamps UTC; IDs UUIDv7 text.
- [ ] Package/migration comments state: **no rounds**; execution grain is `step_id`.

## Tests

- Migration-from-empty via `storetest.Migrated`.
- Constraint tests: duplicate execution per step, bad enums, member FK.
- Repository round-trips for engagement, scenario+steps, execution version increment helper,
  finding+steps, comment+revision, evidence metadata.
- References.AttackVersion count test.

## Notes for the implementer

- SQL portable ANSI (`PLAN.md` §1). Quote reserved names (`"user"` pattern from M1).
- Prefer **no** FK from `app` to `content.*` — lineage ids are text.
- Creating a step **and** its pending execution in one txn is a domain/API concern (`M3-005`);
  schema must allow empty execution set only transiently if you insist on separate inserts — better
  unique + repository `CreateStepWithExecution`.
- `detection_modifiers` as JSON array validated in domain (`M3-008`); DB may CHECK json_type or leave
  to app — document choice.
- Do not name a column `at` (DuckDB reserved — M1-001 note).
