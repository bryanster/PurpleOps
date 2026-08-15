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

/**
 * The signed-in administrator every identity test starts from, and the member
 * beside them. Two fixtures rather than one because most of what M1-017 does is
 * show these two people different screens.
 */
export const adminUserFixture: components['schemas']['CurrentUser'] = {
  id: '0192f1a0-0000-7000-8000-00000000a001',
  email: 'ada@example.test',
  displayName: 'Ada Lovelace',
  platformRole: 'admin',
  mfa: {
    enforced: false,
    required: false,
    enrolled: true,
    satisfied: true,
    recoveryCodesRemaining: 10,
  },
  memberships: [],
  csrfToken: 'test-csrf-token',
}

export const memberUserFixture: components['schemas']['CurrentUser'] = {
  ...adminUserFixture,
  id: '0192f1a0-0000-7000-8000-00000000a002',
  email: 'mel@example.test',
  displayName: 'Mel Chen',
  platformRole: 'member',
}

/** Somebody an administrator requires a second factor of, who has not enrolled. */
export const mustEnrolUserFixture: components['schemas']['CurrentUser'] = {
  ...memberUserFixture,
  mfa: {
    enforced: true,
    required: true,
    enrolled: false,
    satisfied: false,
    recoveryCodesRemaining: 0,
  },
}

export const authProvidersFixture: components['schemas']['AuthProviders'] = {
  password: true,
  sso: [],
}

/** The session the request was made on, and one somewhere else. */
export const currentSessionFixture: components['schemas']['Session'] = {
  id: '0192f1a0-0000-7000-8000-00000000b001',
  current: true,
  createdAt: '2026-02-01T09:00:00Z',
  lastSeenAt: '2026-02-01T09:30:00Z',
  expiresAt: '2026-02-08T09:00:00Z',
  mfaSatisfied: true,
  ip: '198.51.100.10',
  userAgent: 'Mozilla/5.0 (this browser)',
}

export const otherSessionFixture: components['schemas']['Session'] = {
  id: '0192f1a0-0000-7000-8000-00000000b002',
  current: false,
  createdAt: '2026-01-30T18:00:00Z',
  lastSeenAt: '2026-01-31T08:00:00Z',
  expiresAt: '2026-02-06T18:00:00Z',
  mfaSatisfied: false,
  ip: '203.0.113.7',
  userAgent: 'Mozilla/5.0 (a laptop in a hotel)',
}

export const sessionsFixture: components['schemas']['Sessions'] = {
  items: [currentSessionFixture, otherSessionFixture],
}

/** The response `POST /auth/mfa/totp/enroll` gives, with a one-pixel QR code. */
export const totpEnrolmentFixture: components['schemas']['TOTPEnrolment'] = {
  otpauthUri:
    'otpauth://totp/Blacklight:ada@example.test?secret=JBSWY3DPEHPK3PXP&issuer=Blacklight',
  secret: 'JBSWY3DPEHPK3PXP',
  qrCode:
    'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==',
}

export const recoveryCodesFixture: components['schemas']['RecoveryCodes'] = {
  codes: [
    '3K9M-2PTV-XA47-QRJH-58WY',
    '7QW2-9RTX-BM31-KDPZ-64VC',
    'H8N5-3JKQ-YT92-WRVM-71XD',
    'P4C7-6MXB-ZQ38-HNKT-95RG',
  ],
  generatedAt: '2026-02-01T09:00:00Z',
}

/**
 * The first-run state of an installation somebody has already configured, and
 * the one the default handler serves. The wizard's tests serve the other one.
 */
export const setupCompleteFixture: components['schemas']['SetupState'] = {
  completed: true,
  completedAt: '2026-01-02T10:00:00Z',
  completedBy: adminUserFixture.id,
}

export const handlers = [
  get('/version', () => HttpResponse.json(versionFixture)),
  get('/healthz', () => HttpResponse.json(healthyFixture)),

  // The identity screens (M1-017). A signed-in administrator is the default,
  // because it is the state most screens are written for; a test about somebody
  // else overrides these with `server.use(...)`.
  get('/auth/me', () => HttpResponse.json(adminUserFixture)),
  get('/auth/providers', () => HttpResponse.json(authProvidersFixture)),
  get('/auth/sessions', () => HttpResponse.json(sessionsFixture)),

  // An installation somebody has already set up, because that is the state
  // every screen but the wizard is written for. `RequireAuth` reads this for
  // administrators, so a test about any other screen would otherwise be a test
  // about an unhandled request. The wizard's own tests override it.
  get('/setup', () => HttpResponse.json(setupCompleteFixture)),
]

/** The 401 an anonymous browser gets from `GET /auth/me`. */
export function unauthenticated(): HttpResponse<Problem> {
  return problem({
    status: 401,
    code: 'unauthenticated',
    title: 'Unauthorized',
    detail: 'this endpoint needs a signed-in session',
  })
}

/** The 429 the login throttle answers with (M1-004), carrying `Retry-After`. */
export function rateLimited(retryAfterSeconds: number): HttpResponse<Problem> {
  const response = problem({
    status: 429,
    code: 'rate_limited',
    title: 'Too Many Requests',
    detail: 'too many sign-in attempts',
  })
  response.headers.set('Retry-After', String(retryAfterSeconds))
  return response
}

/**
 * A handler for one operation. Wrapping `http.get` keeps the URL construction
 * in one place and the path checked against the generated `paths`.
 */
export function get(
  path: keyof paths,
  resolver: HttpResponseResolver,
): ReturnType<typeof http.get> {
  return http.get(pattern(path), resolver)
}

/**
 * The URL an operation is matched at, with the spec's `{param}` placeholders
 * rewritten to the `:param` form MSW matches on.
 *
 * Without this a templated path is matched literally — braces and all — so the
 * handler never fires and the request fails as unhandled. That failure looks
 * like a broken component rather than a broken fixture, which is an afternoon
 * nobody gets back.
 */
function pattern(path: keyof paths): string {
  return apiUrl(path).replace(/\{(\w+)\}/g, ':$1')
}

/** The same, for an operation that changes something. */
export function del(
  path: keyof paths,
  resolver: HttpResponseResolver,
): ReturnType<typeof http.delete> {
  return http.delete(pattern(path), resolver)
}

export function post(
  path: keyof paths,
  resolver: HttpResponseResolver,
): ReturnType<typeof http.post> {
  return http.post(pattern(path), resolver)
}

/** The same again, for the two operations that patch. */
export function patch(
  path: keyof paths,
  resolver: HttpResponseResolver,
): ReturnType<typeof http.patch> {
  return http.patch(pattern(path), resolver)
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

/**
 * The 403 the MFA enrolment gate sends (M1-008). It is the one code in the
 * vocabulary that refines another, which is what the fallback in `isApiError`
 * is for.
 */
export function mfaEnrolmentRequired(): HttpResponse<Problem> {
  return problem({
    status: 403,
    code: 'mfa_enrolment_required',
    title: 'Forbidden',
    detail: 'a second factor is required of your account; enrol an authenticator to continue',
  })
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
