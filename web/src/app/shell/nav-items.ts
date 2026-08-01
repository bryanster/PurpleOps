/**
 * The left nav, as data.
 *
 * `to` is absent for a section that has no screens yet — M2 to M6 each fill one
 * in. Listing them now is deliberate: the shape of the product is visible from
 * the first screen, and an item arriving later is a one-line change here rather
 * than a nav redesign.
 */
export interface NavItem {
  label: string
  /** Route path, or undefined while the section is unbuilt. */
  to?: string
  /** Short note shown beside an unbuilt item — the milestone that delivers it. */
  pending?: string
}

export const NAV_ITEMS: readonly NavItem[] = [
  { label: 'Version', to: '/system/version' },
  { label: 'Health', to: '/system/health' },
  { label: 'Engagements', pending: 'M3' },
  { label: 'Scenarios', pending: 'M3' },
  { label: 'Content', pending: 'M2' },
  { label: 'Analytics', pending: 'M5' },
  { label: 'Reports', pending: 'M6' },
]
