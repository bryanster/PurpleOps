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

## Acceptance criteria

- [ ] Apply template is atomic (all blocks replaced in one write txn).
- [ ] Template from report does not copy rendered output or version rows.
- [ ] Delete template does not delete reports that were created from it.
- [ ] Observer can save/apply templates (`report.write`).

## Tests

- Round-trip: draft → template → empty draft → apply → block list equal (new instance ids OK).
- Invalid block in template rejected at template write, not only at apply.

## Notes for the implementer

- Cap templates per engagement (e.g. 20).
- `engagement_compare` baseline id inside template params may point at another engagement — leave
  as-is; render/publish authz will fail later if caller cannot read baseline.
