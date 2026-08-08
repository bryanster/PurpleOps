import { QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { vi } from 'vitest'

import { createQueryClient, type GlobalErrorHandlers } from '@/api/query-client'

/**
 * A QueryClient for one test, with the app's own defaults except for retries.
 *
 * Retries are off because a test asserting a failure should not wait out the
 * backoff first; `query-client.test.ts` covers the real retry policy directly.
 * Everything else is the production configuration — a test against a client
 * configured differently from the app is testing something else.
 */
export function createTestQueryClient(handlers: Partial<GlobalErrorHandlers> = {}): {
  queryClient: ReturnType<typeof createQueryClient>
  onUnauthorized: ReturnType<typeof vi.fn>
  onServerError: ReturnType<typeof vi.fn>
} {
  const onUnauthorized = vi.fn(handlers.onUnauthorized)
  const onServerError = vi.fn(handlers.onServerError)

  const queryClient = createQueryClient({ onUnauthorized, onServerError })
  queryClient.setDefaultOptions({
    queries: { ...queryClient.getDefaultOptions().queries, retry: false },
  })

  return { queryClient, onUnauthorized, onServerError }
}

/** Wrapper for `render` and `renderHook`. */
export function queryWrapper(
  queryClient: ReturnType<typeof createQueryClient>,
): ({ children }: { children: ReactNode }) => ReactNode {
  return function Wrapper({ children }: { children: ReactNode }): ReactNode {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  }
}
