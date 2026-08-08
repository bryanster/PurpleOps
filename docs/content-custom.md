# Custom content

User-authored procedure templates, detection rule references, and knowledge-base
notes live under the seeded `custom` content source
(`id = 01900000-0000-7000-8000-000000000005`, version token `current`).

Custom content does **not** require an ATT&CK install. Technique external ids
are optional; when present they must match `T####` or `T####.###`.

## Authz

| Action | Who |
|---|---|
| `content.read` | Any authenticated member (list/get/export) |
| `content.manage` | Platform admin (create/update/delete) |

## HTTP

| Resource | Endpoints |
|---|---|
| Procedure templates | `GET/POST /api/v1/content/custom/procedure-templates`, `GET/PATCH/DELETE …/{id}` |
| Detection rules | `GET/POST /api/v1/content/custom/detection-rules`, `GET/PATCH/DELETE …/{id}` |
| Notes | `GET/POST /api/v1/content/custom/notes`, `GET/PATCH/DELETE …/{id}` |
| Export | `GET /api/v1/content/custom/export?type=&format=yaml\|json` |
| Import | `POST /api/v1/content/custom/import` (multipart; see [`content-v1-import.md`](content-v1-import.md)) |

Library browse endpoints (`GET /content/procedure-templates?sourceId=…`, etc.)
also return custom rows when the custom source is enabled (it is, by seed).

### Procedure structure

Templates preserve structure — platforms, executor, command, cleanup, and
`inputArgs` stay distinct fields. They are never flattened into a single
`actions` string.

### Notes

Markdown body size is capped by `BLACKLIGHT_CONTENT_NOTE_MAX_BYTES` (default
`256KiB`). Oversized bodies answer `400` with a field error on `bodyMarkdown`.

### Delete

Hard delete when nothing references the row. Engagement reference counting
arrives in M3; until then the counter always reports zero, so deletes succeed
for unreferenced custom rows. Referenced deletes will answer `409` with counts.

### Activity

Mutations write platform activity rows:

- `content.custom.created`
- `content.custom.updated`
- `content.custom.deleted`

The entry's `object_type` is `content_procedure_template`,
`content_detection_rule_ref`, or `content_note`. The delta carries a short
`objectType` label and non-secret field changes.

## Export

```sh
# HTTP
curl -b session -o custom.yaml \
  'https://blacklight.example/api/v1/content/custom/export?format=yaml'

# blctl
blctl content export-custom --format yaml -o custom.yaml
blctl content export-custom --format json --type notes
```

YAML exports start with header comments naming the source, license/attribution,
and export timestamp. The document body is:

```yaml
meta:
  sourceName: Custom content
  attribution: User-authored content for this installation.
  exportedAt: 2026-08-05T12:00:00Z
procedureTemplates: [...]
detectionRules: [...]
notes: [...]
```

M2-012's importer accepts this shape (or documents any thin adapter delta).

## Config

| Variable | Default | Meaning |
|---|---|---|
| `BLACKLIGHT_CONTENT_NOTE_MAX_BYTES` | `256KiB` | Max UTF-8 byte length of a note body |

## Import (v1 files)

Operators migrating from PurpleOps/Blacklight v1 can upload `testcases.json`,
a `testcases/*.yaml` tree, or `knowledgebase/*.yaml` via

```sh
POST /api/v1/content/custom/import
blctl content import --path ./custom
```

Details, field mapping, and idempotency rules: [`content-v1-import.md`](content-v1-import.md).
