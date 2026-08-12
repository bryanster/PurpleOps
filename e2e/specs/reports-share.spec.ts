import path from 'node:path'

import { repoRoot } from '../harness/paths'
import { expect, test } from '../harness/test'

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

const attackFixture = path.join(
  repoRoot,
  'internal/content/attack/testdata/enterprise-mini-15.1.json',
)

test.use({
  seed: {
    steps: [
      ['migrate', 'up'],
      {
        args: ['user', 'create', '--email', adminEmail, '--name', 'Publish Lead', '--admin'],
        stdin: adminPassword,
      },
      {
        args: ['user', 'create', '--email', viewerEmail, '--name', 'Guest Viewer'],
        stdin: viewerPassword,
      },
      ['content', 'import-bundle', '--source', 'attack', '--file', attackFixture, '--version', '15.1', '--wait'],
    ],
  },
})

test('publish creates a share, viewer can access, revoke returns 404', async ({
  page,
  browser,
}) => {
  // ── Lead signs in ──────────────────────────────────────────────────────────
  await page.goto('/login')
  await page.getByLabel('Email address').fill(adminEmail)
  await page.getByLabel('Password').fill(adminPassword)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByRole('navigation', { name: 'Sections' })).toBeVisible()

  // ── Create engagement via page.evaluate (has auth cookies after sign-in) ───
  const engagementId = await page.evaluate(async (data) => {
    const csrfToken = document.cookie.split('; ').find((c) => c.startsWith('bl_csrf='))?.split('=')[1] ?? ''
    const resp = await fetch('/api/v1/engagements', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-CSRF-Token': csrfToken,
      },
      body: JSON.stringify(data),
    })
    if (!resp.ok) throw new Error(`create engagement: ${String(resp.status)}`)
    const body = (await resp.json()) as { id: string }
    return body.id
  }, { name: 'Share Test Engagement', attackVersion: '15.1', mode: 'standard' })

  // Navigate to the new engagement.
  await page.goto(`/engagements/${engagementId}`)
  await expect(page.getByRole('heading', { name: 'Share Test Engagement' })).toBeVisible()

  // Navigate to reports tab
  await page.getByRole('link', { name: 'Reports' }).click()

  // Create a report
  await page.getByRole('button', { name: 'New report' }).click()
  await page.getByLabel('Title').fill('Share Test Report')
  await page.getByRole('button', { name: 'Create' }).click()
  await expect(page.getByText('Share Test Report')).toBeVisible()

  // ── Publish ────────────────────────────────────────────────────────────────
  await page.getByRole('button', { name: 'Publish' }).click()

  // Dialog visible — evidence default off
  await expect(page.getByRole('dialog')).toBeVisible()
  await page.getByLabel('Include evidence files').uncheck()

  // Publish
  const publishDialog = page.getByRole('dialog')
  await publishDialog.getByRole('button', { name: 'Publish' }).click()
  await expect(page.getByText('Published version')).toBeVisible()

  // ── Create share link ──────────────────────────────────────────────────────
  await page.getByRole('button', { name: 'Create share link' }).click()

  const shareDialog = page.getByRole('dialog')
  await shareDialog.getByLabel('Label (optional)').fill('E2E test share')
  await shareDialog.getByRole('button', { name: 'Create' }).click()

  // The share link appears in the list
  const shareItem = page.getByText('E2E test share')
  await expect(shareItem).toBeVisible()

  // Extract the share URL
  const linkElement = shareItem.locator('a')
  const shareUrl = await linkElement.getAttribute('href')
  if (!shareUrl) throw new Error('share URL not found')

  // ── Viewer signs in and claims ─────────────────────────────────────────────
  const viewerCtx = await browser.newContext()
  const viewerPage = await viewerCtx.newPage()

  await viewerPage.goto(shareUrl)
  await viewerPage.getByLabel('Email address').fill(viewerEmail)
  await viewerPage.getByLabel('Password').fill(viewerPassword)
  await viewerPage.getByRole('button', { name: 'Sign in' }).click()

  // After sign-in, the share redirects to the view page
  await expect(viewerPage.getByText('Share Test Report')).toBeVisible()

  // ── Revoke from lead page ──────────────────────────────────────────────────
  // eslint-disable-next-line playwright/prefer-web-first-assertions
  await page.getByRole('button', { name: 'Revoke' }).click()
  await expect(page.getByText('E2E test share')).not.toBeVisible()

  // ── Viewer reload → 404 ────────────────────────────────────────────────────
  await viewerPage.reload()
  await expect(viewerPage.getByText('Not found')).toBeVisible()

  await viewerCtx.close()
})
