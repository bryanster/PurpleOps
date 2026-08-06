# M3-012 — CTID emulation plan → Scenario import

**Milestone:** M3 · **Size:** M · **Depends on:** M3-005, M2-010, `docs/content-ctid.md` § M3 import contract

## Why

CTID plans are catalog-only in M2. Import turns a plan into a ready-made Scenario with snapshotted
steps (`PLAN.md` §3, copy-on-use). Ticket id was M3-013 in older docs — **this file is canonical**.

## Scope

**In**

- Spec-first:

  | Method | Path | Action |
  |---|---|---|
  | `POST` | `/engagements/{engagementId}/import-plan` | `workbook.write` |

- Body: `plan_id` (content catalog id) and/or `plan_external_id` + source; optional `name` override;
  optional starting ordinal hint.
- Mapping **exactly** as `docs/content-ctid.md` § M3 import contract:
  - Scenario: name/narrative from plan; `source=ctid`; threat_actor from `adversary_name`;
    `source_ref` + weak `plan_id` lineage.
  - Steps: ordinal from catalog position; snapshot name, description, procedure JSON;
    `technique_external_id` string (may be empty); `attack_version` = **engagement pin**, not plan
    metadata.
  - Each step gets pending execution (`M3-005` helper).
- Before import: `attackpin.AssertPinned` if any step has a technique id you will resolve; optional
  **warn** list in response for technique ids that do not resolve in the pin (still import — empty
  tactic/technique display fields or store unresolved id string per contract).
- Disabled/missing plan → 404/409 via content referencable helpers.
- One transaction for scenario + all steps + executions.
- Activity: `scenario.imported` with plan lineage in delta.
- Update doc references: `docs/content-ctid.md` and `docs/content-copy-on-use.md` ticket id →
  `M3-012`.

**Out**

- Re-import merge/update existing scenario (v1 = always new scenario).
- UI picker (`M3-014` can call this API).

## Acceptance criteria

- [ ] Import FIN6 (or fixture plan) yields scenario with N steps matching catalog order.
- [ ] Catalog re-sync after import does not change stored step procedure snapshots.
- [ ] Engagement pin used for `step.attack_version` even when plan metadata names another version.
- [ ] Blue workbook.write denied.

## Tests

- Store/domain import against M2 CTID fixture.
- Snapshot isolation test (mutate catalog row after import → step unchanged).

## Notes for the implementer

- Reuse scenario/step write domain; do not bypass soft-freeze machinery (new steps are fine).
- Large plans: single txn is ok at this scale; if needed, still one request one txn.
