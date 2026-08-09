# M6-010 — PDF via headless Chromium (`chromedp`)

**Milestone:** M6 · **Size:** M · **Depends on:** M6-009, M0B-011

## Why

Operators need a file to send. One HTML → PDF path avoids a second layout engine. Chromium is already
in the image; compose sandbox was left undecided in `M0B-011` — this ticket closes that.

## Scope

**In**

- `internal/report/pdf` using `chromedp`:
  - Input: HTML bytes (and base URL or temp file) from the shared renderer.
  - Output: PDF bytes; config timeouts; page size A4 (or Letter — **pick A4**, document).
  - `BLACKLIGHT_CHROME_PATH` already exists; fail with clear operator error if binary missing.
- **API:**
  - `GET or POST /engagements/{id}/reports/{reportId}/preview.pdf` — draft PDF, `report.read`,
    same seat scope as HTML preview.
  - Published version PDF in `M6-011`/`M6-012` can reuse the printer on stored HTML.
- **Deploy:** resolve Chromium sandbox for `docker compose` — choose and document one default that
  **works out of the box** for PDF (likely narrow seccomp or documented `seccomp:unconfined` with
  threat note). Update `docs/deploy.md` and compose as needed.
- **CI:** smoke test — render fixture HTML to PDF, assert `%PDF` magic, page count ≥ 1 via a light
  PDF parse or chromedp metrics; **no** pixel goldens. Skip or short-circuit if Chrome absent with
  explicit build tag/`testing.Short` policy matching M0B-013 patterns — prefer run in CI image that
  has Chromium.
- Document browser print-to-PDF fallback for bare metal without Chrome.

**Out**

- DOCX/PPTX; headers/footers editor; PDF/A archival certification.

## Files

- `internal/report/pdf/pdf.go`, tests
- handlers, OpenAPI, `docs/deploy.md`, `compose.yml` if sandbox default changes

## Acceptance criteria

- [ ] `docker compose up` can produce a PDF without manual seccomp research (smoke script or test).
- [ ] Missing Chrome → problem response/log with `BLACKLIGHT_CHROME_PATH` hint, not panic.
- [ ] Timeout bounded (config); hung Chrome killed.
- [ ] Concurrent PDFs do not deadlock the process (limit worker concurrency if needed).

## Tests

- Unit/integration smoke with fixture HTML.
- Optional: page_break block increases page count vs without (best-effort).

## Notes for the implementer

- Reuse one browser context pool carefully; process leak is the common bug.
- Never pass unsanitized user HTML that wasn't produced by our renderer.
- Print background graphics enabled so heatmap colours survive.
