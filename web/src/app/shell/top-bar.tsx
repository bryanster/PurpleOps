import { UserIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Link, useNavigate } from 'react-router'

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
import { useSignedInUser } from '@/features/auth/current-user'
import { ACCOUNT_PATH, LOGIN_PATH, TOKENS_PATH } from '@/features/auth/paths'
import { useLogout } from '@/features/auth/queries'

export function TopBar(): ReactNode {
  const user = useSignedInUser()
  const logout = useLogout()
  const navigate = useNavigate()

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

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            {/* The label carries the name, so a screen reader announces who is
                signed in — which is most of what this menu is for. */}
            <Button variant="ghost" size="icon" aria-label={`Account: ${user.displayName}`}>
              <UserIcon aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-56">
            <DropdownMenuLabel className="flex flex-col gap-0.5">
              <span>{user.displayName}</span>
              <span className="text-muted-foreground text-xs font-normal">{user.email}</span>
              <span className="text-muted-foreground text-xs font-normal">
                {user.platformRole === 'admin' ? 'Administrator' : 'Member'}
              </span>
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild>
              <Link to={ACCOUNT_PATH}>Your account</Link>
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <Link to={TOKENS_PATH}>Service tokens</Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              disabled={logout.isPending}
              onSelect={() => {
                logout.mutate(undefined, {
                  // Both outcomes land on the login screen. A logout that failed
                  // server-side still means this browser is done with the
                  // session, and leaving somebody on a signed-in-looking screen
                  // is the worse of the two wrong answers.
                  onSettled: () => {
                    void navigate(LOGIN_PATH, { replace: true })
                  },
                })
              }}
            >
              Sign out
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>
    </header>
  )
}
