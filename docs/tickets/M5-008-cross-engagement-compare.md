# M5-008 — Cross-engagement compare rollup

**Milestone:** M5 · **Size:** L · **Depends on:** M5-004, M5-005

## Why

`M3-EPIC` dropped retest rounds and told operators to recreate the assessment instead, explicitly
deferring to M5 the question of what replaces round-over-round deltas. This is that answer, and it is
load-bearing: the retest loop is what `M7-002` wants as the README headline, and without a compare
there is nowhere in the product it can be seen.

Two engagements — a baseline and a retest — matched on technique identity, with the delta per
technique. `PLAN.md` §9 steps 6–7 are rewritten around it (epic decision); M6's round-comparison
block becomes the engagement-comparison block over this query.

## Scope

**In**

- `internal/analytics/compare.go` — `Compare(ctx, baseline Scope, current Scope)`.

  **Two scopes, not one.** The caller may hold different seats in the two engagements — lead in the
  retest, blue in the baseline — and each side is filtered by its own. A single scope would quietly
  apply the wrong seat to one half.

- **Matching**, in order:
  1. `(technique_id, subtechnique_id)` — the primary key of the comparison.
  2. `template_id` as a tiebreaker when one technique has several steps on both sides, so
     "the same Atomic test" pairs with itself rather than with an arbitrary sibling.
  3. Anything still unpaired is `addedTechniques` (only in current) or `removedTechniques` (only in
     baseline). Neither is an error; a retest that drops out-of-scope techniques is normal.

- **Per-paired-technique output:** baseline category + ordinal, current category + ordinal, ordinal
  delta, baseline and current protection, and a classification:

  | Class | Meaning |
  |---|---|
  | `improved` | Ordinal rose, or protection strengthened at equal ordinal |
  | `regressed` | Ordinal fell, or protection weakened |
  | `unchanged` | Same on both |
  | `newlyAttempted` / `noLongerAttempted` | Attempted on one side only |
  | `incomparable` | Either side is `unscored` — **not** counted as a change in either direction |

- **`incomparable` is the important one.** An unscored retest step is not an improvement over a
  detected baseline, and a compare that treated NULL as zero would report remediation that did not
  happen. Its count is a required field.
- Summary counts alongside the per-technique rows, so a report can print "14 improved, 2 regressed,
  6 unchanged, 3 incomparable" without summing client-side.
- **ATT&CK pin mismatch:** if the two engagements pin different versions, compare anyway (external
  ids are stable across most versions) and return `pinMismatch: {baseline, current}` in the response
  so the UI and the report block can warn. Refusing would make the common case — a retest months
  later, after a content update — impossible.

**Out**

- Authz over the two engagements — the handler's job (`M5-009`), and it must check **both**.
- A stored `baseline_engagement_id` link (epic decision: pairing is ad-hoc).
- Comparing more than two engagements, or a trend across many.
- Comparing findings, evidence, or narrative — techniques and scores only.
- UI (`M5-014`).

## Acceptance criteria

- [ ] One statement for the paired rollup. Classification may be a SQL `CASE`; it may not be a Go
      loop over rows.
- [ ] Hand-computed against `analyticstest`'s baseline and retest engagements, which already include
      an improved, a regressed, an unchanged, an added and a removed technique.
- [ ] `incomparable` is never `improved` or `regressed`: a fixture pair with a scored baseline and an
      unscored retest lands in `incomparable`, and the summary counts prove it.
- [ ] Sub-techniques pair with sub-techniques. `T1059.001` in the baseline does **not** pair with
      `T1059` in the retest — asserted explicitly, because it is the mistake the join invites.
- [ ] A technique with several steps on both sides pairs by `template_id` where available, and where
      not, aggregates best-of on each side rather than emitting a cross product. Tested with a
      two-steps-each fixture case.
- [ ] Compare is **not symmetric** and does not pretend to be: swapping baseline and current turns
      every `improved` into `regressed`, asserted by running it both ways.
- [ ] Blind is applied per side. Blue in a blind baseline compared against a standard retest sees
      only revealed baseline techniques, and the unrevealed ones do not leak in as
      `newlyAttempted` on the current side. **This is the leak to test for**, and it is subtle: the
      absence of a baseline row must not become evidence of a new technique.
- [ ] Differing ATT&CK pins produce results plus `pinMismatch`, not an error.
- [ ] Comparing an engagement with itself returns everything `unchanged` and nothing else — a cheap
      identity check that catches most join errors.

## Tests

- The full classification table, hand-computed, one fixture row per class.
- Symmetry-inversion test.
- Self-compare identity test.
- Sub-technique non-pairing.
- Multi-step pairing via `template_id`.
- The blind cross-leak case above, from both seats.
- Pin mismatch.

## Notes for the implementer

- Technique identity across two engagements is trustworthy **because of** `M3-EPIC`'s soft freeze:
  once an execution leaves `pending`, `technique_id` / `subtechnique_id` / `procedure` /
  `template_id` are immutable. If a compare returns nonsense, suspect a step edited before freeze
  engaged before suspecting this join.
- `template_id` is a lineage string with no FK (`docs/content-copy-on-use.md`) and may be empty on
  hand-built steps. It is a tiebreaker, never a requirement.
- Best-of aggregation must match `M5-004`'s — same definition, and ideally the same shared fragment.
  Two different notions of "this technique's category" in one milestone is a bug waiting for a
  report to expose it.
