import { describe, expect, it, vi } from 'vitest'

import { ApiError } from './errors'
import { createQueryClient, MAX_QUERY_RETRIES, shouldRetryQuery } from './query-client'

function apiError(status: number): ApiError {
  return new ApiError(`${String(status)} failed`, { status })
}

describe('shouldRetryQuery', () => {
  it('does not retry a client error, however many attempts are left', () => {
    for (const status of [400, 401, 403, 404, 409, 429]) {
      expect(shouldRetryQuery(0, apiError(status))).toBe(false)
    }
  })

  it('retries a server error, up to the bound', () => {
    expect(shouldRetryQuery(0, apiError(500))).toBe(true)
    expect(shouldRetryQuery(MAX_QUERY_RETRIES - 1, apiError(503))).toBe(true)
    expect(shouldRetryQuery(MAX_QUERY_RETRIES, apiError(500))).toBe(false)
  })

  it('retries a network failure, up to the bound', () => {
    // No response, so no ApiError — this is what `fetch` rejects with when the
    // server is not listening or the link drops mid-request.
    const offline = new TypeError('Failed to fetch')

    expect(shouldRetryQuery(0, offline)).toBe(true)
    expect(shouldRetryQuery(MAX_QUERY_RETRIES, offline)).toBe(false)
  })
})

describe('global error handling', () => {
  async function fetchFailing(error: Error) {
    const onUnauthorized = vi.fn()
    const onServerError = vi.fn()
    const queryClient = createQueryClient({ onUnauthorized, onServerError })

    await queryClient
      .query({
        queryKey: ['test', error.message],
        queryFn: () => {
          throw error
        },
        retry: false,
      })
      .catch(() => undefined)

    return { onUnauthorized, onServerError }
  }

  it('sends a 401 to the login redirect', async () => {
    const { onUnauthorized, onServerError } = await fetchFailing(apiError(401))

    expect(onUnauthorized).toHaveBeenCalledOnce()
    expect(onServerError).not.toHaveBeenCalled()
  })

  it('raises a toast for a 5xx', async () => {
    const { onUnauthorized, onServerError } = await fetchFailing(apiError(503))

    expect(onServerError).toHaveBeenCalledOnce()
    expect(onUnauthorized).not.toHaveBeenCalled()
  })

  it('leaves everything else to the screen that asked', async () => {
    // A 404 is about one request and one screen. A global toast for it would be
    // noise on top of the error the screen is already showing.
    const { onUnauthorized, onServerError } = await fetchFailing(apiError(404))

    expect(onUnauthorized).not.toHaveBeenCalled()
    expect(onServerError).not.toHaveBeenCalled()
  })

  it('ignores a failure that is not an API error', async () => {
    const { onUnauthorized, onServerError } = await fetchFailing(new Error('a bug in a queryFn'))

    expect(onUnauthorized).not.toHaveBeenCalled()
    expect(onServerError).not.toHaveBeenCalled()
  })
})
