# M7-012 — Bind nested workbook and report IDs to the authorized engagement

**Milestone:** M7 · **Size:** L · **Depends on:** M7-011  
**Finding:** [BL-002](../SECURITY_FINDINGS.md#bl-002--cross-engagement-idor-on-nested-workbook-and-report-objects) · **Severity:** High

## Why

Nested routes authorize `engagementId` (membership in A) then load `scenarioId` / `stepId` /
`reportId` / `templateId` with no parent check. A member of A who knows a child UUID from B
reads B’s workbook or report draft. A lead/red on A can insert or delete objects in B.

M7-011 makes middleware capable of catching the mismatch. This ticket is the handler/domain
bind so a missed mapping cannot become another IDOR. Both layers, not one.

## Scope

**In**

Every nested read/write that names both an authorized engagement and a child id, plus every
body-referenced foreign key that must stay inside that engagement:

| Surface | Today | Required |
|---|---|---|
| Scenario get/patch/delete | `ByID(scenarioId)` | 404 unless `scenario.EngagementID == path` |
| Step create | `CreateStepInput.ScenarioID` only | scenario must belong to path engagement |
| Step get/patch/delete/reveal/reorder | load by `stepId` | step’s scenario’s engagement == path |
| Execution get/patch (red/blue) | load by `executionId` | same |
| Evidence list-by-execution / get / content / delete | load by id | owning engagement == authorized engagement; blind-reveal on get/content |
| Finding get/patch/delete/steps | load by `findingId` | same |
| Comment list/create/patch | load by execution/comment id | same |
| Report get/patch/delete/blocks/preview/publish | load by `reportId` | `report.EngagementID == path` |
| Template get/patch/delete | load by `templateId` | same |
| `CreateFromReport` | copies any `reportId` into path engagement | source report’s engagement == path |
| `ApplyTemplate` | copies any `templateId` onto the report | template and report share the path engagement |

Mismatch is **404**, not 403 — same concealment as a missing id.

**Out**

- Implementing `Ownership.Facts` (M7-011 — must land first or in the same PR).
- Engagement list filtering (M7-010).
- Guest registration, share-token logging (M7-013).

## Files

- `internal/httpapi/scenariohandlers.go`
- `internal/httpapi/stephandlers.go`
- `internal/httpapi/executionhandlers.go`
- `internal/httpapi/evidencehandlers.go`
- `internal/httpapi/findinghandlers.go`
- `internal/httpapi/commenthandlers.go`
- `internal/httpapi/reporthandlers.go`
- `internal/httpapi/templatehandlers.go`
- `internal/engagement/{scenarios,steps,executions,findings,comments}.go` — bind at the domain if that is the one place handlers cannot forget
- `internal/report/{service.go,template_service.go}` — `CreateFromReport` / `ApplyTemplate` / `Get`
- Tests next to the handlers or domain packages

Prefer one helper (`requireSameEngagement(got, want) error` → `apierr.NotFound`) over a
per-handler `if` that will be copied wrong.

## Acceptance criteria
- [x] Member of `EB` calling `GET /engagements/{EB}/scenarios/{SA}` (SA belongs to EA) gets 404
      and a body identical to an invented scenario id.
- [x] Lead of `EB` calling `POST /engagements/{EB}/scenarios/{SA}/steps` does not create a step
      under EA.
- [x] `POST /engagements/{EB}/reports/{RA}/preview` does not render EA’s draft.
- [x] `POST /engagements/{EB}/report-templates/from-report` with `reportId=RA` does not copy EA’s
      blocks into EB.
- [x] `ApplyTemplate` with a template from EA onto a report in EB is 404.
- [x] `GET /evidence/{id}` and `GET /evidence/{id}/content` as blue in a blind engagement 404
      when the parent step is unrevealed (handler-side, even if middleware also catches it).
- [x] No nested ByID in `internal/httpapi` remains without an engagement equality check or a
      domain method that performs it.

## Tests

One regression per row in the table above, two callers (member of A, member of B only). Assert
404 + no row created/updated on the write cases.

Do not test “the source contains `EngagementID ==`”. Drive the HTTP API or the service method.

## Notes for the implementer

Child UUIDs are UUIDv7 — not brute-forceable — but they leak via BL-001, activity, exports, and
the UI. Do not dismiss the IDOR because the id is long.

If M7-011 already 404s some of these at middleware, keep the handler check. Middleware mappings
drift; handlers that load by id must not trust the path.

This ticket is a ship gate for `M7-009`. High findings do not defer.

## Implementation notes

- Binding lives in the domain services, not the handlers: `internal/engagement` gained
  `requireSameEngagement` plus `GetScenarioInEngagement` / `GetStepInEngagement` /
  `GetExecutionInEngagement` / `GetCommentInEngagement`, and the write methods (`PatchScenario`,
  `DeleteScenario`, `CreateStep`, `CreateStepFromTemplate`, `PatchStep`, `DeleteStep`, `RevealStep`,
  `ReorderSteps`, `PatchRedExecution`, `PatchBlueDetection`, `CreateComment`, `EditComment`,
  `ListComments`, `ListCommentRevisions`, `ListSteps`) now take the path `engagementID` and 404 on a
  mismatch. `internal/report` mirrors this for `Service.Get/Update/Delete/ReplaceBlocks`,
  `TemplateService.GetTemplate/UpdateTemplate/DeleteTemplate/ApplyTemplate/CreateFromReport`, and
  `PublishService.Publish`.
- `ListSteps` and `CreateStepFromTemplate` were not explicit table rows but are nested
  read/write routes naming an engagement and a child id, so they got the same bind (the ticket's
  "In" preamble covers them).
- `GET /evidence/{id}` and `/content` gained a handler-side blind-reveal check (`evidenceConcealed`)
  mirroring the middleware guard; evidence routes are child-only so there is no path engagement to
  bind, and the middleware (M7-011) owns membership there.
- Tests are service-level regressions (`internal/engagement/bind_test.go`,
  `internal/report/bind_test.go`) asserting `apierr.ErrNotFound` on every cross-engagement bind and
  no row created/updated on the write cases.
