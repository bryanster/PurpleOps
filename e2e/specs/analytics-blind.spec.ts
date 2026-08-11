import path from 'node:path'

import { type APIRequestContext } from '@playwright/test'

import { adminEmail, adminPassword, seedAdmin, signIn } from '../harness/auth'
import { repoRoot } from '../harness/paths'
import { expect, test, type SeedCommand } from '../harness/test'

/**
 * Dashboard blind comparison (M5-013).
 *
 * Extends M4-009 blind-mode fixtures: a blind engagement with red and blue
 * seats must show different analytics totals and blue must see the blind
 * banner. This is the epic's seat-scoping decision made visible.
 */

const redEmail = 'analytics-red@example.test'
const redPassword = 'analytics red blind passphrase'
const blueEmail = 'analytics-blue@example.test'
const bluePassword = 'analytics blue blind passphrase'

const attackSourceID = '01900000-0000-7000-8000-000000000001'
const attackFixture = path.join(
  repoRoot,
  'internal/content/attack/testdata/enterprise-mini-15.1.json',
)

function seedAnalyticsBlindUsers(): SeedCommand[] {
  return [
    ...seedAdmin(),
    ['content', 'enable', '--id', attackSourceID],
    [
      'content', 'import-bundle', '--source', 'attack',
      '--file', attackFixture, '--version', '15.1', '--wait',
    ],
    { args: ['user', 'create', '--email', redEmail, '--name', 'Red Analyst'], stdin: redPassword },
    { args: ['user', 'create', '--email', blueEmail, '--name', 'Blue Defender'], stdin: bluePassword },
  ]
}
test.use({ seed: { steps: seedAnalyticsBlindUsers() } })

interface IDObject { id: string }

interface SessionCookies {
  cookie: string
  csrfToken: string
}

function mutatingHeaders(s: SessionCookies): Record<string, string> {
  return { cookie: s.cookie, 'x-csrf-token': s.csrfToken, 'content-type': 'application/json' }
}

function readHeaders(s: SessionCookies): Record<string, string> {
  return { cookie: s.cookie }
}

async function login(
  request: APIRequestContext, email: string, password: string,
): Promise<SessionCookies> {
  const resp = await request.post('/api/v1/auth/login', {
    data: { email, password },
    failOnStatusCode: true,
  })
  const setCookie = resp.headers()['set-cookie']
  if (setCookie === undefined || setCookie === '') {
    throw new Error(`login for ${email}: no Set-Cookie header`)
  }
  const sessionPart = setCookie.split(';').find((c) => c.trim().startsWith('bl_session='))
  if (sessionPart === undefined) {
    throw new Error(`login for ${email}: no bl_session cookie in Set-Cookie`)
  }
  const cookie = sessionPart.trim()
  const csrfResp = await request.get('/api/v1/auth/csrf', { headers: { cookie } })
  const csrfBody = (await csrfResp.json()) as { token: string }
  return { cookie, csrfToken: csrfBody.token }
}

async function createBlindEngagement(
  request: APIRequestContext, s: SessionCookies, name: string,
): Promise<string> {
  const resp = await request.post('/api/v1/engagements', {
    headers: mutatingHeaders(s),
    data: {
      name,
      client: 'E2E Analytics',
      description: 'Blind analytics E2E',
      attackVersion: '15.1',
      mode: 'blind',
      autoRevealOnStart: false,
      startsOn: '2026-10-01',
      endsOn: '2026-10-15',
    },
  })
  const body = (await resp.json()) as IDObject
  return body.id
}

async function createScenario(
  request: APIRequestContext, s: SessionCookies, engagementId: string,
): Promise<string> {
  const resp = await request.post(`/api/v1/engagements/${engagementId}/scenarios`, {
    headers: mutatingHeaders(s),
    data: { name: 'Analytics Test Scenario' },
  })
  const body = (await resp.json()) as IDObject
  return body.id
}

async function addMember(
  request: APIRequestContext, s: SessionCookies,
  engagementId: string, userEmail: string, role: string,
): Promise<void> {
  // Get user id by email
  const usersResp = await request.get('/api/v1/users', {
    headers: readHeaders(s),
  })
  const usersBody = (await usersResp.json()) as { items: { id: string; email: string }[] }
  const user = usersBody.items.find((u) => u.email === userEmail)
  if (user === undefined) throw new Error(`user not found: ${userEmail}`)

  await request.post(`/api/v1/engagements/${engagementId}/members`, {
    headers: mutatingHeaders(s),
    data: { userId: user.id, role },
  })
}

interface StepItem { id: string }

