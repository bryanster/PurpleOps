# Testing

Four layers, run by three commands. This page is mostly about the fourth, because it is the one
with moving parts.

| Layer | Command | What it drives |
|---|---|---|
| Go unit and store | `make test` / `make test-race` | Packages, and a temp DuckDB file for the store |
| Web unit | `make test` | `vitest` + Testing Library, with MSW standing in for the API |
| Container | `make docker-smoke` | The image: boots with no network, persists, has Chromium |
| **End to end** | **`make e2e`** | **A browser against the real binary and a real database** |

[`PLAN.md`](../PLAN.md) §9 sets the bar per layer. Coverage is reported, never gated.

## The end-to-end suite

[`e2e/`](../e2e) is a Playwright project of its own. It drives `bin/blacklight` — the binary, with
this build's SPA embedded — over HTTP, against a DuckDB file created for the run and deleted after
it. Nothing is mocked. If it passes, those pieces genuinely fit together.

```sh
make e2e-browsers   # once, and again after a Playwright upgrade (~150 MB)
make e2e            # builds the SPA and both binaries, then runs the suite
```

`make e2e` builds first on purpose: a suite run against yesterday's binary is worse than no suite,
because it is green about the wrong thing.

Flags go through `E2E_ARGS`, or run Playwright directly from `e2e/`:

```sh
make e2e E2E_ARGS='--grep version'
cd e2e && npx playwright test specs/smoke.spec.ts
```

### It fails when there is nothing to test

This is the point of the harness, and it is worth knowing before the first time you see it.

v1's `global-setup.ts` called `process.exit(0)` when nothing answered on `BASE_URL`. Every run was
green and none of them ran a test — which is worse than having no end-to-end tests at all, because
it buys confidence nothing earned. `PLAN.md` §9 names it.

The replacement has two modes and neither can pass silently:

- **`BASE_URL` unset** — the harness owns the servers. Each spec file gets its own `blacklight`
  process on its own fresh database. A server that never becomes healthy fails the run, quoting the
  tail of its own log.
- **`BASE_URL` set** — you are pointing the suite at a server you started. Global setup probes
  `$BASE_URL/api/v1/healthz` and **throws** if nothing healthy answers within the budget:

  ```
  Error: No healthy Blacklight server answered at http://127.0.0.1:45999/api/v1/healthz
    waited:       3065 ms of a 3000 ms budget
    last attempt: fetch failed: connect ECONNREFUSED 127.0.0.1:45999
  ```

  ```sh
  make run &                                 # in one terminal
  BASE_URL=http://localhost:8080 make e2e    # in another
  ```

### Every spec file starts from a clean database

Each spec file gets its own server and its own DuckDB file, and `--repeat-each` gets a fresh one per
repeat. So the suite means the same thing whatever order it runs in, and a single file run alone
behaves like the same file run last. Prove it whenever you have touched the harness:

```sh
cd e2e
npx playwright test --repeat-each=2
npx playwright test specs/smoke.spec.ts specs/isolation.spec.ts   # and the other way round
npx playwright test specs/isolation.spec.ts                       # each file alone
```

Playwright has no `--shuffle`; naming the files in different orders is how you vary it.

Tests **within** one file share that file's server and run in order, in one worker. That is where a
sequence like "open a round, then execute a step in it" belongs. Files run in parallel across
workers.

### Seeding

Put the system into a known state with `test.use({ seed: { steps: [...] } })` at the top of a spec
file. Each step is an argument vector for [`blctl`](../cmd/blctl), run in order against that file's
fresh database **before its server boots**:

```ts
test.use({
  seed: {
    steps: [
      ['user', 'create', '--email', 'red@example.test', '--role', 'red'],
      ['content', 'sync', '--source', 'attack'],
    ],
  },
})
```

A step that reads stdin takes the object form. `blctl user create` wants the password there and
deliberately not in a flag, because a flag ends up in shell history and in `ps`:

