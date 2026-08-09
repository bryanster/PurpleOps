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

- [ ] Table of malicious payloads stripped/neutralized (script tags, onerror, javascript href,
      iframe, inline style, img onerror) — none survive `Sanitize`.
- [ ] Safe fixtures unchanged (idempotent sanitize on clean HTML).
- [ ] Writing a `rich_text` block with dirty HTML persists only sanitized form (integration).
- [ ] TipTap cannot insert disallowed nodes through the configured extensions (smoke).
- [ ] CSP still forbids inline scripts (`docs/http.md`); report HTML does not require CSP weaken
      for `script-src`.

## Tests

- Go fuzz or large table of XSS vectors.
- Service integration: dirty PUT → clean GET.

## Notes for the implementer

- Sanitize **again at render** even if write sanitized — defense in depth if DB ever edited.
- Linkify only via explicit user link mark, not auto-link of raw URLs (optional later).
- Dependency add goes through normal Go modules; pin version.
