import path from 'node:path'

import type { Page } from '@playwright/test'

import { adminEmail, adminPassword, seedAdmin, signIn } from '../harness/auth'
import { repoRoot } from '../harness/paths'
import { expect, test, type SeedCommand } from '../harness/test'

/**
 * Content library browser (M2-013).
 *
 * Seeds mini ATT&CK + Atomic fixtures through `blctl content import-bundle`
 * before the server boots, then walks the member path: sign in, open Content,
 * search a technique, open a procedure. The fixtures are the same ones the
 * adapter unit tests use — no network.
 */

const memberEmail = 'mel-content@example.test'
const memberPassword = 'a content member passphrase'

/** Builtin seed ids from migration 0011_content.sql. */
const attackSourceID = '01900000-0000-7000-8000-000000000001'
const atomicSourceID = '01900000-0000-7000-8000-000000000002'

const attackFixture = path.join(
  repoRoot,
  'internal/content/attack/testdata/enterprise-mini-15.1.json',
)
const atomicFixture = path.join(repoRoot, 'internal/content/atomic/testdata/atomics-mini.zip')

function seedContentLibrary(): SeedCommand[] {
  return [
    ...seedAdmin(),
    {
      args: ['user', 'create', '--email', memberEmail, '--name', 'Mel Content'],
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

test.use({ seed: { steps: seedContentLibrary() } })

async function openContent(page: Page): Promise<void> {
  await page.getByRole('link', { name: 'Content library' }).click()
  await expect(page).toHaveURL(/\/content$/)
  await expect(page.getByRole('heading', { name: 'Content', level: 1 })).toBeVisible()
}

test('a member browses a technique and a procedure from seeded fixtures', async ({ page }) => {
  await signIn(page, memberEmail, memberPassword)
  await openContent(page)

  // Techniques tab is default once ATT&CK is installed.
  await expect(page.getByRole('tab', { name: 'Techniques' })).toBeVisible()
  await page.getByLabel('Search').fill('T1059')
  await expect(
    page.getByRole('row', { name: /T1059 Command and Scripting Interpreter/ }),
  ).toBeVisible()

  await page.getByRole('button', { name: 'Command and Scripting Interpreter' }).click()
  const techniqueDialog = page.getByRole('dialog')
  await expect(techniqueDialog).toBeVisible()
  await expect(techniqueDialog.getByText(/ATT&CK 15\.1/)).toBeVisible()
  await expect(techniqueDialog.getByText('TA0002')).toBeVisible()
  await page.keyboard.press('Escape')
  await expect(techniqueDialog).toBeHidden()

  await page.getByRole('tab', { name: 'Procedures' }).click()
  await expect(page.getByRole('button', { name: /PowerShell Echo Input/i })).toBeVisible()
  await page.getByRole('button', { name: /PowerShell Echo Input/i }).click()

  const procedureDialog = page.getByRole('dialog')
  await expect(procedureDialog.getByRole('heading', { name: 'Command' })).toBeVisible()
  await expect(procedureDialog.getByRole('heading', { name: 'Cleanup' })).toBeVisible()
  await expect(procedureDialog.getByText(/Write-Host/)).toBeVisible()
  await expect(procedureDialog.getByText(/Remove-Item/)).toBeVisible()

  // Members never see enable/sync on this surface.
  await expect(page.getByRole('button', { name: /sync/i })).toHaveCount(0)
  await expect(page.getByRole('button', { name: /enable/i })).toHaveCount(0)
})

test('use in scenario carries a technique from the library into a workbook', async ({ page }) => {
  const engagementName = 'Library Handoff E2E'
  await signIn(page, adminEmail, adminPassword)

  // Somewhere to import into, with a scenario for the step to land in.
  await page.getByRole('link', { name: 'Engagements' }).click()
  // The header button and the empty-state button say the same thing.
  await page.getByRole('button', { name: 'New engagement' }).first().click()
  const createDialog = page.getByRole('dialog')
  await createDialog.getByLabel('Name').fill(engagementName)
  await createDialog.getByLabel('ATT&CK version').click()
  await page.getByRole('option', { name: /15\.1/ }).click()
  await createDialog.getByRole('button', { name: 'Create' }).click()
  await expect(createDialog).toBeHidden()

  await page.getByRole('link', { name: engagementName }).click()
  await page.getByRole('link', { name: 'Workbook' }).click()
  await page.getByRole('button', { name: 'Add Scenario' }).click()
  const scenarioDialog = page.getByRole('dialog')
  await scenarioDialog.getByPlaceholder('Scenario name').fill('Initial Access')
  await scenarioDialog.getByRole('button', { name: 'Create' }).click()
  await expect(page.getByText('Initial Access')).toBeVisible()

  // The library end: pick the technique, then say where it goes.
  await openContent(page)
  await page.getByLabel('Search').fill('T1059')
  await page.getByRole('button', { name: 'Command and Scripting Interpreter' }).click()
  const techniqueDialog = page.getByRole('dialog')
  await techniqueDialog.getByRole('button', { name: 'Use in scenario' }).click()

  const handoffDialog = page
    .getByRole('dialog')
    .filter({ hasText: 'Use technique in an engagement' })
  await handoffDialog.getByRole('combobox').click()
  await page.getByRole('option', { name: new RegExp(engagementName) }).click()
  await handoffDialog.getByRole('button', { name: 'Continue' }).click()

  // The workbook takes over: Add Step, already carrying the technique.
  await expect(page).toHaveURL(/\/workbook/)
  const addStepDialog = page.getByRole('dialog').filter({ hasText: 'Add Step' })
  await expect(addStepDialog.getByPlaceholder('e.g. T1059')).toHaveValue('T1059')
  await addStepDialog.getByPlaceholder('Step name').fill('Run an interpreter')
  await addStepDialog.getByRole('button', { name: 'Create' }).click()

  await expect(addStepDialog).toBeHidden()
  // The hand-off params are consumed, so the dialog cannot come back.
  await expect(page).toHaveURL(/\/workbook$/)
  await page.getByText('Initial Access').click()
  await expect(page.getByText('Run an interpreter')).toBeVisible()
})

test('an administrator reaches the library without member empty-state copy', async ({ page }) => {
  await signIn(page, adminEmail, adminPassword)
  await openContent(page)
  await expect(page.getByText('Ask an admin to install ATT&CK.')).toHaveCount(0)
  await expect(page.getByRole('tab', { name: 'Techniques' })).toBeVisible()
  await expect(
    page.getByRole('row', { name: /T1059 Command and Scripting Interpreter/ }),
  ).toBeVisible()
})
