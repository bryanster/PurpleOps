import { spawn, type ChildProcess } from 'node:child_process'
import { randomBytes } from 'node:crypto'
import { constants, createWriteStream } from 'node:fs'
import fs from 'node:fs/promises'
import net from 'node:net'
import path from 'node:path'

import { waitForHealthy } from './health'
import { purpleopsBinary } from './paths'
import { runPopsctl } from './popsctl'

/** How long a server gets to exit on SIGTERM before it is killed outright. */
const stopTimeoutMs = 10_000

/** Lines of the server log quoted inline when a start fails. */
const logTailLines = 40

/** The state one server owns on disk. Absent for a server we did not start. */
export interface ServerPaths {
  /** The DuckDB file this server opened. Fresh, and this spec file's alone. */
  readonly dbPath: string
  /** Where uploaded evidence lands. Removed with everything else at teardown. */
  readonly evidenceDir: string
  /** Combined stdout and stderr, attached to any test in this file that fails. */
  readonly logPath: string
  /** stdout of each seed command, in the order they ran. */
  readonly seedOutput: readonly string[]
}

export interface Server {
  /** Where the browser points. */
  readonly baseURL: string
  /**
   * Undefined when `BASE_URL` sent the suite at a server it does not own: there
   * is then no database to seed, no log to attach, and nothing to stop.
   */
  readonly paths: ServerPaths | undefined
  stop(): Promise<void>
}

export interface StartOptions {
  /** A directory of this server's own. Created; never shared. */
  dir: string
  /** `popsctl` argument vectors run against the fresh database before boot. */
  seed: readonly (readonly string[])[]
  /** Named in errors so a failure says which spec file was being set up. */
  label: string
}

/**
 * The on-disk state of a server the harness started, for the specs that assert
 * something about it.
 *
 * Throws rather than returning undefined so a spec can use the result directly.
 * A spec that reaches this against an external server has forgotten to skip
 * itself when `BASE_URL` is set — see `specs/isolation.spec.ts`.
 */
export function managedPaths(server: Server): ServerPaths {
  if (server.paths === undefined) {
    throw new Error(
      'this spec inspects the database the harness created, but BASE_URL sent the run at a ' +
        'server it does not own. Skip the spec when BASE_URL is set.',
    )
  }
  return server.paths
}

/** A server the harness did not start, and therefore must not touch. */
export function externalServer(baseURL: string): Server {
  return {
    baseURL,
    paths: undefined,
    stop: () => Promise.resolve(),
  }
}

/**
 * Starts one `purpleops` process on a fresh database and waits for it to be
 * healthy.
 *
 * Seeding happens here, before the process starts, and not from inside a test.
 * DuckDB gives a database file to one writer at a time, so `popsctl` cannot
 * open a database a running server is holding — a seed step that ran later
 * would fail with a lock error rather than doing anything useful.
 */
export async function startServer(options: StartOptions): Promise<Server> {
  await assertExecutable(purpleopsBinary)

  const dir = options.dir
  const paths = {
    dbPath: path.join(dir, 'purpleops.duckdb'),
    evidenceDir: path.join(dir, 'evidence'),
    logPath: path.join(dir, 'server.log'),
  }
  await fs.mkdir(paths.evidenceDir, { recursive: true })

  const port = await reservePort()
  const baseURL = `http://127.0.0.1:${String(port)}`
  const env = serverEnvironment(paths, baseURL, port)

  const seedOutput: string[] = []
  for (const [index, command] of options.seed.entries()) {
    seedOutput.push(await seedStep(options.label, index, command, env))
  }

  const log = createWriteStream(paths.logPath)
  const child = spawn(purpleopsBinary, [], { cwd: dir, env, stdio: ['ignore', 'pipe', 'pipe'] })
  child.stdout.pipe(log)
  child.stderr.pipe(log)

  // Recorded rather than thrown: the process may die while `waitForHealthy` is
  // between attempts, and an unhandled rejection there would replace a precise
  // message with a stack trace from the event loop.
  let died: string | undefined
  child.once('error', (error) => {
    died = `the process could not be started: ${error.message}`
  })
  child.once('exit', (code, signal) => {
    died =
      signal === null
        ? `the process exited with status ${String(code)} before it became healthy`
        : `the process was killed by ${signal} before it became healthy`
  })

  const server: Server = {
    baseURL,
    paths: { ...paths, seedOutput },
    stop: () => stop(child, log),
  }

  try {
    await waitForHealthy(baseURL, { gaveUp: () => died })
  } catch (error) {
    // Stop first: the log is only complete once the process is gone and the
    // stream is closed, and the log is the half of the story that says why.
    await server.stop()
    throw new Error(
      `${error instanceof Error ? error.message : String(error)}\n\n` +
        (await startupHint(options.label, paths.logPath)),
      { cause: error },
    )
  }
  return server
}

