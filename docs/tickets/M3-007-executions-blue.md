# M3-007 — Executions — blue side

**Milestone:** M3 · **Size:** M · **Depends on:** M3-006, M3-008

## Why

Blue detection is a separate operation with a separate body (`PLAN.md` §4). Scoring vocabulary and
derived outcome live in `M3-008`; this ticket is the HTTP/store write path + optimistic lock on the
same `execution.version` column (shared with red — any side update bumps version).

## Scope

**In**

- Spec-first:

  | Method | Path | Action |
  |---|---|---|
  | `PATCH` | `/executions/{executionId}/detection` | `execution.write_blue` |

- **PATCH body (blue only):** `version` (required), `detection_category`, `detection_modifiers`
  (array; validated against locked enum), `protection`, `detected_at`, `detecting_source`,
  `detecting_rule_ref`, `alert_severity`, `blue_notes`.
- Optimistic lock identical semantics to `M3-006` (shared `version`).
- Validate enums via `internal/domain/scoring` (`M3-008`); unknown modifier → 400.
- `detected_at` before `started_at` (when both set) → 400.
- Set `scored_by` / `scored_at` when category or protection changes (or on any successful detection
  patch — document).
- Blind guard: unrevealed step → conceal deny on write_blue (already in authz tests).
- Closed engagement → 409.
- Activity: `execution.blue_updated`.
- GET responses may include **derived** `outcome` and `mttd_seconds` computed by domain helpers —
  never persisted columns.

**Out**

- Scoring UI (`M3-015`).
- Analytics aggregations (M5).

## Acceptance criteria

- [ ] Red session cannot write_blue; blue cannot write_red.
- [ ] Invalid modifier / category → 400 problem.
- [ ] Stale version → 409.
- [ ] GET execution after score shows derived outcome matching `M3-008` fixtures.
- [ ] Blue cannot score unrevealed blind step (404 conceal).

## Tests

- Handler tests with each category; modifiers multi-select including empty.
- Cross-side authz.
- Version conflict when red updated between blue's GET and PATCH.

## Notes for the implementer

- Do not duplicate enum lists in the handler — import domain package.
- Severity: free text or small enum? Prefer a small optional set (`info`\|`low`\|`medium`\|`high`\|
  `critical`) or free text ≤N chars — pick one in OpenAPI and stick to it.
