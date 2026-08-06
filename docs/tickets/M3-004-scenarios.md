# M3-004 — Scenarios CRUD + reorder

**Milestone:** M3 · **Size:** M · **Depends on:** M3-002

## Why

A scenario is one narrative attack-chain section inside an engagement (`PLAN.md` §2). Manual
scenarios are authored here; CTID import (`M3-012`) reuses the same write path.

## Scope

**In**

- New authz action **`workbook.write`** (`ActionWorkbookWrite`): engagement-scoped, held by
  **lead + red** (and platform admin). Token scope: `engagements:write`. Regenerate `docs/authz.md`.
  Update matrix + regression tests (observer/blue cannot write workbook).
- Spec-first:

  | Method | Path | Action |
  |---|---|---|
  | `GET` | `/engagements/{engagementId}/scenarios` | `engagement.read` |
  | `POST` | `/engagements/{engagementId}/scenarios` | `workbook.write` |
  | `GET` | `/engagements/{engagementId}/scenarios/{scenarioId}` | `engagement.read` |
  | `PATCH` | `/engagements/{engagementId}/scenarios/{scenarioId}` | `workbook.write` |
  | `DELETE` | `/engagements/{engagementId}/scenarios/{scenarioId}` | `workbook.write` |
  | `PUT` | `/engagements/{engagementId}/scenarios/order` | `workbook.write` | body: ordered ids |

- Create/PATCH fields: `name`, `narrative`, `threat_actor`, optional `source` default `manual`,
  `source_ref`. Ordinal assigned dense 1..N on create (append) or via reorder endpoint.
- Delete scenario: cascades steps + executions + dependent findings links / evidence / comments for
  those steps (domain delete service). Refuse if you prefer restrict when findings exist — **default
  cascade** with activity; document.
- Closed/archived engagement: workbook writes → 409.
- Activity: `scenario.created`, `.updated`, `.deleted`, `.reordered`.

**Out**

- Steps (`M3-005`), CTID import (`M3-012`), UI.

## Acceptance criteria

- [ ] `workbook.write` in rule table; blue/observer denied; lead/red allowed.
- [ ] Create two scenarios → ordinals 1,2; reorder swaps; unique `(engagement_id, ordinal)` holds.
- [ ] Delete removes child steps/executions (integration test).
- [ ] Write on `closed` engagement → 409.
- [ ] `make generate` + authz doc drift clean.

## Tests

- Handler CRUD + reorder.
- Authz matrix rows for `workbook.write`.
- Cascade delete fixture.

## Notes for the implementer

- Keep ordinals dense after delete (renumber in same txn) **or** allow gaps and only unique — prefer
  **dense renumber** so UI order == ordinal without sparse holes.
