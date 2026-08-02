import type { APIRequestContext, APIResponse } from '@playwright/test'

import { expect, test } from '../harness/test'

/**
 * M1-008, end to end against a real binary: an administrator turns enforcement
 * on, and the second user is confined to enrolling.
 *
 * This one drives the API rather than the browser. There is no auth UI yet —
 * `M1-017` owns the login, MFA and account screens — so there is nothing to
 * click, and a spec that waited for one would be a spec that does not exist. The
 * request fixture still exercises everything that matters here: the shipped
 * binary, the migrations it applied, its cookie jar, and the middleware chain
 * that decides what a confined session may reach. When `M1-017` lands, the
 * browser version of this belongs beside it.
 *
 * The seeded passwords are in this file on purpose. They belong to two accounts
 * in a database created for this spec file and deleted at teardown.
 */

const adminPassword = 'an administrator passphrase'
const memberPassword = 'a member passphrase entirely'

const admin = 'admin@example.test'
const member = 'member@example.test'

test.use({
  seed: {
    steps: [
      // The database is created empty and the server migrates it at startup —
      // but seeding runs *before* the server, because DuckDB admits one writer
      // at a time. So a seed that writes rows migrates first.
      ['migrate', 'up'],
      {
        args: ['user', 'create', '--email', admin, '--name', 'Ada', '--admin'],
        stdin: adminPassword,
      },
      {
        args: ['user', 'create', '--email', member, '--name', 'Mel'],
        stdin: memberPassword,
      },
    ],
  },
})

/**
 * The shapes this spec reads, narrowed by hand.
 *
 * The generated client (`web/src/api/schema.d.ts`) is the real definition of
 * these, and it belongs to a different npm project — reaching across into it
 * would tie the suite's build to the SPA's. What is written out here is only
 * what is asserted on, so a field that changes shape fails to compile here as
 * well as there.
 */
interface LoginResult {
  status: string
  user?: { mfa: { required: boolean; enrolled: boolean; satisfied: boolean } }
}
interface MFAPolicy {
  requiredForAll: boolean
  requiredForAdmins: boolean
}
interface Problem {
  code: string
}
interface TOTPEnrolment {
  secret: string
}

test('an administrator turns MFA on and the next user is forced through enrolment', async ({
  request,
}) => {
  // Before the policy: an ordinary sign-in, an ordinary session.
  const before = await request.post('/api/v1/auth/login', {
    data: { email: member, password: memberPassword },
  })
  expect(before.ok()).toBeTruthy()
  expect((await bodyOf<LoginResult>(before)).status).toBe('authenticated')

  // The administrator turns it on. A whole policy, not a patch.
  const adminLogin = await request.post('/api/v1/auth/login', {
    data: { email: admin, password: adminPassword },
  })
  expect(adminLogin.ok()).toBeTruthy()

  const written = await request.put('/api/v1/settings/mfa', {
    data: { requiredForAll: true, requiredForAdmins: false },
    // The double-submit token (M1-005). The request context holds the cookies
    // from the sign-in above, and `bl_csrf` is readable by design.
    headers: { 'X-CSRF-Token': await csrfToken(request) },
  })
  expect(written.ok()).toBeTruthy()
  expect(await bodyOf<MFAPolicy>(written)).toMatchObject({ requiredForAll: true })

  // The member signs in again. Same password, different answer — and a session
  // this time that can do exactly one thing.
  const after = await request.post('/api/v1/auth/login', {
    data: { email: member, password: memberPassword },
  })
  expect(after.ok()).toBeTruthy()

  const result = await bodyOf<LoginResult>(after)
  expect(result.status).toBe('mfa_enrolment_required')
  expect(result.user?.mfa).toMatchObject({ required: true, enrolled: false, satisfied: false })

  // The application is closed to them, and the refusal says why in a way a
  // client can act on.
  const refused = await request.post('/api/v1/auth/password', {
    data: { currentPassword: memberPassword, newPassword: 'something else again' },
    headers: { 'X-CSRF-Token': await csrfToken(request) },
  })
  expect(refused.status()).toBe(403)
  expect((await bodyOf<Problem>(refused)).code).toBe('mfa_enrolment_required')

  // And the way out is open: enrolling is what this session exists for.
  const enrolment = await request.post('/api/v1/auth/mfa/totp/enroll', {
    headers: { 'X-CSRF-Token': await csrfToken(request) },
  })
  expect(enrolment.ok()).toBeTruthy()
  expect((await bodyOf<TOTPEnrolment>(enrolment)).secret).toBeTruthy()
})

/** The response body, as the shape this spec expects it to have. */
async function bodyOf<T>(response: APIResponse): Promise<T> {
  const parsed: unknown = await response.json()
  return parsed as T
}

/**
 * The CSRF token this request context is holding, read from the `bl_csrf`
 * cookie the way a browser client does (M1-005) — it is deliberately not
 * `HttpOnly`, because script has to echo it back in the header.
 */
async function csrfToken(request: APIRequestContext): Promise<string> {
  const state = await request.storageState()
  const cookie = state.cookies.find((c) => c.name === 'bl_csrf')
  if (cookie === undefined) {
    throw new Error(
      `no bl_csrf cookie in the request context; it is set alongside the session cookie, so ` +
        `its absence means the sign-in did not establish one. Cookies held: ` +
        state.cookies.map((c) => c.name).join(', '),
    )
  }
  return cookie.value
}
