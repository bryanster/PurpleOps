import { QueryClientProvider } from '@tanstack/react-query'
import { type ReactNode, useState } from 'react'
import { toast } from 'sonner'

import type { ApiError } from './errors'
import { createQueryClient } from './query-client'

/** Where a 401 sends the user. A placeholder route until M1-017 builds the form. */
const LOGIN_PATH = '/login'

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
        // Stub until M1-003: there are no sessions yet, so nothing can reach
        // this. What matters now is that the decision lives in one place
        // instead of in every component that makes a request.
        if (window.location.pathname !== LOGIN_PATH) {
          window.location.assign(LOGIN_PATH)
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
