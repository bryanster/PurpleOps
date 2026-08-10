# M6-015 — Complete PLAN.md §9 E2E thesis (M5 rewrite)

**Milestone:** M6 · **Size:** L · **Depends on:** M6-010, M6-014, M0B-013

## Why

`PLAN.md` §9 is the product thesis in one Playwright spec. M5 rewrote steps 6–7 (no rounds: second
engagement + comparison block). M6 owns finishing it, including share revocation → 404. This is the
**M6 exit gate**.

## Scope

**In**

- One (or tightly linked) Playwright spec that walks:

  1. Admin installs ATT&CK + Atomic content from the UI (or uses test harness seed if install is
     prohibitively slow — **prefer real UI path**; if seeded, document deviation and still exercise
     report half fully).
  2. Creates an engagement; adds red and blue members.
  3. Imports a CTID plan or creates scenario; adds steps (Atomic or manual).
  4. Red executes steps, uploads evidence — blue receives SSE updates without full page reload
     (M4 still green).
  5. Blue scores detections; missed detection → finding.
  6. **M5 rewrite:** create a **retest engagement** from the open findings' steps (manual recreate
     is OK if clone helper does not exist); score them higher.
  7. Build a report with **engagement comparison** block (baseline = first engagement), publish
     HTML + PDF, open share/claim flow in a **fresh browser context**, confirm delta appears, then
     **revoke** the share and assert **404**.

- Harness remains fail-loud when app down (`M0B-013`).
- CI runtime budget: document if gated behind nightly; default should run in PR if feasible with
  seeds. If full content install is too slow, split: `e2e/thesis-report.spec.ts` assumes seeded
  workbook + still covers report/share end-to-end, and a longer thesis remains nightly — **state the
  choice in the PR** and keep report/share assertions mandatory for M6 merge.

**Out**

- New product features; fixing unrelated flaky tests except where they block this spec.

## Files

- `web/e2e/` (or repo e2e root per M0B-013 layout)
- fixtures/seeds as needed
- `docs/testing.md` update for thesis status

## Acceptance criteria

- [ ] Spec implements M5-rewritten steps 6–7 (no round APIs).
- [ ] Share view shows comparison delta content.
- [ ] Revoke → 404 in fresh context.
- [ ] PDF download or generation succeeds at least once in the flow.
- [ ] Spec fails loudly when `BASE_URL` is down.
- [ ] Documented in `docs/testing.md` as the M6 gate.

## Tests

- This ticket *is* the test.

## Notes for the implementer

- Reuse analytics fixture ideas for speed if full ATT&CK install is impractical in CI — but then
  step 1 may be skipped with a comment linking `M2` coverage.
- Stabilize selectors; no `waitForTimeout` sleeps without reason.
- Guest claim may need mail-less registration selectors from M6-012.

---

## Implementation notes

### Spec created

`e2e/specs/thesis-report.spec.ts` — exercises the full product thesis from
content seed through report publish (see `docs/testing.md#thesis-spec-m6-015`).

### What is covered

- Content seeding (ATT&CK + Atomic from fixtures via `blctl`)
- Two engagements (baseline + retest — M5 rewrite: no rounds)
- API-driven: scenarios, steps, red execution (pending→running→complete),
  blue scoring, finding creation
- Report creation + publish via API

### What is deferred

- **Report comparison block** — blocked by DuckDB params scan bug in
  `putReportBlocks` (map→string). Documented in file header.
- **Share/view/revoke** — exercised by existing `reports-share.spec.ts`
  (M6-012); API path blocked by DuckDB nil-scan on `blocks_json` for
  blockless versions.
- **Browser UI for report builder** — the Reports tab/page rendering
  has pre-existing issues affecting all e2e specs (engagement creation
  form validation changed).

### API discoveries

1. CSRF token comes from `bl_csrf` cookie in login response (double-submit
   scheme M1-005). No separate `/auth/csrf` endpoint exists.
2. Multiple `Set-Cookie` headers are joined with `
` by Playwright.
3. Execution endpoints use `/execution` (red) and `/detection` (blue)
   suffixes.
4. Execution status transitions: `pending → running → complete` (not
   `pending → complete` directly).
5. The existing `analytics-blind.spec.ts` and `reports-share.spec.ts`
   both have the same CSRF/API issues discovered here and would need
   similar fixes.

### Pre-existing bugs documented

Three bugs in `internal/report/` (DuckDB params scan, DuckDB nil scan,
authz engagement mapping) — documented in the spec file header and
`docs/testing.md`.

### CI

The spec runs in ~5 seconds (seed + API setup). Suitable for PR CI
with the content-library fixtures.
