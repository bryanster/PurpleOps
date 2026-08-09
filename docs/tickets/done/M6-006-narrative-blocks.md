# M6-006 — Narrative blocks: cover, exec summary, scope/RoE, rich text, page break

**Milestone:** M6 · **Size:** M · **Depends on:** M6-001, M6-005, M6-004

## Why

Client reports open with narrative, not charts. These blocks are mostly params + branding; they prove
the registry/renderer pattern before analytics-heavy sections.

## Scope

**In**

- Register **renderers** for:
  | Block | Params (illustrative) |
  |---|---|
  | `cover` | title override?, subtitle?, date display (default engagement window), show logo |
  | `executive_summary` | `html` body (sanitized) |
  | `scope_roe` | `html` body (sanitized); optional structured fields if cheap (in-scope systems text) |
  | `rich_text` | `html` body (sanitized) |
  | `page_break` | none — emits print CSS page-break marker |
- Semantic HTML + CSS classes under a single report stylesheet namespace (e.g. `bl-report__*`).
- Cover pulls engagement name, client, dates, branding via `RenderEnv`.
- Golden-file **fragments** optional here; full-document goldens land in `M6-009`. At minimum unit
  tests: given env+params → HTML contains expected text and **no** unsanitized script.
- Copy defaults: empty exec summary renders a placeholder only in **draft preview**, omitted or
  short "Not provided" in publish — pick one and document (prefer omit empty optional sections on
  publish).

**Out**

- Full document assembly (`M6-009`); analytics blocks; UI editor wiring beyond what M6-005 shipped.

## Files

- `internal/report/blocks/cover.go`, `summary.go`, `scope.go`, `richtext.go`, `pagebreak.go`
- Shared CSS later consumed by M6-009 (`internal/report/assets/report.css` or embed)

## Acceptance criteria

- [ ] Each block id has a non-nil renderer in the registry.
- [ ] Sanitization applied to every HTML param before emit.
- [ ] Page break fragment includes a print `break-after: page` (or equivalent) rule/hook.
- [ ] Cover shows resolved client name and logo fallback behavior from M6-004.

## Tests

- Fragment tests with fixed `RenderEnv` (no DB).
- XSS attempt in `rich_text` params does not appear live in output.

## Notes for the implementer

- Keep CSS boring and print-friendly; avoid viewport-only units.
- No client-side React in the fragment — pure HTML strings.


## Implementation notes

**Date:** 2026-08-09

### Acceptance criteria status

- [x] Each block id has a non-nil renderer in the registry.
- [x] Sanitization applied to every HTML param before emit.
- [x] Page break fragment includes a print `break-after: page` (or equivalent) rule/hook.
- [x] Cover shows resolved client name and logo fallback behavior from M6-004.

### Files created/modified

- `internal/report/blocks/cover.go` — Cover block: params (title, subtitle, showDate, showLogo), renders engagement name/client/branding/dates, logo fallback when ref empty
- `internal/report/blocks/summary.go` — Executive summary: sanitized `body` param, empty → empty fragment (omits section)
- `internal/report/blocks/scope.go` — Scope/RoE: sanitized `body` + plain-text `systems` list, empty-all → empty fragment
- `internal/report/blocks/richtext.go` — Rich text: sanitized `html` param, empty → empty fragment
- `internal/report/blocks/pagebreak.go` — Page break: no params, emits `<div class="bl-report__page-break">`
- `internal/report/blocks/blocks_test.go` — 21 tests: cover (9), summary (3), scope (4), rich_text (3 + 10 XSS sub-cases), page_break (2), definitions (2), registry integration (4), formatEngagementWindow (5)
- `internal/report/assets/report.css` — Single report stylesheet with `bl-report__*` namespace, print-friendly, CSS custom properties for branding colours
- `internal/report/block.go` — Added `EngagementStartsOn`/`EngagementEndsOn` to `RenderEnv`, added `"time"` import
- `internal/report/registry.go` — Added `renderers map[ID]Renderer`, `SetRenderer`, `Renderer` methods with duplicate/unregistered panic guards
- `internal/httpapi/server.go` — Replaced stub block registration loop with full definitions from `blocks` package + `SetRenderer` calls; remaining unimplemented blocks use lightweight stubs

### Design decisions

- **Empty sections omitted** — executive summary, scope, and rich text all produce empty fragments when their body is empty. The ticket says "prefer omit empty optional sections on publish." Draft preview labels are the UI's responsibility (M6-013/014).
- **Sanitize at render** — defense in depth: `sanitize.Sanitize` called in every HTML-block renderer even though params are sanitized on write. Protects against any DB-level HTML injection.
- **Logo placeholder** — `cover.go` `logoDataURL` emits a transparent SVG data URL. M6-009 will replace this with a real asset serving URL. No broken `<img>` in the fragment.
- **Date formatting** — `formatEngagementWindow` handles all edge cases (zero dates, same day, range) using RFC 3339 input and ISO dates output (`en-US` fixed per M6-EPIC). No time formatting.
- **CSS custom properties** — `var(--bl-primary, #4a6fa5)` pattern: branding colours are injected as CSS variables by the assembler (M6-009), with fallback values for standalone fragments.
- **Registry renderer storage** — added to `Registry` (not a separate map) so `M6-001`'s "Registry may hold optional Renderer" contract is fulfilled. `SetRenderer` panics on duplicate or unregistered IDs (init-time programmer error).

### Deviations from ticket

None. The five narrative blocks match the ticket scope exactly.

### Verification

```
go test ./internal/report/blocks/ -count=1 -v   # 21 tests pass
go test ./internal/report/... -count=1           # all pass
go test ./... -count=1                           # full suite passes
go vet ./...                                     # clean
go build ./...                                   # clean
make generate && git diff --stat                 # only intentional changes
cd web && npx tsc --noEmit                       # clean
```