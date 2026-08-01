import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'

import { waitForHealthy } from './health'
import { externalBaseURL, runDirPrefix, setRunDir } from './paths'

/**
 * Decides, once, which of the two modes this run is in — and refuses to start
 * if the answer is "neither".
 *
 * `BASE_URL` set: an operator is pointing the suite at a deployment they
 * already have. We probe it and **throw** if it never answers. v1's setup
 * called `process.exit(0)` here, which reported success for a run in which no
 * test executed; PLAN.md §9 names that as the reason this harness was rewritten.
 *
 * `BASE_URL` unset: the harness owns the servers. It makes a scratch directory
 * for the run — every database, evidence directory and server log goes under
 * it, and `global-teardown.ts` removes the lot — and each spec file gets its
 * own server inside it (see `harness/pool.ts`).
 */
export default async function globalSetup(): Promise<void> {
  const external = externalBaseURL()
  if (external !== undefined) {
    await waitForHealthy(external, {
      hint:
        `BASE_URL is set, so the harness did not start a server: it expects one to be ` +
        `running there already.\n` +
        `Start one with \`make run\`, point BASE_URL somewhere else, or unset BASE_URL and ` +
        `let the harness manage its own.`,
    })
    return
  }

  setRunDir(await fs.mkdtemp(path.join(os.tmpdir(), runDirPrefix)))
}
