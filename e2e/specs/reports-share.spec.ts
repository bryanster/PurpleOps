import path from 'node:path'

import { type APIRequestContext } from '@playwright/test'

import { repoRoot } from '../harness/paths'
import { expect, test, type SeedCommand } from '../harness/test'

/**
 * Publish → share → view → revoke end-to-end (M6-014).
 *
 * Thin precursor to M6-015 full E2E thesis. Exercises the critical
 * path: lead publishes, creates a share, another user views it, and
 * after revoke the view returns 404.
 */

const adminEmail = 'publish-lead@example.test'
const adminPassword = 'admin publish passphrase'
const viewerEmail = 'guest-viewer@example.test'
const viewerPassword = 'guest viewer passphrase'

const attackSourceID = '01900000-0000-7000-8000-000000000001'
const attackFixture = path.join(
  repoRoot,
  'internal/content/attack/testdata/enterprise-mini-15.1.json',
)

function seedSteps(): SeedCommand[] {
  return [
    ['migrate', 'up'],
    ['content', 'enable', '--id', attackSourceID],
    {
      args: ['user', 'create', '--email', adminEmail, '--name', 'Publish Lead', '--admin'],
      stdin: adminPassword,
    },
    {
      args: ['user', 'create', '--email', viewerEmail, '--name', 'Guest Viewer'],
      stdin: viewerPassword,
    },
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

test.use({ seed: { steps: seedSteps() } })

interface SessionCookies {
  cookie: string
  csrfToken: string
}

function mutatingHeaders(s: SessionCookies): Record<string, string> {
  return { cookie: s.cookie, 'x-csrf-token': s.csrfToken, 'content-type': 'application/json' }
}

async function apiLogin(
  request: APIRequestContext,
  email: string,
  password: string,
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
  const body = (await resp.json()) as { user?: { csrfToken?: string } }
  const csrfToken = body.user?.csrfToken ?? ''
  const cookie = `${sessionPart.trim()}; bl_csrf=${csrfToken}`
  return { cookie, csrfToken }
}

test('publish creates a share, viewer can access, revoke returns 404', async ({
  page,
  browser,
  request,
}) => {
  // ── Lead signs in via API (source of truth for cookies + CSRF) ────────────
  const lead = await apiLogin(request, adminEmail, adminPassword)

  // ── Create an engagement via API (avoids ATT&CK version Select timing) ─────
  const engResp = await request.post('/api/v1/engagements', {
    headers: mutatingHeaders(lead),
    data: {
      name: 'Share Test Engagement',
      client: 'E2E Share',
      description: 'Share E2E',
      attackVersion: '15.1',
      mode: 'standard',
      autoRevealOnStart: false,
    },
  })
  if (!engResp.ok()) throw new Error(`create engagement: ${String(engResp.status())}`)
  const engagementId = ((await engResp.json()) as { id: string }).id

  // ── Lead signs in via UI and navigates to the engagement ───────────────────
  await page.goto('/login')
  await page.getByLabel('Email address').fill(adminEmail)
  await page.getByLabel('Password').fill(adminPassword)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByRole('navigation', { name: 'Sections' })).toBeVisible()

  await page.goto(`/engagements/${engagementId}`)
  await expect(page.getByRole('heading', { name: 'Share Test Engagement' })).toBeVisible()

  // Navigate to reports tab
  await page.getByRole('link', { name: 'Reports' }).click()

  // Create a report
  await page.getByRole('button', { name: 'New report' }).first().click()
  await page.getByLabel('Title').fill('Share Test Report')
  await page.getByRole('button', { name: 'Create' }).click()
  await expect(page.getByText('Share Test Report')).toBeVisible()

  // ── Publish ────────────────────────────────────────────────────────────────
  await page.getByRole('button', { name: 'Publish' }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.getByLabel('Include evidence files').uncheck()
  const publishDialog = page.getByRole('dialog')
  await publishDialog.getByRole('button', { name: 'Publish' }).click()
  await expect(page.getByText('Published version')).toBeVisible()

  // ── Create share link ──────────────────────────────────────────────────────
  await page.getByRole('button', { name: 'Create share link' }).click()
  const shareDialog = page.getByRole('dialog')
  await shareDialog.getByLabel('Label (optional)').fill('E2E test share')
  await shareDialog.getByRole('button', { name: 'Create' }).click()
  const shareItem = page.getByText('E2E test share')
  await expect(shareItem).toBeVisible()
  const shareUrl = await shareItem.locator('a').getAttribute('href')
  if (!shareUrl) throw new Error('share URL not found')

  // ── Viewer signs in and claims ─────────────────────────────────────────────
  const viewerCtx = await browser.newContext()
  const viewerPage = await viewerCtx.newPage()
  await viewerPage.goto(shareUrl)
  await viewerPage.getByLabel('Email address').fill(viewerEmail)
  await viewerPage.getByLabel('Password').fill(viewerPassword)
  await viewerPage.getByRole('button', { name: 'Sign in' }).click()
  await expect(viewerPage.getByText('Share Test Report')).toBeVisible()

  // ── Revoke from lead page ──────────────────────────────────────────────────
  await page.getByRole('button', { name: 'Revoke' }).click()
  await expect(page.getByText('E2E test share')).not.toBeVisible()

  // ── Viewer reload → 404 ────────────────────────────────────────────────────
  await viewerPage.reload()
  await expect(viewerPage.getByText('Not found')).toBeVisible()
  await viewerCtx.close()
})
