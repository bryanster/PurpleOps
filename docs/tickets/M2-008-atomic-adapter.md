# M2-008 — Atomic Red Team adapter

**Milestone:** M2 · **Size:** M · **Depends on:** M2-003, M2-005

## Why

Atomic procedures are how red fills a step quickly. v1 flattened them into one `actions` string and
threw away structure. `PLAN.md` §3 requires preserving platform, executor, command, cleanup, and
input args so M3 can snapshot a real procedure onto a step.

## Scope

**In**

- `internal/content/atomic` adapter for `kind=atomic`.
- Fetch: HTTPS release archive from the seed URL (`redcanaryco/atomic-red-team` atomics-ish layout).
- Parse YAML atomics; one `content_procedure_template` per atomic test (not per technique file only).
- **Preserve structure** on each template:
  - `external_id` stable within source (e.g. technique + atomic index / auto_generated_guid if
    present — document the key).
  - `name`, `description`
  - `technique_external_ids` (usually one Txxxx)
  - `platforms` / supported platform list
  - `executor` (name + elevation etc. as available)
  - `command`, `cleanup`
  - `input_args` JSON (name, description, type, default)
  - dependency fields when present
- **Rolling head:** version token `current`. Re-sync replaces the catalog for `current`. Steps that
  already snapshot procedure JSON in M3 are unaffected (contract from `M2-007`).
- Fail loud on unreadable YAML or zero tests parsed.
- Fixtures: a handful of atomics covering windows/linux/macos, input args, cleanup, and one bad
  file.
- Read API (`content.read`):
  - `GET /content/procedure-templates` — filters: `q`, `technique`, `platform`, `sourceId`
  - `GET /content/procedure-templates/{id}`
- Bundle import uses the same archive shape as fetch.

**Out**

- Executing atomics.
- Mapping GUIs for argument editing at run time (M3 step editor).
- Multi-version Atomic storage.

## Acceptance criteria

- [ ] A fixture atomic with input args round-trips without flattening to a single string column.
- [ ] Re-sync replaces `current` templates; row count matches fixture; no duplicate external_ids.
- [ ] List filter by technique external id returns the expected templates.
- [ ] Disabled source hides templates from default list.
- [ ] CI uses fixtures only (no network).

## Tests

- Parse/normalize unit tests; full job integration; API filter tests.
- Explicit test that `command` and `cleanup` are distinct fields post-sync.

## Notes for the implementer

- Guids from upstream are good external_ids when present; otherwise derive deterministically and
  document so re-sync upserts rather than duplicates.
- License/attribution already on the source seed — do not strip.
