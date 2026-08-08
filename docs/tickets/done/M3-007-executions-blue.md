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

- [x] Red session cannot write_blue; blue cannot write_red.
- [x] Invalid modifier / category → 400 problem.
- [x] Stale version → 409.
- [x] GET execution after score shows derived outcome matching `M3-008` fixtures.
- [x] Blue cannot score unrevealed blind step (404 conceal).

## Implementation notes

- Route: `PATCH /engagements/{engagementId}/executions/{executionId}/detection`
  (follows M3-006 pattern of including `engagementId` as a path parameter for
  authz middleware).
- `alertSeverity`: chosen as a small enum (`info`|`low`|`medium`|`high`|`critical`)
  in the BlueDetectionPatch schema. The Execution response schema retains free-text
  for backward compatibility with existing stored values.
- Scoring domain package (`M3-008`, `internal/domain/scoring`) provides:
  modifier validation (`ValidateModifiers`), category/protection enums, derived
  `outcome` and `mttdSeconds` computed in `executionToWire`.
- `scored_by`/`scored_at` set when detection category or protection changes on
  any successful patch.
- `detected_at` before `started_at` → 400 (validated in domain layer).
- Blue-side fields written through `BluePatchChanges` + `PatchBlue` store method
  with same optimistic-locking pattern as `PatchRed`.
- Authz sweep test updated: blue detection endpoint is now driven as a real
  endpoint (was a stub). Admin/lead/blue get 404 (pass authz, no execution),
  red/observer get 403, outsider gets 404.
- CSRF coverage added for the new route.
- `VerbExecutionBlueUpdated` added to events package for activity logging.

## Tests

- Handler tests with each category; modifiers multi-select including empty.
- Cross-side authz.
- Version conflict when red updated between blue's GET and PATCH.

## Notes for the implementer

- Do not duplicate enum lists in the handler — import domain package.
- Severity: free text or small enum? Prefer a small optional set (`info`\|`low`\|`medium`\|`high`\|
  `critical`) or free text ≤N chars — pick one in OpenAPI and stick to it.
