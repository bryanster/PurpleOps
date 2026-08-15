import type { Page } from '@playwright/test'

import { expect, test } from '../harness/test'
import { totpCode } from '../harness/totp'

/**
 * M1's identity story, through the browser (M1-017).
 *
 * `mfa-enforcement.spec.ts` drives the same server through its API because
 * there was no interface when it was written. This is the other half: the same
 * rules, reached the way a person reaches them, against a real binary and a
 * real DuckDB file. Both are worth having — one proves the middleware, the
 * other proves that a person can get through it.
 *
 * The steps are the ones PLAN.md §9 and this ticket name:
 *
 *   1. `blctl` creates the first administrator, who signs in.
 *   2. That administrator creates a member, who is forced through enrolment.
 *   3. The member is denied the administration screens, in the UI and at the API.
 *   4. The administrator ends the member's sessions; their next action lands on
 *      the sign-in screen.
 *   5. A service token is created, used, revoked, and shown to fail afterwards.
 *
 * The passwords are in this file on purpose: they belong to accounts in a
 * database created for this spec file and deleted at teardown.
 */

const adminEmail = 'ada@example.test'
const adminPassword = 'an administrator passphrase'

const memberEmail = 'mel@example.test'
const memberPassword = 'a member passphrase entirely'

test.use({
  seed: {
    steps: [
      ['migrate', 'up'],
      {
        args: ['user', 'create', '--email', adminEmail, '--name', 'Ada Lovelace', '--admin'],
        stdin: adminPassword,
      },
      // Past its first run: this spec is about identity, and an installation
      // that has never been set up would put the wizard in front of every
      // sign-in here (see setup.spec.ts, which is about that).
      ['setup', 'complete'],
    ],
  },
})

/**
 * Sign in through the form. The accounts here are this file's own — the
 * harness's shared administrator would do for the steps that only need *a*
 * session, but this spec is about who signs in and what happens next.
 */
async function signIn(page: Page, email: string, password: string): Promise<void> {
  await page.goto('/login')
  await page.getByLabel('Email address').fill(email)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Sign in' }).click()
}

test('an administrator signs in, and the first thing they see is the application', async ({
  page,
}) => {
  await signIn(page, adminEmail, adminPassword)

  // Landed inside the shell rather than on the login screen.
  await expect(page.getByRole('navigation', { name: 'Sections' })).toBeVisible()
  await expect(page.getByRole('button', { name: `Account: Ada Lovelace` })).toBeVisible()

  // The administration entries are there for an administrator.
  await expect(page.getByRole('link', { name: 'Users' })).toBeVisible()
  await expect(page.getByRole('link', { name: 'Activity' })).toBeVisible()
})

test('a wrong password says nothing about whether the account exists', async ({ page }) => {
  await signIn(page, adminEmail, 'not the right password')
  const wrongPassword = await page.getByRole('alert').innerText()

  await signIn(page, 'nobody@example.test', adminPassword)
  const unknownAddress = await page.getByRole('alert').innerText()

  // Identical, including the request id line being present or absent — the
  // sign-in form must not become a way to find out who has an account here.
  expect(unknownAddress.replace(/Request \S+/, '')).toBe(wrongPassword.replace(/Request \S+/, ''))
  await expect(page).toHaveURL(/\/login$/)
})

