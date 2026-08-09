# M6-009 — Single HTML rendering path + golden files

**Milestone:** M6 · **Size:** L · **Depends on:** M6-006, M6-007, M6-008, M6-004

## Why

One HTML pipeline feeds draft preview, published versions, share view, and PDF. A second layout
engine is how share and PDF drift.

## Scope

**In**

- `report.Renderer` (name flexible) that:
  1. Loads draft **or** accepts an in-memory block list + resolved branding.
  2. Builds `RenderEnv` (engagement, branding, analytics.Queries, evidence policy, formatters,
     `IncludeEvidence`, `blind.Scope`).
  3. Renders each block in order to fragments; wraps in a full HTML document with embedded/linked
     `report.css`, meta charset, title.
  4. Returns `RenderedDocument{ HTML []byte, AssetManifest, Warnings []string }`.
- **API:**
  - `POST /engagements/{id}/reports/{reportId}/preview` → `text/html` (or JSON wrapper with HTML)
    for the **draft**, `report.read`, seat-scoped. Optional query/body `includeEvidence` default
    true for members with evidence.read.
  - Does **not** persist HTML.
- CSS: print-oriented stylesheet (page margins, break-inside avoids, cover, tables). Embed in
  document for self-contained share/PDF unless assets are strictly same-origin.
- **Golden files:** `internal/report/testdata/*.html` for representative drafts on
  `analyticstest` fixture; compare normalized output (strip changing ids/timestamps with a
  normalizer). `PLAN.md` §9.
- Format helpers: ISO dates, en-US numbers, UTC labels — unit tested.
- Block failure policy for **preview**: one block error becomes an in-document error callout; other
  blocks still render. Publish (`M6-011`) will require zero block errors.

**Out**

- PDF (`M6-010`); persist version (`M6-011`); share auth (`M6-012`); builder UI (`M6-013`).

## Files

- `internal/report/render.go`, `document.go`, `format.go`, `assets/report.css`
- `internal/httpapi` preview handler
- `internal/report/testdata/`, `render_golden_test.go`
- OpenAPI + `docs/api.md`

## Acceptance criteria

- [ ] Preview authz `report.read`; non-member denied.
- [ ] Blue vs lead preview HTML differ on blind fixture (and blue labeled — banner or meta).
- [ ] Golden tests stable in CI (no wall-clock).
- [ ] Document is UTF-8 HTML with no external script tags.
- [ ] Same renderer entry point documented for publish/PDF to call.

## Tests

- Goldens + normalizer tests.
- Handler smoke preview 200.
- CSS includes page-break utilities used by `page_break` block.

## Notes for the implementer

- Prefer complete self-contained HTML for PDF input (chromedp file/URL load).
- Do not use the SPA shell for report documents.
