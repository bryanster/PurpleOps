# M4-009 — Blind mode end to end (SSE + Playwright)

**Milestone:** M4 · **Size:** L · **Depends on:** M4-005, M4-006, M4-008, M3-005

## Why

Blind mode is a product promise and a security boundary. REST conceal was M3;
live collaboration multiplies leak surfaces (SSE live, catch-up, presence focus,
activity rail). This ticket is the proof that blue learns about steps only when
red/lead reveals them — including over the wire.

## Scope

**In**

- **Backend hardening pass** (fix any residual gaps found while writing tests):
  - Live fan-out + catch-up + presence snapshot/SSE + activity list all use the
    same visibility rules for blue in `mode=blind`.
  - Reveal emits `step.revealed`; blue then receives subsequent events for that
    step and can refetch step/execution bodies.
  - Audit: reveal is in activity with actor; rail shows it to members who may
    know (blue sees reveal when it happens — that is the point).
- **API integration tests** (table-driven where possible):
  - Blue subscribed: create unrevealed step as red → blue gets **no** SSE event
    naming that step id.
  - Reveal → blue gets `step.revealed` (and/or structure invalidation) → GET step
    succeeds.
  - Presence: red focuses unrevealed step → blue’s GET presence and SSE show red
    online with **focus stripped**.
  - Catch-up: blue reconnects after remote creates+reveals; only post-visibility
    history appears.
- **Playwright war-room script** (extend M0B-013 harness):
  1. Lead creates blind engagement, adds red+blue.
  2. Red adds scenario/step, leaves unrevealed.
  3. Blue workbook shows no step; no name leakage in empty state.
  4. Red reveals (or auto-reveal path if exercised).
  5. Blue sees step appear without full page reload (SSE path).
  6. Optional: presence avatar visible; focus not leaking labels of hidden steps.
- Docs: short operator note on blind + live updates in existing engagement or
  security doc if one exists; else a paragraph in `docs/` only if needed.

**Out**

- Changing reveal policy (still M3-EPIC).
- Standard-mode regressions except smoke.
- Load/stress (`M4-010`).

## Acceptance criteria

- [ ] All API leak tests above green in CI.
- [ ] Playwright blind script green in CI (or marked consistently with other
      e2e jobs).
- [ ] No intentional blue UI that lists unrevealed counts (“3 hidden steps”)
      unless product already committed — default **no count leakage**.
- [ ] Activity rail + SSE + REST agree on visibility for a fixed fixture.

## Tests

- Go integration tests under `internal/httpapi` or `internal/events`.
- Playwright spec under existing e2e tree.
- Update authz/blind matrix if new event paths need rows.

## Notes for the implementer

- Treat unexpected step id in blue SSE payload as a **security failure**, not a
  flaky test.
- Prefer one `VisibleActivity(subject, entry)` / focus filter used everywhere.
- Auto-reveal-on-start: include at least one API test so live path matches M3-006.
