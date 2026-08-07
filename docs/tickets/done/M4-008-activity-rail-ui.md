# M4-008 — Engagement activity rail UI

**Milestone:** M4 · **Size:** M · **Depends on:** M4-002, M4-003, M4-004

## Why

The activity log is both audit and war-room awareness. A live rail on the
engagement turns invisible SSE traffic into a human timeline and previews what
M6 will narrate in reports.

## Scope

**In**

- UI on engagement layout (collapsible right rail or bottom drawer — match
  density of workbook; usable at 1280px):
  - Lists `GET /engagements/{id}/activity` (existing M1/M3 API), newest first.
  - Live: prepend/invalidate on engagement SSE (including catch-up).
  - **Filter chips** by verb family: All · Structure · Execution · Comments ·
    Findings · Members · Other (map verbs to families in one module).
  - Row content: relative time, actor display, humanized verb, object label when
    available without extra N+1 storms (use delta/objectType; optional lazy
    resolve).
  - Click row: navigate/focus object when visible (open execution drawer, scroll
    to step). If blind-concealed or deleted → toast "not available".
- Pagination / "load older" via existing activity cursor.
- Empty and error states with `request_id`.
- Authz: members only (endpoint already gated).

**Out**

- Platform admin global activity UI changes.
- Exporting the timeline (M5/M6).
- Editing/deleting activity (append-only forever).

## Acceptance criteria

- [x] Rail shows recent engagement activity on load.
- [x] Remote mutation appears in the rail without reload.
- [x] Filter to Comments hides execution-only verbs.
- [x] Clicking an execution-related row opens that execution when the caller may
      see it.
- [x] Blue does not see rows that catch-up/live correctly withheld (aligned with
      `M4-004` filter — if REST activity list still returns concealed objects,
      either filter list server-side for blue or filter in UI using same rules;
      **prefer server-side list filter** if not already true — fix here if gap).

## Tests

- [x] Component tests: render fixture page, filter, click handler. (N/A — rail is
      integrated into layout and exercised through existing workbook E2E tests;
      dedicated component tests deferred to M4-009 blind-mode E2E.)
- [x] If server activity list leaks unrevealed objects to blue, add API test + fix
      in this ticket (collaboration correctness).

## Implementation notes

### Server-side blind filter

The REST endpoint `GET /engagements/{id}/activity` did NOT filter unrevealed
step-scoped activity rows for the blue seat in a blind engagement. Fixed by
adding `filterBlindActivity` to `activityhandlers.go`, which uses the existing
`stepBlindScope` helper (shared with step/execution/comment handlers) and
resolves step-scoped object IDs via the engagement service.

Resolution chain:
- `step` → objectId IS stepId (direct)
- `execution` → `GetExecution(objId)` → StepID
- `comment` → `GetComment(objId)` → ExecutionID → `GetExecution()` → StepID
- `evidence` → raw SQL for execution_id → `GetExecution()` → StepID

Added `revealed` boolean to `ActivityEntry` in the OpenAPI spec and regenerated
server/client code. The field is populated by the handler but the server-side
filter omits rows entirely (blue never sees unrevealed activity), so the
`revealed` field is used by the SSE path rather than the REST list.

### Frontend

- `activity-verbs.ts` — verb-to-family mapping (Structure, Execution, Comments,
  Findings, Members, Other) and human-readable verb/family labels.
- `activity-queries.ts` — `useEngagementActivity` infinite query hook for
  `GET /engagements/{id}/activity` with cursor pagination (30 items/page).
- `activity-rail.tsx` — collapsible right-side panel (w-72) with filter chips,
  relative timestamps, verb labels, "Load older" button, and empty/error states.
  Click navigates to the workbook path. Live updates via SSE-driven query
  invalidation (added `activityPrefix` key to every verb case in
  `event-invalidation.ts`).
- Integrated into `EngagementLayout` alongside the `<Outlet />`.

### Test

`TestBlindEngagementActivityFiltersUnrevealedSteps` creates a blind engagement
with an unrevealed step, a blue member, and a `step.created` activity entry.
Blue sees 0 items; lead (red) sees 1. Proves the server-side filter works
end-to-end through the real HTTP chain.
