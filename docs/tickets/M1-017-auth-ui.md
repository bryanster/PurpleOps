# M1-017 — Login, MFA, account and admin UI

**Milestone:** M1 · **Size:** L · **Depends on:** M1-003 … M1-016, M0B-009

## Why

M1's backend is invisible without this. It is also where the security decisions become usable or
hated: a forced-MFA screen that doesn't explain itself, or a login that says "an error occurred",
turns correct security into support tickets.

## Scope

**In**

- **Login** (`/login`): email + password, SSO buttons rendered only for configured and healthy
  providers, clear error states, throttle/lockout state showing when to retry (`M1-004`).
- **MFA challenge**: 6-digit input with paste support and auto-submit, "use a recovery code
  instead", clear expiry messaging for the pending state (`M1-006`).
- **Forced enrolment** (`M1-008`): blocking screen — QR code, manual secret, confirm field, then
  recovery codes shown **once** with copy and download. No way to navigate past it; the app shell
  is not rendered around it.
- **Account settings** (`/settings/account`): display name, change password, MFA status (enrol,
  disable when allowed, regenerate recovery codes), active sessions list with "revoke", "sign out
  everywhere".
- **Service tokens** (`/settings/tokens`): create with name/scopes/expiry/optional engagement, show
  secret once with an unmissable "you won't see this again" warning, list with prefix and last-used,
  revoke with confirmation.
- **Admin → Users** (`/admin/users`, admin only): list with search and filters, create, edit role
  and status, disable, revoke sessions, MFA-enforced toggle. Destructive actions confirm and say
  what will happen (e.g. "this signs them out of 3 sessions").
- **Admin → Activity** (`/admin/activity`): paginated feed with actor/verb/object filters.
- Route guarding driven by `GET /auth/me`: unauthenticated → login (preserving the intended
  destination as a **relative** path only), non-admin → no admin nav entries and a 403 page if
  reached directly.
- Every mutation is a TanStack Query mutation with correct cache invalidation and optimistic UI only
  where safe (never for anything security-relevant).

**Out**

- Engagement UI (M3+).
- Password reset by email. There is no mail transport; the admin resets via `M1-016`.

## Acceptance criteria

- [ ] Login errors are specific enough to act on but never reveal whether an account exists — the
      copy for wrong-password and unknown-email is identical (`M1-003`).
- [ ] Recovery codes are shown exactly once, with copy and download, and a confirmation checkbox
      ("I've saved these") before proceeding.
- [ ] The token secret is shown once with the same treatment.
- [ ] The forced-enrolment screen cannot be escaped by editing the URL — assert in E2E.
- [ ] `return_to` after login accepts only relative in-app paths (no open redirect, matching
      `M1-009`).
- [ ] Every screen has real loading, empty, and error states — no bare spinners-forever, no blank
      tables. Error states show the request ID from `M0B-007` so a user can quote it.
- [ ] Keyboard-navigable end to end; focus moves into dialogs and returns on close; the MFA input is
      focused on mount.
- [ ] Screen-reader labels on every input; errors associated with their fields via `aria-describedby`.
- [ ] Works in light and dark themes at 1280px and 768px.
- [ ] No API URL strings in components — hooks only (`M0B-009`).
- [ ] Admin nav is absent for non-admins **and** the route is guarded — hiding alone is not access
      control.

## Tests

- Component tests (Vitest + MSW): login success/failure/throttled, MFA verify, recovery-code path,
  forced enrolment, token creation showing the secret once.
- E2E (`M0B-013`), extending the suite toward `PLAN.md` §9's full spec:
  1. `blctl` creates the first admin; admin logs in.
  2. Admin creates a member; member logs in and is forced through MFA enrolment.
  3. Member is denied `/admin/users` in the UI and at the API.
  4. Admin revokes the member's sessions; the member's next action lands on login.
  5. A service token is created, used against the API with `curl`-equivalent, then revoked and shown
     to fail.
