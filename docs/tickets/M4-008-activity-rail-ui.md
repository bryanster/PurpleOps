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
    to step). If blind-concealed or deleted → toast “not available”.
- Pagination / “load older” via existing activity cursor.
- Empty and error states with `request_id`.
- Authz: members only (endpoint already gated).

**Out**

- Platform admin global activity UI changes.
- Exporting the timeline (M5/M6).
- Editing/deleting activity (append-only forever).

## Acceptance criteria

- [ ] Rail shows recent engagement activity on load.
- [ ] Remote mutation appears in the rail without reload.
- [ ] Filter to Comments hides execution-only verbs.
- [ ] Clicking an execution-related row opens that execution when the caller may
      see it.
- [ ] Blue does not see rows that catch-up/live correctly withheld (aligned with
      `M4-004` filter — if REST activity list still returns concealed objects,
      either filter list server-side for blue or filter in UI using same rules;
      **prefer server-side list filter** if not already true — fix here if gap).

## Tests

- Component tests: render fixture page, filter, click handler.
- If server activity list leaks unrevealed objects to blue, add API test + fix
  in this ticket (collaboration correctness).

## Notes for the implementer

- Check whether `GET /engagements/{id}/activity` already conceals blind objects
  for blue. If it returns reveal events with step ids only, that may be ok;
  if it returns step.created for unrevealed steps, fix in store/handler.
- Do not block M4-009 on perfect humanization strings — clarity over poetry.
