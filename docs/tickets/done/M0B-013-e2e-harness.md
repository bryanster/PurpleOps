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

- [x] With no server running and no `BASE_URL`, the harness starts one itself and the smoke spec
      passes.
- [x] With `BASE_URL` pointing at a dead port, the run **fails** with a clear message naming the URL
      and the timeout. Demonstrate this in the PR — it is the reason the ticket exists.
- [x] A deliberately broken assertion produces a trace and a screenshot in CI artifacts, and the
      server log is among them.
- [x] Each spec file starts from a clean database — no order dependence between specs. Prove it by
      running the suite with `--repeat-each=2` and with shuffled order.
- [x] No `waitForTimeout`/arbitrary sleeps. Use web-first assertions and explicit waits.
- [x] Selectors are `getByRole` / `getByLabel` / `data-testid` — never CSS class chains, which will
      break constantly against Tailwind.
- [x] The suite runs in CI on every PR and is a required check.

## Tests

The harness is test infrastructure; its verification is the two demonstrated behaviours above
(passes when healthy, fails loudly when not).

## Notes for the implementer

- Playwright's `webServer` config option can start the binary for you and handles readiness —
  use it rather than hand-rolling process management, but set `reuseExistingServer: false` in CI.
- Add a `data-testid` only when a role/label query genuinely can't express the target. Overusing
  test IDs couples tests to markup as badly as CSS selectors do.

---

## Implementation notes

### `webServer` is not used, and that was the central decision

The implementer note above suggests Playwright's `webServer`. It was tried and rejected: it starts
**one** server for the whole run, and one server means one database shared by every spec file —
which contradicts the fourth acceptance criterion directly. Choosing `webServer` here would have
bought convenience today and an order-dependent suite from M1 onwards, discovered by whoever first
ran a single spec file and got a different answer.

Instead, `e2e/harness/pool.ts` holds one server per worker and replaces it whenever the worker moves
to a new spec file (and to a new `--repeat-each` copy of the same file). Each server gets its own
directory under the run's temp root: its own DuckDB file, its own `evidence/`, its own log. A
restart costs about a second — opening the database and applying migrations — which is the right
price for a suite whose result does not depend on the order it happened to run in.

Process management is still not hand-rolled in any interesting sense: `startServer` spawns, waits
for `/api/v1/healthz`, and terminates with SIGTERM then SIGKILL. What `webServer` would have added
beyond that is the thing that had to go.

The `reuseExistingServer` idea survives as the `BASE_URL` switch, which is the same decision made
explicit: set it and the harness tests the server you started; leave it unset and it owns the
lifecycle. There is no third state where it silently reuses something it did not expect.

### Global setup does less than the scope describes

The scope has global setup starting the server. It does not: it decides the mode, probes an external
`BASE_URL` (throwing if nothing healthy answers), and makes the run's temp directory. Starting
servers belongs to the pool, because the pool starts several. A preflight boot in global setup was
considered for a fail-once-early message and dropped as a second mechanism doing the first one's
job — the pool's failure already quotes the tail of the server's own log, which is strictly more
useful:

```
Error: No healthy PurpleOps server answered at http://127.0.0.1:62835/api/v1/healthz
  waited:       752 ms of a 30000 ms budget
  last attempt: the process exited with status 1 before it became healthy

smoke: the server the harness started never became healthy.

Last 40 lines of /tmp/purpleops-e2e-l33CKG/w0-1-smoke/server.log:
purpleops: PURPLEOPS_DB_PATH: the parent directory does not exist
```

Note the two numbers on the `waited:` line. A run that used its whole budget was waiting; one that
stopped early knows the process it was waiting for is already dead. Reporting only the budget would
have been a small lie in the second case.

### Seeding runs before the server boots, not inside a test

DuckDB admits one writer to a database file at a time, so `popsctl` cannot open a database a running
server is holding. A seed helper callable from within a test would therefore have failed on a lock
the first time anyone used it for real.

So seeding is an option declared at the top of a spec file and applied to the fresh database before
that file's server starts:

```ts
test.use({ seed: [['users', 'create', '--email', 'red@example.test', '--role', 'red']] })
```

The constraint turned out to be a better design than the one it ruled out: what a spec file assumes
about the world is now visible at the top of it rather than buried in a `beforeEach`. A failing seed
step fails the file loudly and never runs its tests against a half-seeded database.

`popsctl` has only `--version` until `M0B-014`, which is what `specs/isolation.spec.ts` seeds with.

### `specs/isolation.spec.ts` — the harness checking itself

