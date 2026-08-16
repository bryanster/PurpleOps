import { describe, expect, it, vi } from 'vitest'

import { ApiError, API_BASE_PATH, createClient, unwrap } from './index'

// These cover the seam between the hand-written wrapper and the generated
// types: where a request goes, what it carries, and how a failure arrives. They
// do not re-test `openapi-fetch` — whether it serialises a query parameter
// correctly is its business, and asserting it here would mean writing the
// request builder out a second time by hand.

const HEALTHY = JSON.stringify({ status: 'ok', checks: { db: 'ok' } })

/** A `fetch` that answers one canned response and records what it was asked. */
function stubFetch(
  body: string,
  init: { status?: number; contentType?: string; headers?: Record<string, string> } = {},
) {
  const calls: Request[] = []
  const fetch = vi.fn(async (input: Request | string | URL): Promise<Response> => {
    calls.push(input instanceof Request ? input : new Request(input))
    return new Response(body, {
      status: init.status ?? 200,
      headers: {
        'content-type': init.contentType ?? 'application/json',
        ...init.headers,
      },
    })
  })
  return { fetch: fetch as unknown as typeof globalThis.fetch, calls }
}

describe('createClient', () => {
  // The reason createClient exists: the document's one server is the relative
  // URL /api/v1, so a caller who passed their origin to openapi-fetch directly
  // would be talking to the SPA's index.html.
  it('appends the API base path', async () => {
    const { fetch, calls } = stubFetch(HEALTHY)
    const client = createClient({ baseUrl: 'https://blacklight.example.com', fetch })

    await client.GET('/healthz')

    expect(calls[0]?.url).toBe(`https://blacklight.example.com${API_BASE_PATH}/healthz`)
  })

  // An operator's BLACKLIGHT_URL very often ends in a slash, and `//api/v1` is
  // redirected by some reverse proxies and 404ed by others.
  it('tolerates a trailing slash on the base URL', async () => {
    const { fetch, calls } = stubFetch(HEALTHY)
    const client = createClient({ baseUrl: 'https://blacklight.example.com///', fetch })

    await client.GET('/healthz')

    expect(calls[0]?.url).toBe(`https://blacklight.example.com${API_BASE_PATH}/healthz`)
  })

  it('refuses an empty base URL', () => {
    expect(() => createClient({ baseUrl: '   ' })).toThrow(TypeError)
  })

  it('sends the service token as a bearer credential', async () => {
    const { fetch, calls } = stubFetch(HEALTHY)
    const client = createClient({
      baseUrl: 'https://blacklight.example.com',
      serviceToken: 'bl_abcd_secret',
      fetch,
    })

    await client.GET('/healthz')

    expect(calls[0]?.headers.get('authorization')).toBe('Bearer bl_abcd_secret')
  })

  it('sends no credential when there is no token', async () => {
    const { fetch, calls } = stubFetch(HEALTHY)
    const client = createClient({ baseUrl: 'https://blacklight.example.com', fetch })

    await client.GET('/healthz')

    expect(calls[0]?.headers.get('authorization')).toBeNull()
  })
})

describe('error handling', () => {
  const problem = JSON.stringify({
    type: 'about:blank',
    title: 'Not Found',
    status: 404,
    code: 'not_found',
    detail: 'no engagement with that id',
  })

  it('throws an ApiError carrying the problem code', async () => {
    const { fetch } = stubFetch(problem, {
      status: 404,
      contentType: 'application/problem+json',
      headers: { 'x-request-id': 'req-123' },
    })
    const client = createClient({ baseUrl: 'https://blacklight.example.com', fetch })

    await expect(
      client.GET('/engagements/{engagementId}', {
        params: { path: { engagementId: '018f4c00-0000-7000-8000-000000000000' } },
      }),
    ).rejects.toMatchObject({
      name: 'ApiError',
      status: 404,
      code: 'not_found',
      requestId: 'req-123',
      message: 'no engagement with that id',
    })
  })

  it('reads Retry-After off a 429', async () => {
    const { fetch } = stubFetch(
      JSON.stringify({
        type: 'about:blank',
        title: 'Too Many Requests',
        status: 429,
        code: 'rate_limited',
      }),
      {
        status: 429,
        contentType: 'application/problem+json',
        headers: { 'retry-after': '240' },
      },
    )
    const client = createClient({ baseUrl: 'https://blacklight.example.com', fetch })

    await expect(
      client.POST('/auth/login', { body: { email: 'a@b.c', password: 'x' } }),
    ).rejects.toMatchObject({
      status: 429,
      retryAfterSeconds: 240,
    })
  })

  // /healthz answers 503 with the same Health shape as its 200 — the
  // interesting part being which dependency is down. Distinguishing that from a
  // failure on the media type is exactly what application/problem+json is for.
  it('does not throw for a documented non-2xx that is not a problem document', async () => {
    const { fetch } = stubFetch(JSON.stringify({ status: 'error', checks: { db: 'error' } }), {
      status: 503,
    })
    const client = createClient({ baseUrl: 'https://blacklight.example.com', fetch })

    const result = await client.GET('/healthz')

    expect(result.response.status).toBe(503)
    expect(result.error).toMatchObject({ status: 'error' })
  })

  it('still errors on a body that is not JSON at all', async () => {
    const { fetch } = stubFetch('<html>502 Bad Gateway</html>', {
      status: 502,
      contentType: 'text/html',
    })
    const client = createClient({ baseUrl: 'https://blacklight.example.com', fetch })

    await expect(client.GET('/healthz')).rejects.toMatchObject({ status: 502 })
  })

  it('returns the typed error instead of throwing when throwOnError is false', async () => {
    const { fetch } = stubFetch(problem, { status: 404, contentType: 'application/problem+json' })
    const client = createClient({
      baseUrl: 'https://blacklight.example.com',
      throwOnError: false,
      fetch,
    })

    const result = await client.GET('/engagements/{engagementId}', {
      params: { path: { engagementId: '018f4c00-0000-7000-8000-000000000000' } },
    })

    expect(result.error).toMatchObject({ code: 'not_found' })
  })
})

describe('unwrap', () => {
  it('returns the body of a successful call', async () => {
    const { fetch } = stubFetch(HEALTHY)
    const client = createClient({ baseUrl: 'https://blacklight.example.com', fetch })

    const health = unwrap(await client.GET('/healthz'))

    expect(health.checks.db).toBe('ok')
  })

  // Reachable when the server answers an undocumented status, or a non-2xx with
  // a plain JSON body that is not what the operation says it returns. Neither
  // should reach a caller as a silent `undefined`.
  it('throws when there is no body to unwrap', () => {
    expect(() => unwrap({ response: new Response(null, { status: 502 }) })).toThrow(ApiError)
  })
})
