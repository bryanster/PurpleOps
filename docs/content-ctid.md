# CTID adversary emulation plans

Blacklight installs **Center for Threat-Informed Defense (CTID)** adversary
emulation plans as a **catalog only** in M2. Plans and ordered steps are
browsable and syncable; turning a plan into an engagement Scenario is
implemented (`M3-012`, `PLAN.md` §3).

## Default source row

Seeded by migration `0011_content` (disabled until an admin enables it):

| Field | Value |
|---|---|
| Kind | `ctid` |
| URL | `https://github.com/center-for-threat-informed-defense/adversary_emulation_library/archive/refs/heads/master.zip` |
| Ref | `master` |

## How the fetch works

The URL above names a GitHub archive of the whole repository — around **640 MB**, and growing with
every tool, binary and packet capture the project adds. The plans this adapter reads come to about
**75 KB** of it. Downloading the archive to reach them eventually stopped working altogether: it
crossed `BLACKLIGHT_CONTENT_MAX_BYTES` (512 MiB by default) and the sync failed with
`download exceeds content max bytes limit`.

So a **GitHub** archive URL is not downloaded. The adapter reads the repository listing
(`GET /repos/{owner}/{repo}/git/trees/{ref}?recursive=1`), picks the plan files out of it with the
same rule `Parse` applies to archive entries, fetches those files from `raw.githubusercontent.com`,
and reassembles them into a small zip with the archive's own layout
(`{repo}-{ref}/{actor}/Emulation_Plan/yaml/…`). One sync moves roughly **2 MB instead of 640 MB** and
takes a few seconds. Everything downstream — parse, the raw snapshot, reprocess-from-raw, offline
bundle import — still sees a zip and is unchanged.

What this means in practice:

- **The raw snapshot is that small zip**, not the upstream archive. Its digest changes when a plan
  changes rather than on every commit to any file in the repository, so re-syncs that changed
  nothing are visible as such.
- **The GitHub API is used unauthenticated**, which is rate limited per address (60 requests an hour)
  and resets hourly. One listing call per sync is well inside it; a `403` from the listing says so in
  the job error.
- **A truncated listing fails the job.** The entries GitHub drops when truncating could be plans, and
  a catalog quietly missing an adversary is worse than a sync that says it could not read the
  repository. Import an offline bundle if you ever hit it.
- **Any other URL is fetched whole, as before** — a mirror, or a zip served from anywhere that is not
  `github.com` / `codeload.github.com`. Offline bundle upload accepts the **same zip bytes** it
  always did.

## Archive shape

Online and offline share one parse path. Acceptable payloads:

1. A GitHub-style zip of the [adversary_emulation_library](https://github.com/center-for-threat-informed-defense/adversary_emulation_library)
   repository (paths like
   `adversary_emulation_library-master/fin6/Emulation_Plan/yaml/FIN6.yaml`).
2. A zip/tar(.gz) whose members sit under `{actor}/Emulation_Plan/yaml/*.yaml`.
3. A single bare plan YAML document (fixtures / tiny imports).

**Supported layout (frozen):** machine-readable full-emulation plan YAML under
`Emulation_Plan/yaml/`. Each file is a YAML sequence whose first element is
`emulation_plan_details` and whose remaining elements are steps (CALDERA /
Atomic-style procedure entries).

**Out of scope / skipped:**

- `planners/` CALDERA planner YAML
- `micro_emulation_plans/`
- `Archive/`, docs, READMEs
- Unknown layouts → job **failed** (no half-import)

Files are streamed entry-by-entry from the archive.

## What is ingested

One `content_emulation_plan` row **per plan file**, plus ordered
`content_emulation_plan_step` rows:

| Plan column | Source |
|---|---|
| `external_id` | `emulation_plan_details.id` when present; else actor directory slug |
| `name` | `adversary_name` |
| `description` | `adversary_description` |
| `adversary_name` | `adversary_name` (threat actor / group text label) |
| `metadata` | JSON: `attack_version`, `format_version`, archive `path`, `actor_slug` |

| Step column | Source |
|---|---|
| `position` (`ordinal` on the wire) | 1-based **document order** in the YAML (dense, unique per plan) |
| `external_id` | step `id` when present; else `{plan_external_id}/{position}` |
| `name` / `description` | step fields |
| `technique_external_id` | `technique.attack_id` when it matches `T####` / `T####.###`; else empty |
| `procedure` | JSON object: platforms, executors (name/command/cleanup), input_arguments, tactic, procedure_group/step labels, cti_source, dependencies |

Missing technique on a step is **allowed** (empty string). The job success
message names how many steps lacked a technique when the count is non-zero.

An archive that yields **zero plans** after parse fails the job loudly.

## Ordinals

Steps are sorted and stored by `position` ascending. Upstream labels such as
`procedure_step: "2.1"` are preserved inside `procedure` for display and M3
import — they are **not** the ordinal. Re-sync of an unchanged file keeps the
same dense 1..N positions for the same document order.

## External ids

Stable within the source so re-sync replaces rather than duplicates:

1. **Plan:** prefer `emulation_plan_details.id`; else actor slug (`fin6`).
2. **Step:** prefer upstream step `id`; else `{plan_external_id}/{position}`.

Duplicate external ids across files fail the job loudly.

## Rolling head

Version token is always `current`. A re-sync stages under `__staging__` then
promotes in one write transaction. A failed re-sync leaves the prior ready
catalog intact.

## Library API

| Method | Path | Notes |
|---|---|---|
| `GET` | `/api/v1/content/emulation-plans` | Filters: `q`, `technique`, `sourceId`, `limit` |
| `GET` | `/api/v1/content/emulation-plans/{id}` | Detail with `steps` sorted by `ordinal` ascending |

`content.read`. Default lists **enabled** sources only.

## M3 import contract

Normative for M3-012 (`POST /engagements/{id}/import-plan`). M2 only catalogs;
M3 snapshots. See also [content-copy-on-use.md](content-copy-on-use.md).

| Scenario / step field (M3) | Catalog source |
|---|---|
| Scenario name / narrative | plan `name` / `description` |
| Scenario `source` | `ctid` |
| Scenario threat actor | plan `adversary_name` (text; optional later resolve against ATT&CK groups) |
| Scenario `source_ref` | plan `external_id` + weak `plan_id` lineage |
| Step ordinal | step `ordinal` (`position`) |
| Step display name | step `name` |
| Step description / procedure body | step `description` + snapshot of `procedure` JSON |
| Step `technique_external_id` | step `technique_external_id` (string; may be empty) |
| Step `attack_version` pin | engagement pin at import time — **not** plan `metadata.attack_version` |
| Executor / platform / command | from `procedure.executors` / `procedure.platforms` when present |

Technique ids are resolved against the engagement's pinned ATT&CK version.
ATT&CK need not be installed before CTID sync. Optional warn if ATT&CK is
present and an id does not resolve in the engagement's pinned version.

Catalog re-sync must not rewrite imported scenario steps (copy-on-use).

## Failures

Unreadable YAML, missing `emulation_plan_details`, missing `adversary_name`,
zero steps, empty archive (no plan YAML), or duplicate external ids → job
`failed` with an operator-readable error naming the file when possible. No
partial best-effort catalog for that run.

## License

Apache-2.0 (Center for Threat-Informed Defense and contributors). SPDX and
attribution live on the source row and are not stripped by the adapter.
