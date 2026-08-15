import { seedFirstRunAdmin, signIn } from '../harness/auth'
import { expect, test } from '../harness/test'

/**
 * First-run setup, from the very first sign-in.
 *
 * Every other spec seeds `setup complete` and starts inside the product. This
 * one deliberately does not, so it meets what an operator actually meets: a
 * fresh database, one administrator, and an empty content library.
 *
 * Whether MITRE answers depends on where the suite is running — a laptop has a
 * route out, CI's sandbox may not — and the screen is written for both. So the
 * assertion is that it resolves to one of its two honest states: a picker with
 * releases in it, or a notice saying upstream could not be reached and pointing
 * at the offline bundle. Asserting only one would be a spec that fails on a
 * train, or one that passes without ever seeing the picker. What is asserted
 * unconditionally is the part that has nothing to do with the network: this is
 * where the first sign-in lands, and finishing it is final.
 *
 * Installing is not walked here. It is a real multi-megabyte download of
 * somebody else's server, and the component tests cover the enable-then-sync
 * order against a fixture.
 */
test.use({ seed: { steps: seedFirstRunAdmin() } })

test('the first sign-in lands on the wizard, and finishing it is not repeated', async ({
  page,
}) => {
  await signIn(page)

  // Not the version screen the other specs land on: an installation nobody has
  // set up sends its administrators here first.
  await expect(page).toHaveURL(/\/setup$/)
  await expect(page.getByRole('heading', { name: /MITRE ATT&CK version/ })).toBeVisible()

  // Either releases to choose from, or the reason there are none. Never a
  // spinner that never resolves, and never an empty box with no explanation.
  const picker = page.getByRole('radio').first()
  const offline = page.getByText(/could not be reached/)
  await expect(picker.or(offline)).toBeVisible()
  // The offline route is on the screen whichever state it is in.
  await expect(page.getByText(/offline bundle/).first()).toBeVisible()

  // And there is a way through it.
  await page.getByRole('button', { name: 'Skip for now' }).click()

  await expect(page).toHaveURL(/\/system\/version$/)
  await expect(page.getByRole('navigation', { name: 'Sections' })).toBeVisible()

  // Finished means finished: a reload lands in the product, and typing the
  // wizard's address goes nowhere.
  await page.reload()
  await expect(page).toHaveURL(/\/system\/version$/)

  await page.goto('/setup')
  await expect(page).not.toHaveURL(/\/setup$/)
})
