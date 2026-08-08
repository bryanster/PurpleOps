import { execFile } from 'node:child_process'
import { promisify } from 'node:util'

import { blctlBinary } from './paths'

const run = promisify(execFile)

/** A seed step gets its own budget: a hung CLI should not eat the test timeout. */
const seedTimeoutMs = 60_000

/**
 * Runs `blctl` against the database described by `env`, and returns its
 * stdout.
 *
 * This is how a spec puts the system into a known state. Seeding through the
 * admin CLI rather than through SQL is deliberate: `INSERT`s in a test fixture
 * drift away from what the application actually writes, and the first thing
 * they stop exercising is the validation that would have caught the bug. A seed
 * that goes through a supported command is also a command that stays supported.
 *
 * Failure throws with both streams attached. A seed step that quietly did
 * nothing is how a suite ends up asserting against an empty database and
 * passing.
 */
export async function runBlctl(
  args: readonly string[],
  env: NodeJS.ProcessEnv,
  stdin?: string,
): Promise<string> {
  try {
    const pending = run(blctlBinary, [...args], { env, timeout: seedTimeoutMs })
    // `blctl user create` reads the password from stdin when stdin is not a
    // terminal, deliberately: a password in a flag ends up in shell history and
    // in `ps`. That makes writing to the child the only way to seed an account
    // whose password a spec knows. Closed either way, so a command that reads
    // stdin and was given nothing sees EOF rather than hanging until the
    // timeout.
    pending.child.stdin?.end(stdin ?? '')
    const { stdout } = await pending
    return stdout
  } catch (error) {
    throw new Error(`blctl ${args.join(' ')}\n${describe(error)}`, { cause: error })
  }
}

function describe(error: unknown): string {
  if (!(error instanceof Error)) {
    return String(error)
  }
  // execFile rejects with an Error carrying the child's output on the side.
  const output = error as Error & { stdout?: string; stderr?: string }
  const parts = [error.message.trim()]
  for (const [stream, text] of [
    ['stdout', output.stdout],
    ['stderr', output.stderr],
  ] as const) {
    if (text !== undefined && text.trim() !== '') {
      parts.push(`  ${stream}: ${text.trim()}`)
    }
  }
  return parts.join('\n')
}
