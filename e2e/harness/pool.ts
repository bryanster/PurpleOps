import path from 'node:path'

import { externalBaseURL, runDir } from './paths'
import { startServer, externalServer, type SeedCommand, type Server } from './server'

/**
 * One worker's server, replaced whenever it starts on a new spec file.
 *
 * This is what makes "every spec file starts from a clean database" true rather
 * than aspirational. A single server for the whole run — which is what
 * Playwright's `webServer` option gives you — means spec two inherits whatever
 * spec one wrote, and the first order-dependent test to appear is found months
 * later by someone running a single file.
 *
 * Restarting costs about a second: opening a DuckDB file and applying
 * migrations. That is the right trade against a suite whose result depends on
 * the order it happened to run in.
 *
 * The unit is the spec file, not the test. Tests within a file share a server
 * on purpose — a file is where a sequence like "create an engagement, then run
 * it" belongs, and `fullyParallel` is off so they stay in order in one worker.
 */
export class ServerPool {
  readonly #workerIndex: number
  #key: string | undefined
  #server: Server | undefined
  #started = 0

  constructor(workerIndex: number) {
    this.#workerIndex = workerIndex
  }

  /**
   * The server for `key`, starting a fresh one — fresh database, seed steps
   * replayed — if this worker was last serving something else.
   */
  async forKey(key: string, seed: readonly SeedCommand[]): Promise<Server> {
    const external = externalBaseURL()
    if (external !== undefined) {
      // Not ours: no database to reset, and resetting someone's running
      // deployment is not a thing a test suite should do quietly.
      this.#server ??= externalServer(external)
      return this.#server
    }

    if (this.#key === key && this.#server !== undefined) {
      return this.#server
    }

    await this.dispose()
    this.#started += 1
    const server = await startServer({
      dir: path.join(runDir(), `w${String(this.#workerIndex)}-${String(this.#started)}-${key}`),
      seed,
      label: key,
    })
    this.#key = key
    this.#server = server
    return server
  }

  async dispose(): Promise<void> {
    const server = this.#server
    this.#server = undefined
    this.#key = undefined
    await server?.stop()
  }
}

/**
 * A directory-safe name for the spec file a test came from, including the
 * repeat index so `--repeat-each` really does start over rather than running
 * the second copy against the first copy's leftovers.
 */
export function keyFor(file: string, repeatEachIndex: number): string {
  const name = path.basename(file).replace(/\.spec\.ts$/, '')
  const safe = name.replace(/[^a-zA-Z0-9._-]/g, '-')
  return repeatEachIndex === 0 ? safe : `${safe}-repeat${String(repeatEachIndex)}`
}
