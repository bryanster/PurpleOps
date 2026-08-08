# M5-014 — Cross-engagement compare UI

**Milestone:** M5 · **Size:** M · **Depends on:** M5-008, M5-013

## Why

The retest loop, made visible. `M3-EPIC` dropped rounds and told operators to recreate the
assessment; `M5-008` gave that workflow a query. This is the screen where somebody sees that the
remediation worked — and it is the screenshot `M7-002` wants as the README headline.

## Scope

**In**

- A **Compare** view under the Analytics tab (`M5-013`), reached from a "Compare with…" control.
- **Baseline picker**: engagements the caller can read, most recent first, same client surfaced
  first, with the current engagement excluded from the default list but selectable (`M5-008`'s
  self-compare identity case is a legitimate sanity check, not an error).
- **Summary row**: improved / regressed / unchanged / newly attempted / no longer attempted /
  incomparable, each a count, each filtering the table below.
- **Delta table**, one row per paired technique: technique id and name, baseline category → current
  category with the ordinal delta, both protections, and the classification. Sorted by regression
  first — the thing a client needs to see is the thing that got worse.
- **`incomparable` is shown, not hidden.** An unscored retest step is not an improvement, and a view
  that silently dropped those rows would overstate remediation. Its count sits in the summary row
  with the others.
- **Pin mismatch warning** when `pinMismatch` is present, naming both versions, explaining that
  technique ids are compared across ATT&CK versions.
- **Blind labelling per side.** The compare is computed under two scopes; if either is filtered, say
  which — "baseline shows revealed steps only" is a different sentence from the single-engagement
  banner and needs its own copy in `docs/analytics.md`.
- Deep-linkable: baseline id in the URL, so a compare can be pasted into a ticket or an email.
- Empty state when nothing pairs — two engagements with no techniques in common is a real answer, not
  an error.

**Out**

- Comparing more than two engagements.
- A stored "this is the retest of that" relationship (epic decision: pairing is ad-hoc).
- Exporting the compare — M6's engagement-comparison block is where it becomes a deliverable.
- Diffing narrative, findings, or evidence.

## Acceptance criteria

- [ ] Baseline picker lists only engagements the caller can read; a 403 from `M5-009`'s baseline
      check surfaces as "you cannot read that engagement", not a broken page.
- [ ] Direction is unmistakable in the UI: which column is baseline and which is current is stated in
      words, not implied by order. Swapping them changes every arrow, and a user who mixes them up
      reports the opposite conclusion to a client.
- [ ] Improvement and regression are distinguishable without colour.
- [ ] `incomparable` rows are visible and counted; a component test asserts they are not filtered out
      of the default view.
- [ ] Pin mismatch renders the warning with both version strings.
- [ ] Blind filtering on either side renders the per-side label.
- [ ] URL round-trips: loading the deep link reproduces the same view.
- [ ] Empty pairing renders the empty state, not a spinner or an error.
- [ ] Playwright: the thesis path — baseline engagement, retest engagement built from the open
      findings' steps, scored higher, compare shows the improvement. This is `PLAN.md` §9 step 6 in
      its rewritten form, and `M6-018` will build on it.

## Tests

- Component tests for each classification, the pin-mismatch banner, the blind labels, the empty
  state.
- URL round-trip.
- The Playwright thesis path above.

## Notes for the implementer

- The picker is the part users will find awkward. An engagement list with client name and date is
  usually enough; resist building a search UI before anyone has too many engagements to scroll.
- Copy matters more than layout here. "Detection improved from telemetry to technique" is a sentence
  a client understands; "ordinal +2" is not, and one of them ends up in a report.
