# M2-012 — Import v1 testcases.json + knowledgebase YAML

**Milestone:** M2 · **Size:** M · **Depends on:** M2-011

## Why

Greenfield migration means no Mongo importer, but operators still have **files**: v1
`custom/testcases.json` and `custom/knowledgebase/*.yaml`. v1's seeder also globbed
`custom/testcases/*.yaml` while the repo shipped `testcases.json` — the importer must accept what
people actually have (`PLAN.md` §3).

## Scope

**In**

- Parser package `internal/content/v1import` (name flexible) supporting:
  1. **Testcases JSON** — the array/object shape v1 used (fixture from documented sample).
  2. **Testcases YAML** — one file or a directory of YAML files (the glob the old seeder expected).
  3. **Knowledgebase YAML** — files under a directory → `content_note` rows (title, body, tags,
     optional technique).
- Map testcases → `content_procedure_template` on the `custom` source, preserving as much structure
  as the v1 shape contains. Where v1 only has a flat actions string, store it in `command` and set
  an `import_warnings` or description note that cleanup/args were absent — **do not invent** fake
  structure.
- HTTP: `POST /content/custom/import` multipart:
  - field `format`: `testcases_json` | `testcases_yaml` | `knowledgebase_yaml` | `auto`
  - file upload (file or zip of files)
  - Runs as `v1_import` job **or** synchronous small import — prefer job if over a threshold, but
    keep one code path. Still respects global job slot if async.
- `blctl content import --format auto --path <file|dir>`
- Dry-run flag (`?dryRun=1` / `--dry-run`): parse + counts + warnings, no writes.
- Activity: `content.import.finished` / `.failed` with counts in delta.
- Fixtures: sample `testcases.json`, a `testcases/*.yaml` tree, KB yaml, and a deliberately messy
  hybrid zip.
- Docs: short migration note — how to copy files from a v1 deploy and import.

**Out**

- Mongo dump import.
- Importing v1 engagements/runs.
- Perfect semantic fidelity for every historical field name — best-effort with warnings is required;
  silent drop of commands is not.

## Acceptance criteria

- [x] Importing the JSON fixture creates N templates with names and commands from the file.
- [x] Importing a directory of YAML testcases matches file count (minus invalids reported).
- [x] KB YAML becomes `content_note` rows with markdown bodies.
- [x] Dry-run writes zero rows and returns the same counts/warnings shape.
- [x] Auto format detection distinguishes JSON testcases vs KB yaml vs zip-of-yaml.
- [x] Partial file failures produce per-file errors without aborting the whole import unless
      `--fail-fast` (default continue; summarize).

## Tests

- Parser tests per format; dry-run; blctl smoke; golden counts on fixtures.

## Notes for the implementer

- Fix the v1 bug by supporting **both** layouts explicitly in code paths and tests named after them.
- Idempotency: re-import same file should upsert by a deterministic external_id derived from v1 ids
  or names — document so operators can re-run safely.

## Implementation notes

- OpenAPI: `POST /content/custom/import` multipart (`file` + `format`) with query
  `dryRun` / `failFast`. Sync → `200 ContentImportReport`; uploads over 1 MiB
  enqueue `v1_import` → `202 ContentSyncJob`. Authz: `content.manage`.
- Parser: `internal/content/v1import` — JSON array/object, YAML testcases
  (incl. `phase` alias), KB yaml, hybrid zip, and M2-011 custom export shape
  under `auto`.
- Domain: `content.Custom.Import` upserts by deterministic external ids
  (`v1:testcase:…`, `v1:kb:…`). Flat v1 `actions` → `command` with warnings;
  cleanup/inputArgs are never invented. KB overview+advice → markdown body.
- Store: `Procedures`/`Notes`/`Detections.ByExternalID` for upsert probes.
- Runner: `JobKindV1Import` branch (`executeV1Import`); `RunnerDeps.Custom`
  required for that path. Global job slot enforced on async only.
- Activity: `content.import.finished` / `.failed`; per-row
  `content.custom.created|updated` still fire on write.
- blctl: `content import --format --path --dry-run --fail-fast`.
- Fixtures under `internal/content/testdata/v1import/` (from pre-clean v1 samples
  + hybrid zip + deliberate broken yaml).
- Docs: `docs/content-v1-import.md`; links from `docs/cli.md` and
  `docs/content-custom.md`.
