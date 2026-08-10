# M7-008 — Performance sanity pass (gates + report load)

**Milestone:** M7 · **Size:** M · **Depends on:** M6-015, M3-016, M4-010, M5-015

## Why

M3–M5 each left a load gate. M6 stacked HTML render, analytics-heavy blocks, and Chromium PDF on the
same process. Cutover needs evidence those budgets **still hold** with reporting on top — not a new
benchmark framework, and not a vibes-based "seems fine".

M5-015 explicitly deferred full report-path measurement to this ticket.

## Scope

**In**

- Re-run and record (completion notes + `docs/testing.md` pointers):

  | Gate | Command / package | Budget source |
  |---|---|---|
  | War-room writes | `M3-016` / `internal/engagement/loadtest` | p95 interactive write |
  | SSE war-room | `M4-010` / events loadtest | publish lag + write non-regression |
  | Analytics queries | `M5-015` / `internal/analytics/loadtest` | per-rollup p95 under write load |

- **Report path sanity** (new measurements, same style as existing gates):
  - Realistic engagement fixture (reuse analytics load fixture scale or the thesis seed).
  - Draft/full report render HTML for a block set that includes analytics + detail blocks (heatmap,
    scorecard, compare, findings, walkthrough) — p95 / max wall clock.
  - Publish path (snapshot + store) once per iteration OK.
  - PDF render smoke under load **N= small** (Chromium is heavy): assert no errors and a documented
    upper bound; do not set a fantasy sub-second PDF budget.
  - Concurrent: analytics or war-room writes overlapping one render should not deadlock or starve
    the serialized writer beyond documented factor.
- CI: keep existing scaled-down gates green; add a **scaled-down** report render assertion if one does
  not exist (fail on accidental N+1 render or loading full ATT&CK in a loop per block).
- If a budget fails: fix query/render/pool — **do not** raise budgets to green the gate. If product
  reality forces a budget change, record evidence and an explicit decision in completion notes
  (same rule as M5-015).

**Out**

- Multi-node perf, CDN, k6 public internet tests.
- Pixel-perfect PDF timing SLOs for enterprise sales sheets.
- Caching layers (still forbidden without a dedicated decision ticket).

## Files

- `internal/report/loadtest` or extension of existing loadtest packages
- `docs/testing.md` (commands + tables)
- This ticket completion notes with numbers

## Acceptance criteria

- [ ] M3-016, M4-010, M5-015 full or documented standard load commands re-run; numbers recorded and
      compared to original completion notes (no silent >2× regression without fix or decision).
- [ ] Report HTML render budget recorded; CI scaled-down gate fails on a deliberate break once during
      development (describe in notes).
- [ ] PDF path: at least smoke under the fixture with zero render errors; resource cleanup (no
      zombie Chromium pile-up after N renders).
- [ ] `docs/testing.md` lists M7-008 commands next to the earlier gates.
- [ ] Writer interactive p95 remains within M3-016 budget family while a render runs (measure).

## Tests

- The load/re-run commands are the tests.
- Mutation or deliberate slow render once to prove the CI tripwire.

## Notes for the implementer

- Reuse fixture generators; do not check in multi-GB evidence.
- Chromium tests may be `BLACKLIGHT_LOADTEST=1` full-only; CI stays honest with HTML render.
- Read M6-010 sandbox/compose notes before blaming PDF slowness on app code.
