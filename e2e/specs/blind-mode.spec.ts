import path from 'node:path'

import { type APIRequestContext } from '@playwright/test'

import { adminEmail, adminPassword, seedAdmin, signIn } from '../harness/auth'
import { repoRoot } from '../harness/paths'
import { expect, test, type SeedCommand } from '../harness/test'

/**
 * Blind mode end-to-end (M4-009).
 *
 * Verifies that blue team members in a blind engagement cannot see unrevealed
 * steps — not through REST, not through SSE, not through presence focus.
 */

const redEmail = 'red-blind@example.test'
const redPassword = 'a red blind passphrase'
const blueEmail = 'blue-blind@example.test'
const bluePassword = 'a blue blind passphrase'

const attackSourceID = '01900000-0000-7000-8000-000000000001'
const attackFixture = path.join(
  repoRoot,
  'internal/content/attack/testdata/enterprise-mini-15.1.json',
)

function seedBlindUsers(): SeedCommand[] {
  return [
    ...seedAdmin(),
    ['content', 'enable', '--id', attackSourceID],
    [
      'content',
      'import-bundle',
      '--source',
      'attack',
      '--file',
      attackFixture,
      '--version',
      '15.1',
      '--wait',
    ],
    {
      args: ['user', 'create', '--email', redEmail, '--name', 'Red Lead'],
      stdin: redPassword,
    },
    {
      args: ['user', 'create', '--email', blueEmail, '--name', 'Blue Team'],
      stdin: bluePassword,
    },
  ]
}
test.use({ seed: { steps: seedBlindUsers() } })

interface IDObject {
  id: string
}

interface StepItem {
  id: string
  name: string
}

interface StepList {
  items: StepItem[]
}

interface PresenceEntry {
  userId: string
  focus?: {
    stepId?: string | null
    executionId?: string | null
  } | null
}

interface PresenceResponse {
  entries: PresenceEntry[]
}

interface SessionCookies {
  /** Cookie header value with both bl_session and bl_csrf. */
  cookie: string
  /** Raw CSRF token value for the X-CSRF-Token header. */
  csrfToken: string
}

/** Headers for state-changing requests (POST, PUT, DELETE). */
function mutatingHeaders(s: SessionCookies): Record<string, string> {
  return { cookie: s.cookie, 'x-csrf-token': s.csrfToken, 'content-type': 'application/json' }
}

/** Headers for read-only requests (GET). */
function readHeaders(s: SessionCookies): Record<string, string> {
  return { cookie: s.cookie }
}

/** Create a blind engagement via API, return its id. */
async function createBlindEngagement(
  request: APIRequestContext,
  s: SessionCookies,
  name: string,
): Promise<string> {
  const resp = await request.post('/api/v1/engagements', {
    headers: mutatingHeaders(s),
    data: {
      name,
      client: 'E2E Client',
      description: 'Blind mode automated test',
      status: 'active',
      startsOn: '2026-01-01',
      endsOn: '2026-06-01',
      attackVersion: '15.1',
      mode: 'blind',
      autoRevealOnStart: false,
    },
  })
  expect(resp.status()).toBe(201)
  return ((await resp.json()) as IDObject).id
}

/** Create a scenario in an engagement, return its id. */
async function createScenario(
  request: APIRequestContext,
  s: SessionCookies,
  engagementId: string,
): Promise<string> {
  const resp = await request.post(`/api/v1/engagements/${engagementId}/scenarios`, {
    headers: mutatingHeaders(s),
    data: { name: 'Scenario 1', ordinal: 0 },
  })
  expect(resp.status()).toBe(201)
  return ((await resp.json()) as IDObject).id
}

/** Add a member to an engagement. */
async function addMember(
  request: APIRequestContext,
  s: SessionCookies,
  engagementId: string,
  userEmail: string,
  role: string,
): Promise<void> {
  const usersResp = await request.get('/api/v1/admin/users', {
    headers: readHeaders(s),
  })
  expect(usersResp.status()).toBe(200)
  const users = (await usersResp.json()) as { items: { id: string; email: string }[] }
  const user = users.items.find((u) => u.email === userEmail)
  if (!user) throw new Error(`user ${userEmail} not found`)

  const resp = await request.post(`/api/v1/engagements/${engagementId}/members`, {
    headers: mutatingHeaders(s),
    data: { userId: user.id, role },
  })
  expect(resp.status()).toBe(201)
}

