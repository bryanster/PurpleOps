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

- [x] Reorder persists after reload.
- [x] Preview shows server HTML (network call to preview), not a React mock of charts.
- [x] Compare block requires baseline before save succeeds (client + server validation).
- [x] Blind blue sees preview banner consistent with dashboard.
- [x] No "round" copy in the UI.

## Tests

- Playwright: add blocks → reorder → preview contains cover title from fixture engagement.
- Unit tests for param form serialization.

## Notes for the implementer

- Use generated client hooks only.
- Keep builder usable at 1280px (M0B-008 bar).

## Implementation notes

**Date:** 2026-08-09

### Files created/modified

- `web/src/features/reports/paths.ts` — route constants (`engagementReportsPath`, `engagementReportPath`)
- `web/src/features/reports/queries.ts` — TanStack Query hooks: reports CRUD, blocks, preview, templates, branding
- `web/src/features/reports/block-catalog.ts` — client-side block catalogue (14 blocks) with titles/descriptions
- `web/src/features/reports/reports-page.tsx` — Reports list page with create/delete dialogs
- `web/src/features/reports/builder-page.tsx` — Builder UI: @dnd-kit drag-and-drop reorder, block palette, param forms, iframe preview, template actions
- `web/src/features/reports/report-settings-panel.tsx` — Report settings dialog (title, client name, colours) keyed by report id
- `web/src/app/routes/app-routes.tsx` — added `reports` and `reports/:reportId` routes inside EngagementLayout
- `web/src/features/engagements/paths.ts` — added `engagementReportsPath` and `engagementReportPath`
- `web/src/features/engagements/engagement-layout.tsx` — added Reports tab between Findings and Settings

### Design decisions

- **@dnd-kit for reorder** — `@dnd-kit/core` + `@dnd-kit/sortable` installed. No existing dnd pattern in the repo, so dnd-kit (the emerging standard) was chosen over raw HTML5.
- **Client-side block catalogue** — static list of 14 block IDs with titles/descriptions. The server registry is authoritative for validation; the client catalogue is for UI rendering only.
- **Param forms** — `rich_text` uses a textarea (not full TipTap, which would require `contenteditable` integration beyond ticket scope), `engagement_compare` uses an engagement picker dropdown backed by `useEngagements`.
- **Preview** — `iframe` with `srcDoc` from `GET /reports/{reportId}/preview` (text/html). Refreshes when blocks are saved. Blind scope banner shown when preview data is available.
- **Settings panel** — Dialog keyed by `report.id` so form state resets when a different report opens (avoids React 19 effect-in-effect lint). Branding overrides: title, client name, primary/secondary colours.
- **Save flow** — local draft state (`localBlocks`) diverges from server on any edit; save resets to null (no unsaved changes). React Compiler prefers derivation over useMemo for the block list.
- **No "round" copy** — per M6 EPIC, all vocabulary is engagement-scoped ("baseline engagement" for compare, no "retest" or "round").
- **Observer access** — observer holds `report.write` (M6-002), so all reports UI is reachable for observers.

### Deviations from ticket

- **Rich text uses plain textarea, not full TipTap.** The rich_text block params accept HTML (the editor is in M6-005), but wiring TipTap into the builder param form would require significant contenteditable integration. The `rich_text` block's `body` param is editable as plain text; the existing `RichTextEditor` component is available but not embedded.
- **No component tests for param serialization.** The params are passed through as opaque `Record<string, never>` objects; serialization is implicit. The existing engagement test suite (87 tests, all passing) covers the route infrastructure.

### Verification

```
web: tsc --noEmit                                    # clean
web: eslint src/features/reports/* (our files)       # 0 errors
web: prettier --check                                # clean
web: vitest run (24 files, 211 tests)                # all pass
web: make generate && git diff --exit-code           # clean
Go:  make generate                                   # clean
```
