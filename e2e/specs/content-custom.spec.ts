import fs from 'node:fs/promises'
import path from 'node:path'
import type { Page } from '@playwright/test'

import { adminEmail, adminPassword, seedAdmin, signIn } from '../harness/auth'
import { repoRoot } from '../harness/paths'
import { expect, test, type SeedCommand } from '../harness/test'

/**
 * Custom content editor + v1 import UI (M2-015).
 *
 * Admin authors a procedure with two input args, imports the v1 testcases
 * fixture through the wizard (dry-run then confirm), and a member sees the
 * imported template in the library without create/import chrome.
 */

const memberEmail = 'mel-custom@example.test'
const memberPassword = 'a custom content member passphrase'

const testcasesFixture = path.join(repoRoot, 'internal/content/testdata/v1import/testcases.json')

function seedCustomContent(): SeedCommand[] {
  return [
    ...seedAdmin(),
    {
      args: ['user', 'create', '--email', memberEmail, '--name', 'Mel Custom'],
      stdin: memberPassword,
    },
  ]
}

test.use({ seed: { steps: seedCustomContent() } })

async function openCustom(page: Page): Promise<void> {
  await page.getByRole('link', { name: 'Custom content' }).click()
  await expect(page).toHaveURL(/\/admin\/content\/custom$/)
  await expect(page.getByRole('heading', { name: 'Custom content', level: 1 })).toBeVisible()
}

test('a member cannot reach custom content admin and has no import controls in the library', async ({
  page,
}) => {
  await signIn(page, memberEmail, memberPassword)
  await expect(page.getByRole('navigation', { name: 'Sections' })).toBeVisible()

  await expect(page.getByRole('link', { name: 'Custom content' })).toHaveCount(0)
  await page.goto('/admin/content/custom')
  await expect(page.getByRole('heading', { name: 'Not yours to see' })).toBeVisible()

  await page.getByRole('link', { name: 'Content' }).click()
  await expect(page).toHaveURL(/\/content$/)
  await expect(page.getByRole('button', { name: 'Import…' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'New procedure' })).toHaveCount(0)
  await expect(page.getByRole('link', { name: 'Import v1 testcases' })).toHaveCount(0)
})

test('admin creates a procedure with two args and imports v1 fixture for members', async ({
  page,
}) => {
  await signIn(page, adminEmail, adminPassword)
  await expect(page.getByRole('navigation', { name: 'Sections' })).toBeVisible()
  await openCustom(page)

  // --- Create procedure with two input args ---
  await page.getByRole('button', { name: 'New procedure' }).first().click()
  const createDialog = page.getByRole('dialog')
  await expect(createDialog.getByRole('heading', { name: 'New procedure template' })).toBeVisible()

  await createDialog.getByLabel('Name').fill('Custom two-arg echo')
  await createDialog.getByLabel('Technique ids').fill('T1059.004')
  await createDialog.getByLabel('Platforms').fill('linux')
  await createDialog.getByLabel('Executor').fill('sh')
  await createDialog.getByLabel('Command').fill('echo "#{first}" "#{second}"')

  await createDialog.getByRole('button', { name: 'Add argument' }).click()
  await createDialog.getByRole('button', { name: 'Add argument' }).click()

  const argNames = createDialog.getByLabel('Name')
  // index 0 = procedure name; 1 and 2 = args
  await argNames.nth(1).fill('first')
  await argNames.nth(2).fill('second')
  await createDialog.getByLabel('Default').nth(0).fill('alpha')
  await createDialog.getByLabel('Default').nth(1).fill('beta')
  await createDialog.getByLabel('Description').nth(1).fill('First arg')
  await createDialog.getByLabel('Description').nth(2).fill('Second arg')

  await createDialog.getByRole('button', { name: 'Create procedure' }).click()
  await expect(createDialog).toBeHidden()

  await expect(page.getByRole('button', { name: 'Custom two-arg echo' })).toBeVisible()
  await page.getByRole('button', { name: 'Custom two-arg echo' }).click()
  const detail = page.getByRole('dialog')
  await expect(detail.getByRole('cell', { name: 'first', exact: true })).toBeVisible()
  await expect(detail.getByRole('cell', { name: 'second', exact: true })).toBeVisible()
  await expect(detail.getByRole('cell', { name: 'alpha', exact: true })).toBeVisible()
  await expect(detail.getByRole('cell', { name: 'beta', exact: true })).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(detail).toBeHidden()

  // --- Import wizard: dry-run then confirm ---
  await page.getByRole('button', { name: 'Import…' }).click()
  const wizard = page.getByRole('dialog')
  await expect(wizard.getByRole('heading', { name: 'Import custom content' })).toBeVisible()

  await wizard.getByLabel('File').setInputFiles(testcasesFixture)
  await wizard.getByRole('button', { name: 'Dry-run preview' }).click()

  await expect(wizard.getByText('dry-run', { exact: true })).toBeVisible()
  // Fixture has two testcases → proceduresCreated 2.
  await expect(wizard.getByText('Procedures created')).toBeVisible()
  await expect(wizard.getByText('2', { exact: true }).first()).toBeVisible()

  await wizard.getByRole('button', { name: 'Confirm import' }).click()
  await expect(wizard.getByRole('button', { name: 'Done' })).toBeVisible()
  await wizard.getByRole('button', { name: 'Done' }).click()
  await expect(wizard).toBeHidden()

  // Imported v1 names should appear on the procedures tab.
  await expect(page.getByRole('button', { name: /Service Execution via sc\.exe/i })).toBeVisible()
  await expect(
    page.getByRole('button', { name: /Dump LSASS Memory Using Task Manager/i }),
  ).toBeVisible()

  // --- Export downloads a non-empty file ---
  const downloadPromise = page.waitForEvent('download')
  await page.getByRole('button', { name: 'Export YAML' }).click()
  const download = await downloadPromise
  expect(download.suggestedFilename()).toMatch(/\.ya?ml$/i)
  const downloadPath = await download.path()
  expect(downloadPath).toBeTruthy()
  if (!downloadPath) throw new Error('Download path is null')
  const stat = await fs.stat(downloadPath)
  expect(stat.size).toBeGreaterThan(0)

  // --- Member sees imported + authored templates in the library, no write chrome ---
  await page.getByRole('button', { name: /^Account:/ }).click()
  await page.getByRole('menuitem', { name: 'Sign out' }).click()
  await expect(page).toHaveURL(/\/login/)

  await signIn(page, memberEmail, memberPassword)
  await page.getByRole('link', { name: 'Content' }).click()
  await expect(page).toHaveURL(/\/content$/)

  await page.getByRole('tab', { name: 'Procedures' }).click()
  await expect(page.getByRole('button', { name: 'Custom two-arg echo' })).toBeVisible()
  await expect(page.getByRole('button', { name: /Service Execution via sc\.exe/i })).toBeVisible()

  await expect(page.getByRole('button', { name: 'New procedure' })).toHaveCount(0)
  await expect(page.getByRole('button', { name: 'Import…' })).toHaveCount(0)
  await expect(page.getByRole('link', { name: 'Custom content' })).toHaveCount(0)
})
