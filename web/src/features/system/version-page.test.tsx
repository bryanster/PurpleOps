import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { get, notFound, versionFixture } from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { createTestQueryClient, queryWrapper } from '@/test/query'

import { VersionPage } from './version-page'

function renderPage() {
  const { queryClient } = createTestQueryClient()
  return render(<VersionPage />, { wrapper: queryWrapper(queryClient) })
}

describe('VersionPage', () => {
  it('shows a loading state, then the build identity', async () => {
    renderPage()

    expect(screen.getByRole('status')).toHaveTextContent('Reading the server version…')

    await waitFor(() => {
      expect(screen.getByText(versionFixture.version)).toBeInTheDocument()
    })
    expect(screen.getByText(versionFixture.commit)).toBeInTheDocument()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
  })

  it('shows the failure and a way out of it', async () => {
    server.use(get('/version', () => notFound('no such build')))

    renderPage()

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('no such build')
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
  })
})