test('the whole identity flow, from an empty installation to a revoked token', async ({
  page,
  browser,
  baseURL,
}) => {
  // ── 1. The administrator signs in ────────────────────────────────────────
  await signIn(page, adminEmail, adminPassword)
  await expect(page.getByRole('navigation', { name: 'Sections' })).toBeVisible()

  // ── 2. …and creates a member who must hold a second factor ───────────────
  await page.getByRole('link', { name: 'Users' }).click()
  await expect(page.getByRole('heading', { name: 'Users', level: 1 })).toBeVisible()

  await page.getByRole('button', { name: 'New user' }).click()
  const createDialog = page.getByRole('dialog')
  await createDialog.getByLabel('Email address').fill(memberEmail)
  await createDialog.getByLabel('Display name').fill('Mel Chen')
  await createDialog.getByLabel('Password', { exact: true }).fill(memberPassword)
  await createDialog.getByRole('checkbox', { name: /Require a second factor/ }).click()
  await createDialog.getByRole('button', { name: 'Create account' }).click()

  // The invite link is the answer, because there is no mail transport (M1-016).
  await expect(createDialog.getByText(/\/login$/)).toBeVisible()
  await createDialog.getByRole('button', { name: 'Done' }).click()
  await expect(page.getByRole('row', { name: /Mel Chen/ })).toBeVisible()

  // The member signs in, in a browser of their own.
  const memberContext = await browser.newContext({ baseURL })
  const memberPage = await memberContext.newPage()
  await signIn(memberPage, memberEmail, memberPassword)

  // Forced enrolment: a session that may do exactly one thing (M1-008).
  await expect(
    memberPage.getByRole('heading', { name: 'Set up two-factor authentication' }),
  ).toBeVisible()

  // …and no way past it by typing an address. This is the assertion the ticket
  // asks for by name.
  for (const address of ['/', '/settings/account', '/admin/users']) {
    await memberPage.goto(address)
    await expect(memberPage).toHaveURL(/\/login\/enrol$/)
    await expect(memberPage.getByRole('navigation', { name: 'Sections' })).toHaveCount(0)
  }

  // Enrol for real: the secret is on the screen, so the spec can produce a code
  // the way an authenticator would.
  const secret = await memberPage.locator('code').first().innerText()
  await memberPage.getByLabel('Six-digit code from your authenticator').fill(totpCode(secret))

  // The recovery codes, shown once, behind a deliberate acknowledgement.
  await expect(memberPage.getByRole('list', { name: 'Recovery codes' })).toBeVisible()
  const proceed = memberPage.getByRole('button', { name: 'Continue to Blacklight' })
  await expect(proceed).toBeDisabled()
  await memberPage.getByRole('checkbox').click()
  await proceed.click()

  await expect(memberPage.getByRole('navigation', { name: 'Sections' })).toBeVisible()

  // ── 3. The member is denied administration, in the UI and at the API ─────
  await expect(memberPage.getByRole('link', { name: 'Users' })).toHaveCount(0)

  await memberPage.goto('/admin/users')
  await expect(memberPage.getByRole('heading', { name: 'Not yours to see' })).toBeVisible()

  // Hiding a link is not access control, so the API is asked the same question.
  const asMember = await memberPage.request.get('/api/v1/users')
  expect(asMember.status()).toBe(403)

  // ── 4. The administrator ends the member's sessions ──────────────────────
  await page.reload()
  const memberRow = page.getByRole('row', { name: /Mel Chen/ })
  await memberRow.getByRole('button', { name: 'Sign out' }).click()

  const confirmation = page.getByRole('alertdialog')
  await expect(confirmation).toContainText('Service tokens they own are not sessions')
  await confirmation.getByRole('button', { name: 'Sign them out' }).click()

  // The member's next action lands on the sign-in screen rather than on a
  // screen full of failed requests.
  await memberPage.goto('/settings/account')
  await expect(memberPage).toHaveURL(/\/login/)
  await memberContext.close()

  // ── 5. A service token: created, used, revoked, refused ──────────────────
  await page.getByRole('link', { name: 'Service tokens' }).click()
  await page.getByRole('button', { name: 'New token' }).click()

  const tokenDialog = page.getByRole('dialog')
  await tokenDialog.getByLabel('Name').fill('an end-to-end token')
  await tokenDialog.getByRole('checkbox', { name: /Read administration/ }).click()
  await tokenDialog.getByRole('button', { name: 'Create token' }).click()

  // The secret, in the only response that will ever carry it.
  //
  // Waiting for the heading first is not a nicety: until the request comes back
  // the dialog is still the creation form, whose scope descriptions are also
  // <code> elements — reading one of those would "pass" this step with a value
  // that is not a token, and fail two lines later for the wrong reason.
  await expect(tokenDialog.getByRole('heading', { name: 'Copy your token now' })).toBeVisible()
  const token = ((await tokenDialog.locator('code').first().textContent()) ?? '').trim()
  expect(token).not.toBe('')

  await tokenDialog.getByRole('checkbox', { name: /I have saved this token/ }).click()
  await tokenDialog.getByRole('button', { name: 'Done' }).click()

  // It works, from something that is not a browser and holds no cookie.
  const bearer = await browser.newContext({ baseURL })
  const withToken = await bearer.request.get('/api/v1/users', {
    headers: { Authorization: `Bearer ${token}` },
  })
  expect(withToken.status()).toBe(200)

  // Revoked through the interface…
  await page
    .getByRole('row', { name: /an end-to-end token/ })
    .getByRole('button', { name: 'Revoke' })
    .click()
  const revokeConfirmation = page.getByRole('alertdialog')
  await expect(revokeConfirmation).toContainText('stops working at its next request')
  await revokeConfirmation.getByRole('button', { name: 'Revoke token' }).click()
  await expect(page.getByRole('row', { name: /an end-to-end token/ })).toContainText('revoked')

  // …and refused at its very next request, with no cached copy anywhere.
  const afterRevocation = await bearer.request.get('/api/v1/users', {
    headers: { Authorization: `Bearer ${token}` },
  })
  expect(afterRevocation.status()).toBe(401)
  await bearer.close()
})
