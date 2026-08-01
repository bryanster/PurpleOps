/**
 * Hand-written calls to the two public system endpoints.
 *
 * TEMPORARY, and deliberately confined to this directory: M0B-009 generates a
 * typed client from api/openapi.yaml and wires it to TanStack Query, at which
 * point every type and every fetch below is deleted. Nothing outside
 * src/features/system imports from here, so that deletion stays a local one.
 *
 * The shapes mirror the `Version`, `Health` and `Problem` schemas in
 * api/openapi.yaml. They are not the source of truth — that file is.
 */

/** Matches internal/httpapi.BasePath. Same-origin in production; proxied by Vite in dev. */
const API_BASE = '/api/v1'

export interface Version {
  version: string
  commit: string
  buildDate: string
}

export type HealthState = 'ok' | 'error'

export interface Health {
  status: HealthState
  checks: { db: HealthState }
}

/** An API error carrying the `code` from the RFC 9457 problem document (M0B-007). */
export class ApiError extends Error {
  readonly status: number
  readonly code: string | undefined
  /** The request ID the server echoed. Worth quoting in a bug report. */
  readonly instance: string | undefined

  constructor(message: string, init: { status: number; code?: string; instance?: string }) {
    super(message)
    this.name = 'ApiError'
    this.status = init.status
    this.code = init.code
    this.instance = init.instance
  }
}

export async function fetchVersion(signal?: AbortSignal): Promise<Version> {
  return await getJSON<Version>('/version', signal)
}

/**
 * A 503 from /healthz is a health *report*, not a failure: the body is the same
 * `Health` shape either way and says which dependency is down. Treating it as
 * an error would throw away the only useful part of the response.
 */
export async function fetchHealth(signal?: AbortSignal): Promise<Health> {
  return await getJSON<Health>('/healthz', signal, [200, 503])
}

async function getJSON<T>(
  path: string,
  signal?: AbortSignal,
  okStatuses: readonly number[] = [200],
): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: { Accept: 'application/json' },
    signal,
  })

  if (!okStatuses.includes(response.status)) {
    throw await problemFrom(response)
  }
  return (await response.json()) as T
}

/**
 * Turn a failed response into an ApiError, using the problem document when the
 * server sent one. Anything else — an HTML error page from a proxy in front of
 * us, a truncated body — still has to produce a sensible message rather than a
 * JSON parse error the user cannot act on.
 */
async function problemFrom(response: Response): Promise<ApiError> {
  const fallback = `${String(response.status)} ${response.statusText}`.trim()

  if (!response.headers.get('content-type')?.includes('json')) {
    return new ApiError(fallback, { status: response.status })
  }

  try {
    const body: unknown = await response.json()
    if (typeof body !== 'object' || body === null) {
      return new ApiError(fallback, { status: response.status })
    }
    const problem = body as {
      title?: unknown
      detail?: unknown
      code?: unknown
      instance?: unknown
    }
    const message =
      stringOrUndefined(problem.detail) ?? stringOrUndefined(problem.title) ?? fallback
    return new ApiError(message, {
      status: response.status,
      code: stringOrUndefined(problem.code),
      instance: stringOrUndefined(problem.instance),
    })
  } catch {
    return new ApiError(fallback, { status: response.status })
  }
}

function stringOrUndefined(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined
}