/**
 * The environment the server and every seed command see.
 *
 * It starts from the developer's environment with every `PURPLEOPS_` variable
 * stripped. A `.env` exported into the shell — which is the normal way to run
 * the server by hand — would otherwise decide which database the suite writes
 * to, and the first thing anyone would notice is their own data changing.
 */
function serverEnvironment(
  paths: { dbPath: string; evidenceDir: string },
  baseURL: string,
  port: number,
): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = {}
  for (const [key, value] of Object.entries(process.env)) {
    if (!key.startsWith('PURPLEOPS_')) {
      env[key] = value
    }
  }

  // `development`, because the suite drives a browser over plain http on
  // loopback and production posture makes session cookies Secure (M1). The
  // production path over TLS is the container smoke test's job, not this one's.
  env.PURPLEOPS_ENV = 'development'
  env.PURPLEOPS_ADDR = `127.0.0.1:${String(port)}`
  env.PURPLEOPS_BASE_URL = baseURL
  env.PURPLEOPS_DB_PATH = paths.dbPath
  env.PURPLEOPS_EVIDENCE_DIR = paths.evidenceDir
  // Real entropy per server: config rejects placeholders and low-variety values.
  env.PURPLEOPS_SESSION_SECRET = randomBytes(32).toString('base64')
  // Debug and text: this log is read by a human staring at a failed run.
  env.PURPLEOPS_LOG_LEVEL = 'debug'
  env.PURPLEOPS_LOG_FORMAT = 'text'
  return env
}

async function seedStep(
  label: string,
  index: number,
  command: readonly string[],
  env: NodeJS.ProcessEnv,
): Promise<string> {
  try {
    return await runPopsctl(command, env)
  } catch (error) {
    throw new Error(
      `${label}: seed step ${String(index + 1)} failed — the spec never ran.\n` +
        (error instanceof Error ? error.message : String(error)),
      { cause: error },
    )
  }
}

/**
 * Asks the kernel for a free port and immediately gives it back, so the server
 * can be told which port to bind *and* the matching PURPLEOPS_BASE_URL, which
 * it needs at startup and cannot be told after the fact.
 *
 * Binding port 0 in the server instead would avoid the gap between releasing
 * this socket and the server claiming it, but then nothing could compute the
 * base URL. The gap is small and loopback-only; if something does steal the
 * port, the server fails to bind and says so in its log.
 */
function reservePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const socket = net.createServer()
    socket.once('error', reject)
    socket.listen(0, '127.0.0.1', () => {
      const address = socket.address()
      if (address === null || typeof address === 'string') {
        socket.close(() => {
          reject(new Error(`could not read the reserved port (got ${String(address)})`))
        })
        return
      }
      const { port } = address
      socket.close(() => {
        resolve(port)
      })
    })
  })
}

/** Terminates the process and closes the log, without leaving either behind. */
async function stop(child: ChildProcess, log: NodeJS.WritableStream): Promise<void> {
  if (child.exitCode === null && child.signalCode === null) {
    const exited = new Promise<void>((resolve) => {
      child.once('exit', () => {
        resolve()
      })
    })
    child.kill('SIGTERM')

    let timer: NodeJS.Timeout | undefined
    const gaveUp = new Promise<'timeout'>((resolve) => {
      timer = setTimeout(() => {
        resolve('timeout')
      }, stopTimeoutMs)
    })
    if ((await Promise.race([exited.then(() => 'exited' as const), gaveUp])) === 'timeout') {
      child.kill('SIGKILL')
      await exited
    }
    clearTimeout(timer)
  }
  await new Promise<void>((resolve) => {
    log.end(resolve)
  })
}

async function assertExecutable(binary: string): Promise<void> {
  try {
    await fs.access(binary, constants.X_OK)
  } catch {
    throw new Error(
      `${binary} is missing or not executable.\n` +
        `The suite drives a real build. Run \`make build\` (or \`make e2e\`, which does it for you).`,
    )
  }
}

/** The tail of the server's own log, which is where the answer usually is. */
async function startupHint(label: string, logPath: string): Promise<string> {
  const tail = await readTail(logPath)
  const lines = [`${label}: the server the harness started never became healthy.`]
  if (tail !== '') {
    lines.push('', `Last ${String(logTailLines)} lines of ${logPath}:`, tail)
  }
  return lines.join('\n')
}

async function readTail(logPath: string): Promise<string> {
  const contents = await fs.readFile(logPath, 'utf8').catch(() => '')
  return contents.split('\n').slice(-logTailLines).join('\n').trim()
}
