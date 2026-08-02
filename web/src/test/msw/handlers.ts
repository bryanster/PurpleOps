import { http, HttpResponse, type HttpResponseResolver } from 'msw'

import { apiUrl } from '@/api/client'
import { PROBLEM_MEDIA_TYPE, type Problem, REQUEST_ID_HEADER } from '@/api/errors'
import type { components, paths } from '@/api/schema'

/**
 * The fake server every test talks to.
 *
 * Handlers and fixtures are typed from `api/openapi.yaml` — `apiUrl` only
 * accepts a path the spec has, and each body is the generated schema type — so
 * a fixture cannot drift into describing a response the real server would never
 * send. That is the failure mode that makes mocked frontend tests worthless.
 *
 * This is the fixture library for the rest of the project: add to it rather
 * than building a one-off response inline, and override it per test with
 * `server.use(...)` for the cases a particular test is about.
 */

export const versionFixture: components['schemas']['Version'] = {
  version: 'v2.0.0-test',
  commit: 'abcdef123456',
  buildDate: '2026-02-01T12:00:00Z',
}

export const healthyFixture: components['schemas']['Health'] = {
  status: 'ok',
  checks: { db: 'ok' },
}

export const unhealthyFixture: components['schemas']['Health'] = {
  status: 'error',
  checks: { db: 'error' },
}

/** The request ID the fake server echoes, for tests that assert it survives. */
export const TEST_REQUEST_ID = '018f3b2c-7a41-7c3e-9b0d-2f1a4c6e8d90'

export const handlers = [
  get('/version', () => HttpResponse.json(versionFixture)),
  get('/healthz', () => HttpResponse.json(healthyFixture)),
]

/**
 * A handler for one operation. Wrapping `http.get` keeps the URL construction
 * in one place and the path checked against the generated `paths`.
 */
export function get(
  path: keyof paths,
  resolver: HttpResponseResolver,
): ReturnType<typeof http.get> {
  return http.get(apiUrl(path), resolver)
}

/** The same, for an operation that changes something. */
export function post(
  path: keyof paths,
  resolver: HttpResponseResolver,
): ReturnType<typeof http.post> {
  return http.post(apiUrl(path), resolver)
}

/**
 * An RFC 9457 problem document, served the way the real server serves one:
 * `application/problem+json`, the status repeated in the body, and the request
 * ID in the header as well as in `instance`.
 */
export function problem(init: {
  status: number
  code: components['schemas']['ProblemCode']
  title: string
  detail?: string
  errors?: components['schemas']['FieldError'][]
}): HttpResponse<Problem> {
  const body: Problem = {
    type: 'about:blank',
    title: init.title,
    status: init.status,
    code: init.code,
    instance: TEST_REQUEST_ID,
    ...(init.detail === undefined ? {} : { detail: init.detail }),
    ...(init.errors === undefined ? {} : { errors: init.errors }),
  }
  return HttpResponse.json(body, {
    status: init.status,
    headers: {
      'content-type': PROBLEM_MEDIA_TYPE,
      [REQUEST_ID_HEADER]: TEST_REQUEST_ID,
    },
  })
}

/** The 404 the server sends for a resource that is not there. */
export function notFound(detail = 'no such thing'): HttpResponse<Problem> {
  return problem({ status: 404, code: 'not_found', title: 'Not Found', detail })
}

/** The 500 the server sends for anything it cannot classify. */
export function internalError(): HttpResponse<Problem> {
  return problem({
    status: 500,
    code: 'internal',
    title: 'Internal Server Error',
    detail: 'the server failed to handle this request',
  })
}