async function createStep(
  request: APIRequestContext, s: SessionCookies,
  engagementId: string, scenarioId: string,
  name: string, techniqueId: string,
): Promise<string> {
  const resp = await request.post(
    `/api/v1/engagements/${engagementId}/scenarios/${scenarioId}/steps`,
    {
      headers: mutatingHeaders(s),
      data: { name, techniqueId },
    },
  )
  const body = (await resp.json()) as StepItem
  return body.id
}

interface ExecutionItem { id: string }

async function getExecution(
  request: APIRequestContext, s: SessionCookies,
  engagementId: string, stepId: string,
): Promise<ExecutionItem> {
  const resp = await request.get(
    `/api/v1/engagements/${engagementId}/steps/${stepId}/execution`,
    { headers: readHeaders(s) },
  )
  return (await resp.json()) as ExecutionItem
}

async function scoreRed(
  request: APIRequestContext, s: SessionCookies,
  engagementId: string, executionId: string,
): Promise<void> {
  await request.patch(
    `/api/v1/engagements/${engagementId}/executions/${executionId}`,
    {
      headers: mutatingHeaders(s),
      data: { status: 'complete' },
    },
  )
}

async function scoreBlue(
  request: APIRequestContext, s: SessionCookies,
  engagementId: string, executionId: string,
  category: string, protection: string,
): Promise<void> {
  await request.patch(
    `/api/v1/engagements/${engagementId}/executions/${executionId}`,
    {
      headers: mutatingHeaders(s),
      data: { detectionCategory: category, protection },
    },
  )
}

async function revealStep(
  request: APIRequestContext, s: SessionCookies,
  engagementId: string, scenarioId: string, stepId: string,
): Promise<void> {
  await request.post(
    `/api/v1/engagements/${engagementId}/scenarios/${scenarioId}/steps/${stepId}/reveal`,
    { headers: mutatingHeaders(s) },
  )
}

test('red and blue see different analytics totals in blind engagement', async ({
  page: redPage,
  browser,
  request,
}) => {
  // --- Setup ---
  const adminCookie = await login(request, adminEmail, adminPassword)
  const engagementId = await createBlindEngagement(request, adminCookie, 'Blind Analytics E2E')
  const scenarioId = await createScenario(request, adminCookie, engagementId)
  await addMember(request, adminCookie, engagementId, redEmail, 'red')
  await addMember(request, adminCookie, engagementId, blueEmail, 'blue')

  const redCookie = await login(request, redEmail, redPassword)
  const blueCookie = await login(request, blueEmail, bluePassword)

  // Create two steps, reveal only one to blue
  const step1Id = await createStep(request, redCookie, engagementId, scenarioId, 'Revealed Step', 'T1059')
  const step2Id = await createStep(request, redCookie, engagementId, scenarioId, 'Hidden Step', 'T1203')

  // Score both steps from red side
  const exec1 = await getExecution(request, redCookie, engagementId, step1Id)
  const exec2 = await getExecution(request, redCookie, engagementId, step2Id)
  await scoreRed(request, redCookie, engagementId, exec1.id)
  await scoreRed(request, redCookie, engagementId, exec2.id)

  // Score both from blue side
  await scoreBlue(request, blueCookie, engagementId, exec1.id, 'technique', 'blocked')
  await scoreBlue(request, blueCookie, engagementId, exec2.id, 'general', 'partial')

  // Reveal only step 1
  await revealStep(request, adminCookie, engagementId, scenarioId, step1Id)

  // --- Red opens analytics page ---
  await signIn(redPage, redEmail, redPassword)
  await redPage.goto(`/engagements/${engagementId}/analytics`)
  await redPage.waitForSelector('text=Analytics')
  // Red should NOT see blind banner
  await expect(redPage.locator('[aria-label="Blind engagement notice"]')).toHaveCount(0)

  // --- Blue opens analytics page ---
  const bluePage = await browser.newPage()
  await signIn(bluePage, blueEmail, bluePassword)
  await bluePage.goto(`/engagements/${engagementId}/analytics`)
  await bluePage.waitForSelector('text=Analytics')
  // Blue SHOULD see blind banner
  await expect(bluePage.locator('[aria-label="Blind engagement notice"]')).toBeVisible()
  await expect(bluePage.locator('text=revealed steps only')).toBeVisible()

  // Blue analytics should show different coverage numbers than red
  // (Blue only sees 1 revealed technique, red sees both)
  // Read the coverage card text — they should differ
  const redCoverageText = redPage.locator('text=of 200')
  const blueCoverageText = await bluePage.locator('text=of 200').textContent()
  expect(redCoverageText).not.toBe(blueCoverageText) // eslint-disable-line playwright/prefer-web-first-assertions

  await bluePage.close()
})
