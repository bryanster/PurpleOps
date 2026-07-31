# M0B-013 — Playwright harness that fails loudly

**Milestone:** M0b · **Size:** M · **Depends on:** M0B-011, M0B-012

## Why

Straight from `PLAN.md` §9: v1's `global-setup.ts` **exited 0 and skipped the whole suite** when
nothing answered on `BASE_URL`. Every run was green and none of them tested anything. That is worse
than having no E2E tests, because it buys false confidence.

The harness comes now, empty, so the full E2E spec in `PLAN.md` §9 has somewhere to land as features
arrive — rather than being written under deadline pressure at M6.

## Scope

**In**

- `e2e/` — Playwright with TypeScript.
- Global setup that:
  - starts the server against a **fresh temp DuckDB file** (or reuses a running one if `BASE_URL` is
    set explicitly),
  - waits for `/api/v1/healthz` with a bounded timeout,
  - **throws** if the server never becomes healthy. Never `process.exit(0)`, never skip.
- Global teardown removing the temp database and evidence directory.
- Seeding hook: a documented way to put the system into a known state before a spec. Prefer
  `popsctl` commands (`M0B-014`) over direct SQL, so seeding exercises supported paths.
- One real smoke spec: load the app, assert the shell renders, assert the version screen shows the
  version the binary reports.
- Artifacts on failure: trace, screenshot, video, plus the **server log** — a failed E2E without the
  server's side of the story wastes an hour.
- CI job wired into `M0B-012`'s workflow, uploading artifacts.
- `docs/testing.md`: how to run E2E locally, headed, and how to debug with the trace viewer.

**Out**

- The full product-thesis spec from `PLAN.md` §9. It is built up ticket by ticket across M1–M6, and
  M6 owns its completion.

## Acceptance criteria

- [ ] With no server running and no `BASE_URL`, the harness starts one itself and the smoke spec
      passes.
- [ ] With `BASE_URL` pointing at a dead port, the run **fails** with a clear message naming the URL
      and the timeout. Demonstrate this in the PR — it is the reason the ticket exists.
- [ ] A deliberately broken assertion produces a trace and a screenshot in CI artifacts, and the
      server log is among them.
- [ ] Each spec file starts from a clean database — no order dependence between specs. Prove it by
      running the suite with `--repeat-each=2` and with shuffled order.
- [ ] No `waitForTimeout`/arbitrary sleeps. Use web-first assertions and explicit waits.
- [ ] Selectors are `getByRole` / `getByLabel` / `data-testid` — never CSS class chains, which will
      break constantly against Tailwind.
- [ ] The suite runs in CI on every PR and is a required check.

## Tests

The harness is test infrastructure; its verification is the two demonstrated behaviours above
(passes when healthy, fails loudly when not).

## Notes for the implementer

- Playwright's `webServer` config option can start the binary for you and handles readiness —
  use it rather than hand-rolling process management, but set `reuseExistingServer: false` in CI.
- Add a `data-testid` only when a role/label query genuinely can't express the target. Overusing
  test IDs couples tests to markup as badly as CSS selectors do.
