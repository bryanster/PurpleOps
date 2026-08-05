# M2-015 — Custom content editor + v1 import UI

**Milestone:** M2 · **Size:** M · **Depends on:** M2-011, M2-012, M2-013

## Why

Custom CRUD and v1 file import are admin workflows that should not require `blctl` on day one.
This closes the M2 UI loop: author, import, export, browse.

## Scope

**In**

- Custom library section (admin write, everyone read via library):
  - Procedure template create/edit form: name, description, technique ids, platforms, executor,
    command, cleanup, input args editor (list of name/default/description).
  - Detection rule create/edit: title, techniques, level, body (textarea/monaco-optional — textarea
    fine in M2).
  - Note create/edit: title, markdown body, tags, optional technique.
  - Delete with confirm.
  - Export button → downloads YAML or JSON from API.
- Import wizard:
  - File picker (json/yaml/zip)
  - Format auto + override
  - Dry-run preview (counts + warnings + errors)
  - Confirm import → job or result summary
- Validation errors mapped to fields; server problem details shown with request id.
- Deep links from library empty states for admin ("Import v1 testcases").

**Out**

- Collaborative editing, revision history.
- Rich markdown WYSIWYG (textarea + preview is enough).

## Acceptance criteria

- [x] Admin creates a procedure with two input args; detail view shows both after save.
- [x] Dry-run import of fixture JSON shows expected counts and writes nothing (assert via reload).
- [x] Confirm import creates browsable templates in the library.
- [x] Export downloads a non-empty file; re-import dry-run accepts it or documents limitations.
- [x] Member can view custom items but has no create/edit/import controls.

## Tests

- Component tests for form validation and dry-run flow (MSW).
- E2E: admin import fixture → member sees template in library.

## Notes for the implementer

- Input args editor: prefer simple field array UI over JSON textarea for the happy path.
- Keep import wizard steps explicit so warnings are read before confirm.

## Implementation notes

- **UI:** Admin page at `/admin/content/custom` (`RequireAdmin`, nav "Custom content"). Tabs for
  procedures / detections / notes with create/edit dialogs, detail drawers, and delete confirm.
  Procedure input args are a field array (name/type/default/description), not JSON.
- **Import:** Multi-step wizard (pick → dry-run preview → confirm). File stays on the input until
  submit. Dry-run never writes; confirm may answer 200 report or 202 job for large uploads.
- **Export:** YAML/JSON download via plain `fetch` + blob (openapi-fetch dual content-type is awkward
  for file save). Empty blob is refused.
- **Library:** Members keep read-only browse. Admin empty states deep-link to Custom content
  ("Import v1 testcases" / "Create custom procedure"). No write chrome on `/content` for members.
- **Queries:** `custom-queries.ts` — list + CRUD mutations invalidate `contentKeys.all` so library
  lists refresh. Multipart import matches the sources bundle pattern (FormData at submit time).
- **Tests:** `custom-page.test.tsx` (create two args, field errors, dry-run→confirm, export blob,
  delete, patch). E2E `content-custom.spec.ts` seeds admin+member, authors procedure, imports
  `internal/content/testdata/v1import/testcases.json`, exports YAML, member sees templates without
  write controls. Shell keyboard-tab count bumped for the new nav link.
- **Verified:** web vitest 116; e2e content-custom 2 passed; `tsc -b` clean.
