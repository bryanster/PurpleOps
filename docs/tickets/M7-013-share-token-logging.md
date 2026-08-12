# M7-013 — Share tokens in logs; unthrottled claim/password

**Milestone:** M7 · **Size:** M · **Depends on:** M6-012, M1-004  
**Finding:** [BL-005](../SECURITY_FINDINGS.md#bl-005--share-tokens-land-in-access-logs-claimpassword-are-public-and-unthrottled) · **Severity:** Medium

## Why

Share capability lives in the URL path (`/api/v1/report-views/{token}`). The access logger
records `r.URL.Path` specifically to *avoid* query-string secrets. Path tokens were not
considered. `authorize` refusals and problem logs do the same.

Claim and password routes are `x-authz-public`, CSRF-exempt, and absent from
`credentialAccounts`. Share passwords have no `minLength` in the schema and skip
`password.Validate`. `maxGrants` is check-then-insert with no transactional constraint.

A log line plus any local account (or a weak share password) is a published report.

## Scope

**In**

- Redact share tokens in access logs, authorization logs, and problem logs
  (`/api/v1/report-views/[redacted]` or equivalent). Do not log the raw path for these routes.
- Throttle `POST /report-views/{token}/claim` and `POST /report-views/{token}/password` with the
  existing limiter: per token-prefix (or token hash) **and** per source IP. Same 429 shape as login.
- Enforce a real password policy on share create (`password.Validate` or at least the documented
  8-character minimum in both OpenAPI and `CreateShare`).
- Enforce `maxGrants` inside the write transaction so concurrent claims cannot exceed the cap.
- Keep HTML/PDF grant-gated (already). Do not make the password cookie the only view gate without
  a separate ticket — M6-012 already deferred cookie re-check at view time.

**Out**

- Implementing `GuestRegister` (still stubbed; stay stubbed).
- Checking `bl_report_share` on HTML/PDF (deferred in M6-012; file a follow-up if you take it).
- Changing token entropy or hash construction.

## Files

- `internal/httpapi/logging.go`
- `internal/httpapi/authorize.go` / `internal/httpapi/apierr` (path in refusal logs)
- `internal/httpapi/throttle.go` — add the two share routes to `credentialAccounts` (or a sibling table)
- `internal/report/share.go` — `CreateShare` password policy; `ClaimShare` grant cap in-tx
- `internal/store/report/share.go` / `internal/store/migrate/sql/` — unique or transactional cap if needed
- `api/openapi.yaml` — `CreateReportShare.password` `minLength`
- `internal/httpapi/csrf.go` — leave exemptions; they stay public-ish

## Acceptance criteria

- [ ] An info-level request log for `GET /api/v1/report-views/{token}` does not contain the token.
- [ ] A 401/404 problem log for the same path does not contain the token.
- [ ] After N failed `POST .../password` attempts (existing account/source thresholds), the next
      is 429 with `Retry-After`.
- [ ] `CreateShare` with a 3-character password is 400.
- [ ] Two concurrent claims against `maxGrants=1` produce one grant and one 403, never two grants.

## Tests

- Logger test: wrap a share-view request, assert the logged `path` attr is redacted.
- Throttle test: reuse the login-throttle helpers against the password route.
- `CreateShare` validation test for short / empty / omitted password (omit remains allowed).
- Concurrent `ClaimShare` with `maxGrants=1`.

## Notes for the implementer

Redact in one helper used by every log site. A single `logging.go` change is not enough —
`authorize.go` and `apierr` also print `r.URL.Path`.

Argon2id on the password route is already expensive; throttling is still required so a stolen
token cannot be pounded from many sources without hitting the source limiter.

Medium: not a silent defer, but not a `M7-009` blocker unless the ship owner promotes it.
