# M5-009 — Analytics read endpoints + blind scoping + authz

**Milestone:** M5 · **Size:** L · **Depends on:** M5-004, M5-005, M5-006, M5-007, M5-008

## Why

One source, two consumers (`PLAN.md` §5). Every number the dashboard shows and every number an M6
report block prints comes through these endpoints, from the queries M5 already wrote. If M6 grows a
second aggregation path because these were awkward, the two will disagree in front of a client.

This is also where the seat is resolved. The rollups take a `blind.Scope`; something has to build it,
correctly, once.

## Scope

**In**

- **`api/openapi.yaml` first** (README § Conventions — spec before Go or TS):

  | Path | Returns |
  |---|---|
  | `GET /engagements/{engagementId}/analytics/coverage` | `M5-004`, both denominators |
  | `GET /engagements/{engagementId}/analytics/distribution` | `M5-005`, all four distributions in one payload |
  | `GET /engagements/{engagementId}/analytics/mttd` | `M5-006` |
  | `GET /engagements/{engagementId}/analytics/burndown?interval=` | `M5-007` |
  | `GET /engagements/{engagementId}/analytics/compare?baseline={id}` | `M5-008` — path engagement is *current* |

- **Authz: `x-authz-action: report.read`** on every one (epic decision — no new action). The existing
  middleware (`M1-013`) resolves the path engagement and decides; **zero handler-local role checks**.
- **Compare's second engagement is the exception, and is handled explicitly.** The middleware can
  authorize one engagement from the path; the handler must then check `report.read` on `baseline`
  through `authz.Can` — the same shape `M4-001` used for per-topic authz, and for the same reason.
  A caller who may read the current engagement and not the baseline gets 403, never a partial compare.
- **Blind scope resolution, shared.** `handlers.stepBlindScope` already exists at
  `internal/httpapi/stephandlers.go:280`. Lift it to a shared helper rather than writing a second
  one — two functions that decide what blue may see is how v1 ended up with two definitions of blue
  (`internal/store/blind` package doc). Compare resolves a scope **per engagement**.
- **Every response carries `blindFiltered: bool`** so the UI can label the view (`M5-013`).
  It carries **no count of what was withheld** — "12 steps hidden" is the leak the seat scoping
  exists to prevent, restated as a number.
- Errors use the `M0B-007` problem shape. Unknown `interval`, malformed `baseline`, baseline equal to
  current (allowed — see `M5-008`'s identity check), missing engagement: all covered.
- Regenerate the TS client (`M0B-009`); `make generate && git diff --exit-code` clean.
- Document the endpoints in `docs/api.md` and the definitions in `docs/analytics.md`.

**Out**

- Exports of any kind (`M5-010`…`M5-012`) — those are their own media types and their own tickets.
- UI (`M5-013`, `M5-014`).
- Caching, `ETag`, or conditional requests. No cache in M5 (epic decision); revisit only with
  `M5-015` numbers in hand.

## Acceptance criteria

- [ ] Spec edited before handlers; strict server regenerated; drift gate green.
- [ ] Every endpoint carries `x-authz-action: report.read` and `x-authz-because`. The authz sweep
      test (`internal/httpapi/authzsweep_test.go`) covers the new operations with no exemptions.
- [ ] **No handler-local role check** anywhere except compare's explicit `authz.Can` on `baseline`,
      which is commented with why it cannot be middleware.
- [ ] An observer can read every analytics endpoint (`report.read` is all-members) — asserted, since
      it is the case a reviewer will assume is wrong.
- [ ] A service token with `reports:read` succeeds; one with only `engagements:read` gets 403.
- [ ] Blue in a blind engagement gets seat-scoped numbers and `blindFiltered: true`; lead gets the
      full numbers and `blindFiltered: false`. The same engagement, two callers, two payloads —
      asserted side by side in one test so the difference is visible in the source.
- [ ] No response field anywhere counts, sums, or otherwise implies the number of withheld steps.
      Reviewed as a checklist item against every new schema.
- [ ] Compare: caller with `report.read` on current but not baseline → 403, and the body reveals
      nothing about the baseline beyond its id, which they already sent.
- [ ] Responses validate against the spec in test (`PLAN.md` §9 API contract layer).
- [ ] A closed or archived engagement still serves analytics. Reports get written after an engagement
      closes; that is most of the point.

## Tests

- Handler tests per endpoint: member seats, observer, admin, non-member, service tokens with each
  scope.
- The paired blue/lead blind assertion above.
- Compare authz: both-allowed, current-only, baseline-only, neither.
- Contract validation against `openapi.yaml`.
- `docs/api.md` examples exercised, or at least kept honest by a test that parses them.

## Notes for the implementer

- `handlers.topicAllowed` in `internal/httpapi/events.go` is the precedent for authorizing a second
  resource inside a handler; follow its shape rather than inventing one.
- Analytics handlers construct `analytics.Scope` and call `Queries`. They do not compute. If a
  handler grows arithmetic, it belongs in `internal/analytics` where the fixture tests can reach it.
- `report.read` maps to token scope `reports:read` (`internal/authz/policy.go`) — no new scope, and
  no change to the M1-014 matrix beyond the new operations appearing in the sweep.
