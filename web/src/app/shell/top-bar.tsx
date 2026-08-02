import { UserIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link } from 'react-router'

import { ThemeToggle } from '@/app/theme/theme-toggle'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'

export function TopBar(): ReactNode {
  return (
    <header className="flex h-14 shrink-0 items-center justify-between border-b px-4">
      <Link
        to="/"
        className="focus-visible:ring-ring/50 rounded-md font-semibold tracking-tight outline-none focus-visible:ring-3"
      >
        Blacklight
      </Link>

      <div className="flex items-center gap-1">
        <ThemeToggle />

        {/* Placeholder. M1-017 replaces this with the signed-in user, their
            role, and links to account and admin. */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" aria-label="Account">
              <UserIcon aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuLabel>Not signed in</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem disabled>Sign in — arrives in M1</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  )
}
