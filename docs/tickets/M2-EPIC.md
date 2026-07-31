# M2 — Content system (epic)

**State:** needs refinement before implementation · **Depends on:** M1 complete

## Goal

Reference content becomes **installable from the UI** (`PLAN.md` §3), not baked into a seeder. Every
source is a row users enable, sync, version and disable. First boot stays instant and offline —
directly replacing v1's ~1 GB git-clone-on-first-boot.

## Candidate tickets

| ID | Title | Notes |
|---|---|---|
| M2-001 | `content_source` registry: schema, CRUD API, status model | `id, kind, name, url, ref, enabled, status, last_synced_at, item_count, error` |
| M2-002 | Adapter interface + background job runner | `Fetch → Parse → Normalize → Upsert`; resumable, cancellable, progress reported |
| M2-003 | Sync progress over SSE | Overlaps M4's hub — decide whether M4's hub lands early or M2 ships a narrow version |
| M2-004 | ATT&CK adapter, **multi-version** | `mitre-attack/attack-stix-data`; tactics, techniques, sub-techniques, data sources, mitigations, groups, software. Multiple versions coexist |
| M2-005 | ATT&CK version pinning surface | Engagements pin a version (`PLAN.md` §2); syncs add versions, never mutate a pinned one |
| M2-006 | Atomic Red Team adapter | `procedure_template` **preserving structure** — platform, executor, command, cleanup, input args. Do not flatten to one string as v1 did |
| M2-007 | Sigma adapter | `detection_rule_ref` indexed by technique. Reference only — never executed or deployed |
| M2-008 | CTID emulation-plan adapter | `emulation_plan` + ordered `emulation_plan_step`; consumed as a ready-made Scenario in M3 |
| M2-009 | Custom content CRUD | `source='custom'`, editable in UI, exportable as YAML/JSON |
| M2-010 | Import of existing v1 formats | `custom/testcases.json` and `custom/knowledgebase/*.yaml`. Note: v1's seeder globbed `custom/testcases/*.yaml` while the repo shipped `custom/testcases.json` — the import must handle what actually exists |
| M2-011 | Content browser UI | Install / sync / disable, per-source status, error surfacing, search across techniques |

## Open questions to resolve before writing tickets

1. **Sync without internet.** Air-gapped installs are plausible for this audience. Do we support an
   offline bundle upload per source? If yes it changes the adapter interface, so decide first.
2. **Storage of raw upstream data.** Keep the fetched STIX/YAML for reproducibility, or only the
   normalized rows? Affects disk footprint and the ability to re-normalize after an adapter bugfix.
3. **Disable semantics.** Does disabling a source hide its content or delete it? Engagements
   referencing it must not break — probably hide, with deletion a separate explicit action.
4. **Sync scheduling.** Manual only in v1, or a periodic background refresh? `PLAN.md` says
   "live-synced from upstream" but pins per engagement, so manual is defensible and simpler.
5. **Content licensing.** ATT&CK, Atomic and Sigma have distinct licences and attribution
   requirements. Confirm what must be displayed in-app and in exported reports.

## Risks

- **Upstream schema drift** — adapters must fail loudly with a useful error rather than importing
  garbage. Test against checked-in fixtures so CI needs no network (`PLAN.md` §9).
- **Sync writes are the largest write volume in the system.** They must go through the serialized
  writer (`M0B-003`) in batches without starving interactive users. Load-test this.
- Version-pinning correctness is subtle; get `M2-005` reviewed with the same care as `M1-012`.
