# M5 — Analytics (epic)

**State:** needs refinement · **Depends on:** M3

## Goal

Turn the execution data into the numbers a purple-team programme is judged on — in **SQL, not
application loops** (`PLAN.md` §5). This is the payoff for choosing DuckDB: v1's N×M read pattern
becomes single statements.

## Candidate tickets

| ID | Title | Notes |
|---|---|---|
| M5-001 | Analytics query layer | `internal/analytics`, one file per rollup, every query tested against a seeded fixture DB |
| M5-002 | Coverage per technique / tactic | Feeds the heatmap and the Navigator layer |
| M5-003 | Detection-category distribution | Uses the `none`..`technique` ordinal from M3-008 |
| M5-004 | MTTD analysis | Percentiles, not just means — the mean is misleading with a few undetected outliers |
| M5-005 | Protection rate | `blocked` / `partial` / `not_blocked` breakdown |
| M5-006 | Round-over-round deltas | **Blocked on product:** M3 dropped rounds. Re-scope to cross-engagement compare or defer. |
| M5-007 | Findings burndown | Open → resolved over time, per round |
| M5-008 | Read endpoints for all of the above | Consumed by both the UI and M6's report blocks — one source, two consumers |
| M5-009 | ATT&CK Navigator layer export | One query now (`PLAN.md` §5) |
| M5-010 | Data exports: JSON, CSV, full engagement archive | **With a round-trip test** — v1's `export/entire` wrote `export.csv` while `import/entire` read `export.json` |
| M5-011 | Dashboard UI: heatmap and scorecards | Replaces v1's hexagon graphic with a real heatmap |

## Open questions to resolve before writing tickets

1. **Denominator for coverage.** Coverage of *attempted* techniques, or of the whole ATT&CK matrix?
   They tell very different stories and clients will quote whichever we print. Decide, then label
   it unambiguously in the UI and in reports.
2. **MTTD when detection never happened.** Excluded, or counted as infinite/censored? Excluding
   inflates performance; include an explicit "undetected" count next to any MTTD figure.
3. **Cross-engagement analytics.** Does a programme-level view across engagements exist in v1 of the
   rebuild, or is everything scoped to one engagement? `PLAN.md` implies per-engagement; confirm.
4. **Rounds beyond two.** **M3 dropped rounds entirely** (recreate assessment). This question and
   M5-006 need a new framing before refinement — do not assume `round_id` exists.
5. **Navigator layer version.** Which Navigator schema version do we emit, and does it need to track
   the engagement's pinned ATT&CK version?

## Risks

- Analytics correctness errors are the ones clients notice, because they end up in a report with a
  logo on it. Every query needs a fixture-based test with **hand-computed** expected values — not
  values captured from the implementation's own output, which only tests that the code hasn't
  changed.
- Resist adding a caching layer before measuring. At this data volume DuckDB should answer in
  milliseconds; a cache would add staleness bugs to solve a problem that may not exist.
