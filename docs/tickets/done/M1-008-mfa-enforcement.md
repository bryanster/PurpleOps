# M1-008 — Admin-enforced MFA

**Milestone:** M1 · **Size:** M · **Depends on:** M1-006, M1-007

## Why

The exact defect from `PLAN.md` §4:

> today `MFA=True` only redirects users who already enrolled, so anyone who skips `/mfa/register`
> logs in with a password alone.

The bug is that enforcement was checked against *enrolment state* instead of against *policy*. The
fix is that policy is evaluated first, and an unenrolled user under policy gets a session that can
do exactly one thing: enrol.

## Scope

**In**

- Policy at two levels: a platform-wide setting (`mfa_required_for_all`, and separately
  `mfa_required_for_admins`) plus the per-user `mfa_enforced` flag from `M1-001`. Effective
  requirement is the OR of the applicable ones.
- Settings storage + admin endpoints to read/update the platform policy (admin-only via `M1-013`).
- Login outcomes become a small state machine — implement it explicitly, don't scatter booleans:

  | Password valid | MFA required | Enrolled | Result |
  |---|---|---|---|
  | ✗ | — | — | 401 |
  | ✓ | no | — | full session |
  | ✓ | yes | yes | pending → verify → full session |
  | ✓ | yes | **no** | **enrolment-only session** |

- The enrolment-only session may reach *only* the TOTP enrol/confirm endpoints and `GET /auth/me`.
  Everything else is 403 with a `code` the UI can act on (e.g. `mfa_enrolment_required`).
- Enabling the policy while users are logged in: existing sessions without `mfa_satisfied` are
  downgraded to enrolment-only at their next request, not silently grandfathered.
- SSO users (`M1-009`, `M1-010`) are exempt when the IdP asserts an MFA claim — define the behaviour
  explicitly even if the initial answer is "SSO users are exempt from local MFA policy", and write
  it in `docs/security.md`.
- UI: a blocking enrolment screen, no dismiss, no way to navigate past it.

**Out**

- Per-engagement MFA requirements. Platform-level only.

## Acceptance criteria

- [x] **Regression case (must exist by name):** policy on, user never enrolled → they cannot reach
      any application endpoint with a password alone. Name the test after this ticket.
- [x] An enrolment-only session is 403 on a normal API call, with a distinguishable `code`.
- [x] After completing enrolment, the session is rotated and becomes fully privileged in one step —
      no re-login required.
- [x] Turning on `mfa_required_for_all` immediately affects already-logged-in users on their next
      request.
- [x] A user with `mfa_enforced` set individually is required even when the platform policy is off.
- [x] An admin cannot remove their own MFA while a policy requires it — the endpoint refuses, rather
      than leaving them enforced-but-unenrolled.
- [x] Turning the policy **off** does not delete anyone's enrolment.
- [x] The last remaining admin cannot lock the platform out of itself; if a path exists, it's
      `blctl user reset-mfa` (`M1-007`) and it's documented.

## Tests

- Table-driven over the state machine above — every row, plus the individually-enforced variant.
- Session-downgrade test: session created before the policy, request after.
- Enrolment-only scope test enumerating a representative set of endpoints.
- E2E (`M0B-013`): admin turns on enforcement, second user logs in and is forced through enrolment.

---

## Implementation notes

### There are three answers, not two, and the third is a 401

The ticket's state machine has one row for "required and not satisfied". Live sessions have two,
because a session that never presented a factor can belong to somebody who *has* one — sign in on
two browsers, enrol in one, and the other is holding exactly that when the policy goes on.

Confining that session to enrolment would be a dead end: `POST /auth/mfa/totp/enroll` refuses with
409 while a confirmed authenticator exists, so the screen it was sent to could not do anything.
Letting it through would mean the policy not applying to the people best able to satisfy it. So it
is `session.ErrNoSession` → 401 → sign in again → the challenge that asks for the factor they hold.
`TestAnEnrolledSessionThatNeverPresentedAFactorIsSignedOut` states it.

Nothing is revoked in the process. The requirement is evaluated per request, so a policy turned on
and then off leaves the sessions it interrupted usable rather than having quietly ended them —
`Service.confineToEnrolment` writes nothing.

