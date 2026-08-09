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
