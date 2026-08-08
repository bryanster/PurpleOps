# M3 — Core domain (epic)

**State:** refined · **Depends on:** M1, M2 complete

## Goal

The product itself: **Engagement → Scenario → Step → Execution**, scored in ATT&CK Evaluations
terms (`PLAN.md` §2). Red and blue fill a shared workbook; findings track remediation work; the
engagement is the unit of record.

## Decisions (locked)

| Topic | Decision |
|---|---|
| **Rounds** | **Not in v1.** `PLAN.md` §2's retest-round model is deferred. Operators who need a post-remediation pass **recreate the assessment** (new engagement). No `round` table, no `(step_id, round_id)` grain, no round-2 materialization flow. **This is a deliberate PLAN.md deviation** — raise before re-introducing rounds; M5 "round-over-round" and the PLAN.md §9 E2E step 6 must be re-scoped when those milestones are refined. |
| Execution grain | **One execution per step** (`UNIQUE(step_id)`). Creating a step creates a `pending` execution in the same transaction. |
| Step edit after execution | **Soft freeze.** Once any execution row for the step has left `pending` (or has any red/blue write), identity fields are immutable: `technique_id` / `subtechnique_id` / `tactic_id`, `procedure` JSON, `template_id`, and attack lineage. Name, objective, target, tools, controls, ordinal remain editable. |
| Concurrent scoring | **Optimistic locking.** `execution.version` (integer, starts at 1); every red/blue PATCH requires the caller's version; mismatch → **409** with current row. No last-write-wins. |
| Evidence limits | Defaults: **25 MiB per file**, **2 GiB per engagement** (configurable). Content-addressed by sha256; delete execution/comment drops the **link** only; blob GC when refcount hits 0. MIME allowlist; never serve as executable or inline-HTML. |
| `detection_modifiers` | Locked PLAN.md set, multi-select, not mutually exclusive: `alert`, `correlated`, `delayed`, `config_change`, `residual_artifact`. Empty set allowed. Modifiers never alter the category ordinal (0–4). |
| Blind reveal | **Lead or red** may reveal a step (`revealed_at = now`). Engagement setting `auto_reveal_on_start` (bool, default false): when true, first red transition to `running` (or `complete` if skipped running) reveals the step. Closing an engagement does **not** bulk-reveal. Query-layer filter stays (`internal/store/blind`, `M1-013`). |
| Structure writers | **Lead + red** create/edit/reorder scenarios and steps. Blue and observer read only (plus blue detection writes on executions). |
| Authz additions | `workbook.write` (lead+red) for scenario/step structure + reveal; `evidence.write` (lead+red+blue, not observer) with side enforced in domain. Existing `engagement.*`, `member.*`, `execution.*`, `comment.write`, `finding.write` stay. |
| Milestone shape | **Single M3**, schema → APIs → imports → UI → load-test gate. No formal 3a/3b split. |
| Copy-on-use | Honor `docs/content-copy-on-use.md` and CTID import mapping in `docs/content-ctid.md`. No live FK from `app` → `content`. |
| Attack pin | `engagement.attack_version` required on create (or set before first step that resolves ATT&CK); `attackpin.AssertPinned` / `ResolveTechnique` on library-backed creates. Wire `attackpin.References` so version delete 409s when engagements pin it. |
| Outcome | **Derived, never stored** (`M3-008`). |
| Activity | Engagement-scoped verbs for structure, execution, evidence, comment, finding, reveal (same-txn via store `After` / `M1-015` pattern). SSE fan-out remains M4. |

## Tickets

Build roughly in this order — the dependency chain is real.

| ID | Title | Size | Depends on |
|---|---|---|---|
| [M3-001](M3-001-domain-schema.md) | `app` domain schema: engagement, scenario, step, execution, finding, evidence, comment | L | M1, M2 |
| [M3-002](M3-002-engagement-crud.md) | Engagement CRUD, status lifecycle, attack pin, mode | M | M3-001, M2-007 |
| [M3-003](M3-003-membership-api.md) | Engagement membership management API | M | M3-002, M1-012 |
| [M3-004](M3-004-scenarios.md) | Scenarios CRUD + reorder | M | M3-002 |
| [M3-005](M3-005-steps.md) | Steps CRUD, copy-on-use, soft freeze, reveal | L | M3-004, M2-007 |
| [M3-006](M3-006-executions-red.md) | Executions — red side PATCH + optimistic lock | M | M3-005 |
| [M3-007](M3-007-executions-blue.md) | Executions — blue side PATCH + optimistic lock | M | M3-006, M3-008 |
| [M3-008](M3-008-scoring-domain.md) | Scoring domain: category ordinal, modifiers, derived outcome, MTTD | M | M3-001 |
| [M3-009](M3-009-evidence-store.md) | Evidence blob store + upload/download API | L | M3-006 |
| [M3-010](M3-010-comments.md) | Comments on executions + edit history | S | M3-006 |
| [M3-011](M3-011-findings.md) | Findings + `finding_step` join | M | M3-006 |
| [M3-012](M3-012-ctid-import.md) | CTID emulation plan → Scenario import | M | M3-005, M2-010 |
| [M3-013](M3-013-atomic-to-step.md) | Atomic / procedure template → Step | M | M3-005, M2-008 |
| [M3-014](M3-014-engagement-ui.md) | Engagement UI: board / workbook | L | M3-003…M3-011, M0B-009 |
| [M3-015](M3-015-scoring-ui.md) | Scoring UI: 5-button scale + modifiers | M | M3-007, M3-008, M3-014 |
| [M3-016](M3-016-concurrency-load-test.md) | War-room concurrency load test (**gate before M4–M6**) | M | M3-007, M3-009 |

## Risks

- **PLAN.md drift on rounds.** The plan, E2E thesis, and M5/M6 epics still describe retest rounds. This epic is authoritative for M3 implementation; refine M5/M6/PLAN when those milestones open — do not silently re-add `round` mid-M3.
- Soft freeze must be enforced in **domain + tests**, not only UI — report correctness depends on stable technique/procedure identity on scored steps.
- Optimistic locking needs a version column before red/blue handlers land; do not ship PATCH without it.
- Evidence GC and quota are easy to get wrong under shared sha256; refcount tests are mandatory.
- **M3-016** validates the central architectural bet (`PLAN.md` §8). If it fails, fix the writer/batching here — do not defer past M3.

## Out of milestone (do not pull in)

- Retest **rounds**, round open/close, round-2 "re-run open findings", cross-engagement compare UI.
- SSE live updates / presence (M4) — activity rows only.
- Analytics rollups, Navigator, exports (M5).
- Report builder (M6).
- Engagement **clone** helper (nice-to-have later; recreate is manual in M3).
- Custom ATT&CK objects.
