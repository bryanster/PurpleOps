import type { components } from './schema'

/**
 * The error half of the API contract, as types generated from
 * `api/openapi.yaml`. Every failure this API can produce is one of these
 * documents (M0B-007); nothing here is hand-written from a screenshot of a
 * response.
 */
export type Problem = components['schemas']['Problem']
export type ProblemCode = components['schemas']['ProblemCode']
export type FieldError = components['schemas']['FieldError']

/** The media type every error is served as. Not `application/json`. */
export const PROBLEM_MEDIA_TYPE = 'application/problem+json'

/** Where the server echoes the request ID (`internal/httpapi.RequestIDHeader`). */
export const REQUEST_ID_HEADER = 'X-Request-Id'

/**
 * How long to wait after a 429, in seconds. The spec makes it required on every
 * rate-limited answer, and it is a *header* rather than a body field — so it is
 * read here, once, rather than by whichever screen happens to care.
 *
 * The screen that cares most is the login form (M1-004): "too many attempts" on
 * its own is a dead end, and "try again in 4 minutes" is something a person can
 * act on.
 */
export const RETRY_AFTER_HEADER = 'Retry-After'

/**
 * Every code in the spec, so an unrecognised one can be rejected at runtime
 * rather than typed optimistically.
 *
 * `Record<ProblemCode, true>` is exhaustive in both directions: adding a code to
 * `api/openapi.yaml` fails to compile here until it is listed, and a code listed
 * here that the spec does not have fails too.
 */
const PROBLEM_CODES: Record<ProblemCode, true> = {
  validation_failed: true,
  unauthenticated: true,
  forbidden: true,
  mfa_enrolment_required: true,
  not_found: true,
  method_not_allowed: true,
  conflict: true,
  rate_limited: true,
  internal: true,
}

/**
 * Codes that refine another, and the code each falls back to.
 *
 * The server allows a status to carry more than one code only when the extra
 * one is strictly more specific (`internal/httpapi/apierr`, `refinements`), so
 * a client that has not been taught a refinement can always treat it as what it
 * refines. This is that fallback, in the one place it belongs: a screen that
 * handles `forbidden` handles `mfa_enrolment_required` too unless it says
 * otherwise, and M1-017's enrolment screen is what will say otherwise.
 */
const PROBLEM_CODE_REFINES: Partial<Record<ProblemCode, ProblemCode>> = {
  mfa_enrolment_required: 'forbidden',
}

export function isProblemCode(value: unknown): value is ProblemCode {
  return typeof value === 'string' && Object.hasOwn(PROBLEM_CODES, value)
}

/**
 * A failed API call, carrying the machine-readable half of the problem document
 * that caused it.
 *
 * Callers branch on `code`, never on `message` — that is prose for a human and
 * the server is free to reword it. `requestId` is what a user quotes in a bug
 * report and an operator greps the log for.
 */
export class ApiError extends Error {
  readonly status: number
  readonly code: ProblemCode | undefined
  readonly detail: string | undefined
  readonly errors: readonly FieldError[]
  readonly requestId: string | undefined

  /**
   * Seconds to wait before trying again, from `Retry-After`. Defined only on a
   * rate-limited answer, and `undefined` rather than `0` everywhere else — a
   * screen must be able to tell "wait four minutes" from "no waiting involved".
   */
  readonly retryAfterSeconds: number | undefined

  constructor(
    message: string,
    init: {
      status: number
      code?: ProblemCode
      detail?: string
      errors?: readonly FieldError[]
      requestId?: string
      retryAfterSeconds?: number
    },
  ) {
    super(message)
    this.name = 'ApiError'
    this.status = init.status
    this.code = init.code
    this.detail = init.detail
    this.errors = init.errors ?? []
    this.requestId = init.requestId
    this.retryAfterSeconds = init.retryAfterSeconds
  }

  /** The message for `field`, when the server named one. */
  fieldError(field: string): string | undefined {
    return this.errors.find((entry) => entry.field === field)?.message
  }
}

/**
 * True when `error` is an [ApiError], optionally one carrying a specific code.
 *
 * A refinement satisfies the code it refines: asking for `forbidden` is also
 * true of `mfa_enrolment_required`, because that is what "strictly more
 * specific" has to mean for a caller who has not been taught the narrower one.
 * Asking for the narrower one is never true of the wider one.
 */
export function isApiError(error: unknown, code?: ProblemCode): error is ApiError {
  if (!(error instanceof ApiError)) {
    return false
  }
  if (code === undefined || error.code === code) {
    return true
  }
  return error.code !== undefined && PROBLEM_CODE_REFINES[error.code] === code
}

/**
 * Build an [ApiError] from a failed response.
 *
 * Anything that is not a well-formed problem document still has to produce a
 * usable error: a gateway in front of the server returning an HTML page, a
 * truncated body, a 502 with nothing in it. Those become an ApiError with a
 * status and no code — never a `SyntaxError` from the JSON parser, which tells
 * the user nothing and hides the status that would have.
 *
 * The response body is read from a clone, so the caller still holds an unread
 * one if it decides not to throw this.
 */
export async function apiErrorFromResponse(response: Response): Promise<ApiError> {
  const requestId = response.headers.get(REQUEST_ID_HEADER) ?? undefined
  const retryAfterSeconds = readRetryAfter(response)
  const fallback = `${String(response.status)} ${response.statusText}`.trim()

  const problem = await readProblem(response)
  if (!problem) {
    return new ApiError(fallback, { status: response.status, requestId, retryAfterSeconds })
  }

  const detail = nonEmptyString(problem.detail)
  return new ApiError(detail ?? nonEmptyString(problem.title) ?? fallback, {
    status: response.status,
    code: isProblemCode(problem.code) ? problem.code : undefined,
    detail,
    errors: readFieldErrors(problem.errors),
    retryAfterSeconds,
    // The header is authoritative — it is set by the middleware that owns the
    // ID — but a problem document forwarded without its headers still has it.
    requestId: requestId ?? nonEmptyString(problem.instance),
  })
}

/**
 * `Retry-After` as a number of seconds, or undefined.
 *
 * Only the delta-seconds form is read. RFC 9110 also allows an HTTP date, and
 * this server never sends one — a client that guessed at the date form would be
 * parsing something it has never seen, and the failure mode of getting it wrong
 * is telling somebody to wait until 1970.
 */
function readRetryAfter(response: Response): number | undefined {
  const raw = response.headers.get(RETRY_AFTER_HEADER)
  if (raw === null) {
    return undefined
  }
  const seconds = Number.parseInt(raw.trim(), 10)
  return Number.isFinite(seconds) && seconds >= 0 ? seconds : undefined
}

function nonEmptyString(value: unknown): string | undefined {
  return typeof value === 'string' && value !== '' ? value : undefined
}

/** The parsed body, as far as it can be trusted: every member is `unknown`. */
type UntrustedProblem = Partial<Record<keyof Problem, unknown>>

async function readProblem(response: Response): Promise<UntrustedProblem | undefined> {
  if (!response.headers.get('content-type')?.includes('json')) {
    return undefined
  }
  try {
    const body: unknown = await response.clone().json()
    return typeof body === 'object' && body !== null ? body : undefined
  } catch {
    return undefined
  }
}

function readFieldErrors(value: unknown): readonly FieldError[] {
  if (!Array.isArray(value)) {
    return []
  }
  return value.filter(
    (entry: unknown): entry is FieldError =>
      typeof entry === 'object' &&
      entry !== null &&
      typeof (entry as FieldError).field === 'string' &&
      typeof (entry as FieldError).message === 'string',
  )
}
