import path from 'node:path'

import type { Page } from '@playwright/test'

import { adminEmail, adminPassword, seedAdmin, signIn } from '../harness/auth'
import { repoRoot } from '../harness/paths'
import { expect, test, type SeedCommand } from '../harness/test'

/**
 * Sources admin UI (M2-014).
 *
 * Seeds ATT&CK offline via blctl (same mini fixture as the library e2e) so the
 * page can show licenses, item counts, and custom's non-deletable rule without
 * depending on a live network fetch. Component tests cover the job-slot gate
 * and API error surfaces with MSW.
 */

const memberEmail = 'mel-sources@example.test'
const memberPassword = 'a sources member passphrase'

const attackSourceID = '01900000-0000-7000-8000-000000000001'
const atomicSourceID = '01900000-0000-7000-8000-000000000002'

const attackFixture = path.join(
  repoRoot,
  'internal/content/attack/testdata/enterprise-mini-15.1.json',
)
const atomicFixture = path.join(repoRoot, 'internal/content/atomic/testdata/atomics-mini.zip')

function seedSourcesAdmin(): SeedCommand[] {
  return [
    ...seedAdmin(),
    {
      args: ['user', 'create', '--email', memberEmail, '--name', 'Mel Sources'],
      stdin: memberPassword,
    },
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
    ['content', 'enable', '--id', atomicSourceID],
    ['content', 'import-bundle', '--source', 'atomic', '--file', atomicFixture, '--wait'],
  ]
}

test.use({ seed: { steps: seedSourcesAdmin() } })

async function openSources(page: Page): Promise<void> {
  await page.getByRole('link', { name: 'Content sources' }).click()
  await expect(page).toHaveURL(/\/admin\/content\/sources$/)
  await expect(page.getByRole('heading', { name: 'Content sources', level: 1 })).toBeVisible()
}

test('a member is refused the sources admin route', async ({ page }) => {
  await signIn(page, memberEmail, memberPassword)
  await expect(page.getByRole('navigation', { name: 'Sections' })).toBeVisible()

  await page.goto('/admin/content/sources')
  await expect(page.getByRole('heading', { name: 'Not yours to see' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Content sources' })).toHaveCount(0)
})

test('an administrator sees sources, licenses, and cannot delete custom', async ({ page }) => {
  await signIn(page, adminEmail, adminPassword)
  await expect(page.getByRole('navigation', { name: 'Sections' })).toBeVisible()
  await openSources(page)

  const attackRow = page.getByRole('row', { name: /MITRE ATT&CK Enterprise/ })
  await expect(attackRow).toBeVisible()
  await expect(attackRow.getByText('Enabled')).toBeVisible()
  // Seeded mini fixture has at least one technique object.
  await expect(attackRow.getByRole('cell', { name: /^[1-9]\d*$/ })).toBeVisible()

  await page.getByRole('button', { name: 'MITRE ATT&CK Enterprise' }).click()
  const detail = page.getByRole('dialog')
  await expect(detail.getByText('Apache-2.0')).toBeVisible()
  await expect(detail.getByText(/MITRE Corporation/i)).toBeVisible()
  await expect(detail.getByText('15.1')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(detail).toBeHidden()

  const customRow = page.getByRole('row', { name: /Custom content/ })
  await expect(customRow.getByRole('button', { name: 'Delete' })).toHaveCount(0)
  await expect(attackRow.getByRole('button', { name: 'Delete' })).toBeVisible()

  // Live mutation: disable then re-enable Atomic without a network sync.
  const atomicRow = page.getByRole('row', { name: /Atomic Red Team/ })
  await atomicRow.getByRole('button', { name: 'Disable' }).click()
  const confirm = page.getByRole('alertdialog')
  await expect(confirm).toContainText('leave browse, search, and pickers')
  await confirm.getByRole('button', { name: 'Disable source' }).click()
  await expect(page.getByText(/is disabled/i)).toBeVisible()
  await expect(atomicRow.getByText('Disabled')).toBeVisible()

  await atomicRow.getByRole('button', { name: 'Enable' }).click()
  await expect(page.getByText(/is enabled/i)).toBeVisible()
  await expect(atomicRow.getByText('Enabled')).toBeVisible()
})