/** Sign in and return session + CSRF cookies for API requests. */
async function login(
  request: APIRequestContext,
  email: string,
  password: string,
): Promise<SessionCookies> {
  const resp = await request.post('/api/v1/auth/login', {
    headers: { 'content-type': 'application/json' },
    data: { email, password },
  })
  expect(resp.status()).toBe(200)
  // Playwright joins multiple Set-Cookie headers with \n.
  const raw = resp.headers()['set-cookie']
  if (!raw) throw new Error('no set-cookie header')
  const lines = raw.split('\n')
  const sessionLine = lines.find((l) => l.startsWith('bl_session='))
  if (!sessionLine) throw new Error('no bl_session cookie')
  const csrfLine = lines.find((l) => l.startsWith('bl_csrf='))
  if (!csrfLine) throw new Error('no bl_csrf cookie')
  const sessionPair = sessionLine.split(';')[0] ?? ''
  const csrfPair = csrfLine.split(';')[0] ?? ''
  return {
    cookie: `${sessionPair}; ${csrfPair}`,
    csrfToken: csrfPair.replace('bl_csrf=', ''),
  }
}

/** Create a step via API, return its id. */
async function createStep(
  request: APIRequestContext,
  s: SessionCookies,
  engagementId: string,
  scenarioId: string,
  name: string,
): Promise<string> {
  const resp = await request.post(
    `/api/v1/engagements/${engagementId}/scenarios/${scenarioId}/steps`,
    {
      headers: mutatingHeaders(s),
      data: {
        name,
        objective: 'Test objective',
        techniqueId: 'T1059.004',
        procedure: {},
      },
    },
  )
  const bodyText = await resp.text()
  expect(resp.status(), `create step "${name}": ${bodyText}`).toBe(201)
  return (JSON.parse(bodyText) as IDObject).id
}

/** Reveal a step to the blue team. */
async function revealStep(
  request: APIRequestContext,
  s: SessionCookies,
  engagementId: string,
  scenarioId: string,
  stepId: string,
): Promise<void> {
  const resp = await request.post(
    `/api/v1/engagements/${engagementId}/scenarios/${scenarioId}/steps/${stepId}/reveal`,
    { headers: mutatingHeaders(s) },
  )
  const bodyText = await resp.text()
  expect(resp.status(), `reveal step: ${bodyText}`).toBe(200)
}

test('blue cannot see unrevealed steps in blind engagement, then sees after reveal', async ({
  page,
  request,
}) => {
  // --- Setup: admin creates blind engagement with red+blue ---
  const adminCookie = await login(request, adminEmail, adminPassword)
  const engagementId = await createBlindEngagement(request, adminCookie, 'Blind Reveal E2E')
  const scenarioId = await createScenario(request, adminCookie, engagementId)
  await addMember(request, adminCookie, engagementId, redEmail, 'red')
  await addMember(request, adminCookie, engagementId, blueEmail, 'blue')

  // Red signs in and creates an unrevealed step.
  const redCookie = await login(request, redEmail, redPassword)
  const stepId = await createStep(request, redCookie, engagementId, scenarioId, 'Hidden Step')

  // --- Blue signs in and tries to access the step ---
  await signIn(page, blueEmail, bluePassword)
  await expect(page.getByRole('navigation', { name: 'Sections' })).toBeVisible()

  // Blue GETs the step via API — should get 404.
  const blueCookie = await login(request, blueEmail, bluePassword)
  const blueStepResp = await request.get(
    `/api/v1/engagements/${engagementId}/scenarios/${scenarioId}/steps/${stepId}`,
    { headers: readHeaders(blueCookie) },
  )
  expect(
    blueStepResp.status(),
    `blue GET unrevealed step should be 404, got ${String(blueStepResp.status())}: ${await blueStepResp.text()}`,
  ).toBe(404)

  // --- Red reveals the step ---
  await revealStep(request, redCookie, engagementId, scenarioId, stepId)

  // --- Blue can now see the step ---
  const blueStepAfterResp = await request.get(
    `/api/v1/engagements/${engagementId}/scenarios/${scenarioId}/steps/${stepId}`,
    { headers: readHeaders(blueCookie) },
  )
  expect(
    blueStepAfterResp.status(),
    `blue GET revealed step should be 200, got ${String(blueStepAfterResp.status())}: ${await blueStepAfterResp.text()}`,
  ).toBe(200)

  const stepBody = (await blueStepAfterResp.json()) as StepItem
  expect(stepBody.name).toBe('Hidden Step')
})

