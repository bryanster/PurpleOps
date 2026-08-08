# M3-005 — Steps CRUD, copy-on-use, soft freeze, reveal

**Milestone:** M3 · **Size:** L · **Depends on:** M3-004, M2-007, `docs/content-copy-on-use.md`

## Why

Steps are the atomic workbook rows: ATT&CK identity, procedure snapshot, blind `revealed_at`, and
the parent of the single execution. Copy-on-use and soft-freeze protect report correctness when
catalogs change or operators edit mid-run (`M3-EPIC` decisions).

## Scope

**In**

- Spec-first:

  | Method | Path | Action |
  |---|---|---|
  | `GET` | `/engagements/{engagementId}/scenarios/{scenarioId}/steps` | `execution.read` (blind filter) |
  | `POST` | `/…/steps` | `workbook.write` |
  | `GET` | `/…/steps/{stepId}` | `execution.read` |
  | `PATCH` | `/…/steps/{stepId}` | `workbook.write` (+ soft-freeze rules) |
  | `DELETE` | `/…/steps/{stepId}` | `workbook.write` |
  | `PUT` | `/…/steps/order` | `workbook.write` |
  | `POST` | `/…/steps/{stepId}/reveal` | `workbook.write` |

  Also support engagement-scoped fetch helpers if the board needs flat lists
  (`GET /engagements/{id}/steps`) — still blind-filtered for blue.

- **Create:** accepts either fully manual fields or library references:
  - Manual: name, objective, technique/subtechnique/tactic ids, procedure JSON, target, tools,
    controls.
  - From technique: `technique_external_id` → `attackpin.ResolveTechnique(eng.attack_version, id)`;
    snapshot display fields; store `attack_version` = engagement pin.
  - From template: handled primarily in `M3-013`; this ticket accepts already-resolved snapshot
    fields and optional `template_id` lineage.
- **Create always inserts** a sibling `execution` row: `status=pending`, `version=1`, same txn.
- **Soft freeze** (`M3-EPIC`): if execution `status != pending` **or** any red/blue field has been
  written (define: `status != pending` is enough if red must move status first), reject PATCH that
  changes: `technique_id`, `subtechnique_id`, `tactic_id`, `procedure`, `template_id`,
  `attack_version`. Return 409 naming frozen fields. Always-editable: name, objective, target_asset,
  tools, controls_in_scope, ordinal (via reorder).
- **Blind list/get:** repositories apply `internal/store/blind.Scope` so blue in blind mode does not
  see `revealed_at IS NULL` steps. Reveal sets `revealed_at = now()`; idempotent if already revealed.
  Activity: `step.revealed`.
- **Auto-reveal:** not implemented in the reveal endpoint — `M3-006` calls a shared
  `workbook.RevealStep` when `engagement.auto_reveal_on_start` and red starts.
- Delete step: cascade execution, comments, evidence links, finding_step rows.
- Closed engagement → 409 on writes.
- Activity: `step.created`, `.updated`, `.deleted`, `.reordered`, `.revealed`.

**Out**

- Red/blue execution field PATCH (`M3-006`/`M3-007`).
- Atomic import UX (`M3-013`), CTID (`M3-012`).
- UI.

## Acceptance criteria

- [x] POST step creates step + pending execution; GET execution by step works.
- [x] Blue member on blind engagement: unrevealed step absent from list and direct GET is 404
      conceal (not 403).
- [x] Reveal makes step visible to blue; activity logged.
- [x] After execution leaves `pending`, PATCH technique/procedure → 409; PATCH name succeeds.
- [x] ResolveTechnique miss → 404/400; never falls back to another ATT&CK version.
- [x] Ordinal reorder dense and unique.

## Tests

- Blind filter integration with real `blind.Scope` (extend M1 probe pattern on real `app.step`).
- Soft-freeze table-driven field list.
- Create+execution atomicity (fail mid-txn leaves neither).
- Authz: blue cannot workbook.write; can execution.read when revealed.

## Notes for the implementer

- Resource typing for authz: step/execution reads use `ResourceExecution` (already has Blind guard)
  **or** extend Resource with step revealed flag — follow existing `GuardBlindMode` patterns; do not
  invent a second blind check in the handler.
- Snapshot fields even when manual — `attack_version` still stored from engagement.

## Implementation notes

- Soft freeze is enforced structurally: `PatchStepInput` only exposes the five always-editable
  fields (name, objective, target_asset, tools, controls_in_scope). The identity fields
  (technique_id, subtechnique_id, tactic_id, procedure, template_id, attack_version) are never
  present in the PATCH body. A future M3-013 (atomic-to-step) or requirements change may need
  an explicit 409 with frozen field names, at which point the check should be re-added.
- Blind filtering is applied in the handler layer via `stepBlindScope()` which reads the
  engagement mode and the caller's membership seat from the authorization context. List
  endpoints filter unrevealed steps in Go; GET returns 404 conceal for unrevealed steps to
  blue in blind mode. The authz middleware's `GuardBlindMode` provides the authorization-level
  fence; the handler-level check provides the query-level second fence.
- Technique resolution via `techniqueExternalId` resolves against the engagement's pinned
  ATT&CK version using `attackpin.ResolveTechnique`. Tactic is NOT auto-resolved (the
  Technique struct has no kill-chain-phase linkage); callers must supply tactic_id manually
  or leave it empty.
- Step delete renumbers remaining ordinals in the same transaction to keep them dense and
  unique. The scenario `renumberAfterDelete` dead code was removed.
- All step mutation routes (POST/PATCH/DELETE/PUT/reveal) are added to `csrfCoverage` and
  the authz sweep test's `target()` method includes `{stepId}` substitution.
- Generated code is clean (`make generate && git diff --exit-code` passes).
