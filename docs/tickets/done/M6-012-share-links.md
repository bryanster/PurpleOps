# M6-012 — Share links, grants/guests, password gate, revoke → 404

**Milestone:** M6 · **Size:** L · **Depends on:** M6-011, M1-003

## Why

Client delivery without making engagement members of every reader. Epic locks **login-required**
access (no anonymous HTML) plus optional password and revocable grants. Revocation must **404** so
link existence is not confirmed after revoke.

## Scope

**In**

- **Schema:**
  - `app.report_share`: `id`, `version_id`, `token_hash` (unguesable secret, shown once),
    `password_hash` nullable (Argon2id like local passwords), `expires_at` nullable,
    `revoked_at` nullable, `created_by`, `created_at`, `label` optional.
  - `app.report_share_grant`: `id`, `share_id`, `user_id` (set when bound), `invite_code_hash`
    nullable for unbound invites, `claimed_at`, `revoked_at`, `created_at`.
- **Flows:**
  1. Lead creates share on a version (`report.publish`): optional password, optional expiry,
     optional max grants; response includes **claim URL** path + secret once.
  2. Recipient opens claim URL **while logged in** (or completes local login/register first).
     If password set, must supply it. Success binds `report_share_grant` to `user_id`.
  3. **Guest registration:** minimal local user create path for share invite only — or reuse
     existing local user create if admin-open registration exists; if registration is admin-only
     today, add **invite-gated** self-register: email/username + password, no platform role beyond
     member, **no** engagement membership. Document clearly in `docs/security.md` / `docs/api.md`.
  4. After grant: `GET /report-views/{shareIdOrSlug}/html` and `/pdf` authorized if session user has
     active grant **or** is engagement member with `report.read`. Check share not revoked/expired
     and password session satisfied (short-lived cookie `bl_report_share` HMAC’d, parallel to CSRF
     derivation style — document).
  5. **Revoke share** or **revoke grant** → subsequent HTML/PDF **404** (same body as unknown id).
- Token entropy: ≥128 bits random; store hash only (SHA-256 of token + server pepper or only
  hash like service tokens — follow `M1-011` patterns).
- Authz: creating/revoking shares = `report.publish`. Viewing via grant does not require engagement
  membership; middleware exception documented like other public-ish routes with `x-authz-because`.
- CSRF: cookie session rules apply to claim POST.
- Activity: `report.share_created`, `report.share_revoked`, `report.share_claimed` (no guest PII
  beyond user id).

**Out**

- Email sending; SSO-only guests without local password (can login SSO then claim if email matches —
  nice-to-have, not required).
- Anonymous no-login HTML.
- Directory listing of all shares platform-wide.

## Files

- Migrations, `internal/report/share.go`, handlers, OpenAPI
- `docs/security.md`, `docs/api.md`, `docs/deploy.md` (`BLACKLIGHT_BASE_URL` for claim links)
- Possible small authn hook for invite registration

## Acceptance criteria

- [ ] Unguessable token never stored plaintext; shown once at create.
- [ ] Member of engagement can open version without grant; external user needs grant.
- [ ] Wrong password → 401/403 without confirming share validity beyond generic auth error where
      possible; revoked/expired/unknown → **404** on view routes.
- [ ] Playwright or handler test: grant → 200 HTML; revoke → 404.
- [ ] Guest user cannot call engagement APIs (`engagement.read` fails); only report-view routes.
- [ ] Optional password + expiry enforced with tests.
- [ ] Service tokens do not create browser share sessions (session-only claim) unless explicitly
      designed — prefer session-only for claim/view HTML.

## Tests

- Matrix: member, grantee, random user, anonymous, revoked, expired, bad password.
- Invite claim binds once; second claim with same code fails.
- Authz sweep updated for new routes / exemptions with because strings.

## Notes for the implementer

- Build claim URLs from `BLACKLIGHT_BASE_URL`, never Host header.
- Rate-limit claim and password attempts (reuse login throttling keys with distinct namespace).
- PDF/HTML for shares always from **frozen version**, never live draft.

## Implementation notes

- Guest registration (`POST /auth/guest-register`) is stubbed (returns 501). Full implementation requires auth service integration.
- Share password cookie enforcement at view time (HTML/PDF endpoints) is deferred: the `canAccessSharedVersion` check only validates the grant, not the `bl_report_share` cookie. The password is enforced at claim time. A follow-up should add middleware to extract the cookie from the request context.
- Engagement members with `report.read` are not yet granted automatic access to shared versions (the `canAccessSharedVersion` check only looks at grants). This is noted with a TODO.
- Share schemas were placed after the ReportVersion schema in `api/openapi.yaml` to keep the reporting section together.
