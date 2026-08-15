import path from 'node:path'

import { type APIRequestContext, type Locator, type Page } from '@playwright/test'

import { adminEmail, adminPassword, seedAdmin, signIn } from '../harness/auth'
import { repoRoot } from '../harness/paths'
import { expect, test, type SeedCommand } from '../harness/test'

/**
 * The engagement status lifecycle, driven by the buttons that move it.
 *
 * This spec exists because the transition was broken everywhere at once and
 * nothing caught it: app.engagement(status) was indexed, DuckDB rewrites an
 * UPDATE of an indexed column into DELETE + INSERT, and the RESTRICT foreign
 * keys pointing at the engagement refused the delete half. Creating an
 * engagement seats its lead, so every engagement had a referencing row and
 * every transition answered 500. The Go tests hold the API to the state
 * machine; this one holds the screens to it, which is where it was noticed.
 */

const attackSourceID = '01900000-0000-7000-8000-000000000001'
const attackFixture = path.join(
  repoRoot,
  'internal/content/attack/testdata/enterprise-mini-15.1.json',
)

function seedWithAttack(): SeedCommand[] {
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
  ]
}

test.use({ seed: { steps: seedWithAttack() } })

interface IDObject {
  id: string
}

interface SessionCookies {
  cookie: string
  csrfToken: string
}

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

/**
 * Create an engagement through the API. It lands in draft with the caller
 * seated as its lead — the referencing row the bug tripped over — and the spec
 * then walks it forward through the UI.
 */
async function createEngagement(
  request: APIRequestContext,
  s: SessionCookies,
  name: string,
): Promise<string> {
  const resp = await request.post('/api/v1/engagements', {
    headers: { cookie: s.cookie, 'x-csrf-token': s.csrfToken, 'content-type': 'application/json' },
    data: {
      name,
      client: 'E2E Client',
      description: 'Status lifecycle automated test',
      startsOn: '2026-01-01',
      endsOn: '2026-06-01',
      attackVersion: '15.1',
      mode: 'standard',
      autoRevealOnStart: false,
    },
  })
  const body = await resp.text()
  expect(resp.status(), `create engagement: ${body}`).toBe(201)
  return (JSON.parse(body) as IDObject).id
}

/**
 * The badge beside the engagement's name in the page header, which renders the
 * status verbatim in lower case — the row as the server last returned it,
 * rather than the button that was clicked.
 */
function headerStatus(page: Page): Locator {
  return page.getByText(/^(draft|active|closed|archived)$/)
}

test('the overview page walks an engagement from draft to archived', async ({ page, request }) => {
  const admin = await login(request, adminEmail, adminPassword)
  const engagementId = await createEngagement(request, admin, 'Status Lifecycle E2E')

  await signIn(page)
  await page.goto(`/engagements/${engagementId}`)

  await expect(page.getByRole('heading', { name: 'Status Lifecycle E2E' })).toBeVisible()
  await expect(headerStatus(page)).toHaveText('draft')

  // Draft → active. The badge is the assertion: a 500 left the status where it
  // was while a toast reported the failure.
  await page.getByRole('button', { name: 'Activate' }).click()
  await expect(headerStatus(page)).toHaveText('active')

  // Active → closed.
  await page.getByRole('button', { name: 'Close' }).click()
  await expect(headerStatus(page)).toHaveText('closed')

  // Closed → archived. Reachable from this page at all only since the closed
  // gate was lifted here: archiving is the way *out* of closed.
  await page.getByRole('button', { name: 'Archive' }).click()
  await expect(headerStatus(page)).toHaveText('archived')

  // Archived is terminal, so the page offers nowhere else to go.
  await expect(page.getByText('Transition to:')).toHaveCount(0)

  // And it survives a reload, which is the difference between the row moving
  // and the cache moving.
  await page.reload()
  await expect(headerStatus(page)).toHaveText('archived')
})

test('the settings page moves the status too', async ({ page, request }) => {
  const admin = await login(request, adminEmail, adminPassword)
  const engagementId = await createEngagement(request, admin, 'Status Settings E2E')

  await signIn(page)
  await page.goto(`/engagements/${engagementId}/settings`)

  await expect(headerStatus(page)).toHaveText('draft')
  await page.getByRole('button', { name: 'Activate' }).click()
  await expect(headerStatus(page)).toHaveText('active')
})
