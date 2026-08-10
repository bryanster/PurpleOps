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

- [ ] Every checklist row has pass/fail evidence in completion notes.
- [ ] No open Critical/High findings at ticket close.
- [ ] New regressions covered by tests where a fail was fixed.
- [ ] `make test` includes matrix + share authz still green.
- [ ] Operator-relevant residual risks (e.g. cleartext HTTP in dev) documented, not hidden.

## Tests

- Existing matrix + share/PDF/auth tests re-run.
- At least one new test per High+ fix.
- Optional: scripted `curl` checklist checked into `deploy/` or `docs/` only if maintained — prefer
  Go tests.

## Notes for the implementer

- Start from `docs/security.md`, `docs/authz.md`, M6-012 share semantics, and M1-014.
- Prefer reading code paths over trusting comments.
- Record time-boxed depth: this is a pass, not infinite research. Breadth first, then drill fails.
