import { HttpResponse } from 'msw'
import { afterEach, describe, expect, it } from 'vitest'

import {
  get,
  internalError,
  notFound,
  post,
  problem,
  TEST_REQUEST_ID,
  unhealthyFixture,
  versionFixture,
} from '@/test/msw/handlers'
import { server } from '@/test/msw/server'

import { api, API_BASE_PATH, API_BASE_URL, apiUrl } from './client'
import { ApiError, isApiError } from './errors'

/** The rejection from a call, as an unknown rather than a promise assertion. */
async function failureOf(call: Promise<unknown>): Promise<unknown> {
  return await call.then(
    () => {
      throw new Error('expected the call to reject')
    },
    (cause: unknown) => cause,
  )
}

describe('apiUrl', () => {
  it('builds a same-origin URL under the API base path', () => {
    expect(apiUrl('/version')).toBe(`${API_BASE_URL}/version`)
    expect(API_BASE_URL).toBe(`${window.location.origin}${API_BASE_PATH}`)
  })
})

describe('the problem middleware', () => {
  it('returns the typed body of a successful response', async () => {
    const { data, error } = await api.GET('/version')

    expect(error).toBeUndefined()
    expect(data).toEqual(versionFixture)
  })

  it('turns a 404 problem document into an ApiError carrying its code', async () => {
    server.use(get('/version', () => notFound('no such build')))

    const failure = await failureOf(api.GET('/version'))

    // The acceptance criterion: a typed error, not a SyntaxError from parsing.
    expect(isApiError(failure, 'not_found')).toBe(true)
    if (!(failure instanceof ApiError)) {
      throw new Error('unreachable')
    }
    expect(failure.status).toBe(404)
    expect(failure.detail).toBe('no such build')
    expect(failure.message).toBe('no such build')
  })

  it('carries the echoed request ID, so a user can quote it', async () => {
    server.use(get('/version', () => internalError()))

    const failure = await failureOf(api.GET('/version'))

    expect(failure).toBeInstanceOf(ApiError)
    expect((failure as ApiError).requestId).toBe(TEST_REQUEST_ID)
  })

  it('carries the field errors of a validation failure', async () => {
    server.use(
      get('/version', () =>
        problem({
          status: 400,
          code: 'validation_failed',
          title: 'Bad Request',
          detail: 'the request body is not valid',
          errors: [
            { field: 'members[0].role', message: 'must be one of lead, red, blue, observer' },
          ],
        }),
      ),
    )

    const failure = await failureOf(api.GET('/version'))

    expect(failure).toBeInstanceOf(ApiError)
    expect((failure as ApiError).errors).toEqual([
      { field: 'members[0].role', message: 'must be one of lead, red, blue, observer' },
    ])
  })

  it('produces an ApiError for an error body that is not JSON at all', async () => {
    // What a reverse proxy in front of the server sends when the server is
    // unreachable. Parsing it as a problem document throws SyntaxError, which
    // tells the user nothing and hides the 502 that would have.
    server.use(
      get(
        '/version',
        () =>
          new HttpResponse('<html><body>502 Bad Gateway</body></html>', {
            status: 502,
            headers: { 'content-type': 'text/html' },
          }),
      ),
    )

    const failure = await failureOf(api.GET('/version'))

    expect(failure).toBeInstanceOf(ApiError)
    expect((failure as ApiError).status).toBe(502)
    expect((failure as ApiError).code).toBeUndefined()
    expect((failure as ApiError).message).toContain('502')
  })

  it('leaves a documented non-2xx body alone', async () => {
    // GET /healthz answers 503 with the same Health shape as its 200. Throwing
    // that away would discard the only useful part: which dependency is down.
    server.use(get('/healthz', () => HttpResponse.json(unhealthyFixture, { status: 503 })))

    const { error, response } = await api.GET('/healthz')

    expect(response.status).toBe(503)
    expect(error).toEqual(unhealthyFixture)
  })
})

describe('the CSRF middleware', () => {
  /** Records the headers of whatever request reaches the fake server. */
  function captureLogout(): { headers?: Headers } {
    const captured: { headers?: Headers } = {}
    server.use(
      post('/auth/logout', ({ request }) => {
        captured.headers = request.headers
        return new HttpResponse(null, { status: 204 })
      }),
    )
    return captured
  }

  afterEach(() => {
    document.cookie = 'bl_csrf=; max-age=0'
  })

  it('attaches the token from the cookie to a state-changing request', async () => {
    // The acceptance criterion for the client half of M1-005: no component
    // touches this, and no call site opts in.
    document.cookie = 'bl_csrf=the-derived-token'
    const captured = captureLogout()

    await api.POST('/auth/logout')

    expect(captured.headers?.get('X-CSRF-Token')).toBe('the-derived-token')
  })

  it('sends no token on a request that changes nothing', async () => {
    document.cookie = 'bl_csrf=the-derived-token'
    const captured: { headers?: Headers } = {}
    server.use(
      get('/version', ({ request }) => {
        captured.headers = request.headers
        return HttpResponse.json(versionFixture)
      }),
    )

    await api.GET('/version')

    expect(captured.headers?.get('X-CSRF-Token')).toBeNull()
  })

  it('sends no header when the browser holds no cookie', async () => {
    // The server answers this with a 403 carrying a fresh cookie, so the retry
    // works. Inventing a value here would only turn that into a confusing one.
    const captured = captureLogout()

    await api.POST('/auth/logout')

    expect(captured.headers?.get('X-CSRF-Token')).toBeNull()
  })

  it('reads its own cookie rather than the first one it finds', async () => {
    document.cookie = 'other=first'
    document.cookie = 'bl_csrf=the-derived-token'
    const captured = captureLogout()

    await api.POST('/auth/logout')

    expect(captured.headers?.get('X-CSRF-Token')).toBe('the-derived-token')
    document.cookie = 'other=; max-age=0'
  })
})
