# M2-010 — CTID emulation-plan catalog adapter

**Milestone:** M2 · **Size:** M · **Depends on:** M2-003, M2-005

## Why

CTID adversary emulation plans are the fastest path to a realistic scenario. M2 only **catalogs**
them (`emulation_plan` + ordered steps). Turning a plan into an engagement Scenario is M3-012 —
but M3 should find complete, ordered, structured rows waiting.

## Scope

**In**

- `internal/content/ctid` adapter for `kind=ctid`.
- Fetch: HTTPS archive/bundle from the seed URL for the Center for Threat-Informed Defense
  adversary emulation library (document the exact artifact; pin default `ref` on the seed row).
- Parse plans → `content_emulation_plan` + ordered `content_emulation_plan_step`:
  - plan: name, description, threat actor / group refs as text or external ids, source metadata
  - step: `ordinal`, name, description, `technique_external_id`(s), structured procedure-ish payload
    when upstream provides commands (JSON), executor/platform when present
- Rolling `current` replace; keep step ordinals stable and unique per plan.
- Read API (`content.read`):
  - `GET /content/emulation-plans`
  - `GET /content/emulation-plans/{id}` (include ordered steps in detail, or
    `.../steps` subresource — prefer detail-with-steps for fewer round-trips)
- Fixtures: one small plan with ≥3 steps and technique links; one broken plan.
- Document M3 import contract: which fields map to `scenario` / `step` snapshots (link from
  `M2-007` copy-on-use doc).

**Out**

- `POST /engagements/{id}/import-plan` (M3-012).
- Editing upstream plans in UI (custom plans are not this ticket).

## Acceptance criteria

- [x] Detail response returns steps sorted by `ordinal` ascending, dense or as upstream (document).
- [x] Re-sync replaces plans without duplicate external_ids.
- [x] Missing technique on a step is allowed (null) but named in parse warnings if common.
- [x] CI fixtures only; loud failure on empty catalog after parse.

## Tests

- Parse/order tests; integration sync; GET detail shape test.

## Notes for the implementer

- Upstream formats vary by plan repo layout — freeze one supported layout in fixtures and docs;
  unknown layouts fail the job rather than half-import.
- Do not require ATT&CK to be installed before CTID sync; technique ids are strings until M3 pin
  resolve. Optional warn if ATT&CK is present and an id does not resolve in latest version.

## Implementation notes

- Package: `internal/content/ctid`. Registered by default in `httpapi` and
  `blctl` when `ContentAdapters` does not already supply `kind=ctid`.
- Fetch GETs the seed URL (GitHub archive zip of
  `center-for-threat-informed-defense/adversary_emulation_library`, ref `master`).
  Fixture-friendly via `Adapter.FetchBytes`. Offline bundle uses the same archive shape.
- Parse walks `{actor}/Emulation_Plan/yaml/*.yaml` inside zip/tar(.gz). Bare plan YAML
  accepted for fixtures. Skips `planners/`, `micro_emulation_plans/`, Archive, docs.
  Unknown layouts / missing `emulation_plan_details` / zero steps fail the job.
- Plan external ids: prefer `emulation_plan_details.id`; else actor directory slug.
  Step external ids: prefer step `id`; else `{plan_external_id}/{1-based-position}`.
- Ordinals: 1-based **document order** (dense). Upstream `procedure_step` labels live
  inside `procedure` JSON only. Documented in `docs/content-ctid.md`.
- Technique: single `technique_external_id` TEXT (CTID steps carry at most one
  `attack_id`). Empty allowed; job success message reports missing-technique count.
- Migration `0015_emulation_library.sql`: nullable `adversary_name`, `metadata` JSON
  on plans; `procedure` JSON on steps (DuckDB cannot ADD COLUMN with NOT NULL/DEFAULT);
  list indexes on `(source_id, version, external_id)`.
- Apply is stage-and-promote: rows land under `content.StagingVersion` (`__staging__`),
  then `PromoteEmulationVersion` deletes `current` and renames staging in one
  `store.Write` transaction. Failed re-sync leaves the prior ready catalog intact.
- Library list/get handlers on `EmulationPlans` with `EnabledOnly` default. OpenAPI
  paths under `/content/emulation-plans`; detail includes ordered `steps`.
- Authz sweep pins **custom** for the "sync" 409 case (CTID now has an adapter).
- Operator docs: `docs/content-ctid.md` (M3 import contract). Linked from
  `docs/content-copy-on-use.md` and `docs/content-bundles.md`.
