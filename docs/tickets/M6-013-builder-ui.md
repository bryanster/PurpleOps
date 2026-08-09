# M6-013 — Builder UI: blocks, reorder, params, HTML preview

**Milestone:** M6 · **Size:** L · **Depends on:** M6-002, M6-003, M6-004, M6-005, M6-009, M0B-009

## Why

Section-picker is the product surface: toggle blocks, reorder, configure params, preview the same
HTML the client will get (modulo publish flags).

## Scope

**In**

- SPA routes under engagement: **Reports** list + **builder** for one draft.
- **Block palette** from registry catalogue (static list matching server ids is OK if API
  `GET /report-blocks` catalog not shipped — prefer small catalog endpoint on `report.read` for
  titles/descriptions/params schema).
- Drag-and-drop reorder (existing dnd pattern if any; else `@dnd-kit` or HTML5 — match repo norms).
- Per-block param forms:
  - rich text → TipTap (`M6-005`)
  - compare → engagement picker (baseline) limited to engagements user can `report.read`
  - scenario multi-select, verbosity selects, etc.
- **Preview pane:** `iframe` `srcDoc` or blob URL fed by preview HTML endpoint (`M6-009`). Refresh
  on explicit Preview click and/or debounced save. Label seat scope when `blindFiltered`.
- Save draft, apply template, save-as-template entry points (`M6-003`).
- Branding fields on report settings side panel (`M6-004` overrides).
- Empty states; validation errors from API surfaced per block.
- Observer can use builder (`report.write`).

**Out**

- Publish/share management UI (`M6-014`); pixel page preview; collaborative cursors.

## Files

- `web/src/features/reports/*`
- routes wired into engagement shell next to Workbook / Dashboard
- tests: component + Playwright smoke open builder

## Acceptance criteria

- [ ] Reorder persists after reload.
- [ ] Preview shows server HTML (network call to preview), not a React mock of charts.
- [ ] Compare block requires baseline before save succeeds (client + server validation).
- [ ] Blind blue sees preview banner consistent with dashboard.
- [ ] No "round" copy in the UI.

## Tests

- Playwright: add blocks → reorder → preview contains cover title from fixture engagement.
- Unit tests for param form serialization.

## Notes for the implementer

- Use generated client hooks only.
- Keep builder usable at 1280px (M0B-008 bar).
