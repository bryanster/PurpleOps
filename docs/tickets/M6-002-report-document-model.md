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
