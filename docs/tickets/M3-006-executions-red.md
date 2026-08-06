# M3-006 — Executions — red side

**Milestone:** M3 · **Size:** M · **Depends on:** M3-005

## Why

Field safety comes from the schema, not `if` statements (`PLAN.md` §4). Red writes through a
**separate** endpoint and body from blue so a blue client cannot send red fields. Optimistic locking
prevents silent war-room overwrites (`M3-EPIC`).

## Scope

**In**

- Spec-first:

  | Method | Path | Action |
  |---|---|---|
  | `GET` | `/executions/{executionId}` | `execution.read` |
  | `GET` | `/engagements/{engagementId}/executions` | `execution.read` | filters: scenario, status |
  | `PATCH` | `/executions/{executionId}/execution` | `execution.write_red` |

- **PATCH body (red only):** `version` (required), `status`, `started_at`, `ended_at`, `command_run`,
  `source_host`, `target_host`, `red_notes`. No detection fields in the schema for this operation.
- **Optimistic lock:** `WHERE id = ? AND version = ?`; success increments `version` by 1; 0 rows →
  **409** with current execution payload (or problem+extension). Do not apply partial updates on
  conflict.
- Status transitions (enforce in domain): from `pending` → `running`\|`skipped`\|`blocked`;
  `running` → `complete`\|`blocked`\|`skipped`; terminal states editable for notes/hosts/command
  with lead/red still holding write — allow note fixes without status change. Illegal jumps → 409.
- **Auto-reveal:** when status becomes `running` or `complete` (if jumping), and engagement
  `mode=blind` and `auto_reveal_on_start=true` and step `revealed_at IS NULL`, set reveal in same
  txn via shared helper from `M3-005`.
- Set `executed_by` on first non-pending transition if empty (caller subject).
- Timestamps: if client omits `started_at` on →`running`, server may set UTC now; document.
- Closed engagement → 409.
- Activity: `execution.red_updated` (delta JSON of changed fields; no huge notes dump if avoidable).
- Blind: blue cannot read unrevealed (existing guard).

**Out**

- Blue detection PATCH (`M3-007`).
- Evidence upload (`M3-009`).
- UI.

## Acceptance criteria

- [ ] OpenAPI has distinct `RedExecutionPatch` vs later `BlueDetectionPatch` types.
- [ ] Stale `version` → 409; successful patch returns new version = old+1.
- [ ] Blue token/session calling red endpoint → 403.
- [ ] Auto-reveal fires only when setting enabled and engagement blind.
- [ ] Response validation against spec in handler tests.

## Tests

- Concurrent two patches: second 409 (serialized writer still both attempt; version enforces).
- Status transition table.
- Authz regression: blue cannot write_red; red cannot call blue path (blue path may be stub until
  M3-007 — skip or land routes together).

## Notes for the implementer

- Prefer landing route stubs for blue in the same OpenAPI PR only if it keeps generate green; else
  M3-007 adds the path.
- Never accept full-row PUT that mixes sides.
