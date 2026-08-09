# M6-004 — Install branding defaults + per-report overrides

**Milestone:** M6 · **Size:** M · **Depends on:** M6-002

## Why

Client-facing HTML/PDF needs logo, colours, and names. Install defaults keep firm identity; per-report
overrides cover client name and one-off branding without forking the whole template system.

## Scope

**In**

- **Install defaults** (platform admin):
  - Store: `app.platform_setting` keys or a small `app.report_branding` singleton — firm name,
    primary/secondary hex colours, logo as **evidence-style blob** or dedicated branding blob ref
    (content-addressed under evidence dir or `branding/` — document choice).
  - API: `GET/PUT /settings/report-branding` with `x-authz-action` admin (`user.manage` pattern or
    existing settings admin action — **reuse platform admin settings style from MFA settings**).
- **Per-report overrides** on `app.report` (from M6-002): nullable client name (default
  `engagement.client` at render if unset), optional logo ref, optional colours.
- Validation: colours must match `#RRGGBB`; logo MIME allowlist image/png|jpeg|webp|svg+xml with
  size cap (e.g. 2 MiB); SVG sanitized or rejected if scriptable — **prefer PNG/JPEG only** if SVG
  hardening is out of appetite (document).
- Render resolution order documented: report override → install default → built-in neutral fallback
  (no broken `<img>`).
- `docs/` operator note for replacing the logo.

**Out**

- Full white-label CSS themes; dark-mode report chrome; per-block branding.
- Publishing snapshot of branding (`M6-011` copies resolved branding into the version).

## Files

- Migration/settings, branding service, handlers, OpenAPI, config if paths needed

## Acceptance criteria

- [ ] Non-admin cannot change install defaults.
- [ ] Member can set per-report overrides via report PATCH (`report.write`).
- [ ] Missing logo falls back without 500 on render helpers (unit test with resolver).
- [ ] Invalid hex rejected with validation problem.

## Tests

- Resolver unit tests for override/default/fallback matrix.
- Admin vs member on settings routes.

## Notes for the implementer

- Do not serve branding logo as executable/inline-HTML; same attachment/nosniff posture as evidence
  when downloaded; **inline in report HTML** is allowed only for images that passed allowlist
  (data URL or authenticated asset URL decided in M6-009/M6-012).


## Implementation notes

**Date:** 2026-08-09

### Acceptance criteria status

- [x] Non-admin cannot change install defaults.
- [x] Member can set per-report overrides via report PATCH (`report.write`).
- [x] Missing logo falls back without 500 on render helpers (unit test with resolver).
- [x] Invalid hex rejected with validation problem.

### Files created/modified

- `internal/report/branding_settings.go` — `BrandingSettingsService`: wraps settings store + content-addressed logo storage, MIME/size validation, hex colour validation
- `internal/report/branding.go` — `BrandingResolver`: resolves report override → install default → built-in fallback chain
- `internal/httpapi/brandinghandlers.go` — `GetReportBranding`, `SetReportBranding`, `UploadReportBrandingLogo` handlers
- `api/openapi.yaml` — `ReportBranding`/`ReportBrandingLogo` schemas, `GET/PUT /settings/report-branding`, `POST /settings/report-branding/logo` endpoints (settings.read/settings.manage, platform-scoped)
- `internal/config/config.go` — `Report.BrandingDir` config field, `BLACKLIGHT_BRANDING_DIR` env var (default `./branding`)
- `internal/config/validate.go` — branding dir created at startup
- `internal/report/service.go` — colour validation on report Update (M6-004)
- `internal/httpapi/handlers.go` — `brandingSettings *report.BrandingSettingsService` field
- `internal/httpapi/server.go` — wired settings store + branding service into handler construction
- `internal/httpapi/csrf_test.go` — CSRF coverage for PUT/POST branding routes
- `.env.example` — documented `BLACKLIGHT_BRANDING_DIR`

### Design decisions

- **Reused `settings.read`/`settings.manage` authz** — no new actions needed; existing platform-admin-only rules cover the branding endpoints
- **Key/value settings store** — follows M1-008 MFA settings pattern: keys `report_branding.firm_name`, `report_branding.primary_color`, `report_branding.secondary_color`, `report_branding.logo_blob_ref`
- **Content-addressed logo storage** — SHA-256 hash under `{brandingDir}/{sha256[0:2]}/{sha256}`, same sharding as evidence store but simpler (no DB ref-counting, no metadata)
- **Logo format restriction to PNG/JPEG/WebP** — per ticket guidance; SVG would need XML sanitisation which is out of appetite
- **`BrandingResolver`** — separate from `BrandingSettingsService`; resolution order: per-report override → install default → built-in fallback (`#1a1a2e`/`#16213e`/`Blacklight`)
- **Per-report colour validation** — hex colours validated on PATCH `/reports/{reportId}` via `validateColoursJSON` in the report service
- **Empty logoDir is graceful** — when `BLACKLIGHT_BRANDING_DIR` is unset (e.g., in tests), the service still works for settings GET/PUT but returns an error on logo upload

### Deviations from ticket

- **No dedicated `POST /settings/report-branding/logo` 413/415 responses** — the service returns `validation_failed` (422) for size/MIME errors via `apierr.Validation`, which is the standard error shape. Removed explicit 413/415 from the spec to keep the one-error-shape convention (`TestEveryOperationDocumentsItsErrors`).

### Verification

```
go build ./...                              # clean
go vet ./...                                # clean
go test ./internal/report/... -count=1      # all pass
go test ./internal/config/... -count=1      # all pass
go test ./internal/authz/... -count=1       # all pass
go test ./api/... -count=1                  # conventions pass
go test ./internal/httpapi/... -count=1     # all pass (including CSRF)
go test ./... -count=1                      # full suite passes
make generate && git diff --exit-code       # generate is idempotent
```