### `mfa_enrolment_required` is a new problem code, and the 1:1 rule became a rule about refinements

`M0B-007` gave every problem code its own HTTP status, tested by `TestEachStatusBelongsToOneCode`.
This ticket needs a 403 a client can act on differently — "enrol, here" is not "you may not do this"
— and 403 was taken.

Inventing a status would be worse, so the invariant was generalised rather than deleted. A code may
share a status only by declaring itself a **refinement** of another in `apierr.refinements`, and a
refinement must be strictly more specific, so a client that has never heard of it can treat it as
the code it refines and be right, merely less helpful. Two *unrelated* codes on one status is still
a test failure — `TestEachStatusBelongsToOneCodeOrARefinementOfIt`. The fallback is implemented on
the client side too: `isApiError(err, 'forbidden')` is true of `mfa_enrolment_required`, and not the
reverse.

`errors.Is(err, apierr.ErrForbidden)` is deliberately **false** of it on the server. The Go callers
that ask are asking "was this a permission failure", and the answer they want there is the narrow
one.

### A confined session may also log out, sign in, and read `/healthz`

The ticket says "only the TOTP enrol/confirm endpoints and `GET /auth/me`". `enrolmentOnlyRoutes`
adds four more, each with its reason next to it:

- **`POST /auth/logout`.** Ending a session grants nothing, and somebody on a shared machine who
  does not want to enrol on it needs a way out that is not waiting for the cookie to expire. "No way
  to navigate past it" is about the application, not about the door.
- **`POST /auth/login`, `…/totp/verify`, `…/recovery/verify`.** These do not consult the session
  cookie at all, so gating them would turn "sign in as an administrator and fix this" into a 403
  that depends on what was left in the cookie jar. It is the argument `csrfExemptRoutes` already
  makes about the same three routes.
- **`GET /healthz`, `GET /version`.** Public, and answered for callers with no session whatsoever.

The list is enforced from both sides. `TestAConfinedSessionCanOnlyReachTheEnrolmentRoutes` walks the
router: a route not on the list must answer 403 `mfa_enrolment_required`, a route on it must not, and
an entry naming a route the router does not serve fails as stale. An endpoint added in M2 is refused
until somebody writes down why it should not be — the same mechanism `M1-005` used, for the same
reason.

### An enrolled account is still challenged with the policy off

`decideLogin` answers `outcomeChallenge` whenever a confirmed factor exists, whether or not anybody
requires one. The ticket's table writes that row as "—"; refusing to ask for a factor somebody
deliberately set up would be the wrong way to read it, and it is `M1-006`'s rule kept rather than a
new one. Both the table test and `TestDecideLogin` name it as the deviation.

### Settings are a key/value table

`app.platform_setting(key, value, updated_at, updated_by)` rather than a typed column per decision.
A single-row typed table reads better in SQL and costs a migration for every checkbox added across
M2–M6. What is given up is the database checking a value's shape; what replaces it is that the only
writer is a typed encoder in Go and the only reader is the matching decoder, which reports a value it
cannot read rather than guessing.

Absence is the default and it must be this one: a fresh installation has no rows, requires nothing,
and does not confine its first administrator to enrolling before they have seen the product. Turning
a policy off writes `false` rather than deleting the row, so "never set" and "set and then turned
off" stay distinguishable through `updated_by`.

A value that is present and unreadable is an error, not a `false`. It can only arrive by hand-editing
the database, and the resulting failure — every authenticated request answering 500 — is loud in the
way that deserves.

### SSO exemption: `password_hash IS NULL`

The ticket asks for the behaviour to be written down even if the answer is "exempt". It is exempt,
and the rule is that the local requirement applies to accounts that can sign in locally. An account
with no local password presents no password here, so there is no local sign-in for a local second
factor to stand behind. The exemption covers the per-user flag too — a requirement somebody has no
way to satisfy is a lockout, not a policy — and an account holding *both* a local password and an
SSO identity is **not** exempt.

