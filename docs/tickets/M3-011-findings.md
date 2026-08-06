# M3-011 — Findings + finding_step join

**Milestone:** M3 · **Size:** M · **Depends on:** M3-006

## Why

Findings track remediation work raised from the workbook (`PLAN.md` §2). Without rounds they do
**not** drive automatic re-execution materialization (`M3-EPIC`); they still matter for owners,
status, reports (M6), and operators recreating an assessment later.

## Scope

**In**

- Spec-first:

  | Method | Path | Action |
  |---|---|---|
  | `GET` | `/engagements/{engagementId}/findings` | `engagement.read` |
  | `POST` | `/engagements/{engagementId}/findings` | `finding.write` |
  | `GET` | `/findings/{findingId}` | `engagement.read` |
  | `PATCH` | `/findings/{findingId}` | `finding.write` |
  | `DELETE` | `/findings/{findingId}` | `finding.write` | lead or author policy — prefer lead+writers |
  | `PUT` | `/findings/{findingId}/steps` | `finding.write` | replace step id set |

- Fields: title, description, severity (`info`\|`low`\|`medium`\|`high`\|`critical`),
  recommendation, owner (user id optional), status (`open`\|`in_progress`\|`resolved`\|
  `accepted_risk`), `created_from_execution` optional on create.
- `finding.write` already held by lead+red+blue (`writers` in policy) — confirm matrix; observers
  read-only.
- Steps in join must belong to the same engagement (validate).
- Filters: status, severity, owner.
- Activity: `finding.created`, `.updated`, `.deleted`, `.steps_changed`.
- Closed engagement: allow status→resolved (prefer allow patch); create new on closed → 409.

**Out**

- Round-2 re-run action (explicitly removed).
- SLA / due dates.
- UI beyond what board embeds (`M3-014`).

## Acceptance criteria

- [ ] Create finding linked to execution + two steps; GET returns steps; wrong-engagement step → 400.
- [ ] Observer cannot POST; can GET.
- [ ] Status transitions free within enum (no heavy state machine) unless you add one — document.
- [ ] Delete removes finding_step rows.

## Tests

- Handler CRUD + step replace validation.
- Authz writers vs observer.

## Notes for the implementer

- `created_from_execution` is lineage only; do not cascade-delete finding when execution stays
  (executions aren’t deleted independently often). On step delete, drop join rows (`M3-005`
  cascade).
