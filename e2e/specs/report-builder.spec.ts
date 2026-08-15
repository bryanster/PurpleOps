import path from 'node:path'

import { type APIRequestContext } from '@playwright/test'

import { repoRoot } from '../harness/paths'
import { expect, test, type SeedCommand } from '../harness/test'

/**
 * The report builder page, driven the way a user drives it: add blocks, write
 * content, save, preview.
 *
 * reports-share.spec.ts covers publish → share → view, and thesis-report.spec.ts
 * covers the API path. Neither touches the builder UI, which is where the two
 * loops a user actually spends their time live — arranging blocks and seeing
 * what they render as.
 */

const adminEmail = 'builder-lead@example.test'
const adminPassword = 'builder lead passphrase entirely'

const attackSourceID = '01900000-0000-7000-8000-000000000001'
const attackFixture = path.join(
  repoRoot,
  'internal/content/attack/testdata/enterprise-mini-15.1.json',
)

function seedSteps(): SeedCommand[] {
  return [
    ['migrate', 'up'],
    ['setup', 'complete'],
    ['content', 'enable', '--id', attackSourceID],
    {
      args: ['user', 'create', '--email', adminEmail, '--name', 'Builder Lead', '--admin'],
      stdin: adminPassword,
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

interface Sess {
  cookie: string
  csrf: string
}

async function apiLogin(request: APIRequestContext): Promise<Sess> {
  const resp = await request.post('/api/v1/auth/login', {
    data: { email: adminEmail, password: adminPassword },
    failOnStatusCode: true,
  })
  const raw = resp.headers()['set-cookie']
  if (raw === undefined || raw === '') throw new Error('login: no Set-Cookie')
  const lines = raw.split('\n')
  const val = (prefix: string): string => {
    const ln = lines.find((c) => c.trim().startsWith(`${prefix}=`))
    if (ln === undefined) throw new Error(`login: no ${prefix} cookie`)
    const first = ln.trim().split(';')[0]
    if (first === undefined) throw new Error(`login: malformed ${prefix} cookie`)
    const value = first.split('=')[1]
    if (value === undefined) throw new Error(`login: malformed ${prefix} cookie`)
    return value
  }
  return {
    cookie: `bl_session=${val('bl_session')}; bl_csrf=${val('bl_csrf')}`,
    csrf: val('bl_csrf'),
  }
}

function mh(s: Sess): Record<string, string> {
  return { cookie: s.cookie, 'x-csrf-token': s.csrf, 'content-type': 'application/json' }
}

test('report builder: add blocks, save, and preview', async ({ page, request }) => {
  const s = await apiLogin(request)

  const engResp = await request.post('/api/v1/engagements', {
    headers: mh(s),
    data: {
      name: 'Builder Engagement',
      client: 'Builder Co',
      description: 'E2E',
      attackVersion: '15.1',
      mode: 'standard',
      startsOn: '2026-10-01',
      endsOn: '2026-10-15',
    },
    failOnStatusCode: true,
  })
  const engId = ((await engResp.json()) as { id: string }).id

  const repResp = await request.post(`/api/v1/engagements/${engId}/reports`, {
    headers: mh(s),
    data: { title: 'Builder Report' },
    failOnStatusCode: true,
  })
  const repId = ((await repResp.json()) as { id: string }).id

  // Sign the browser in through the real form, then open the builder.
  await page.goto('/login')
  await page.getByLabel('Email').fill(adminEmail)
  await page.getByLabel('Password').fill(adminPassword)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await page.waitForURL((url) => !url.pathname.startsWith('/login'))

  await page.goto(`/engagements/${engId}/reports/${repId}`)
  await expect(page.getByRole('heading', { name: 'Builder Report' })).toBeVisible()

  // ── Add blocks from the palette ───────────────────────────────────────────
  // Evidence appendix is here because its own server-side default (`limit: 50`)
  // used to come back as a parameter the validator refused, so a report
  // containing it could be saved once and never again.
  const palette = page.getByRole('heading', { name: 'Add block' }).locator('..')
  await palette.getByRole('button', { name: 'Cover', exact: true }).click()
  await palette.getByRole('button', { name: 'Rich text' }).click()
  await palette.getByRole('button', { name: 'Evidence appendix' }).click()

  await page.locator('.ProseMirror').fill('Builder narrative paragraph.')

  // ── Save ──────────────────────────────────────────────────────────────────
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await expect(page.getByText('Blocks saved')).toBeVisible()

  // The save must survive a reload: this is what the user comes back to.
  await page.reload()
  await expect(page.locator('.ProseMirror')).toContainText('Builder narrative paragraph.')

  // ── Preview ───────────────────────────────────────────────────────────────
  await page.getByRole('button', { name: 'Preview' }).click()
  const frame = page.frameLocator('iframe[title="Report preview"]')
  await expect(frame.getByText('Builder narrative paragraph.')).toBeVisible()

  // ── Save again ────────────────────────────────────────────────────────────
  // Every save after the first sends back the params the server itself stored,
  // defaults included. That round trip is what broke.
  await page.locator('.ProseMirror').fill('Revised narrative paragraph.')
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await expect(page.getByText('Blocks saved')).toBeVisible()
  await expect(frame.getByText('Revised narrative paragraph.')).toBeVisible()
})

// The reports list: what it says about each report, and whether its Delete
// button works. Both were broken in the product — the row measured a `blocks`
// array the list response does not carry, so every report read "0 blocks", and
// Delete hit a DuckDB foreign-key limitation on any report that had a block,
// which is every report anybody had opened.
test('reports list: block counts are real, and Delete removes the report', async ({
  page,
  request,
}) => {
  const s = await apiLogin(request)

  const engResp = await request.post('/api/v1/engagements', {
    headers: mh(s),
    data: {
      name: 'List Engagement',
      client: 'List Co',
      description: 'E2E',
      attackVersion: '15.1',
      mode: 'standard',
      startsOn: '2026-10-01',
      endsOn: '2026-10-15',
    },
    failOnStatusCode: true,
  })
  const engId = ((await engResp.json()) as { id: string }).id

  const withBlocks = await request.post(`/api/v1/engagements/${engId}/reports`, {
    headers: mh(s),
    data: { title: 'Has Blocks' },
    failOnStatusCode: true,
  })
  const withBlocksId = ((await withBlocks.json()) as { id: string }).id

  await request.post(`/api/v1/engagements/${engId}/reports`, {
    headers: mh(s),
    data: { title: 'Stays Empty' },
    failOnStatusCode: true,
  })

  await request.put(`/api/v1/engagements/${engId}/reports/${withBlocksId}/blocks`, {
    headers: mh(s),
    data: {
      blocks: [
        { blockId: 'cover', params: {} },
        { blockId: 'rich_text', params: { html: '<p>Narrative.</p>' } },
      ],
    },
    failOnStatusCode: true,
  })

  await page.goto('/login')
  await page.getByLabel('Email').fill(adminEmail)
  await page.getByLabel('Password').fill(adminPassword)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await page.waitForURL((url) => !url.pathname.startsWith('/login'))

  await page.goto(`/engagements/${engId}/reports`)

  // Each count is read from its own row, so a per-row number cannot pass by
  // being right somewhere else on the page.
  const rowOf = (title: string) => page.getByRole('link', { name: title }).locator('..')
  await expect(rowOf('Has Blocks').getByText('2 blocks')).toBeVisible()
  await expect(rowOf('Stays Empty').getByText('0 blocks')).toBeVisible()

  // Delete the one that has blocks — the case the foreign keys refused.
  await page.getByRole('button', { name: 'Delete Has Blocks' }).click()
  const confirm = page.getByRole('alertdialog')
  await expect(confirm).toBeVisible()
  await confirm.getByRole('button', { name: 'Delete' }).click()

  // The dialog closes only when the mutation reports success, and the row goes
  // only when the refreshed list no longer carries it.
  await expect(confirm).toBeHidden()
  await expect(page.getByRole('link', { name: 'Has Blocks' })).toHaveCount(0)
  await expect(page.getByRole('link', { name: 'Stays Empty' })).toBeVisible()

  // And it is gone on the server, not just off the screen.
  await page.reload()
  await expect(page.getByRole('link', { name: 'Has Blocks' })).toHaveCount(0)
})
