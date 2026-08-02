import createClient, { type Middleware } from 'openapi-fetch'

import { ApiError, apiErrorFromResponse, PROBLEM_MEDIA_TYPE } from './errors'
import type { paths } from './schema'

/**
 * The one place in the SPA that knows an API URL.
 *
 * Every request goes through the generated client below, whose paths come from
 * `schema.d.ts` — so a path that is not in `api/openapi.yaml`, or a response
 * field the spec does not describe, is a `tsc` failure rather than a 404 a user
 * finds. `eslint.config.js` forbids an `/api/…` string literal anywhere else.
 */

/** Matches `internal/httpapi.BasePath`, and the `servers` entry in the spec. */
export const API_BASE_PATH = '/api/v1'

/**
 * Absolute, resolved against the page's own origin. The SPA is served by the
 * API server itself (M0B-010) and proxied to it in dev, so same-origin is
 * always the right answer — and an absolute URL is additionally what Node's
 * `fetch` needs under Vitest, where there is no document to resolve a relative
 * one against.
 */
export const API_BASE_URL = `${window.location.origin}${API_BASE_PATH}`

/**
 * The absolute URL an operation is served at. For test doubles and for anything
 * that needs a URL rather than a call — the path argument is checked against the
 * generated `paths`, so a stale one does not compile.
 */
export function apiUrl(path: keyof paths): string {
  return `${API_BASE_URL}${path}`
}

/**
 * Turn an error response into a thrown [ApiError].
 *
 * A non-2xx that carries a problem document is a failure and nothing else, so
 * it is raised rather than returned — callers deal in data, and TanStack Query
 * needs a rejected promise to see a failure at all.
 *
 * A non-2xx that carries a *documented* body is left alone: `GET /healthz`
 * answers 503 with the same `Health` shape as its 200, because the interesting
 * part is which dependency is down. Distinguishing the two on the media type is
 * exactly what `application/problem+json` is for.
 */
/**
 * The cookie the server issues the CSRF token in. Matches
 * `session.CSRFCookieName`; it is deliberately not `HttpOnly`, because reading
 * it here is the whole mechanism.
 */
const CSRF_COOKIE = 'pops_csrf'

/** The header the token is echoed in. Matches `httpapi.CSRFHeader`. */
const CSRF_HEADER = 'X-CSRF-Token'

/** The methods that change nothing, and so carry no token. */
const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS'])

/** The value of one cookie, or undefined when the browser holds none. */
function readCookie(name: string): string | undefined {
  const prefix = `${name}=`
  for (const entry of document.cookie.split(';')) {
    const trimmed = entry.trim()
    if (trimmed.startsWith(prefix)) {
      return decodeURIComponent(trimmed.slice(prefix.length))
    }
  }
  return undefined
}

/**
 * Attach the double-submit CSRF token to every state-changing request (M1-005).
 *
 * It is middleware so that no component, hook or mutation ever thinks about
 * CSRF: the one thing a caller could get wrong here is forgetting, and there is
 * nothing to forget. The server refuses a cookie-authenticated `POST`, `PUT`,
 * `PATCH` or `DELETE` without it with a 403.
 *
 * A missing cookie sends no header, which is a 403 the server answers with a
 * fresh cookie — so the next attempt works rather than the client having to
 * detect and repair anything.
 */
const csrfMiddleware: Middleware = {
  onRequest({ request }) {
    if (SAFE_METHODS.has(request.method.toUpperCase())) {
      return undefined
    }
    const token = readCookie(CSRF_COOKIE)
    if (token !== undefined) {
      request.headers.set(CSRF_HEADER, token)
    }
    return request
  },
}

const problemMiddleware: Middleware = {
  async onResponse({ response }) {
    if (response.ok) {
      return undefined
    }
    const contentType = response.headers.get('content-type') ?? ''
    if (!contentType.includes(PROBLEM_MEDIA_TYPE) && contentType.includes('json')) {
      return undefined
    }
    throw await apiErrorFromResponse(response)
  },
}

export const api = createClient<paths>({
  baseUrl: API_BASE_URL,
  // Sessions are cookies (M1-003). `include` rather than the `same-origin`
  // default so that a deployment fronted by a proxy on another host still
  // sends them; the cookie's own `SameSite=Strict` is what actually restricts
  // it, and that is set by the server.
  credentials: 'include',
  // openapi-fetch reads `globalThis.fetch` once, when the client is created.
  // Calling it through this closure binds it per request instead, which costs
  // nothing and means anything that replaces fetch after this module is
  // imported is actually used — MSW in the tests, and whatever instrumentation
  // wants to wrap it later.
  fetch: (request) => globalThis.fetch(request),
})

api.use(csrfMiddleware)
api.use(problemMiddleware)

/**
 * The success body of a call, or a thrown [ApiError].
 *
 * `problemMiddleware` has already thrown for every error this API documents, so
 * by the time a result reaches here it has data — but the generated types
 * cannot know that, and `data` is optional in all of them. This is the one
 * place that reconciles the two, rather than every query doing it with a `!`.
 */
export function unwrap<T>(result: { data?: T; error?: unknown; response: Response }): T {
  if (result.data !== undefined) {
    return result.data
  }
  // Reachable only if the server answers an undocumented status, or a non-2xx
  // with a JSON body that is not a problem document and not what the operation
  // says it returns. Both are contract breaches, and neither should be a
  // silent `undefined` handed to a component.
  throw new ApiError(`unexpected ${String(result.response.status)} response`, {
    status: result.response.status,
  })
}
