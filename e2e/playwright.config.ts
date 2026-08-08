import { defineConfig, devices } from '@playwright/test'

/**
 * Blacklight end-to-end configuration.
 *
 * Read `harness/global-setup.ts` first: it decides whether this run starts its
 * own servers or tests one that is already up, and it is where a run that has
 * nothing to test dies rather than reporting success.
 *
 * There is deliberately no `webServer` block. It would start one server for the
 * whole run, and one server means one database shared by every spec file —
 * which is the order dependence this suite is supposed to be free of.
 * `harness/pool.ts` starts a server per spec file instead, and explains the
 * trade.
 */
export default defineConfig({
  testDir: './specs',
  outputDir: './test-results',

  globalSetup: './harness/global-setup.ts',
  globalTeardown: './harness/global-teardown.ts',

  // Off, so the tests inside one spec file stay in one worker and run in the
  // order they are written. That is what lets a file share a server — and a
  // file is the right home for a sequence like "open a round, then execute a
  // step in it". Files still run in parallel across workers.
  fullyParallel: false,

  // `test.only` left in a spec passes locally and silently drops everything
  // else in the file from CI.
  forbidOnly: Boolean(process.env.CI),

  // No retries, on purpose. A retry turns a flaky test green and moves the cost
  // to whoever debugs it in three months. If something here is flaky, that is
  // the bug — either in the product or in the spec.
  retries: 0,

  // A cold start opens DuckDB and applies migrations; CI's disk is not fast.
  timeout: 60_000,
  expect: { timeout: 10_000 },

  reporter: process.env.CI
    ? [['github'], ['list'], ['html', { open: 'never' }]]
    : [['list'], ['html', { open: 'never' }]],

  use: {
    // Everything needed to answer "what happened" without re-running: what the
    // browser did, what it looked like, and — attached by the `server` fixture
    // — what the server made of it.
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',

    actionTimeout: 15_000,
    navigationTimeout: 30_000,
  },

  // One browser. The product is an internal tool used beside a terminal, and a
  // second engine here would double the runtime to re-check the same server.
  // Chromium is also what M6 renders PDFs with, so it is the one that matters.
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
