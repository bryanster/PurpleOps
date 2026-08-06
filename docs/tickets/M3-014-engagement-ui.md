# M3-014 — Engagement UI: board / workbook

**Milestone:** M3 · **Size:** L · **Depends on:** M3-003…M3-011, M3-012, M3-013, M0B-009

## Why

The main working surface. Red and blue need role-appropriate views of the same engagement without
M4 live updates yet (refresh/query is enough; cache invalidation on mutation).

## Scope

**In**

- Routes (names indicative):
  - `/engagements` — list + create dialog (pin picker from `GET /content/attack/versions`, mode,
    auto-reveal toggle).
  - `/engagements/:id` — overview: status, members, mode badge, attack pin, dates.
  - `/engagements/:id/workbook` — **board**: scenarios as sections, steps as rows, execution status
    chips, detection category when present.
  - `/engagements/:id/findings` — findings list/detail.
  - `/engagements/:id/settings` — lead: mode, auto-reveal, status transitions, danger delete.
- **Role-appropriate UI:**
  - Hide workbook write controls for blue/observer (no add scenario/step).
  - Blind blue: only revealed steps; no reveal controls; no unrevealed names in empty placeholders.
  - Red/lead: reveal button on unrevealed steps; red execution editor entry points.
  - Blue/lead: detection scoring entry (full control chrome can wait for `M3-015`, but navigation to
    score must exist).
  - Observer: read + comment composer only.
- Members panel: add/re-seat/remove using `M3-003` APIs; user picker from existing admin/member user
  list endpoints as available.
- Import actions: “Import CTID plan”, “Add step from procedure” using M3-012/M3-013.
- Evidence list + upload affordance on execution detail drawer.
- Comments thread on execution detail.
- All data via generated client hooks (`M0B-009`); no raw URLs.
- Loading / empty / error states with problem `request_id`.
- Keyboard navigable; light/dark; usable at 1280 and 768.

**Out**

- SSE/live presence (M4).
- Full scoring control polish (`M3-015` owns the 5-button scale UX).
- Analytics dashboard (M5).
- Report builder (M6).

## Acceptance criteria

- [ ] Lead creates engagement with pin, adds red+blue members, creates scenario+step manually.
- [ ] Red sets execution running; with auto-reveal off, blue cannot see step until lead/red reveals.
- [ ] Blue sees step after reveal; observer can comment.
- [ ] CTID import from UI produces ordered steps on the board.
- [ ] Closed engagement disables structure edits in UI (matches API 409).

## Tests

- Component tests (MSW) for blind blue empty vs revealed, role-gated buttons.
- E2E slice: create engagement → members → scenario → step → reveal → comment (scoring detailed in
  M3-015).

## Notes for the implementer

- TanStack Query keys include engagement id + role-sensitive fields; invalidate on mutations.
- Do not prefetch other engagements’ workbook data.
- Prefer a drawer for execution detail over full page hops for war-room density.
