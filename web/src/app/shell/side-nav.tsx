import type { ReactNode } from 'react'
import { NavLink } from 'react-router'

import { Badge } from '@/components/ui/badge'
import { useSignedInUser } from '@/features/auth/current-user'
import { isAdmin } from '@/features/auth/queries'
import { cn } from '@/lib/utils'

import { NAV_SECTIONS, type NavItem } from './nav-items'

export function SideNav(): ReactNode {
  const user = useSignedInUser()
  const admin = isAdmin(user)

  // A section whose every entry is hidden is dropped along with its heading — a
  // member should not see an empty "Administration" group, which would tell
  // them what they are missing rather than simply not being about them.
  const sections = NAV_SECTIONS.map((section) => ({
    ...section,
    items: section.items.filter((item) => admin || item.adminOnly !== true),
  })).filter((section) => section.items.length > 0)

  return (
    <nav aria-label="Sections" className="w-56 shrink-0 overflow-y-auto border-r p-3 max-md:w-44">
      <div className="flex flex-col gap-4">
        {sections.map((section, index) => (
          <div key={section.label ?? `section-${String(index)}`} className="flex flex-col gap-1">
            {section.label !== undefined && (
              <h2 className="text-muted-foreground px-3 text-xs font-medium tracking-wide uppercase">
                {section.label}
              </h2>
            )}
            <ul className="flex flex-col gap-0.5">
              {section.items.map((item) => (
                <li key={item.label}>
                  <NavEntry item={item} />
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </nav>
  )
}

function NavEntry({ item }: { item: NavItem }): ReactNode {
  if (item.to === undefined) {
    // Not a link and not a disabled <button>: an unbuilt section is not an
    // interactive control, so it stays out of the tab order instead of being a
    // stop that does nothing.
    return (
      <div className="text-muted-foreground flex items-center justify-between rounded-md px-3 py-2 text-sm">
        {item.label}
        {item.pending !== undefined && (
          <Badge variant="secondary" title={`Arrives in milestone ${item.pending}`}>
            {item.pending}
          </Badge>
        )}
      </div>
    )
  }

  return (
    <NavLink
      to={item.to}
      className={({ isActive }) =>
        cn(
          'block rounded-md px-3 py-2 text-sm transition-colors',
          'focus-visible:ring-ring/50 outline-none focus-visible:ring-3',
          isActive
            ? 'bg-muted text-foreground font-medium'
            : 'text-muted-foreground hover:bg-muted hover:text-foreground',
        )
      }
    >
      {item.label}
    </NavLink>
  )
}
