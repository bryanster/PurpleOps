# M2-010 — CTID emulation-plan catalog adapter

**Milestone:** M2 · **Size:** M · **Depends on:** M2-003, M2-005

## Why

CTID adversary emulation plans are the fastest path to a realistic scenario. M2 only **catalogs**
them (`emulation_plan` + ordered steps). Turning a plan into an engagement Scenario is M3-013 —
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

- `POST /engagements/{id}/import-plan` (M3-013).
- Editing upstream plans in UI (custom plans are not this ticket).

## Acceptance criteria

- [ ] Detail response returns steps sorted by `ordinal` ascending, dense or as upstream (document).
- [ ] Re-sync replaces plans without duplicate external_ids.
- [ ] Missing technique on a step is allowed (null) but named in parse warnings if common.
- [ ] CI fixtures only; loud failure on empty catalog after parse.

## Tests

- Parse/order tests; integration sync; GET detail shape test.

## Notes for the implementer

- Upstream formats vary by plan repo layout — freeze one supported layout in fixtures and docs;
  unknown layouts fail the job rather than half-import.
- Do not require ATT&CK to be installed before CTID sync; technique ids are strings until M3 pin
  resolve. Optional warn if ATT&CK is present and an id does not resolve in latest version.
