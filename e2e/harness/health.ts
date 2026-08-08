/**
 * Readiness, and the error message that is the whole reason this ticket exists.
 *
 * v1's `global-setup.ts` called `process.exit(0)` when nothing answered on
 * `BASE_URL`: every run was green and none of them tested anything. Nothing
 * here ever returns "fine" for a server that never answered — it throws, and it
 * says which URL it probed, for how long, and what the last attempt reported.
 */

/** Path the server mounts its health check on (`api/openapi.yaml`). */
const healthPath = '/api/v1/healthz'

/** How long one attempt may hang before it counts as a failure and we retry. */
const attemptTimeoutMs = 2_000

/** Gap between attempts. Short: a local server is usually up within a second. */
const pollIntervalMs = 100

/** Default budget. Generous enough for a cold start under CI's disk. */
export const defaultHealthTimeoutMs = Number(process.env.BLACKLIGHT_E2E_HEALTH_TIMEOUT_MS ?? 30_000)

export interface WaitOptions {
  /** Total budget, in milliseconds. */
  timeoutMs?: number
  /**
   * Called between attempts. Returning a string means the thing we are waiting
   * for has died — a server process that exited — and waiting out the rest of
   * the budget would only delay a failure we already know about. The string is
   * quoted in the error.
   */
  gaveUp?: () => string | undefined
  /** Appended to the error: what the reader should do about it. */
  hint?: string
}

/**
 * Polls `${baseURL}/api/v1/healthz` until it answers 200, or throws.
 *
 * A 503 is a *reachable* server reporting a broken dependency, so it is
 * retried like a refused connection: during startup the database may not be
 * open yet.
 */
export async function waitForHealthy(baseURL: string, options: WaitOptions = {}): Promise<void> {
  const timeoutMs = options.timeoutMs ?? defaultHealthTimeoutMs
  const url = healthURL(baseURL)
  const started = Date.now()
  const deadline = started + timeoutMs
  const elapsed = (): number => Date.now() - started

  let lastAttempt: string
  for (;;) {
    const gaveUp = options.gaveUp?.()
    if (gaveUp !== undefined) {
      throw new Error(describeFailure(url, elapsed(), timeoutMs, gaveUp, options.hint))
    }

    try {
      const response = await fetch(url, { signal: AbortSignal.timeout(attemptTimeoutMs) })
      // Drain the body even when we are about to retry, so the connection is
      // released rather than left for the garbage collector.
      const body = (await response.text()).trim()
      if (response.ok) {
        return
      }
      lastAttempt = `HTTP ${String(response.status)}${body === '' ? '' : `: ${body}`}`
    } catch (error) {
      lastAttempt = describeError(error)
    }

    if (Date.now() >= deadline) {
      throw new Error(describeFailure(url, elapsed(), timeoutMs, lastAttempt, options.hint))
    }
    await delay(pollIntervalMs)
  }
}

/** The health URL for a base URL, so the error names exactly what was probed. */
export function healthURL(baseURL: string): string {
  try {
    return new URL(healthPath, baseURL).toString()
  } catch (error) {
    throw new Error(
      `BASE_URL is not a URL: ${JSON.stringify(baseURL)}. ` +
        `It must be absolute, for example http://127.0.0.1:8080.`,
      { cause: error },
    )
  }
}

function describeFailure(
  url: string,
  elapsedMs: number,
  timeoutMs: number,
  lastAttempt: string,
  hint: string | undefined,
): string {
  const lines = [
    `No healthy Blacklight server answered at ${url}`,
    // Both numbers, because they differ for a reason worth seeing: a run that
    // used its whole budget was waiting, and one that stopped early knows the
    // thing it was waiting for is already dead.
    `  waited:       ${String(elapsedMs)} ms of a ${String(timeoutMs)} ms budget`,
    `  last attempt: ${lastAttempt}`,
  ]
  if (hint !== undefined) {
    lines.push('', hint)
  }
  return lines.join('\n')
}

/**
 * Node's fetch wraps the interesting part — ECONNREFUSED, ENOTFOUND — in a
 * generic "fetch failed", so the cause is unwrapped here. Without this every
 * dead-port failure reads the same as every DNS failure.
 */
function describeError(error: unknown): string {
  if (!(error instanceof Error)) {
    return String(error)
  }
  const cause = error.cause
  if (cause instanceof Error && cause.message !== '') {
    return `${error.message}: ${cause.message}`
  }
  if (error.name === 'TimeoutError') {
    return `no response within ${String(attemptTimeoutMs)} ms`
  }
  return error.message
}

/**
 * A poll interval, not a guess about how long something takes. The ban on
 * arbitrary sleeps is about specs: `page.waitForTimeout` in a test asserts
 * nothing and fails on a slow machine. This loop has a condition and a deadline.
 */
function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}
