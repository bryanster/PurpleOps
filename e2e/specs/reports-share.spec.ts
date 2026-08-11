import path from 'node:path'
import { repoRoot } from '../harness/paths'

/**
 * Publish → share → view → revoke end-to-end (M6-014).
 *
 * Thin precursor to M6-015 full E2E thesis. Exercises the critical
 * path: lead publishes, creates a share, another user views it, and
 * after revoke the view returns 404.
 */

const adminEmail = 'publish-lead@example.test'
const adminPassword = 'admin publish passphrase'

const attackFixture = path.join(
  repoRoot,
  'internal/content/attack/testdata/enterprise-mini-15.1.json',
)
const viewerEmail = 'guest-viewer@example.test'
const viewerPassword = 'guest viewer passphrase'

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

  // ── Create an engagement (needed for reports) ──────────────────────────────
  await page.getByRole('link', { name: 'Engagements' }).click()
  await page.getByRole('button', { name: 'New engagement' }).click()
  await page.getByLabel('Name').fill('Share Test Engagement')
  // Select ATT&CK version (required field).
  await page.getByLabel('ATT&CK version').click()
  await page.getByRole('option', { name: '15.1' }).click()
  await page.getByRole('button', { name: 'Create' }).click()
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
  const evidenceCheckbox = page.getByLabel('Include evidence')
  await expect(evidenceCheckbox).not.toBeChecked()

  // Publish
  await page.getByRole('button', { name: 'Publish' }).last().click()
  await expect(page.getByText('Report published')).toBeVisible()

  // ── Open versions panel and create a share ─────────────────────────────────
  await page.getByRole('button', { name: 'Versions' }).click()
  await expect(page.getByRole('dialog')).toBeVisible()
  await expect(page.getByText(/v1/)).toBeVisible()

  // Create share on the version
  await page.getByRole('button', { name: 'Share' }).click()
  await page.getByRole('button', { name: 'Create share link' }).click()

  // Fill share form
  const shareDialog = page.getByRole('dialog')
  await shareDialog.getByLabel('Label (optional)').fill('E2E test share')
  await shareDialog.getByRole('button', { name: 'Create' }).click()

  // Get the claim URL from the one-time display
  const claimUrlElement = shareDialog.locator('code')
  await expect(claimUrlElement).toHaveText(/\/claim\//)
  const claimUrl = (await claimUrlElement.textContent()) ?? ''

  // ── Viewer signs in and claims ─────────────────────────────────────────────
  const viewerContext = await browser.newContext()
  const viewerPage = await viewerContext.newPage()

  await viewerPage.goto('/login')
  await viewerPage.getByLabel('Email address').fill(viewerEmail)
  await viewerPage.getByLabel('Password').fill(viewerPassword)
  await viewerPage.getByRole('button', { name: 'Sign in' }).click()
  await expect(viewerPage.getByRole('navigation', { name: 'Sections' })).toBeVisible()

  // Extract path from absolute URL
  const url = new URL(claimUrl)
  await viewerPage.goto(url.pathname)

  // Claim the share
  await viewerPage.getByRole('button', { name: /claim access/i }).click()

  // Should redirect to HTML view
  await expect(viewerPage.locator('iframe[title="Shared report"]')).toBeVisible({
    timeout: 10000,
  })

  // ── Lead revokes the share ─────────────────────────────────────────────────
  // Back in the lead's page, revoke the share
  await page.getByRole('button', { name: /revoke share/i }).click()
  await expect(page.getByText('Share revoked')).toBeVisible()
  const claimURL = new URL(claimUrl)
  await viewerPage.goto(claimURL.pathname)
  // The share info endpoint returns a 404, which the claim page renders as "Report not found"
  await expect(viewerPage.getByText(/report not found/i)).toBeVisible()

  await viewerContext.close()
})
