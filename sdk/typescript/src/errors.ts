import type { components } from './schema'

/**
 * The error half of the API contract, as types generated from
 * `api/openapi.yaml`. Every failure this API can produce is one of these
 * documents; nothing here is hand-written from a screenshot of a response.
 */
export type Problem = components['schemas']['Problem']
export type ProblemCode = components['schemas']['ProblemCode']
export type FieldError = components['schemas']['FieldError']

/** The media type every error is served as. Not `application/json`. */
export const PROBLEM_MEDIA_TYPE = 'application/problem+json'

/** Where the server echoes the request ID (`internal/httpapi.RequestIDHeader`). */
export const REQUEST_ID_HEADER = 'X-Request-Id'

/** How long to wait after a 429, in seconds. Required on every rate-limited answer. */
export const RETRY_AFTER_HEADER = 'Retry-After'

/**
 * A failure the API described.
 *
 * Thrown rather than returned, because a script that forgets to check a return
 * value carries on with data it does not have — and the alternative, making
 * every caller narrow a union before reading a field, is the cost this SDK is
 * meant to remove. `code` is the one to branch on: it is a closed set, and it
 * is stable across releases in a way that HTTP status alone is not.
 */
export class ApiError extends Error {
  /** The HTTP status. */
  readonly status: number
  /** The problem code, when the response carried a problem document. */
  readonly code?: ProblemCode
  /** Per-field failures, present on `validation_failed`. */
  readonly fieldErrors?: FieldError[]
  /** The server's request ID, which is what a support conversation starts from. */
  readonly requestId?: string
  /** Seconds to wait, parsed from `Retry-After` on a 429. */
  readonly retryAfterSeconds?: number
  /** The document itself, for anything the fields above do not cover. */
  readonly problem?: Problem

  constructor(
    message: string,
    init: {
      status: number
      code?: ProblemCode
      fieldErrors?: FieldError[]
      requestId?: string
      retryAfterSeconds?: number
      problem?: Problem
    },
  ) {
    super(message)
    this.name = 'ApiError'
    this.status = init.status
    if (init.code !== undefined) this.code = init.code
    if (init.fieldErrors !== undefined) this.fieldErrors = init.fieldErrors
    if (init.requestId !== undefined) this.requestId = init.requestId
    if (init.retryAfterSeconds !== undefined) this.retryAfterSeconds = init.retryAfterSeconds
    if (init.problem !== undefined) this.problem = init.problem
  }
}

/** A finite `Retry-After` in seconds, or undefined when the header is absent or junk. */
function retryAfterSeconds(response: Response): number | undefined {
  const raw = response.headers.get(RETRY_AFTER_HEADER)
  if (raw === null) return undefined
  const seconds = Number(raw)
  // The header may also be an HTTP date. Nothing in this API sends one, so a
  // non-numeric value is dropped rather than half-parsed into a wrong number.
  return Number.isFinite(seconds) ? seconds : undefined
}

/**
 * Build an [ApiError] from a failed response, reading the problem document when
 * there is one.
 *
 * A body that is unreadable or is not a problem document still produces an
 * error — with the status and nothing invented. A proxy that returns its own
 * HTML 502 is the ordinary case for that.
 */
export async function apiErrorFromResponse(response: Response): Promise<ApiError> {
  const contentType = response.headers.get('content-type') ?? ''
  const requestId = response.headers.get(REQUEST_ID_HEADER) ?? undefined
  const retryAfter = retryAfterSeconds(response)

  const base = {
    status: response.status,
    ...(requestId !== undefined && { requestId }),
    ...(retryAfter !== undefined && { retryAfterSeconds: retryAfter }),
  }

  if (!contentType.includes(PROBLEM_MEDIA_TYPE)) {
    return new ApiError(`HTTP ${String(response.status)}`, base)
  }

  let problem: Problem
  try {
    problem = (await response.json()) as Problem
  } catch {
    return new ApiError(`HTTP ${String(response.status)}`, base)
  }

  return new ApiError(problem.detail || problem.title || `HTTP ${String(response.status)}`, {
    ...base,
    code: problem.code,
    ...(problem.errors !== undefined && { fieldErrors: problem.errors }),
    problem,
  })
}
