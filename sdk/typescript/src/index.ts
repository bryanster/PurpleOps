import createOpenapiFetchClient, { type Client, type Middleware } from 'openapi-fetch'

import { apiErrorFromResponse, ApiError, PROBLEM_MEDIA_TYPE } from './errors'
import type { paths } from './schema'

/**
 * A typed client for the Blacklight API.
 *
 * Every path, parameter, request body and response in this package comes from
 * `api/openapi.yaml` by way of `src/schema.d.ts`, which `make generate`
 * overwrites. Calling a path that is not in the document, or reading a field
 * the server does not send, is a compile error rather than a surprise at
 * runtime.
 *
 * ```ts
 * import { createClient, ApiError } from 'blacklight-sdk'
 *
 * const blacklight = createClient({
 *   baseUrl: 'https://blacklight.example.com',
 *   serviceToken: process.env.BLACKLIGHT_TOKEN,
 * })
 *
 * const { data } = await blacklight.GET('/engagements', { params: { query: { limit: 50 } } })
 * for (const engagement of data?.items ?? []) console.log(engagement.name)
 * ```
 */

export * from './errors'
export type { components, operations, paths } from './schema'

/** Matches `internal/httpapi.BasePath`, and the `servers` entry in the spec. */
export const API_BASE_PATH = '/api/v1'

/** What [createClient] returns: `openapi-fetch` bound to this API's paths. */
export type BlacklightClient = Client<paths>

export interface CreateClientOptions {
  /**
   * The deployment's origin — `https://blacklight.example.com` — with no API
   * path on it. [API_BASE_PATH] is appended.
   *
   * The document declares its one server as a *relative* URL, because the SPA
   * is served from the same origin as the API and an absolute URL would pin
   * every deployment to one host. A client outside the browser has to be told
   * which host, which is what this is.
   */
  baseUrl: string

  /**
   * A service token — the `bl_<prefix>_<secret>` string shown once when the
   * token was created — sent as `Authorization: Bearer …` on every request.
   *
   * The other credential this API accepts is the browser session cookie, which
   * this SDK deliberately does not help you obtain: a token can be scoped and
   * expired by an administrator, and driving the login and MFA endpoints from a
   * script to get a cookie instead is working around that. (The SPA is the
   * cookie's client, and it has its own in `web/src/api`.)
   */
  serviceToken?: string

  /** Extra headers on every request. */
  headers?: Record<string, string>

  /**
   * A `fetch` to use instead of the global one — a test double, or an
   * instrumented or proxied fetch.
   */
  fetch?: typeof globalThis.fetch

  /**
   * Turn a documented failure into a thrown [ApiError]. On by default.
   *
   * Set it to `false` to get `openapi-fetch`'s `{ data, error }` back
   * untouched, where `error` is the typed problem document. That is the better
   * shape when a caller handles every failure; the default is the better one
   * for a script, where an unhandled failure should stop it rather than carry
   * on with `undefined`.
   */
  throwOnError?: boolean
}

/**
 * Attach the service token to every request.
 *
 * Middleware rather than a default header so that the token is read per
 * request: a caller that rotates one does not have to rebuild the client.
 */
function bearerMiddleware(token: string): Middleware {
  return {
    onRequest({ request }) {
      request.headers.set('Authorization', `Bearer ${token}`)
      return request
    },
  }
}

/**
 * Throw for a failure the API documented, and leave everything else alone.
 *
 * The media type is what distinguishes the two. `GET /healthz` answers 503 with
 * the same `Health` shape as its 200 — the interesting part being which
 * dependency is down — and that is `application/json`, not a problem document,
 * so it comes back as data.
 */
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

/**
 * Build a client for the Blacklight deployment at `baseUrl`.
 *
 * @throws {TypeError} if `baseUrl` is empty.
 */
export function createClient(options: CreateClientOptions): BlacklightClient {
  const baseUrl = options.baseUrl.trim()
  if (baseUrl === '') {
    throw new TypeError(
      'blacklight: baseUrl is empty; pass the deployment origin, such as https://blacklight.example.com',
    )
  }

  const client = createOpenapiFetchClient<paths>({
    // Trailing slashes on both halves would produce `//api/v1`, which some
    // reverse proxies redirect and others 404.
    baseUrl: `${baseUrl.replace(/\/+$/, '')}${API_BASE_PATH}`,
    ...(options.headers !== undefined && { headers: options.headers }),
    // openapi-fetch reads `globalThis.fetch` once, when the client is created.
    // Calling it through a closure binds it per request instead, which costs
    // nothing and means anything that replaces fetch afterwards is actually
    // used.
    fetch: options.fetch ?? ((request) => globalThis.fetch(request)),
  })

  if (options.serviceToken !== undefined && options.serviceToken !== '') {
    client.use(bearerMiddleware(options.serviceToken))
  }
  if (options.throwOnError !== false) {
    client.use(problemMiddleware)
  }

  return client
}

/**
 * The success body of a call, or a thrown [ApiError].
 *
 * With the default `throwOnError`, every documented failure has already thrown
 * by the time a result reaches here — but the generated types cannot know that,
 * and `data` is optional in all of them. This reconciles the two in one place
 * rather than at every call site with a `!`.
 */
export function unwrap<T>(result: { data?: T; error?: unknown; response: Response }): T {
  if (result.data !== undefined) {
    return result.data
  }
  // Reachable when the server answers an undocumented status, or a non-2xx with
  // a plain JSON body that is not what the operation says it returns. Both are
  // contract breaches, and neither should be a silent `undefined`.
  throw new ApiError(`unexpected ${String(result.response.status)} response`, {
    status: result.response.status,
  })
}
