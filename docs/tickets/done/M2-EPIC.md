# M2 — Content system (epic)

**State:** refined · **Depends on:** M1 complete

## Goal

Reference content becomes **installable from the UI** (`PLAN.md` §3), not baked into a seeder. Every
source is a row users enable, sync, version and disable. First boot stays instant and offline —
directly replacing v1's ~1 GB git-clone-on-first-boot.

## Decisions (locked)

| Topic | Decision |
|---|---|
| Air-gap | Offline **bundle upload in M2** (UI + same parse path as fetch). Archive shape matches online HTTPS releases. |
| Raw upstream | Keep the **last successful raw snapshot per source/version** on disk. Enables reprocess without network. |
| Disable | **Hide + block new refs.** Rows stay; browse/search/pickers omit them; APIs refuse new references. Delete is separate and ref-checked. |
| Schedule | **Manual only** (UI Sync / `blctl content sync`). No periodic refresher in M2. |
| Progress transport | Land a **minimal shared SSE hub** in M2; M4 extends it. No throwaway progress channel. |
| Authz | **Platform admin** for every content mutation. `content.read` for everyone (members see empty CTA until an admin installs). |
| ATT&CK domain | **Enterprise only.** |
| Version model | **One `content_source`, many version snapshots** for ATT&CK. Atomic / Sigma / CTID are **single rolling head** (re-sync replaces catalog). |
| Pin surface | **M2** ships version catalog + resolve helpers + invariants (`internal/content/attackpin`, `docs/content-copy-on-use.md`); **M3** wires `engagement.attack_version`. |
| Fetch | **HTTPS release archives / known bundle URLs** — no git binary. Offline bundle is the same bytes. |
| Custom entities | `procedure_template` + `detection_rule_ref` + `content_note` (KB). Not custom tactics/techniques. |
| Concurrency | **One sync job globally.** Fetch may be slow; writes stay on the serialized writer. |
| Jobs | **DB-backed** job rows. Restart marks in-flight jobs interrupted (not silent). |
| Delete | Allowed only when **nothing references**; else **409** with counts. |
| Sigma | Ingest rules that carry **ATT&CK technique mappings only**. |
| CTID | **Catalog ingest only** in M2. Scenario import is M3-012. |
| v1 import | **UI upload + `blctl content import`**, shared parser. |
| UI | Full: sources admin + library browser + custom editor + import. |
| Licensing | Store SPDX + attribution per source; show in UI detail; include in export headers. |
| Object IDs | Surrogate **UUIDv7** PK + unique `(source_id, version, external_id)`. |
| Builtin rows | Migration seeds ATT&CK / Atomic / Sigma / CTID **disabled** with default URLs. |
| Copy-on-use | M3 steps **snapshot** procedure/plan fields; `template_id` is lineage only. M2 documents the contract. |
| Reprocess | Admin **Reprocess** from last raw snapshot (no re-fetch). |
| Search | Structured filters + substring on name / external ID / description. No FTS engine in M2. |
| Limits | Configurable; defaults **512 MiB** bundle/download, **30 m** job timeout. |
| Activity | Platform activity verbs for source/sync/custom/import lifecycle. |

## Tickets

Build roughly in this order — the dependency chain is real.

| ID | Title | Size | Depends on |
|---|---|---|---|
| [M2-001](M2-001-content-schema.md) | `content` schema: sources, versions, jobs, raw snapshots, seed rows | L | M1 |
| [M2-002](done/M2-002-source-registry-api.md) | Source registry API, enable/disable/delete, authz actions | M | M2-001 |
| [M2-003](M2-003-adapter-and-job-runner.md) | Adapter interface + global DB-backed job runner | L | M2-001, M2-002 |
| [M2-004](M2-004-sse-hub.md) | Minimal shared SSE hub + sync progress | L | M2-003, M1-013 |
| [M2-005](M2-005-bundle-upload-and-reprocess.md) | Offline bundle upload + reprocess-from-raw | M | M2-003, M2-004 |
| [M2-006](M2-006-attack-adapter.md) | ATT&CK Enterprise adapter (multi-version) | L | M2-003, M2-005 |
| [M2-007](M2-007-attack-version-pin-surface.md) | ATT&CK version catalog & pin surface | M | M2-006 |
| [M2-008](M2-008-atomic-adapter.md) | Atomic Red Team adapter | M | M2-003, M2-005 |
| [M2-009](M2-009-sigma-adapter.md) | Sigma adapter (technique-mapped rules) | M | M2-003, M2-005 |
| [M2-010](M2-010-ctid-adapter.md) | CTID emulation-plan catalog adapter | M | M2-003, M2-005 |
| [M2-011](M2-011-custom-content-api.md) | Custom content API: templates, rules, notes | M | M2-001, M2-002 |
| [M2-012](M2-012-v1-format-import.md) | Import v1 `testcases.json` + knowledgebase YAML | M | M2-011 |
| [M2-013](M2-013-library-browser-ui.md) | Content library browser UI | L | M2-006…M2-011, M0B-009 |
| [M2-014](M2-014-sources-admin-ui.md) | Sources admin UI: sync, bundle, status, reprocess | L | M2-002…M2-005, M0B-009 |
| [M2-015](M2-015-custom-and-import-ui.md) | Custom editor + v1 import UI | M | M2-011, M2-012, M2-013 |
| [M2-016](M2-016-sync-write-load-test.md) | Sync write load test (serialized writer fairness) | M | M2-006, M2-008 |

## Risks

- **Upstream schema drift** — adapters must fail loudly with a useful error rather than importing
  garbage. Test against checked-in fixtures so CI needs no network (`PLAN.md` §9).
- **Sync writes are the largest write volume in the system.** They must go through the serialized
  writer (`M0B-003`) in batches without starving interactive users. **M2-016** is the gate.
- Version-pinning correctness is subtle; get **M2-007** reviewed with the same care as `M1-012`.
- The M2 SSE hub must stay minimal — topic fan-out, authz, backpressure — so M4 can own presence
  and engagement streams without a rewrite.
- Offline bundle and online fetch **must share one parse path**; two parsers will drift.

## Out of milestone (do not pull in)

- Periodic / scheduled sync.
- ATT&CK Mobile / ICS.
- Multi-version storage for Atomic / Sigma / CTID.
- CTID → Scenario import (M3).
- Engagement `attack_version` column and UI (M3); M2 only prepares the pin API.
- Full M4 collaboration features (presence, engagement event topics) beyond what the thin hub needs.
- Custom ATT&CK objects (tactics/techniques/groups).