Not in the scope, added deliberately. Two of the acceptance criteria are claims about the harness
that nothing else would notice breaking: that a spec file's database is its own and fresh, and that
the seed hook ran against it. The spec asserts both. It also gives the suite a second file, without
which "no order dependence between specs" cannot be demonstrated at all.

It skips itself when `BASE_URL` is set — there is no harness-created database to inspect. That is a
conditional skip with a stated reason, not the unconditional one this ticket exists to prevent;
`eslint-plugin-playwright`'s `no-skipped-test` is set to `error` with `allowConditional: true` so the
difference is enforced rather than reviewed.

### Two acceptance criteria are lint rules now

`playwright/no-wait-for-timeout` and `playwright/no-skipped-test` are errors, so an arbitrary sleep
or a blanket skip fails the build instead of needing a reviewer to spot it. `no-floating-promises`
is on for the same reason: an un-awaited `expect` passes whatever the application did, which is the
most common way a Playwright suite quietly stops testing anything.

### Demonstrations

| Criterion | Command | Result |
|---|---|---|
| Starts its own server | `make e2e` | 3 passed |
| Dead `BASE_URL` fails loudly | `BASE_URL=http://127.0.0.1:45999 PURPLEOPS_E2E_HEALTH_TIMEOUT_MS=3000 npx playwright test` | exit 1, message naming the URL, the elapsed time, the budget and `ECONNREFUSED` |
| Missing binary fails loudly | `PURPLEOPS_E2E_BINARY=/nonexistent/purpleops npx playwright test` | exit 1, "is missing or not executable … run `make build`" |
| Server dies at startup | binary replaced with one that exits 1 | exit 1, with the server's own stderr quoted |
| Artifacts on failure | version assertion changed to a wrong value | `trace.zip`, `test-failed-1.png`, `video.webm`, `error-context.md` and `purpleops-server.log` in `test-results/<test>/` |
| Clean database per file | `--repeat-each=2`, both file orders, each file alone | pass in every arrangement; `PURPLEOPS_E2E_KEEP=1` shows four distinct server directories for two files × two repeats |

Playwright has no `--shuffle`, so order is varied by naming the spec files explicitly in different
orders. `docs/testing.md` says so where someone will read it.

### Smaller things

- **`PURPLEOPS_ENV=development`** for the harness's servers. The suite drives a browser over plain
  HTTP on loopback, and production posture makes session cookies `Secure` (M1). The production path
  over TLS is the container smoke test's territory. `harness/server.ts` says this where it is set.
- **Every `PURPLEOPS_*` variable is stripped** from the environment the server is started with. A
  `.env` sourced into a developer's shell is the normal way to run the server by hand, and it must
  not be able to decide which database the suite writes to — the first symptom would have been
  someone's own data changing.
- **The port is reserved by the harness**, not by binding `:0` in the server: `PURPLEOPS_BASE_URL`
  has to be known at startup and cannot be set after the fact.
- **The server log is copied into the test's output directory** before being attached, rather than
  attached where it lies. Global teardown deletes the run directory, and an attachment pointing into
  it would be a dead link by the time anyone clicked it. Copying also puts the log in
  `test-results/` beside the trace, so the CI artifact carries it with no extra wiring.
- **`e2e/` is not an ESM package.** Playwright's CommonJS path is the one that resolves
  extensionless relative imports; `"type": "module"` would mean writing `./server.js` in every
  import of a `.ts` file. `tsconfig.json` explains it.
- **`make e2e` uses `npm run`, not `npm exec`.** Only the former runs with `e2e/` as the working
  directory; `npm exec` stays in the caller's, where Playwright finds no config and collects every
  `*.test.ts` under `web/` instead. Found the hard way.
- **`e2e/node_modules` is excluded in `.golangci.yml`**, for the reason `web/node_modules` already
  was: npm packages sometimes ship Go sources, and `flatted` fails `errcheck`.
- **`make help`'s grep pattern gained `0-9`** — it had been silently dropping any target with a
  digit in its name, which until now was none of them.
- `make lint` and the CI lint job now cover `e2e/`; Dependabot has a third npm entry for it,
  ungrouped from `web/` so a Playwright bump cannot hold up a security patch for the SPA.

### Where the rest of it goes

The `PLAN.md` §9 spec — content install, an engagement over two rounds, a shared report — lands a
milestone at a time, as the ticket says. What exists now is the place for it to land and the two
guarantees it will rely on: a run that could not test anything fails, and every spec file starts
from a clean database. Both are documented in [`docs/testing.md`](../../testing.md) and in
[`e2e/README.md`](../../../e2e/README.md).
