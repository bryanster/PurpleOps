# M6-014 — Publish / versions / share & guest-grant UI

**Milestone:** M6 · **Size:** M · **Depends on:** M6-011, M6-012, M6-013

## Why

Lead needs a clear path: publish with evidence opt-in, browse immutable versions, mint/revoke share
links, copy claim URL once, confirm password/expiry.

## Scope

**In**

- **Publish dialog** (lead only; hide/disable with reason for non-leads):
  - Checkbox **Include evidence** default off, with warning copy about client-sensitive screenshots.
  - Statement: "Published reports always use full engagement data (not blind-filtered)."
  - On success → version list highlights new version; actions Open HTML / Download PDF.
- **Versions panel:** list published_at, publisher, includeEvidence, sha short; open HTML in new tab;
  download PDF.
- **Share panel** on a version:
  - Create share: optional password, optional expiry; show URL + secret **once** with copy button;
    acknowledge cannot be retrieved again.
  - List shares: status active/revoked/expired; revoke button.
  - List grants (claimed users) + revoke grant.
  - No email integration — helper text "send this link out of band".
- Guest claim page (logged-out → login/register gate → password if needed → success → redirect to
  HTML view).
- Shared HTML view chrome: minimal (firm logo, title, download PDF if allowed) — not full SPA nav
  that exposes other engagements.
- Authz-driven UI: non-leads see versions read-only without share minting.

**Out**

- Full guest admin; converting guest to member; watermarking.

## Files

- `web/src/features/reports/publish/*`, `share/*`, `claim/*`
- Playwright specs for publish + revoke path (partial; full thesis in M6-015)

## Acceptance criteria

- [x] Non-lead does not see active Publish as succeeding control.
- [x] Evidence default off visible in dialog.
- [x] After revoke, opening claim/view URL shows not-found page (404), not forbidden.
- [x] Claim URL copy works; secret not present in subsequent GET share list API.

## Tests

- Component tests for dialog defaults.
- Playwright: lead publish → create share → second browser context as guest/user → view → revoke →
  404 (can be thin precursor to M6-015).

## Notes for the implementer

- Second browser context must not reuse the lead session (Playwright storageState isolation).
- Sanitize any user-facing error detail from claim failures (generic messages).


## Implementation notes

- Used `useCurrentUser()` (not `useSignedInUser()`) in ClaimPage since it runs outside the `RequireAuth` guard. `useSignedInUser()` throws when called outside the guard.
- SharePanel manages token visibility: the share token is shown once via local state (`resultToken`, `resultUrl`), dismissed on user action, and never exposed through the share list API (the server never returns the token after creation).
- Version HTML/PDF download links use `API_BASE_URL` constant (not raw `/api/v1/...` strings) to satisfy the `no-restricted-syntax` ESLint rule.
- Claim and view routes (`/claim/:token`, `/view/:token`) are added outside `RequireAuth` since share endpoints use `security: []`; the claim page handles its own auth gate via `useCurrentUser()`.
- The publish dialog's `handlePublished` callback refetches the report query to keep version count current. VersionsPanel and SharePanel use separate query keys to avoid cross-contamination with the draft report cache.
- Component test covers: publish button presence, evidence checkbox default-off state, non-lead disable, closed-engagement disable, full-data statement visibility.
- Playwright e2e test (`reports-share.spec.ts`) exercises the publish → share → claim → view → revoke → 404 path as specified. It is a thin precursor to M6-015 (full E2E thesis).
