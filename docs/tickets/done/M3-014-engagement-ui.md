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

- [x] Lead creates engagement with pin, adds red+blue members, creates scenario+step manually.
- [x] Red sets execution running; with auto-reveal off, blue cannot see step until lead/red reveals.
- [x] Blue sees step after reveal; observer can comment.
- [x] CTID import from UI produces ordered steps on the board.
- [x] Closed engagement disables structure edits in UI (matches API 409).

## Implementation notes

- All pages built in `web/src/features/engagements/`: paths, queries (TanStack Query hooks for every
  M3 endpoint), roles helper, engagement layout shell with sub-navigation context, engagements list
  page, overview page, workbook page, findings page, settings page.
- `EngagementLayout` provides `EngagementCtx` context with `{ engagementId, role, closed }` so tabs
  never re-derive the caller's role from `GET /auth/me`.
- Workbook page bundles execution drawer, red/blue editors, comments, evidence, import CTID dialog,
  and step-from-template dialog in a single 1675-line file.
- 11 component tests: engagements list (3), workbook red (4), workbook blind blue (4). All pass.
- Nav updated: Engagements active (was `pending: 'M3'`), Scenarios removed (accessible from workbook
  or engagement overview). Content nav item relabeled to "Content library".
- Go lint has 4 pre-existing issues in `internal/httpapi/evidencehandlers.go` (M3-009) — not from
  this ticket.
- `make generate` clean; `vite build` passes.

## Notes for the implementer

- TanStack Query keys include engagement id + role-sensitive fields; invalidate on mutations.
- Do not prefetch other engagements’ workbook data.
- Prefer a drawer for execution detail over full page hops for war-room density.
