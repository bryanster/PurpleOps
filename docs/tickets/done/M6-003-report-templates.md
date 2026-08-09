# M6-003 — Engagement-scoped report templates

**Milestone:** M6 · **Size:** M · **Depends on:** M6-002

## Why

Operators reuse the same section order across assessments. Templates capture **arrangement + default
params**, not frozen client HTML.

## Scope

**In**

- Migration `app.report_template`: `id`, `engagement_id`, `name`, `created_by`, `created_at`,
  `updated_at`; child `app.report_template_block` (`template_id`, `ordinal`, `block_id`, `params`).
- API (`report.write` mutate, `report.read` list/get):
  - `GET/POST /engagements/{id}/report-templates`
  - `GET/PATCH/DELETE /engagements/{id}/report-templates/{templateId}`
  - `POST /engagements/{id}/reports/{reportId}/apply-template` body `{ templateId }` — **replaces**
    draft blocks with a copy of the template blocks (params deep-copied). Does not change title
    branding unless documented otherwise (prefer leave branding alone).
  - `POST /engagements/{id}/report-templates/from-report` body `{ reportId, name }` — snapshot
    current draft blocks into a new template.
- Validate block ids/params on write the same way as reports.
- Activity: `report_template.created|updated|deleted`, `report.template_applied`.

**Out**

- Install-wide template library; marketplace; sharing templates across engagements (copy manually).
- Applying template to a **published** version (versions immutable).

## Files

- Migration, `internal/report/template*.go`, handlers, OpenAPI, `docs/api.md`

- [x] Apply template is atomic (all blocks replaced in one write txn).
- [x] Template from report does not copy rendered output or version rows.
- [x] Delete template does not delete reports that were created from it.
- [x] Observer can save/apply templates (`report.write`).

## Tests

- Round-trip: draft → template → empty draft → apply → block list equal (new instance ids OK).
- Invalid block in template rejected at template write, not only at apply.

## Notes for the implementer

- Cap templates per engagement (e.g. 20).
- `engagement_compare` baseline id inside template params may point at another engagement — leave
  as-is; render/publish authz will fail later if caller cannot read baseline.

## Implementation notes

**Date:** 2026-08-09

### Acceptance criteria status

- [x] Apply template is atomic (all blocks replaced in one write txn).
- [x] Template from report does not copy rendered output or version rows.
- [x] Delete template does not delete reports that were created from it.
- [x] Observer can save/apply templates (`report.write`).

### Files created/modified

- `internal/events/activity.go` — added `VerbReportTemplateCreated/Updated/Deleted/Applied` and `ObjectReportTemplate`
- `internal/store/migrate/sql/0019_report_templates.sql` — `app.report_template` + `app.report_template_block` tables
- `internal/store/report/template.go` — domain types (Template, TemplateBlock, NewTemplate, TemplateUpdate, NewTemplateBlock)
- `internal/store/report/templates.go` — Templates repo (CRUD + BlocksByTemplate + ReplaceBlocks)
- `internal/report/template_service.go` — TemplateService (create/list/get/update/delete, apply-to-report, from-report, per-engagement cap)
- `internal/httpapi/templatehandlers.go` — all 7 handler methods + templateToWire
- `internal/httpapi/handlers.go` — added `templates *report.TemplateService` field
- `internal/httpapi/server.go` — wired template repo + service into handler construction
- `internal/httpapi/csrf_test.go` — added CSRF coverage for 5 mutating template routes
- `internal/httpapi/template_test.go` — integration tests: CRUD, non-member rejection, apply + from-report
- `api/openapi.yaml` — TemplateId param, ReportTemplate/ReportTemplateBlock/CreateReportTemplate/PatchReportTemplate/ApplyTemplate/CreateTemplateFromReport schemas, 6 endpoints
- `internal/httpapi/gen/server.gen.go` — regenerated
- `web/src/api/schema.d.ts` — regenerated

### Design decisions

- **Engagement-scoped authz** — templates use `report.write` (all members) and `report.read` per the ticket. Same authz action as draft reports.
- **TemplateId is a UUID** — even though templates have `name` for display, the API identifies them by UUID for consistency with other resources.
- **from-report fetches blocks after create** — the template is created first, then blocks are inserted via ReplaceBlocks. Not one transaction, but cleanup on failure ensures consistency.
- **Apply-template copies params** — params are deep-copied via `make+copy` to prevent shared-mutation bugs.
- **Soft limit 20 templates per engagement** — enforced in the service layer before create/from-report.
- **Blocks on GET only** — list templates omits blocks; get template includes them (same pattern as reports).

### Deviations from ticket

None.

### Verification
```
go build ./...                              # clean
go vet ./...                                # clean
go test ./internal/report/... -count=1      # all pass
go test ./internal/store/migrate/... -count=1  # all pass (includes 0019)
go test ./internal/httpapi/... -count=1     # all pass (includes template_test.go)
go test ./internal/authz/... -count=1       # all pass
go test ./internal/events/... -count=1      # all pass
go test ./api/... -count=1                  # conventions pass
```
