import { renderHook, waitFor } from '@testing-library/react'
import { HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'

import { type ApiError, isApiError } from '@/api/errors'
import { get, internalError, unhealthyFixture, versionFixture } from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { createTestQueryClient, queryWrapper } from '@/test/query'

import { useHealth, useVersion } from './queries'

function renderUseVersion() {
  const { queryClient, onServerError } = createTestQueryClient()
  const result = renderHook(() => useVersion(), { wrapper: queryWrapper(queryClient) })
  return { ...result, onServerError }
}

describe('useVersion', () => {
  it('goes from pending to the version the server reports', async () => {
    const { result } = renderUseVersion()

    expect(result.current.isPending).toBe(true)
    expect(result.current.data).toBeUndefined()

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })
    expect(result.current.data).toEqual(versionFixture)
  })

  it('goes from pending to a typed error, and raises the global 5xx toast', async () => {
    server.use(get('/version', () => internalError()))

    const { result, onServerError } = renderUseVersion()

    expect(result.current.isPending).toBe(true)

    await waitFor(() => {
      expect(result.current.isError).toBe(true)
    })
    expect(isApiError(result.current.error, 'internal')).toBe(true)
    expect(result.current.data).toBeUndefined()

    // The screen renders the error *and* the app says the server is unwell —
    // the split described in api/README.md.
    expect(onServerError).toHaveBeenCalledOnce()
    expect((onServerError.mock.calls[0]?.[0] as ApiError).status).toBe(500)
  })
})

describe('useHealth', () => {
  it('treats a 503 as the health report it is, not a failure', async () => {
    server.use(get('/healthz', () => HttpResponse.json(unhealthyFixture, { status: 503 })))

    const { queryClient } = createTestQueryClient()
    const { result } = renderHook(() => useHealth(), { wrapper: queryWrapper(queryClient) })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })
    expect(result.current.data).toEqual(unhealthyFixture)
    expect(result.current.error).toBeNull()
  })
})