`M1-009`/`M1-010` refine this when there is an assertion to read: an IdP that says what it verified
should be what satisfies the requirement. Written up in `docs/security.md`; the cases are in
`TestMFAPolicyRequires`, which is where they can be tested at all, since an account with no password
cannot reach the login path.

### The admin check is in the service, and it is temporary

`PLAN.md` §4 says no handler makes its own role decision, and `M1-013` moves every such decision into
one middleware. That does not exist yet, and settings endpoints reachable by any signed-in account
would be v1's ungated `/manage/access` with a different path. So `requireAdmin` sits in
`internal/authn`, on the same side of the boundary as the rule it protects, with a comment saying
that deleting it in `M1-013` is one edit in one file.

### `blctl user reset-mfa` now prints one of two warnings

The old sentence — "the account now signs in with a password and nothing else" — is only true when
nothing requires a factor of them. Under a policy this command turns a lockout into an enrolment
instead, and printing the old warning would tell an operator their deployment is less protected than
it is. `resetMFAResult.MFAStillRequired` decides which is printed; `authn.DecodeMFAPolicy` is
exported so the command line and the server read a stored policy the same way. The command still
touches the factor and nothing else — not the password, the flag, the policy or any session.

### The UI is `M1-017`'s

`web/` has no auth UI: the login route is still a placeholder, and `M1-017` owns login, MFA, account
and admin screens. Building a blocking enrolment screen here would mean building a login page first,
which is that ticket. This follows the precedent `M1-007` set for the same reason.

What landed instead is the fact the screen needs, so `M1-017` does not have to change the API to
render it: **`mfa.required`** on `CurrentUser` (the effective requirement — the platform policy or
the per-user flag, documented so that a client blocking on `enforced` is visibly the wrong choice),
the `mfa_enrolment_required` login status, and the 403 code on everything else.

### The end-to-end spec drives the API, not the browser

`specs/mfa-enforcement.spec.ts` seeds two accounts through `blctl`, signs both in, has the
administrator turn enforcement on, and watches the second account get confined and enrol its way
out — against the shipped binary, its migrations, its cookie jar and the real middleware chain.
There is nothing to click yet; when `M1-017` lands, the browser version belongs beside it.

### A Playwright landmine in the harness, found by needing two seed steps

`test.use({ seed: [...] })` took a bare array. Playwright decides whether an option value is a
`[value, options]` tuple with `Array.isArray(v) && typeof v[1] === 'object'` — so **any** seed of two
or more steps was silently read as a tuple and resolved to the first step alone. The documented
two-step example in `docs/testing.md` would not have worked. One seed step was under the threshold,
which is why nothing had noticed.

The option is now `{ steps: [...] }` (`SeedPlan`, which carries the explanation), and a step may take
the object form `{ args, stdin }` — `blctl user create` reads the password from stdin and
deliberately not from a flag, so writing to the child is the only way to seed an account whose
password a spec knows.

### Verified

`make lint test build` green, `make generate` idempotent, and the full Playwright suite passes —
including the two `e2e` files that failed `prettier --check` before this branch was touched
(`harness/paths.ts`, `README.md`), which are now formatted.

Driven by hand against `./bin/blacklight` on a real DuckDB file with two accounts. A member signs in
(`authenticated`, `required: false`); a member reading `/settings/mfa` → 403 `forbidden`; the
administrator turns `requiredForAll` on; the member's **existing** cookie, unchanged, then answers
200 on `/auth/me` with `required: true, enforced: false` and 403 `mfa_enrolment_required` on
`/auth/password` — no re-login involved. The administrator who turned it on is confined by it at
their own next request. A fresh login answers `mfa_enrolment_required` with a session cookie; enrol
and confirm from that session → ten recovery codes, a rotated token, `satisfied: true`, and
`/auth/password` answering 204 on the same cookie while the pre-confirmation token answers 401. A
later sign-in answers `mfa_required`. `blctl user reset-mfa --email MEL@Example.com` prints the
"still required" note, and the next sign-in is `mfa_enrolment_required` with `enrolled: false` — the
break-glass path leads to enrolling again rather than to a password being enough. No recovery code
appears anywhere in the server log.
