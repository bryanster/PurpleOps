import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query'

import { ApiError } from './errors'

/** How many times a request is retried after the first attempt fails. */
export const MAX_QUERY_RETRIES = 2

/**
 * Retry a network failure or a server failure; never retry a client one.
 *
 * A 4xx is an answer: the resource is not there, the caller may not have it,
 * the body was wrong. Repeating the request produces the same answer three
 * times, delays the error the user needs to see, and — for a 429 — makes the
 * thing it is complaining about worse.
 */
export function shouldRetryQuery(failureCount: number, error: Error): boolean {
  if (error instanceof ApiError && error.status >= 400 && error.status < 500) {
    return false
  }
  return failureCount < MAX_QUERY_RETRIES
}

/**
 * What the app does about a failure no single screen owns.
 *
 * They are arguments rather than imports so that this module stays testable
 * without a router or a toast host, and so the wiring is visible in one place
 * (`query-provider.tsx`).
 */
export interface GlobalErrorHandlers {
  /** A session expired or was never there. Stubbed until M1-003 ships login. */
  onUnauthorized: () => void
  /** The server broke. The user cannot act on it, so it is a toast, not a page. */
  onServerError: (error: ApiError) => void
}

/**
 * The app's single QueryClient.
 *
 * Every default here is deliberate; the library's own are tuned for a public
 * web app with anonymous readers, and this is an internal tool with a handful
 * of concurrent users and long-lived tabs.
 */
export function createQueryClient(handlers: GlobalErrorHandlers): QueryClient {
  return new QueryClient({
    queryCache: new QueryCache({
      onError: (error) => {
        reportGlobally(error, handlers)
      },
    }),
    mutationCache: new MutationCache({
      onError: (error) => {
        reportGlobally(error, handlers)
      },
    }),
    defaultOptions: {
      queries: {
        // Long enough that moving between two screens does not refire every
        // request; short enough that something changed in another tab shows up
        // while the user is still wondering whether it did. M4 adds SSE-driven
        // invalidation, after which this stops being the main freshness
        // mechanism.
        staleTime: 30_000,

        retry: shouldRetryQuery,
        // Default exponential backoff (1s, 2s, …) is kept: the failures worth
        // retrying are a restarting server or a dropped link, and both take
        // longer than an immediate second attempt.

        // This is a tool that lives in a tab next to a terminal. Refetching
        // every visible query each time it regains focus would be constant
        // background traffic for data that changes when someone acts on it.
        refetchOnWindowFocus: false,
        // A reconnect is different: the tab was demonstrably offline, so what
        // it shows may be arbitrarily old. That default (true) stays.

        // Errors are rendered by the screen that asked for the data, which can
        // say what failed and offer a retry. Throwing to the error boundary
        // would replace the whole screen for one failed request.
        throwOnError: false,
      },
      mutations: {
        // A mutation is not safe to repeat blindly — a retried POST that
        // actually succeeded creates two of something.
        retry: false,
      },
    },
  })
}

/**
 * The two failures every screen would otherwise handle identically.
 *
 * Everything else — a 404, a validation failure, a network error — belongs to
 * the screen that made the request, which has the context to say what it means.
 */
function reportGlobally(error: Error, handlers: GlobalErrorHandlers): void {
  if (!(error instanceof ApiError)) {
    return
  }
  if (error.status === 401) {
    handlers.onUnauthorized()
    return
  }
  if (error.status >= 500) {
    handlers.onServerError(error)
  }
}