```ts
test.use({
  seed: {
    steps: [
      {
        args: ['user', 'create', '--email', 'lead@example.test', '--name', 'Lee', '--admin'],
        stdin: 'the password this spec signs in with',
      },
    ],
  },
})
```

The `{ steps: [...] }` wrapper is not decoration. Playwright reads any option value that is an array
whose second element is an object as a `[value, options]` fixture tuple, and a bare array of two
seed steps is exactly that — the option would silently resolve to the first step alone. `SeedPlan`
in `harness/server.ts` carries the full explanation.

Two reasons it is `blctl` and not SQL. A fixture full of `INSERT`s drifts away from what the
application actually writes, and the first thing it stops exercising is the validation that would
have caught the bug. And DuckDB admits one writer to a file at a time, so a seed step *during* a
test would fail on the running server's lock — before the server starts is not a limitation, it is
the only correct place.

A failing seed step fails the spec file loudly. It never runs the tests against a half-seeded
database.

`blctl` has the subcommand tree since `M0B-014` — `blctl --help` lists it, and
[`docs/cli.md`](cli.md) explains it. The two commands above are registered but not implemented
yet: they arrive with M1 and M2, and a seed step that uses one fails loudly today, which is the
correct outcome for a spec that depends on a feature nobody has built.

## Debugging a failure

Everything below runs from `e2e/`.

```sh
npx playwright test --headed          # watch it happen in a real window
npx playwright test --ui              # the UI mode runner: step, pick locators, re-run one test
npx playwright test --debug           # Playwright Inspector, paused at the first action
```

After a failed run, four things are waiting in `test-results/<test>/`:

| File | Answers |
|---|---|
| `trace.zip` | What the browser did, step by step, with a DOM snapshot at each one |
| `test-failed-1.png` | What the screen looked like when it gave up |
| `video.webm` | How it got there |
| `blacklight-server.log` | What the **server** thought of it — requests, status codes, errors |

The server log is the one that is usually missing from other people's suites, and the one that
usually has the answer. It is attached to the failing test automatically.

```sh
npx playwright show-trace test-results/<test>/trace.zip
make e2e-report     # the HTML report of the last run, with all of the above linked
```

When the question is what was actually *in* the database, keep it:

```sh
BLACKLIGHT_E2E_KEEP=1 npx playwright test
# → BLACKLIGHT_E2E_KEEP is set; databases and server logs kept in /tmp/blacklight-e2e-XXXX
```

Each spec file's directory under there holds its `blacklight.duckdb`, its `evidence/`, and its
`server.log`. Open the database with `duckdb`. Nothing else in the run refers to it, so delete it
when you are done.

### Writing specs

- **Query by role, label or text** — `getByRole('link', { name: 'Health' })`, not a class chain.
  The UI is Tailwind, so a class chain is a list of styling decisions: it breaks when someone
  adjusts a margin, and still passes when the value it was meant to check is wrong. Reach for
  `data-testid` only when no role or label can express the target.
- **No sleeps.** `page.waitForTimeout` asserts nothing and fails on a slow machine. Web-first
  assertions (`await expect(locator).toHaveText(...)`) already retry until they pass or time out.
  `eslint-plugin-playwright` fails the build on both this and an unconditionally skipped test.
- **`await` everything.** An un-awaited `expect` passes whatever the application did.
  `no-floating-promises` catches it.
- **No retries are configured**, deliberately. A retry turns a flaky test green and hands the cost
  to whoever debugs it in three months. Flakiness here is a bug — in the product or in the spec.

The suite grows one milestone at a time towards the single spec `PLAN.md` §9 describes: install
content, run an engagement over two rounds, and share a report. M6 owns finishing it.

## What CI runs

Every job in [`.github/workflows/ci.yml`](../.github/workflows/ci.yml), on every pull request; see
[`docs/contributing.md`](contributing.md) for the table and for branch protection. The `End-to-end`
job uploads `e2e/playwright-report` and `e2e/test-results` as the `e2e-report` artifact on every
run, passing or not.
