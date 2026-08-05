# Atomic Red Team content

Blacklight installs **Atomic Red Team** procedure templates from the published
GitHub archive. Structure is preserved: platforms, executor, command, cleanup,
and input args stay distinct fields so M3 can snapshot a real procedure onto a
step (`PLAN.md` §3).

## Default source row

Seeded by migration `0011_content` (disabled until an admin enables it):

| Field | Value |
|---|---|
| Kind | `atomic` |
| URL | `https://github.com/redcanaryco/atomic-red-team/archive/refs/heads/master.zip` |
| Ref | `master` |

Fetch GETs the URL as-is. Offline bundle upload accepts the **same zip bytes**.

## Archive shape

Online and offline share one parse path. Acceptable payloads:

1. A GitHub-style zip of the repository (paths like
   `atomic-red-team-master/atomics/T1059.001/T1059.001.yaml`).
2. A zip/tar(.gz) whose members sit under an `atomics/` directory.
3. A single bare technique YAML document (fixtures / tiny imports).

Non-test paths (`src/`, `bin/`, indexes) are skipped.

## What is ingested

One `content_procedure_template` row **per `atomic_tests` entry** (not one row
per technique file):

| Column | Source |
|---|---|
| `external_id` | `auto_generated_guid` when present; else `{attack_technique}/{zero-based-index}` |
| `name` / `description` | test name / description |
| `technique_external_ids` | `[attack_technique]` |
| `platforms` | `supported_platforms` (lowercased) |
| `executor` | `executor.name` |
| `elevation_required` | `executor.elevation_required` |
| `command` | `executor.command`, or `executor.steps` for `manual` |
| `cleanup` | `executor.cleanup_command` (empty when absent) |
| `input_args` | JSON array of `{name, description, type, default}` |
| `dependency_executor_name` / `dependencies` | when present; deps as JSON text |

## External ids

Stable within the source so re-sync replaces rather than duplicates:

1. **Prefer** upstream `auto_generated_guid` (UUID assigned by Atomic CI).
2. **Otherwise** derive `{attack_technique}/{index}` where `index` is the
   zero-based position in that file's `atomic_tests` array.

Duplicate external ids across files fail the job loudly.

## Rolling head

Version token is always `current`. A re-sync stages under `__staging__` then
promotes in one write transaction. A failed re-sync leaves the prior ready
catalog intact. Engagement steps that snapshot procedure JSON in M3 are
unaffected — see [content-copy-on-use.md](content-copy-on-use.md).

## Library API

| Method | Path | Notes |
|---|---|---|
| `GET` | `/api/v1/content/procedure-templates` | Filters: `q`, `technique`, `platform`, `sourceId`, `limit` |
| `GET` | `/api/v1/content/procedure-templates/{id}` | `404` when source disabled |

`content.read`. Default lists **enabled** sources only.

## Failures

Unreadable YAML, empty `atomic_tests`, missing `attack_technique` / executor /
platforms, or zero tests after normalize → job `failed` with an operator-readable
error naming the file when possible. No partial best-effort catalog for that
run.

## License

MIT (Red Canary and contributors). SPDX and attribution live on the source row
and are not stripped by the adapter.
