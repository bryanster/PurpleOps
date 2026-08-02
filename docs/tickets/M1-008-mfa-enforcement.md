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

- [ ] **Regression case (must exist by name):** policy on, user never enrolled → they cannot reach
      any application endpoint with a password alone. Name the test after this ticket.
- [ ] An enrolment-only session is 403 on a normal API call, with a distinguishable `code`.
- [ ] After completing enrolment, the session is rotated and becomes fully privileged in one step —
      no re-login required.
- [ ] Turning on `mfa_required_for_all` immediately affects already-logged-in users on their next
      request.
- [ ] A user with `mfa_enforced` set individually is required even when the platform policy is off.
- [ ] An admin cannot remove their own MFA while a policy requires it — the endpoint refuses, rather
      than leaving them enforced-but-unenrolled.
- [ ] Turning the policy **off** does not delete anyone's enrolment.
- [ ] The last remaining admin cannot lock the platform out of itself; if a path exists, it's
      `blctl user reset-mfa` (`M1-007`) and it's documented.

## Tests

- Table-driven over the state machine above — every row, plus the individually-enforced variant.
- Session-downgrade test: session created before the policy, request after.
- Enrolment-only scope test enumerating a representative set of endpoints.
- E2E (`M0B-013`): admin turns on enforcement, second user logs in and is forced through enrolment.
