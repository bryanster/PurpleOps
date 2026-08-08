import type { Page } from '@playwright/test'

import type { SeedCommand } from './test'

/**
 * Getting a browser signed in, for the specs whose subject is something else.
 *
 * Every screen but the sign-in ones needs a session as of M1-017, so a spec
 * about the version screen now has to sign in first. That is one line here
 * rather than four in each file — and it goes through the real form, because a
 * helper that reached past the interface would stop noticing the day the
 * interface broke.
 */

/** The account [seedAdmin] creates, and the credentials [signIn] defaults to. */
export const adminEmail = 'harness-admin@example.test'
export const adminPassword = 'a harness administrator passphrase'

/**
 * Seed steps that leave one administrator in the database.
 *
 * `migrate up` comes first because seeding writes rows and the server has not
 * started yet — DuckDB admits one writer at a time, so a seed that writes has
 * to migrate for itself.
 */
export function seedAdmin(): SeedCommand[] {
  return [
    ['migrate', 'up'],
    {
      args: ['user', 'create', '--email', adminEmail, '--name', 'Harness Admin', '--admin'],
      stdin: adminPassword,
    },
  ]
}

/** Sign in through the form, and leave the browser wherever that lands. */
export async function signIn(
  page: Page,
  email: string = adminEmail,
  password: string = adminPassword,
): Promise<void> {
  await page.goto('/login')
  await page.getByLabel('Email address').fill(email)
  await page.getByLabel('Password').fill(password)
  await page.getByRole('button', { name: 'Sign in' }).click()
}
