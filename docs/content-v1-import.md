# Importing v1 custom content

Blacklight v2 does not import Mongo dumps. Operators who still have files from a
v1 deploy can load them into the singleton **custom** source.

## What to copy from a v1 install

| v1 path | Format flag | Becomes |
|---|---|---|
| `custom/testcases.json` | `testcases_json` | `content_procedure_template` rows |
| `custom/testcases/*.yaml` | `testcases_yaml` | `content_procedure_template` rows |
| `custom/knowledgebase/*.yaml` | `knowledgebase_yaml` | `content_note` rows |
| a zip of any of the above | `auto` | mixed |

v1's seeder globbed `custom/testcases/*.yaml` while the repo shipped
`testcases.json`. Both layouts are accepted on purpose.

The M2-011 custom export document (`GET /content/custom/export`) is also
accepted under `auto` and re-upserts by `externalId`.

## HTTP

```http
POST /api/v1/content/custom/import?dryRun=false&failFast=false
Content-Type: multipart/form-data

file: <bytes>
format: auto|testcases_json|testcases_yaml|knowledgebase_yaml
```

Requires `content.manage` (platform admin). Small uploads answer `200` with a
[`ContentImportReport`](../api/openapi.yaml). Uploads over 1 MiB enqueue a
`v1_import` job and answer `202` (same global job slot as content sync).

`dryRun=true` always runs synchronously and never writes.

## CLI

```sh
# File
blctl content import --format testcases_json --path ./testcases.json

# Directory the old seeder expected
blctl content import --format auto --path ./custom

# Knowledgebase tree
blctl content import --format knowledgebase_yaml --path ./custom/knowledgebase

# Parse only
blctl content import --dry-run --path ./custom/testcases.json
```

## Mapping rules

### Testcases → procedure templates

| v1 field | Destination |
|---|---|
| `name` (else `objective`, else filename) | `name` |
| `actions` | `command` (flat string — **not** invented structure) |
| `objective`, `tactic`/`phase`, `provider`, `tags`, `tools`, `rednotes` | folded into `description` |
| `mitreid` when `T####` / `T####.###` | `techniqueExternalIds` |
| `uuid` | preferred external id seed |

Cleanup and `inputArgs` are left empty for pure v1 sources. The importer emits a
warning so operators know structure was not fabricated.

### Knowledgebase → notes

| v1 field | Destination |
|---|---|
| `mitreid` / filename | `title` (`Knowledge: T####`) |
| `overview` + `advice` (+ provider) | markdown `bodyMarkdown` |
| `provider`, `tags` | `tags` |
| `mitreid` when valid | `techniqueExternalId` |

## Idempotency

Re-import is safe. External ids are deterministic:

- testcase: `v1:testcase:<uuid>` or `v1:testcase:<slug(name)>` or content hash
- note: `v1:kb:<MITREID>` or `v1:kb:<slug(filename)>`
- custom export: the export's own `externalId`

A second run updates matching rows and reports `*Updated` counts.

## Partial failures

Per-file parse/apply errors are collected in the report's `errors` array. The
default is continue-and-summarize. Pass `failFast=true` / `--fail-fast` to stop
at the first error.

## Activity

Successful writes record `content.import.finished` with counts in the delta.
Hard parse failures on a non-dry-run path record `content.import.failed`.
Individual upserts still emit `content.custom.created` / `.updated`.
