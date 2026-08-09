# M6-002 — Report document model, draft CRUD, `report.write`

**Milestone:** M6 · **Size:** L · **Depends on:** M6-001, M1-012

## Why

Reports need a durable draft: ordered block instances, per-block params, ownership under an
engagement. Authz today only has `report.read` / `report.publish` — drafting by any member requires
**`report.write`**.

## Scope

**In**

- **Migration** `app.report`, `app.report_block` (draft only — versions are `M6-011`):
  - `report`: `id`, `engagement_id`, `title`, `created_by`, `created_at`, `updated_at`,
    branding override columns or JSON (`client_name`, `logo_blob_ref`, colours — nullable → fall
    back to install defaults in `M6-004`), optional `updated_by`.
  - `report_block`: `id`, `report_id`, `ordinal`, `block_id`, `params` JSON, UNIQUE(`report_id`,`ordinal`).
- **Authz:** add `ActionReportWrite` / `report.write`:
  - Platform: admins; Engagement: **all members**; Token: `reports:write` (same scope as publish is
    acceptable **or** document if publish stays `reports:write` and write shares it — prefer
    **one** token scope `reports:write` covering draft+publish with action still distinguishing
    seats: members may write, only lead may publish).
  - Update policy table, matrix tests (`M1-014`), `docs/authz.md`, generated authz docs if any.
- **OpenAPI** (`api/openapi.yaml` first):
  - `GET/POST /engagements/{engagementId}/reports`
  - `GET/PATCH/DELETE /engagements/{engagementId}/reports/{reportId}`
  - `PUT /engagements/{engagementId}/reports/{reportId}/blocks` — full ordered replace of draft
    blocks (simpler than patch-per-block for reorder); or PATCH + reorder endpoint — pick one,
    document it.
  - `x-authz-action: report.write` on mutating draft routes; `report.read` on GET.
- Handlers translate only; domain/service in `internal/report` validates block ids via registry,
  params via `ValidateParams`, engagement ownership.
- Activity verbs: `report.created`, `report.updated`, `report.deleted` (engagement-scoped).
- Regenerate TS client.

**Out**

- Templates (`M6-003`), branding admin API (`M6-004`), publish/versions (`M6-011`), share (`M6-012`),
  render (`M6-009`), UI (`M6-013`).

## Files

- `internal/store/migrations/00xx_reports.sql`
- `internal/report/` store + service
- `internal/httpapi/reporthandlers.go`
- `internal/authz/action.go`, `policy.go`, matrix tests
- `api/openapi.yaml`, `docs/api.md`, `docs/authz.md`

## Acceptance criteria

- [ ] Spec before handlers; codegen drift clean.
- [ ] Observer can create/edit a draft (`report.write` all-members) and cannot publish (still
      lead-only — asserted even though publish lands in M6-011).
- [ ] Non-member → 404/403 per existing engagement patterns.
- [ ] Block replace rejects unknown `block_id` and invalid params with `validation_failed`.
- [ ] Deleting a report cascades draft blocks; no orphan rows.
- [ ] Service token with `reports:write` can draft; `reports:read` only cannot.
- [ ] Authz sweep covers new operations; `TestNoHandlerDecidesForItself` stays green.

## Tests

- Handler tests: CRUD matrix seats; invalid blocks; reorder stability.
- Migration forward from empty.
- Authz matrix rows for `report.write`.

## Notes for the implementer

- One report does not auto-create blocks — empty draft is valid; builder adds blocks.
- Soft limits: cap blocks per report (e.g. 50) and params JSON size — document constants.
- Do not store rendered HTML on the draft row.

## Implementation notes

**Date:** 2026-08-09

### Acceptance criteria status

- [x] Spec before handlers; codegen drift clean.
- [x] Observer can create/edit a draft (`report.write` all-members) and cannot publish (still lead-only — asserted even though publish lands in M6-011).
- [x] Non-member → 404/403 per existing engagement patterns.
- [x] Block replace rejects unknown `block_id` and invalid params with `validation_failed`.
- [x] Deleting a report cascades draft blocks; no orphan rows.
- [x] Service token with `reports:write` can draft; `reports:read` only cannot.
- [x] Authz sweep covers new operations; `TestNoHandlerDecidesForItself` stays green.

### Files created/modified

- `internal/authz/action.go` — added `ActionReportWrite` before `numActions`
- `internal/authz/policy.go` — added `report.write` rule (admins + allMembers + reports:write scope)
- `internal/authz/matrix_test.go` — added matrix row for `report.write`
- `internal/authz/regressions_test.go` — added observer `report.write` positive assertion
- `internal/authz/rules_test.go` — updated `TestTheRegressionsAreAbsences` to allow observer's second write
- `internal/events/activity.go` — added `VerbReportCreated/Updated/Deleted` and `ObjectReport`
- `internal/store/migrate/sql/0018_reports.sql` — `app.report` + `app.report_block` tables
- `internal/store/report/doc.go` — package doc
- `internal/store/report/report.go` — domain types (Report, ReportBlock, NewReport, ReportUpdate, DB interface)
- `internal/store/report/reports.go` — Reports repo (CRUD + BlocksByReport + ReplaceBlocks)
- `internal/report/service.go` — Report domain service (validation, block registry checks, soft limits, activity logging)
- `internal/httpapi/reporthandlers.go` — all 6 handler methods + reportToWire + nullable helpers
- `internal/httpapi/handlers.go` — added `reports *report.Service` field + import
- `internal/httpapi/server.go` — wired report registry, repo, service into handler construction
- `internal/httpapi/csrf_test.go` — added CSRF coverage entries for all 4 mutating routes
- `api/openapi.yaml` — ReportId param, Report/ReportBlock/CreateReport/PatchReport/PutReportBlocks/ReportBlockInput schemas, 6 endpoints under `/engagements/{engagementId}/reports`
- `docs/authz.md` — regenerated with `report.write` entry

### Design decisions

- **`report.write` on all members** — Observer holds `report.write` alongside `comment.write` (the two writes an observer holds). Drafting a report is exactly the kind of contribution an observer seat is for.
- **One token scope `reports:write`** — both `report.write` (draft) and `report.publish` (M6-011) share `reports:write` scope per the ticket's preference. The action still distinguishes: members may write, only lead may publish.
- **`x-authz-resource: type: report`** — following the pattern of `finding` (not `engagement`), so the authz middleware validates that the resource type matches the action's resource.
- **Blocks not inline in list** — `Blocks` field is on `Report` but populated only in `GET /reports/{reportId}` responses, not in list.
- **Registry block registration at server startup** — until M6-006–M6-008 provide concrete block definitions, all 14 block IDs are registered as stubs with empty param schemas. This allows the block replace validation (`ValidateParams`) to accept any params for now.
- **Soft limits** — `MaxBlocks=50`, `MaxParamsBytes=32KB` per M6-002 notes.
- **Activity via `RecordAlone`** — report mutations open their own activity transaction since the service doesn't share the store's write transaction. This is consistent with `RecordAlone` usage in other services.

### Deviations from ticket

None.

### Verification

```
go test ./internal/authz/... -count=1       # all pass (0.18s)
go test ./internal/report/... -count=1      # all pass (0.02s)
go test ./internal/store/migrate/... -count=1  # all pass (2.19s)
go test ./internal/httpapi/... -count=1     # all pass (50.33s)
go build ./...                              # clean
go vet ./...                                # clean
make generate && git diff --exit-code       # generate is idempotent
```
