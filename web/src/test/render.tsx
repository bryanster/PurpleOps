import { render, type RenderResult } from '@testing-library/react'
import type { ReactNode } from 'react'
import { MemoryRouter } from 'react-router'

import { QueryClientProvider } from '@tanstack/react-query'

import { ThemeProvider } from '@/app/theme/theme-provider'
import { TooltipProvider } from '@/components/ui/tooltip'
import { Toaster } from '@/components/ui/sonner'
import { CurrentUserContext } from '@/features/auth/current-user'
import type { CurrentUser } from '@/features/auth/queries'

import { createTestQueryClient } from './query'

/**
 * Render a screen with the providers the application gives it: a QueryClient
 * with the app's own defaults, a router, and — for anything below the auth
 * guard — the signed-in user.
 *
 * The theme provider is here because it is in `main.tsx` for the same reason:
 * it wraps everything, including the sign-in screens, which carry a theme
 * toggle of their own. The toast host is here because the shell renders one,
 * and a screen's confirmation of what it just did is part of what the screen
 * does — a test that could not see it would be asserting half the behaviour.
 *
 * `user` is what decides whether the tree is "signed in". It is passed
 * explicitly rather than read from the fake server, so a test can render an
 * administrator's screen and a member's screen without two rounds of
 * request-and-wait, and so a test that is *about* the guard can leave it out
 * and let the real `GET /auth/me` decide.
 */
export function renderWithProviders(
  ui: ReactNode,
  options: { user?: CurrentUser; route?: string } = {},
): RenderResult & { onUnauthorized: ReturnType<typeof createTestQueryClient>['onUnauthorized'] } {
  const { queryClient, onUnauthorized } = createTestQueryClient()
  const result = render(
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <ThemeProvider>
          <MemoryRouter initialEntries={[options.route ?? '/']}>
            {options.user === undefined ? (
              ui
            ) : (
              <CurrentUserContext value={options.user}>{ui}</CurrentUserContext>
            )}
            <Toaster />
          </MemoryRouter>
        </ThemeProvider>
      </TooltipProvider>
    </QueryClientProvider>,
  )

  return { ...result, onUnauthorized }
}
