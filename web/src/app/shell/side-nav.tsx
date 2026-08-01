import type { ReactNode } from 'react'
import { NavLink } from 'react-router'

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

import { NAV_ITEMS } from './nav-items'

export function SideNav(): ReactNode {
  return (
    <nav aria-label="Sections" className="w-56 shrink-0 border-r p-3 max-md:w-44">
      <ul className="flex flex-col gap-0.5">
        {NAV_ITEMS.map((item) =>
          item.to === undefined ? (
            // Not a link and not a disabled <button>: an unbuilt section is not
            // an interactive control, so it stays out of the tab order instead
            // of being a stop that does nothing.
            <li
              key={item.label}
              className="text-muted-foreground flex items-center justify-between rounded-md px-3 py-2 text-sm"
            >
              {item.label}
              {item.pending !== undefined && (
                <Badge variant="secondary" title={`Arrives in milestone ${item.pending}`}>
                  {item.pending}
                </Badge>
              )}
            </li>
          ) : (
            <li key={item.label}>
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
            </li>
          ),
        )}
      </ul>
    </nav>
  )
}
