import fs from 'node:fs/promises'
import path from 'node:path'

import { runDirIfSet, runDirPrefix } from './paths'

/**
 * Removes the run's databases, evidence directories and server logs.
 *
 * Anything worth keeping from a failed run has already been attached to the
 * test that failed (`harness/test.ts`), so the report survives this. What is
 * left here is a few megabytes of DuckDB files per run, which add up fast on a
 * machine where the suite is run all day.
 *
 * `PURPLEOPS_E2E_KEEP=1` leaves it all in place and prints where, for the
 * failures the attached log does not explain — when the question is what is
 * actually *in* the database, you need the file.
 *
 * The prefix is checked again before deleting. This is an `rm -rf` driven by an
 * environment variable, and a typo somewhere upstream should not be able to aim
 * it at a home directory.
 */
export default async function globalTeardown(): Promise<void> {
  const dir = runDirIfSet()
  if (dir === undefined || !path.basename(dir).startsWith(runDirPrefix)) {
    return
  }
  if (process.env.PURPLEOPS_E2E_KEEP !== undefined && process.env.PURPLEOPS_E2E_KEEP !== '') {
    console.log(`PURPLEOPS_E2E_KEEP is set; databases and server logs kept in ${dir}`)
    return
  }
  await fs.rm(dir, { recursive: true, force: true })
}
