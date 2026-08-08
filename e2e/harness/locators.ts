import type { Locator, Page } from '@playwright/test'

/**
 * The value beside a label, in the two-column tables the system screens use.
 *
 * Queried by role and by the label the user actually reads. Nothing here knows
 * about class names: the UI is Tailwind, so a class chain is a list of styling
 * decisions, and a spec pinned to those breaks every time somebody adjusts a
 * margin — while still passing when the value it was supposed to check is
 * wrong.
 */
export function fieldValue(page: Page, label: string): Locator {
  return page
    .getByRole('row')
    .filter({ has: page.getByRole('cell', { name: label, exact: true }) })
    .getByRole('cell')
    .nth(1)
}
