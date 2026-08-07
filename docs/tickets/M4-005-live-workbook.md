# M4-005 — Live workbook updates + 409 conflict toast

**Milestone:** M4 · **Size:** M · **Depends on:** M4-003, M4-004, M3-014, M3-015

## Why

`PLAN.md` §9 step 4: blue’s browser reflects red’s progress without reload.
M3 UI is mutation-invalidation only; this ticket makes the shared board and
execution drawer track remote scorers and surfaces optimistic-lock collisions
honestly.

## Scope

**In**

- Workbook board + execution drawer react to engagement SSE via the M4-003 map:
  - Remote red status/notes → status chips and red fields update after refetch.
  - Remote blue detection/score → category/modifiers/outcome update.
  - Structure: scenario/step create, reorder, delete, reveal → board reshapes.
  - Evidence/comment/finding badges or counts refresh when those queries exist.
- **409 conflict UX** (local writer loses the race):
  - Toast explaining stale version.
  - Reload the execution (and related) row from server.
  - Leave the user to re-apply edits — **no** automatic field merge (`M4-EPIC`).
- Visual polish (light): brief highlight on rows that changed from SSE (optional
  but recommended); must not flash the entire board on every event.
- Ensure closed engagement still receives read-side live updates (comments
  allowed after close per M3-010) but write controls stay disabled.
- Playwright or component-level demo path: two users, red runs step, blue sees
  status without manual refresh (may share setup with `M4-009`).

**Out**

- Presence avatars (`M4-006`).
- Comment unread (`M4-007`).
- Activity rail (`M4-008`).
- Changing optimistic lock server semantics.

## Acceptance criteria

- [ ] Two sessions on the same engagement: red → `running`/`complete` appears on
      blue’s board without reload within a small bound (SSE + refetch).
- [ ] Blue scores detection; red’s open drawer shows new category after event.
- [ ] Stale PATCH shows toast and refreshed version; successful retry works.
- [ ] Reveal makes a new row appear for blue (with blind rules) without reload.
- [ ] No full-app spinner on each event.

## Tests

- Component tests with injected events for chip/drawer updates.
- Handler-level 409 path already exists — FE test that mock 409 triggers toast +
  invalidate.
- Optional Playwright smoke tagged for war-room.

## Notes for the implementer

- Prefer invalidating the minimum keys; debounce burst events (e.g. rapid
  typing saves if any) if the UI janks — only if measured.
- Do not open a second EventSource beside the layout subscription.
