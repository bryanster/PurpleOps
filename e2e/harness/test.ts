import fs from 'node:fs/promises'

import { test as base, type TestInfo } from '@playwright/test'

import { keyFor, ServerPool } from './pool'
import type { Server } from './server'

export { expect } from '@playwright/test'

/**
 * A `popsctl` argument vector, e.g. `['users', 'create', '--email', 'red@x']`.
 */
export type SeedCommand = readonly string[]

export interface HarnessOptions {
  /**
   * Commands run against this spec file's fresh database, in order, before its
   * server boots. Declare them at the top of a spec file:
   *
   * ```ts
   * test.use({ seed: [['users', 'create', '--email', 'red@example.test']] })
   * ```
   *
   * Before boot, not inside a test: DuckDB admits one writer to a file at a
   * time, so `popsctl` cannot open a database a running server is holding.
   * That constraint is also a feature — seeding is a fact about the spec file,
   * visible at the top of it, rather than something buried in a `beforeEach`.
   */
  seed: readonly SeedCommand[]
}

export interface HarnessFixtures {
  /**
   * The server this spec file is running against, and where its state lives.
   * Requesting it is rarely necessary — `baseURL` already points at it, so
   * `page.goto('/')` lands in the right place.
   */
  server: Server
}

export interface HarnessWorkerFixtures {
  serverPool: ServerPool
}

export const test = base.extend<HarnessOptions & HarnessFixtures, HarnessWorkerFixtures>({
  seed: [[], { option: true }],

  serverPool: [
    async ({}, use, workerInfo) => {
      const pool = new ServerPool(workerInfo.workerIndex)
      await use(pool)
      await pool.dispose()
    },
    { scope: 'worker' },
  ],

  server: async ({ serverPool, seed }, use, testInfo) => {
    const server = await serverPool.forKey(keyFor(testInfo.file, testInfo.repeatEachIndex), seed)
    await use(server)

    // A failed end-to-end test without the server's side of the story is an
    // hour of guessing. The trace says what the browser did; this says what the
    // server thought of it.
    if (testInfo.status !== testInfo.expectedStatus) {
      await attachServerLog(testInfo, server)
    }
  },

  // Overriding Playwright's own `baseURL` option, rather than exposing a
  // separate one, so every built-in that respects it — page.goto, request,
  // expect(page).toHaveURL — points at this spec file's server with no
  // per-test wiring.
  baseURL: async ({ server }, use) => {
    await use(server.baseURL)
  },
})

async function attachServerLog(testInfo: TestInfo, server: Server): Promise<void> {
  const paths = server.paths
  if (paths === undefined) {
    return
  }
  const body = await fs.readFile(paths.logPath).catch(() => undefined)
  if (body === undefined || body.byteLength === 0) {
    return
  }

  // Copied into the test's own output directory rather than attached from
  // where it lies. Global teardown deletes the run directory, and an
  // attachment pointing into it would be a dead link by the time anyone
  // clicked it. Copying also puts the log in `test-results/` beside the trace
  // and the screenshot, so the CI artifact contains it without extra wiring.
  const copy = testInfo.outputPath('purpleops-server.log')
  await fs.writeFile(copy, body)
  await testInfo.attach('purpleops-server.log', { path: copy, contentType: 'text/plain' })
}
