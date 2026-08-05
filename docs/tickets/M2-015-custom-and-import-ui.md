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

- [ ] Admin creates a procedure with two input args; detail view shows both after save.
- [ ] Dry-run import of fixture JSON shows expected counts and writes nothing (assert via reload).
- [ ] Confirm import creates browsable templates in the library.
- [ ] Export downloads a non-empty file; re-import dry-run accepts it or documents limitations.
- [ ] Member can view custom items but has no create/edit/import controls.

## Tests

- Component tests for form validation and dry-run flow (MSW).
- E2E: admin import fixture → member sees template in library.

## Notes for the implementer

- Input args editor: prefer simple field array UI over JSON textarea for the happy path.
- Keep import wizard steps explicit so warnings are read before confirm.
