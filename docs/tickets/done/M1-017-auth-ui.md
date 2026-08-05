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

- [x] Login errors are specific enough to act on but never reveal whether an account exists — the
      copy for wrong-password and unknown-email is identical (`M1-003`).
- [x] Recovery codes are shown exactly once, with copy and download, and a confirmation checkbox
      ("I've saved these") before proceeding.
- [x] The token secret is shown once with the same treatment.
- [x] The forced-enrolment screen cannot be escaped by editing the URL — assert in E2E.
- [x] `return_to` after login accepts only relative in-app paths (no open redirect, matching
      `M1-009`).
- [x] Every screen has real loading, empty, and error states — no bare spinners-forever, no blank
      tables. Error states show the request ID from `M0B-007` so a user can quote it.
- [x] Keyboard-navigable end to end; focus moves into dialogs and returns on close; the MFA input is
      focused on mount.
- [x] Screen-reader labels on every input; errors associated with their fields via `aria-describedby`.
- [x] Works in light and dark themes at 1280px and 768px.
- [x] No API URL strings in components — hooks only (`M0B-009`).
- [x] Admin nav is absent for non-admins **and** the route is guarded — hiding alone is not access
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

## Implementation notes

**`api/openapi.yaml` · `internal/authz/` · `internal/authn/sessions.go` · `internal/httpapi/sessionhandlers.go` ·
`web/src/features/{auth,account,admin,tokens}/` · `e2e/specs/auth.spec.ts`**

### The sessions panel needed an API that did not exist

The scope asks for "active sessions list with revoke, sign out everywhere" on the account screen.
There was no endpoint for any of it: `POST /users/{userId}/sessions/revoke` is `user.manage`, so an
ordinary member could not even reach their own. **Agreed before it was written**, three operations
were added rather than dropping the panel:

| Operation | Authz |
|---|---|
| `GET /auth/sessions` | `session.read` |
| `DELETE /auth/sessions/{sessionId}` | `session.manage` |
| `POST /auth/sessions/revoke-others` | `session.manage` |

`x-authz-self` was not available for these: `api.Requirements` refuses it on an operation with a
path parameter, which is the rule that keeps "acts on your own account" from being a claim anybody
can make about `/users/{userId}`. So `ResourceSession` and two actions joined `internal/authz`,
modelled on the service-token pair they read almost identically to — everybody holds them, over
their own rows only, and `GuardSessionOnly` keeps a service token out. `docs/security.md` has the
four properties that matter; the store layer already had `ListByUser`, `Revoke` and
`RevokeOthersForUser`, so no migration was involved.

`session.Manager.Live` filters with `usable`, the same function `Resolve` applies to the cookie, so
the list cannot disagree with what would actually authenticate. `RevokeOwned` scopes the lookup to
the caller before revoking, so somebody else's session identifier is a 404 exactly as an invented
one is.

### The recovery codes were being thrown away by the route guard

The end-to-end suite caught this, and it is the bug worth reading about. `useConfirmTotp` invalidated
`auth/me` on success, as every other privilege-changing mutation does. But the forced-enrolment
screen renders *under the guard that reads that query*: the refetch flipped `mfa.enrolled` to true,
the guard redirected to the application, and the ten recovery codes — which exist in exactly one
response and are then unrecoverable — were unmounted before the person could save them.

The fix is that confirming invalidates nothing; `useMarkEnrolmentComplete` does it at the point the
person says they have saved the codes. `enrolment-page.test.tsx` renders the page under the real
guard with a fake server that changes its answer mid-flow, which is the only arrangement in which
the regression is reachable.

### Deviations and deliberate omissions

- **No platform MFA policy screen.** `GET/PUT /settings/mfa` has no interface; the scope lists a
  per-user "MFA-enforced toggle" and that is what `/admin/users` has. Turning the requirement on for
  the whole installation is still an API call. Worth a follow-up ticket, not a widening of this one.
- **`ApiError` gained `retryAfterSeconds`**, read from the `Retry-After` header in `errors.ts`. The
  lockout message is required to say when it lifts, and the header is where that number is.
- **The 401 handling was split.** A 401 from the session query is the ordinary answer for a
  signed-out browser and belongs to the route guard, which redirects in-router and keeps the
  destination; a 401 from anything else is a session dying mid-use and still triggers the document
  navigation in `query-provider.tsx`. `SESSION_QUERY_KEY` lives in `query-client.ts` so both halves
  name the same query.
- **`OtpInput` has no `maxLength`.** The browser applies it before the component strips non-digits,
  so a code pasted as `492 817` — which is how several authenticators render one — arrived as five
  digits. A component test covers the paste.
- **The smoke spec now signs in.** Every screen but the sign-in ones is behind a session, so
  `e2e/harness/auth.ts` seeds an administrator and signs in through the real form. `harness/totp.ts`
  computes codes so the suite can get *through* an enrolment rather than only up to one; it is a
  second implementation on purpose, since two agreeing is the point.
- **Sessions are unpaginated**, like `GET /auth/tokens`: one person's own credentials, bounded by
  how many browsers they use, and a page boundary in the middle of "where am I signed in" would be a
  list somebody could act on half of.
