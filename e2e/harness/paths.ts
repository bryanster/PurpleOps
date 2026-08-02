import path from 'node:path'

/**
 * Where the harness finds the things it drives.
 *
 * The suite tests a *binary*, not a package: `bin/blacklight` serving the SPA it
 * embedded, over HTTP, against a DuckDB file. Everything below points at build
 * output, so a stale `bin/` is a stale test run — `make e2e` builds first for
 * exactly that reason.
 */

/** Repository root. This file is `<root>/e2e/harness/paths.ts`. */
export const repoRoot = path.resolve(__dirname, '..', '..')

/** The server under test. Override to run the suite against another build. */
export const blacklightBinary =
  process.env.BLACKLIGHT_E2E_BINARY ?? path.join(repoRoot, 'bin', 'blacklight')

/** The admin CLI, which is how specs seed — see `harness/test.ts`. */
export const blctlBinary = process.env.BLACKLIGHT_E2E_BLCTL ?? path.join(repoRoot, 'bin', 'blctl')

/**
 * An already-running server to test against instead of starting our own, or
 * undefined for the normal case where the harness owns the lifecycle.
 *
 * The variable is `BASE_URL` because that is what v1's harness used and what
 * anyone's muscle memory reaches for. Unlike v1's, an unreachable value here is
 * a failed run rather than a skipped one — see `global-setup.ts`.
 */
export function externalBaseURL(): string | undefined {
  const value = process.env.BASE_URL?.trim()
  return value === undefined || value === '' ? undefined : value
}

/**
 * The scratch directory for one `playwright test` invocation: every database,
 * evidence directory and server log the run creates lives under it, and
 * `global-teardown.ts` removes the whole thing.
 *
 * It is passed from global setup to the worker processes through the
 * environment, which is the only channel they share — workers are separate
 * processes, forked after global setup has run.
 */
const runDirVariable = 'BLACKLIGHT_E2E_RUN_DIR'

/** Prefix of the temp directory name, checked again before teardown deletes it. */
export const runDirPrefix = 'blacklight-e2e-'

export function setRunDir(dir: string): void {
  process.env[runDirVariable] = dir
}

export function runDirIfSet(): string | undefined {
  return process.env[runDirVariable]
}

/**
 * The run directory, or an error explaining that global setup did not run.
 * Reaching this with nothing set means the suite was started in a way that
 * bypassed `playwright.config.ts`.
 */
export function runDir(): string {
  const dir = runDirIfSet()
  if (dir === undefined) {
    throw new Error(
      `${runDirVariable} is not set: global setup did not run, so there is nowhere to put ` +
        `this run's databases. Start the suite through playwright.config.ts.`,
    )
  }
  return dir
}
