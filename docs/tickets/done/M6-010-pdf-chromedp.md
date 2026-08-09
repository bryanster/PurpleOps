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

---

## Implementation notes

### Routes added to OpenAPI spec (M6-009 gap fixed)

M6-009 left `POST /preview` (HTML) as an extra route outside the OpenAPI spec.
The request validator (`validate.go`) rejects routes not in the spec, so the
HTML preview was unreachable. This ticket adds both preview routes to the spec:

- `POST /engagements/{id}/reports/{id}/preview` → `text/html` (M6-009)
- `POST /engagements/{id}/reports/{id}/preview.pdf` → `application/pdf` (M6-010)

Both use `report.read` scope, carry `CSRFToken` parameter, and accept optional
`?includeEvidence=true` query param. Handlers were converted from raw
`http.HandlerFunc` to generated `StrictServerInterface` methods.

### chromedp version

Pinned to `v0.14.2` — the last version supporting Go 1.25. Later versions (v0.15+)
require Go ≥ 1.26. The repo uses `GOTOOLCHAIN=local` and cannot auto-upgrade.

### Printer design

Single browser process shared across renders via `chromedp.NewExecAllocator` with
`chromedp.Headless`, `chromedp.NoFirstRun`, `chromedp.NoDefaultBrowserCheck`,
and `chromedp.DisableGPU`. New tab per `RenderPDF` call, serialized by mutex.
Context deadlines bound each render; hung Chrome is killed via context cancellation.

### Sandbox

No compose changes needed — `shm_size: 512mb` and `init: true` were already set
by M0B-011. Chromium sandbox is ON; operators needing it disabled use documented
options in `docs/deploy.md`. `BLACKLIGHT_CHROME_PATH` validates the binary at
server start (prints warning if missing, server continues without PDF support).
PDF endpoint returns 503 with clear message when printer is nil.

### Test note

`TestNoHandlerDecidesForItself` was already failing before this ticket (imports
`authz` in `reporthandlers.go`). Not caused by or fixed in this work.
