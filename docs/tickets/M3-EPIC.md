# M3 — Core domain (epic)

**State:** needs refinement · **Depends on:** M1, M2

## Goal

The product itself: Engagement → Scenario → Step, executed per **round**, scored in ATT&CK
Evaluations terms (`PLAN.md` §2). The retest loop is the differentiator — `(step_id, round_id)` is
the grain, so every before/after delta is a self-join.

## Candidate tickets

| ID | Title | Notes |
|---|---|---|
| M3-001 | Engagement CRUD + status lifecycle | Includes `attack_version` pinning and `mode` (`standard`\|`blind`) |
| M3-002 | Engagement membership management | Uses the `member.manage` action already defined in `M1-012` |
| M3-003 | Rounds | Ordinal, name ("Baseline", "Post-remediation"), open/close semantics |
| M3-004 | Scenarios | Ordinal, narrative, source (`manual`\|`ctid`\|`imported`), threat actor |
| M3-005 | Steps | ATT&CK technique/sub-technique/tactic (version-pinned), structured `procedure` JSON, target asset, tools, controls in scope, `revealed_at` |
| M3-006 | Executions — red side | `PATCH /executions/{id}/execution`: status, timings, command run, hosts, red notes |
| M3-007 | Executions — blue side | `PATCH /executions/{id}/detection`: detection category, modifiers, protection, `detected_at`, source/rule, severity, blue notes. **Separate endpoint and schema from red** (`PLAN.md` §4) |
| M3-008 | Scoring domain logic | `detection_category` ordinal `none`=0 … `technique`=4; outcome **derived, never entered**; MTTD = `detected_at − started_at` |
| M3-009 | Evidence store | Content-addressed by sha256, dedup for free; caption, side, size/type limits, safe serving (no inline HTML execution) |
| M3-010 | Comments | Per execution, edit history |
| M3-011 | Findings | Severity, owner, status, `created_from_execution`, `finding_step` join — the retest driver |
| M3-012 | Round-2 flow | "Re-run the steps behind open findings" as a first-class action |
| M3-013 | CTID emulation plan → Scenario import | Consumes M2-008's tables |
| M3-014 | Atomic template → Step | Preserves structure from M2-006 |
| M3-015 | Engagement UI: board / workbook | The main working surface — red and blue see role-appropriate views |
| M3-016 | Scoring UI | 5-button scale with hover definitions; modifiers optional and collapsed (`PLAN.md` §8) |
| M3-017 | Concurrency load test | Simulated war room: 20 users scoring concurrently, per `PLAN.md` §8. **Gate before building M4–M6 on top** |

## Open questions to resolve before writing tickets

1. **Round semantics.** Does opening round 2 copy all steps, or only those behind open findings?
   `PLAN.md` implies the latter is the common path but doesn't forbid a full re-run. Decide both the
   default and whether the other is available.
2. **Editing a step after execution.** If a step's procedure changes between rounds, is the round-1
   execution still comparable? Options: immutable steps once executed, versioned steps, or a warning.
   This affects report correctness, so decide early.
3. **Concurrent scoring conflicts.** Two blue users scoring the same execution — last-write-wins,
   optimistic locking with a version column, or field-level merge? Optimistic locking is the safe
   default; it needs a column, so decide before the schema lands.
4. **Evidence size limits and retention.** Max upload size, total quota, and what deleting an
   execution does to content-addressed blobs shared with another execution.
5. **`detection_modifiers` vocabulary.** `PLAN.md` lists alert, correlated, delayed, config_change,
   residual_artifact. Confirm this is final and whether any are mutually exclusive.
6. **Blind mode reveal.** Who reveals a step and when — the lead manually, on execution start, or on
   round close? Enforcement is already in the query layer (`M1-013`); the policy is undecided.

## Risks

- This milestone is the biggest by far. Consider splitting M3 into 3a (structure: engagement,
  scenario, step, round) and 3b (execution, scoring, evidence, findings) so each can land green.
- The load test (M3-017) validates the central architectural bet from `PLAN.md` §1. If it fails, it
  fails here — before M4–M6 are built on the assumption. Do not defer it to the end of the milestone.
