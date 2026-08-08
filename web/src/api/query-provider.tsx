import { QueryClientProvider } from '@tanstack/react-query'
import { type ReactNode, useState } from 'react'
import { toast } from 'sonner'

import { loginUrlFor } from '@/features/auth/return-to'

import type { ApiError } from './errors'
import { createQueryClient } from './query-client'

/**
 * Owns the QueryClient and the two failures that are the whole app's problem
 * rather than one screen's.
 *
 * The client is created once and lives above the shell, so a screen unmounting
 * mid-request does not tear down the cache.
 */
export function QueryProvider({ children }: { children: ReactNode }): ReactNode {
  const [queryClient] = useState(() =>
    createQueryClient({
      onUnauthorized: () => {
        // A document navigation rather than a router one, deliberately: losing
        // a session should drop every piece of in-memory state with it, and
        // the SPA is served from this same origin (M0B-010) so it costs one
        // request.
        //
        // This is the session-died-mid-use path only. A browser that simply has
        // no session never reaches here — `SESSION_QUERY_KEY` in
        // query-client.ts explains why — so this cannot fire on the sign-in
        // screens, and the guard clause below is a second line of defence
        // against a redirect loop rather than the mechanism.
        const here = `${window.location.pathname}${window.location.search}`
        if (!window.location.pathname.startsWith('/login')) {
          // Carrying where they were, so that signing in again lands back on
          // the screen the expiry interrupted rather than on the home page.
          window.location.assign(loginUrlFor(here))
        }
      },
      onServerError: (error: ApiError) => {
        toast.error('The server failed to answer that request.', {
          // One id, so a screen firing three queries that all fail against a
          // server that is down stacks one toast rather than three.
          id: 'server-error',
          description: error.requestId
            ? `Try again in a moment. Quote request ${error.requestId} if it keeps happening.`
            : 'Try again in a moment.',
        })
      },
    }),
  )

  return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
}
