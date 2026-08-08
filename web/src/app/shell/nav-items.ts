import {
  ACCOUNT_PATH,
  ADMIN_ACTIVITY_PATH,
  ADMIN_USERS_PATH,
  TOKENS_PATH,
} from '@/features/auth/paths'
import { CONTENT_CUSTOM_PATH, CONTENT_PATH, CONTENT_SOURCES_PATH } from '@/features/content/paths'
import { ENGAGEMENTS_PATH } from '@/features/engagements/paths'

export interface NavItem {
  label: string
  to?: string
  pending?: string
  adminOnly?: boolean
}

export interface NavSection {
  label?: string
  items: readonly NavItem[]
}

export const NAV_SECTIONS: readonly NavSection[] = [
  {
    items: [{ label: 'Engagements', to: ENGAGEMENTS_PATH }],
  },
  {
    label: 'Content',
    items: [{ label: 'Content library', to: CONTENT_PATH }],
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
