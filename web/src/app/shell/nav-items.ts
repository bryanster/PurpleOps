import {
  ACCOUNT_PATH,
  ADMIN_ACTIVITY_PATH,
  ADMIN_USERS_PATH,
  TOKENS_PATH,
} from '@/features/auth/paths'
import { CONTENT_CUSTOM_PATH, CONTENT_PATH, CONTENT_SOURCES_PATH } from '@/features/content/paths'

/**
 * The left nav, as data.
 *
 * `to` is absent for a section that has no screens yet — M2 to M6 each fill one
 * in. Listing them now is deliberate: the shape of the product is visible from
 * the first screen, and an item arriving later is a one-line change here rather
 * than a nav redesign.
 *
 * `adminOnly` marks an entry a member never sees. Hiding it is *not* what keeps
 * them out — `RequireAdmin` guards the route and the server refuses the
 * requests behind it (M1-013) — it is so that the nav describes what this
 * person can actually do rather than advertising a locked door.
 */
export interface NavItem {
  label: string
  /** Route path, or undefined while the section is unbuilt. */
  to?: string
  /** Short note shown beside an unbuilt item — the milestone that delivers it. */
  pending?: string
  /** Rendered only for a caller with the `admin` platform role. */
  adminOnly?: boolean
}

export interface NavSection {
  label?: string
  items: readonly NavItem[]
}

export const NAV_SECTIONS: readonly NavSection[] = [
  {
    items: [
      { label: 'Engagements', pending: 'M3' },
      { label: 'Scenarios', pending: 'M3' },
      { label: 'Content', to: CONTENT_PATH },
      { label: 'Analytics', pending: 'M5' },
      { label: 'Reports', pending: 'M6' },
    ],
  },
  {
    label: 'Settings',
    items: [
      { label: 'Your account', to: ACCOUNT_PATH },
      { label: 'Service tokens', to: TOKENS_PATH },
    ],
  },
  {
    label: 'Administration',
    items: [
      { label: 'Users', to: ADMIN_USERS_PATH, adminOnly: true },
      { label: 'Activity', to: ADMIN_ACTIVITY_PATH, adminOnly: true },
      { label: 'Content sources', to: CONTENT_SOURCES_PATH, adminOnly: true },
      { label: 'Custom content', to: CONTENT_CUSTOM_PATH, adminOnly: true },
    ],
  },
  {
    label: 'System',
    items: [
      { label: 'Version', to: '/system/version' },
      { label: 'Health', to: '/system/health' },
    ],
  },
]
