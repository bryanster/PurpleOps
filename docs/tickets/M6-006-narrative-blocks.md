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
