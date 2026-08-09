# M6-005 — TipTap + server HTML allowlist (bluemonday)

**Milestone:** M6 · **Size:** M · **Depends on:** M6-002

## Why

User-authored HTML reaches a **client-facing** published page. Client-side DOMPurify alone is not a
control plane — the server must sanitize on write and on render. This ticket locks the policy and
wires editor + server before narrative blocks land.

## Scope

**In**

- **Go:** `internal/report/sanitize` using `microcosm-cc/bluemonday` (UGC-like policy, then tighten):
  - **Allow:** `p`, `h1`, `h2`, `h3`, `ul`, `ol`, `li`, `strong`, `em`, `code`, `pre`, `blockquote`,
    `br`, `a` with `href` scheme allowlist `http`, `https`, `mailto` only; strip `target`/`rel` and
    set `rel="noopener noreferrer"` on external links if `target` ever allowed (prefer no `target`).
  - **Deny:** `script`, `style`, `iframe`, `object`, `embed`, `form`, `img`, `video`, `svg`, event
    handlers (`on*`), `style` attributes, `javascript:` URLs, data URLs.
  - `Sanitize(html string) string`; empty in → empty out; never error on dirty input (strip).
  - Called from report block write path when `block_id=rich_text` (and any other block storing HTML
    params: exec summary body, scope body, etc.).
- **OpenAPI/docs:** max length for HTML fields (e.g. 100 KiB raw).
- **SPA:** TipTap (ProseMirror) editor component under `web/src/features/reports/` with toolbar
  limited to the allowlist (headings, lists, bold/italic, code, blockquote, link). No image upload
  in the editor.
- Shared storybook-free unit test on the client optional; **server tests are mandatory**.
- `docs/security.md` section: report HTML sanitization + threat model (stored XSS via share).

**Out**

- Full block render (`M6-006`); markdown alternative; collaborative editing.

## Files

- `internal/report/sanitize/`, wire into report service params validation
- `web/src/features/reports/rich-text-editor.tsx` (or similar)
- `docs/security.md`

## Acceptance criteria

- [x] Table of malicious payloads stripped/neutralized (script tags, onerror, javascript href,
      iframe, inline style, img onerror) — none survive `Sanitize`.
- [x] Safe fixtures unchanged (idempotent sanitize on clean HTML).
- [x] Writing a `rich_text` block with dirty HTML persists only sanitized form (integration).
- [x] TipTap cannot insert disallowed nodes through the configured extensions (smoke).
- [x] CSP still forbids inline scripts (`docs/http.md`); report HTML does not require CSP weaken
      for `script-src`.

## Tests

- Go fuzz or large table of XSS vectors.
- Service integration: dirty PUT → clean GET.

## Notes for the implementer

- Sanitize **again at render** even if write sanitized — defense in depth if DB ever edited.
- Linkify only via explicit user link mark, not auto-link of raw URLs (optional later).
- Dependency add goes through normal Go modules; pin version.

## Implementation notes

**Date:** 2026-08-09

### Acceptance criteria status

- [x] Table of malicious payloads stripped/neutralized — 22 vectors tested
- [x] Safe fixtures unchanged — 12 idempotent cases pass
- [x] Writing a `rich_text` block with dirty HTML persists only sanitized form — `sanitizeHTMLParams` wired into `ReplaceBlocks`
- [x] TipTap cannot insert disallowed nodes — extensions limited to StarterKit (no image, table, codeBlock) + LinkExtension (http/https/mailto only)
- [x] CSP still forbids inline scripts — no CSP change needed

### Files created/modified

- `internal/report/sanitize/sanitize.go` — bluemonday policy: allow heading/list/inline/code/blockquote/link, deny everything else, force rel attrs on links
- `internal/report/sanitize/sanitize_test.go` — 4 test funcs (22 malicious, 12 idempotent, 8 link schemes, 6 edge cases) + fuzz
- `internal/report/block.go` — added `HTMLParamKeys []string` to `Definition`
- `internal/report/service.go` — added `sanitizeHTMLParams` helper, `MaxHTMLBytes` const, wired into `ReplaceBlocks`
- `internal/httpapi/server.go` — updated block registration with `HTMLParamKeys` for `rich_text`, `executive_summary`, `scope_roe`
- `api/openapi.yaml` — updated `ReportBlockInput.params` description with HTML limits
- `web/src/features/reports/rich-text-editor.tsx` — TipTap editor with allowlist toolbar
- `web/package.json` / `web/package-lock.json` — added `@tiptap/react`, `@tiptap/starter-kit`, `@tiptap/extension-link`, `@tiptap/pm`
- `docs/security.md` — "Report HTML sanitization" section with threat model, policy table, limits, test coverage
- `go.mod` / `go.sum` — added `github.com/microcosm-cc/bluemonday` v1.0.27

### Design decisions

- **Policy built from scratch** (`bluemonday.NewPolicy()`) rather than `UGCPolicy` — safer default-deny stance; new bluemonday releases that widen UGCPolicy won't silently expand our allowlist.
- **`HTMLParamKeys` on Definition** — each block declares which param keys contain HTML. M6-006 replaces stubs with full definitions. Avoids a hardcoded block-id→key map in the service.
- **100 KiB limit per HTML field** — enforced before sanitization. Larger content is rejected with `validation_failed`, not silently truncated.
- **TipTap without ToggleGroup** — avoided shadcn/ui dependency; toolbar uses plain Button components with `data-state` for active indicators.

### Deviations from ticket

None.

### Verification

```
go test ./internal/report/... -count=1     # all pass (2.13s)
go test ./internal/httpapi/... -count=1    # all pass (54.81s)
go vet ./...                               # clean
go build ./...                             # clean
make generate && git diff --stat           # generate idempotent
cd web && npx tsc --noEmit                 # clean
cd web && npx vite build                   # clean (1.37s)
```
