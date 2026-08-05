# M2-011 — Custom content API: templates, rules, notes

**Milestone:** M2 · **Size:** M · **Depends on:** M2-001, M2-002

## Why

User-authored content is a first-class source (`PLAN.md` §3), not a side file drop. Operators need
CRUD for procedure templates, detection rule refs, and KB notes under the seeded `custom` source,
exportable as YAML/JSON.

## Scope

**In**

- All mutations attach to the singleton `kind=custom` source (`source='custom'` in product language).
- Spec-first endpoints (`content.manage` write, `content.read` read), examples:

  | Resource | Endpoints |
  |---|---|
  | Procedure templates | `GET/POST /content/custom/procedure-templates`, `GET/PATCH/DELETE …/{id}` |
  | Detection rules | `GET/POST /content/custom/detection-rules`, `GET/PATCH/DELETE …/{id}` |
  | Notes | `GET/POST /content/custom/notes`, `GET/PATCH/DELETE …/{id}` |
  | Export | `GET /content/custom/export?type=&format=yaml|json` |

- Bodies mirror structured fields from `M2-001` / Atomic / Sigma — especially procedure structure
  (no flattened actions string).
- Validation: non-empty name/title; technique external ids optional but if present must look like
  MITRE ids (`T\d{4}(\.\d{3})?`); markdown body size cap (configurable, sane default).
- Delete: 409 if referenced (M2 stub ref counter always 0 until M3); soft-disable not required for
  custom rows — hard delete ok when unreferenced.
- Export: YAML/JSON document suitable for re-import (`M2-012` should accept custom export shape or
  document the delta). Include license/attribution header comments for the installation name.
- Activity: `content.custom.created`, `.updated`, `.deleted` with object type in delta.
- `blctl content export-custom --format yaml|json -o file`

**Out**

- UI editor (`M2-015`).
- Custom tactics/techniques/groups.
- Versioning of custom objects.

## Acceptance criteria

- [ ] Member can read custom content; only admin can mutate.
- [ ] Creating a procedure template with command + cleanup + input_args persists structure.
- [ ] Invalid technique id format → 400 field error.
- [ ] Export then re-open as JSON/YAML contains all three types when present.
- [ ] Deletes are activity-logged; response never includes other users' secrets (n/a but keep delta
      clean).

## Tests

- Handler CRUD tests per type; validation cases; export round-trip snapshot test.
- Authz matrix rows for `content.manage`.

## Notes for the implementer

- Reuse list/get handlers with library endpoints where possible (`sourceId=custom` filter) to avoid
  two list shapes — custom write paths can still be under `/content/custom/...`.
- Do not require ATT&CK install to author custom templates.
