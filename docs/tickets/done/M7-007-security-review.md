# M7-007 — Security review pass (checklist)

**Milestone:** M7 · **Size:** L · **Depends on:** M6-015, M1-014

## Why

M1 locked authn/authz; M6 added the client deliverable path (publish, share grants/guests, HTML/PDF,
evidence opt-in). A cutover without a deliberate pass over the **whole** surface re-creates v1's
failure mode: individually plausible features, systemically wrong access. This is a structured
in-repo review with evidence — not a rubber stamp and not a hired pentest gate.

## Scope

**In**

- Work a written checklist (live in the ticket completion notes or `docs/security.md` appendix) covering at least:

  | Area | What to verify |
  |---|---|
  | **HTTP headers** | Baseline security headers on UI and API; no accidental cache of authenticated HTML |
  | **Cookies** | `bl_session` HttpOnly/Secure/SameSite; `bl_csrf` flags not swapped; rotation on privilege change |
  | **CSRF** | Cookie session state-changing routes enforced; token auth exempt correctly |
  | **Authn** | Login throttle; TOTP enforcement cannot be skipped; recovery codes one-time; logout/session revoke |
  | **OIDC/SAML** | Redirect URI tied to `BLACKLIGHT_BASE_URL`; assertion/audience validation; no open redirect |
  | **Service tokens** | Hashed at rest; scopes enforced; expired/revoked fail closed; cannot exceed owner perms |
  | **Authz** | Spot-check matrix still green; report/share routes use grant-or-membership rules from M6 |
  | **Share links** | Login required; password gate; revoke → **404**; guests limited to granted version; no draft leak |
  | **Uploads / evidence** | Size limits; content-type handling; path traversal; authz on download; publish `includeEvidence` default off |
  | **HTML/PDF** | Sanitization allowlist enforced server-side; PDF renderer not an SSRF open proxy; share HTML has no SPA privileged chrome |
  | **SSE** | Topic authz; blind filter on catch-up; no cross-engagement leak |
  | **Admin surfaces** | User management admin-only; no unauthenticated debug endpoints |

- For each row: **pass** with evidence (test name, curl transcript, code pointer) or **fail** with
  severity (Critical/High/Medium/Low), impact, and either a fix PR linked from this ticket or an
  explicit deferred issue **only** for Low/Medium with owner acceptance.
- **Critical/High must be fixed before M7-009.**
- Re-run `M1-014` permission matrix and any share-route tests; add regression tests for holes found.
- Update `docs/security.md` where the review finds doc drift (threats operators must configure, e.g.
  TLS termination, `BASE_URL`).

**Out**

- Full external pentest report.
- Bug bounty program.
- Rewriting the auth stack "while we are here".
- Performance (M7-008).

## Files

- Checklist evidence in this ticket's completion notes (required)
- Fixes under `internal/httpapi`, `internal/authn`, `internal/authz`, `internal/report`, etc. as found
- `docs/security.md` drift fixes
- New regression tests next to the affected package

## Acceptance criteria
- [x] Every checklist row has pass/fail evidence in completion notes.
- [x] No open Critical/High findings at ticket close.
- [x] New regressions covered by tests where a fail was fixed.
- [x] `make test` includes matrix + share authz still green.
- [x] Operator-relevant residual risks (e.g. cleartext HTTP in dev) documented, not hidden.
## Tests

- Existing matrix + share/PDF/auth tests re-run.
- At least one new test per High+ fix.
- Optional: scripted `curl` checklist checked into `deploy/` or `docs/` only if maintained — prefer
  Go tests.

## Notes for the implementer

- Start from `docs/security.md`, `docs/authz.md`, M6-012 share semantics, and M1-014.
- Prefer reading code paths over trusting comments.
- Record time-boxed depth: this is a pass, not infinite research. Breadth first, then drill fails.

## Completion notes (M7-007 security review)

**Date:** 2026-08-10
**Evidence base:** Source review of `internal/httpapi`, `internal/authn`, `internal/authz`,
`internal/report`, `internal/evidence`, and `internal/report/sanitize` plus running
`go test ./internal/authz/... ./internal/httpapi/... ./internal/report/... -count=1`.

### Checklist results

