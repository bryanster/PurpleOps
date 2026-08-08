# SigmaHQ detection references

Blacklight installs **Sigma** rules as **reference only** detection content.
Rules are never executed, deployed, or converted to product-specific queries
(`PLAN.md` §3). Only rules that already carry ATT&CK technique mappings are
stored so the library stays technique-relevant.

## Default source row

Seeded by migration `0011_content` (disabled until an admin enables it):

| Field | Value |
|---|---|
| Kind | `sigma` |
| URL | `https://github.com/SigmaHQ/sigma/archive/refs/heads/master.zip` |
| Ref | `master` |

Fetch GETs the URL as-is. Offline bundle upload accepts the **same zip bytes**.

## Archive shape

Online and offline share one parse path. Acceptable payloads:

1. A GitHub-style zip of the repository (paths like
   `sigma-master/rules/windows/process_creation/….yml`).
2. A zip/tar(.gz) whose members sit under `rules/` or `rules-*` directories
   (`rules-emerging-threats`, `rules-threat-hunting`, `rules-dfir`, …).
3. A single bare rule YAML document (fixtures / tiny imports).

Non-rule paths (`.github/`, `tests/`, `docs/`, `deprecated/`,
`rules-placeholder/`) are skipped. Files are streamed entry-by-entry from the
archive — the adapter does not buffer every rule body before parsing.

## What is ingested

One `content_detection_rule_ref` row **per technique-mapped rule**:

| Column | Source |
|---|---|
| `external_id` | rule `id` when present; else archive-relative path |
| `name` / `description` | `title` / `description` |
| `technique_external_ids` | from tags (see below) |
| `level` | `level` |
| `rule_status` | `status` |
| `logsource` | `logsource` object as JSON |
| `rule_yaml` | full original YAML body (display/copy) |

Rules with **no** accepted technique tags are **skipped** (not stored). The job
message reports the skip count, for example:

```text
applied 1204 detection rules, skipped 318 unmapped
```

An archive of only unmapped rules succeeds with zero rows and a non-zero skip
count.

## Technique tag patterns

Conservative extraction — wrong links are worse than skips. Only these tags
become technique external ids:

| Tag | Stored id |
|---|---|
| `attack.t1059` | `T1059` |
| `attack.t1059.001` | `T1059.001` |
| `ATTACK.T1059.001` | `T1059.001` (case-insensitive) |

**Rejected** (among others):

- Tactic-only tags: `attack.execution`, `attack.persistence`
- Software / group tags: `attack.s0002`, `attack.g0016`
- Bare ids without the `attack.` prefix: `t1059`
- Any other `attack.*` label that is not `t` + 4 digits (+ optional `.` + 3 digits)

## External ids

Stable within the source so re-sync replaces rather than duplicates:

1. **Prefer** upstream rule `id` (UUID assigned by SigmaHQ).
2. **Otherwise** use the archive-relative path with forward slashes, after
   stripping a single GitHub archive root prefix (`sigma-master/`, …).

Duplicate external ids across files fail the job loudly.

## Rolling head

Version token is always `current`. A re-sync stages under `__staging__` then
promotes in one write transaction. A failed re-sync leaves the prior ready
catalog intact.

## Library API

| Method | Path | Notes |
|---|---|---|
| `GET` | `/api/v1/content/detection-rules` | Filters: `q`, `technique`, `level`, `sourceId`, `limit` |
| `GET` | `/api/v1/content/detection-rules/{id}` | `404` when source disabled; includes full `ruleYaml` |

`content.read`. Default lists **enabled** sources only.

API descriptions state that rules are never executed. UI copy (later tickets)
must say the same.

## Failures

Unreadable YAML, missing `title`, empty archive (no rule YAML), or duplicate
external ids → job `failed` with an operator-readable error naming the file when
possible. No partial best-effort catalog for that run.

## License

LGPL-2.1-or-later (SigmaHQ contributors), unless a rule file says otherwise.
SPDX and attribution live on the source row and are not stripped by the adapter.