test('blue presence focus stripped for unrevealed steps in blind engagement', async ({
  request,
}) => {
  // --- Setup ---
  const adminCookie = await login(request, adminEmail, adminPassword)
  const engagementId = await createBlindEngagement(request, adminCookie, 'Blind Presence E2E')
  const scenarioId = await createScenario(request, adminCookie, engagementId)
  await addMember(request, adminCookie, engagementId, redEmail, 'red')
  await addMember(request, adminCookie, engagementId, blueEmail, 'blue')

  // Red creates an unrevealed step.
  const redCookie = await login(request, redEmail, redPassword)
  const stepId = await createStep(request, redCookie, engagementId, scenarioId, 'Hidden Step')

  // Red sets presence with focus on the unrevealed step.
  const presenceId = crypto.randomUUID()
  await request.put(`/api/v1/engagements/${engagementId}/presence?presenceId=${presenceId}`, {
    headers: mutatingHeaders(redCookie),
    data: {
      presenceId,
      focus: { stepId },
    },
  })

  // Blue GETs presence — should see red online but focus stripped.
  const blueCookie = await login(request, blueEmail, bluePassword)
  const presenceResp = await request.get(`/api/v1/engagements/${engagementId}/presence`, {
    headers: readHeaders(blueCookie),
  })
  expect(presenceResp.status()).toBe(200)
  const presence = (await presenceResp.json()) as PresenceResponse
  expect(presence.entries).toHaveLength(1)
  const entry = presence.entries[0]
  if (!entry) throw new Error('Expected first presence entry')
  expect(entry.userId).toBeTruthy()
  // Focus must be absent or nil for blue in blind mode.
  expect(entry.focus?.stepId ?? null).toBeNull()
})

test('blue cannot list unrevealed steps from engagement steps endpoint', async ({ request }) => {
  // --- Setup ---
  const adminCookie = await login(request, adminEmail, adminPassword)
  const engagementId = await createBlindEngagement(request, adminCookie, 'Blind List E2E')
  const scenarioId = await createScenario(request, adminCookie, engagementId)
  await addMember(request, adminCookie, engagementId, redEmail, 'red')
  await addMember(request, adminCookie, engagementId, blueEmail, 'blue')

  // Red creates an unrevealed step.
  const redCookie = await login(request, redEmail, redPassword)
  await createStep(request, redCookie, engagementId, scenarioId, 'Hidden Step')

  // Blue lists engagement steps — should be empty.
  const blueCookie = await login(request, blueEmail, bluePassword)
  const stepsResp = await request.get(`/api/v1/engagements/${engagementId}/steps`, {
    headers: readHeaders(blueCookie),
  })
  expect(stepsResp.status()).toBe(200)
  const stepsBody = (await stepsResp.json()) as StepList
  expect(stepsBody.items).toHaveLength(0)

  // Red lists steps — should see the step.
  const redStepsResp = await request.get(`/api/v1/engagements/${engagementId}/steps`, {
    headers: readHeaders(redCookie),
  })
  expect(redStepsResp.status()).toBe(200)
  const redStepsBody = (await redStepsResp.json()) as StepList
  expect(redStepsBody.items).toHaveLength(1)
})