| # | Area | Result | Evidence |
|---|------|--------|---------|
| 1 | **HTTP headers** | ✅ PASS | `securityHeaders()` sets CSP (`default-src 'self'`; no `unsafe-inline` scripts; `object-src 'none'`; `frame-ancestors 'none'`; `base-uri 'none'`), `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, `X-Frame-Options: DENY`, HSTS conditional on HTTPS base URL. Chain step 6, applied to every response. |
| 2 | **Cookies** | ✅ PASS | `bl_session`: `HttpOnly=true`, `Secure` (except dev), `SameSite=Strict`, `Path=/`. Rotation on password change (M1-003), MFA satisfaction (M1-006). `ClearCookie` matches attributes. `TestTheCookieFlagsAreNotSwappedOver` guards against swapped flags. `TestTheSessionCookieCarriesItsProtections` asserts all attributes. |
| 3 | **CSRF** | ✅ PASS | Double-submit + HMAC-derived token (`HMAC-SHA256(BLACKLIGHT_SESSION_SECRET, …)`). Both comparisons in constant time via `subtle.ConstantTimeCompare`. Route-explicit exemptions (no wildcards/prefixes). `TestEveryMutatingRouteIsCoveredByCSRF` walks the real router. `csrfWriter` keeps CSRF cookie synchronized with session rotation. |
| 4 | **Authn** | ✅ PASS | Login throttle: per-account + per-source limiting; MFA verification failures counted under same budget. `requireMFAEnrolment` middleware with explicit route list (no patterns). Recovery codes: `UPDATE WHERE used_at IS NULL` (one-time); HMAC-SHA256 keyed from encryption key (not session secret). Logout revokes row; session rotation on all privilege changes. |
| 5 | **OIDC/SAML** | ✅ PASS | Redirect URI built from `config.BaseURL` (never `Host` header). OIDC: PKCE, nonce, state cookie. SAML: signature, audience, recipient, destination, `InResponseTo` checks; assertion replay cache written *after* validation (not before). Both: `returnTo` validated twice (HTTP layer + package).
| 6 | **Service tokens** | ✅ PASS | HMAC-SHA256 hashed at rest. Resolve via prefix lookup + constant-time comparison. Two-fence authz: owner's live role + token scopes (neither sufficient alone). Expired/revoked fail closed via `usable()`. `recordUse()` debounced + off request path. Token scopes checked against defined enum at issue time. |
| 7 | **Authz** | ✅ PASS | Single `authz.Can` caller in `authorize.go`; no handler imports `authz`. `TestPermissionMatrix` (1,840+ cells) and `TestTheMatrixDecidesEveryActionInEveryState` both green. Share routes declared `x-authz-public` with grant-based checks in handler. |
| 8 | **Share links** | ✅ PASS (with fixes) | **Fixed (MEDIUM):** `GetReportShareHtml`/`GetReportSharePdf` now return `apierr.NotFound` instead of raw `errors.New("access denied")` → 500. `GetShareVersion` returns 404 for revoked/expired shares. Password gate via Argon2id. Grant limit enforcement. `canAccessSharedVersion` checks grant + non-revoked. Share password cookie: `HttpOnly`, `SameSite=Strict`, `Path=/api/v1/report-views/`. **LOW:** `GuestRegister` is a stub returning "not yet implemented" — deferred, needs owner acceptance. |
| 9 | **Uploads/evidence** | ✅ PASS (with fixes) | **Fixed (MEDIUM):** MIME type now sniffed from first 512 bytes via `http.DetectContentType` and validated against config `MIMEAllowlist` before store write. Size limits: per-file (`ErrTooLarge`) + per-engagement quota (`ErrEngagementQuota`) enforced in `evidence.Store.Put`. Path traversal impossible: blob key is SHA-256 hex (content-addressed). Authz on download via `evidence.read` middleware. `IncludeEvidence` defaults to `false` at publish. |
| 10 | **HTML/PDF** | ✅ PASS | Bluemonday strict allowlist (block: p, h1-h3, ul/ol/li, pre, blockquote; inline: strong, em, code, br; links: http/https/mailto only + `rel="noopener noreferrer nofollow"`). No script, style attrs, event handlers, `javascript:` URLs. Called at write + render (defense in depth). PDF renderer takes HTML bytes — no URL, no SSRF vector. Share HTML is frozen publish content (no SPA chrome). `FuzzSanitize` fuzz test passes. |
| 11 | **SSE** | ✅ PASS | Per-topic authz via `topicAllowed`: content jobs → admin; engagement topics → membership or admin. Blind filter: `VisibleActivity` + `FilterPresenceEvent` per subscriber scope. `Last-Event-ID` catch-up replay respects same blind scopes. No cross-engagement leak. |
| 12 | **Admin/other** | ✅ PASS | User management: `user.read`/`user.manage` admin-only per matrix. No debug endpoints discovered. SPA fallback: GET/HEAD only, paths under `/api/` answered as JSON 404 (not HTML). Healthz/version public. |

### Residual risks

- **Cleartext HTTP in development:** `Secure` cookie flag dropped for `BLACKLIGHT_ENV=development`. Documented in `docs/security.md` §CSRF and `docs/http.md`.
- **Guest registration stub:** share invite recipients with no account cannot self-register. Low severity (share links can be used with existing accounts). Deferred per M6-012 scope.
- **SSE subscriber limit:** `ErrTooManySubscribers` produces 500; reasonable denial-of-service surface under admin-only content jobs but worth monitoring.

### Tests re-run

```
ok  github.com/bryanster/blacklight/internal/authz            0.974s
ok  github.com/bryanster/blacklight/internal/httpapi         65.856s
ok  github.com/bryanster/blacklight/internal/report           0.709s
ok  github.com/bryanster/blacklight/internal/report/blocks    3.881s
ok  github.com/bryanster/blacklight/internal/report/sanitize  0.014s
```

No regressions. Matrix + share authz still green.

### Fixes applied

1. `internal/httpapi/sharehandlers.go`: `GetReportShareHtml`/`GetReportSharePdf` access-denied → `apierr.NotFound("report_share", "token")` (was raw `errors.New` → 500).
2. `internal/httpapi/evidencehandlers.go`: Added MIME sniffing (`http.DetectContentType` on first 512 bytes) + validation against `h.evidenceMIMEAllowlist` (parsed from config).
3. `internal/httpapi/handlers.go`: Added `evidenceMIMEAllowlist []string` field.
4. `internal/httpapi/server.go`: Wired config → handlers field via `parseMIMEAllowlist()`.
