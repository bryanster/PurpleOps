# M6-011 — Publish, immutable versions, evidence opt-in, lead scope

**Milestone:** M6 · **Size:** L · **Depends on:** M6-009, M6-002

## Why

"The report we sent the client" must not change when the workbook or draft does. Publish is the
`report.publish` act already named in authz — lead only.

## Scope

**In**

- Migration `app.report_version`:
  - `id`, `report_id`, `ordinal`/`version_n`, `title`, `published_by`, `published_at`
  - `include_evidence` bool
  - `blind_scope` marker always `lead_full` (stored for honesty)
  - `blocks_json` — frozen copy of block id+params
  - `branding_json` — resolved branding snapshot
  - `html` (or filesystem blob ref) — full rendered HTML
  - `content_sha256`, `pdf_sha256` nullable until generated
  - immutable: **no UPDATE** of content columns after insert (enforce in store).
- **API** (`report.publish` unless noted):
  - `POST /engagements/{id}/reports/{reportId}/publish` body:
    `{ includeEvidence?: bool }` default **false**.
    - Renders with **lead/full** blind scope always (ignore caller's blue seat — only lead can call).
    - Fails entire publish if any block errors or compare baseline unauthorized.
    - Returns version metadata.
  - `GET .../reports/{reportId}/versions` — `report.read`
  - `GET .../versions/{versionId}` metadata
  - `GET .../versions/{versionId}/html` — `text/html` members with `report.read`
  - `GET .../versions/{versionId}/pdf` — generate-once or cache PDF bytes from frozen HTML
- Draft remains editable after publish; new publish creates version N+1. No silent mutation of old
  versions when draft changes.
- Activity: `report.published` with version id in delta.
- Asset handling: if `includeEvidence`, rewrite evidence URLs in HTML to version-scoped authenticated
  routes; if not, ensure no evidence bytes embedded.

**Out**

- Share grants (`M6-012`); unpublish delete (optional hard-delete version — out; revoke is share-side).
- Editing a version in place.

## Files

- Migration, version store, publish service, handlers, OpenAPI, docs

## Acceptance criteria

- [ ] Observer cannot publish (403); lead can.
- [ ] Red/blue cannot publish.
- [ ] Published HTML does not change when draft blocks later change (integration: publish → edit draft
      → GET version HTML identical / same sha256).
- [ ] Default `includeEvidence=false` strips evidence from version HTML.
- [ ] Publish always full scope even if we later allow non-lead (guard test with forced scope).
- [ ] PDF endpoint returns stable bytes for the same version after first generation (cached).

## Tests

- Immutability test above.
- Authz matrix on publish.
- Failure: invalid compare baseline aborts publish, no partial version row.

## Notes for the implementer

- Store HTML in DB if size allows; if not, content-addressed file under data dir with DB hash —
  document limit.
- Version list newest-first.
