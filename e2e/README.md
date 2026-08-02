# e2e/

The end-to-end suite: Playwright driving a browser against a real `bin/blacklight`, serving the SPA
it embedded, over HTTP, against a DuckDB file created for the run. Nothing here is mocked.

**[`../docs/testing.md`](../docs/testing.md) is the guide** — how to run it, how to seed a spec, and
how to debug a failure. This file is the map of the directory.

```sh
make e2e-browsers   # once: download Chromium (~150 MB)
make e2e            # from the repository root: builds, then runs
```

| Path                         | What it is                                                                                                   |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------ |
| `specs/`                     | The tests. `smoke.spec.ts` is the product; `isolation.spec.ts` is the harness checking itself                |
| `harness/global-setup.ts`    | Picks the mode — own the servers, or test the one on `BASE_URL` — and **throws** if there is nothing to test |
| `harness/global-teardown.ts` | Deletes the run's databases, evidence and logs (`BLACKLIGHT_E2E_KEEP=1` to keep them)                         |
| `harness/pool.ts`            | One server per spec file, on its own fresh database                                                          |
| `harness/server.ts`          | Starting, seeding and stopping one `blacklight` process                                                       |
| `harness/test.ts`            | The extended `test`: the `seed` option, the `server` fixture, `baseURL`                                      |
| `harness/health.ts`          | Readiness, and the error message this whole ticket exists for                                                |
| `harness/blctl.ts`         | Seeding, through the admin CLI rather than through SQL                                                       |

## Why it looks like this

v1's `global-setup.ts` called `process.exit(0)` when nothing answered on `BASE_URL`. Every run was
green and none of them ran a test — false confidence, which is worse than no coverage
([`PLAN.md`](../PLAN.md) §9). Two things follow from that, and they are the constraints to preserve
when extending this:

1. **A run that could not test anything fails.** No skip, no early exit, no zero status. Whatever
   goes wrong, the message names the URL it probed, how long it waited, and what the server said.
2. **Every spec file starts from a clean database.** A suite whose result depends on the order it
   happened to run in is a suite nobody trusts, and the first order-dependent test is always found
   months later by someone running a single file.

There is deliberately no `webServer` block in `playwright.config.ts`: it would start one server for
the whole run, and one server means one database shared by every spec file. `harness/pool.ts`
carries the reasoning.

## Environment

| Variable                          | Effect                                                                                     |
| --------------------------------- | ------------------------------------------------------------------------------------------ |
| `BASE_URL`                        | Test a server that is already running instead of starting any. Unreachable ⇒ the run fails |
| `BLACKLIGHT_E2E_KEEP`              | Keep the run's databases and server logs, and print where                                  |
| `BLACKLIGHT_E2E_BINARY`            | The server binary to drive (default `bin/blacklight`)                                       |
| `BLACKLIGHT_E2E_BLCTL`           | The CLI seeding uses (default `bin/blctl`)                                               |
| `BLACKLIGHT_E2E_HEALTH_TIMEOUT_MS` | Readiness budget, in milliseconds (default 30000)                                          |

Every other `BLACKLIGHT_*` variable in your shell is **stripped** before the server is started: a
`.env` sourced into a terminal must not be able to decide which database the suite writes to.
