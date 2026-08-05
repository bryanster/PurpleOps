import { seedAdmin, signIn } from '../harness/auth'
import { reportedBuild } from '../harness/build-info'
import { fieldValue } from '../harness/locators'
import { expect, test } from '../harness/test'

/**
 * The one spec M0b has: the binary serves an application, and the application
 * can reach the API it was served by.
 *
 * PLAN.md §9 describes the spec this suite is being built for — content
 * install, an engagement, two rounds, a report and a share link. It arrives a
 * milestone at a time. What matters now is that the scaffolding underneath it
 * is honest: this run started a real server, on a real database, and would have
 * failed loudly if it had not.
 *
 * It signs in first because as of M1-017 there is nothing to look at without a
 * session — which is itself worth having a spec walk into, since "the shell
 * renders" and "the shell renders for anybody" are different claims.
 */
test.use({ seed: { steps: seedAdmin() } })

test('the shell renders and the version screen agrees with the binary', async ({ page }) => {
  await signIn(page)

  await expect(page.getByRole('link', { name: 'Blacklight' })).toBeVisible()
  await expect(page.getByRole('navigation', { name: 'Sections' })).toBeVisible()

  // The root redirects; M2–M6 give the product a real landing screen and this
  // becomes a redirect to that instead.
  await expect(page).toHaveURL(/\/system\/version$/)
  await expect(page.getByRole('heading', { name: 'Version', level: 1 })).toBeVisible()

  // The whole point. These only agree if the ldflags reached the binary, the
  // binary embedded this build of the SPA, and the generated client read the
  // generated server's answer — which is four tickets' worth of plumbing that
  // nothing else checks end to end.
  const build = await reportedBuild()
  await expect(fieldValue(page, 'Version')).toHaveText(build.version)
  await expect(fieldValue(page, 'Commit')).toHaveText(build.commit)
  await expect(fieldValue(page, 'Built')).toHaveText(build.buildDate)
})

test('the nav reaches the health screen, and it reports a live database', async ({ page }) => {
  await signIn(page)

  await page.getByRole('link', { name: 'Health' }).click()

  await expect(page.getByRole('heading', { name: 'Health', level: 1 })).toBeVisible()
  // `ok` for the database means the server opened the temp DuckDB file this
  // spec was given and can query it — not merely that the process is up.
  await expect(fieldValue(page, 'Overall')).toHaveText('ok')
  await expect(fieldValue(page, 'Database')).toHaveText('ok')
})
