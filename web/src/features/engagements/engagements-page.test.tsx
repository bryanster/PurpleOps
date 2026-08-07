import { describe, expect, test } from 'vitest'
import { screen, waitFor } from '@testing-library/react'

import type { components } from '@/api/schema'
import {
  adminUserFixture,
  get,
} from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { ENGAGEMENTS_PATH } from './paths'
import { EngagementsPage } from './engagements-page'

function stubEmptyList(): void {
  server.use(
    get('/engagements', () =>
      Response.json({ items: [], nextCursor: undefined }, { status: 200 }),
    ),
  )
}

function renderList(user: components['schemas']['CurrentUser'] = adminUserFixture): void {
  renderWithProviders(<EngagementsPage />, { user, route: ENGAGEMENTS_PATH })
}

describe('EngagementsPage', () => {
  test('renders heading and empty state', async () => {
    stubEmptyList()
    renderList()

    await screen.findByRole('heading', { name: 'Engagements' })
    await waitFor(() => {
      expect(screen.getByText('No engagements')).toBeDefined()
    })
  })

  test('renders engagement rows', async () => {
    server.use(
      get('/engagements', () =>
        Response.json(
          {
            items: [
              {
                id: '0192a000-0000-7000-8000-000000000001',
                name: 'Q4 Assessment',
                client: 'Acme Corp',
                description: 'Q4 purple team',
                status: 'active',
                startsOn: '2026-10-01',
                endsOn: '2026-10-15',
                attackVersion: '15.1',
                mode: 'blind',
                autoRevealOnStart: false,
                createdBy: adminUserFixture.id,
                createdAt: '2026-09-20T10:00:00Z',
                updatedAt: '2026-09-20T10:00:00Z',
              },
            ],
            nextCursor: undefined,
          },
          { status: 200 },
        ),
      ),
    )

    renderList()

    await screen.findByText('Q4 Assessment')
    expect(screen.getByText('Acme Corp')).toBeDefined()
    expect(screen.getByText('Active')).toBeDefined()
    expect(screen.getByText('Blind')).toBeDefined()
  })

  test('shows create button', async () => {
    stubEmptyList()
    renderList()

    await screen.findByRole('heading', { name: 'Engagements' })
    expect(screen.getByRole('button', { name: /new engagement/i })).toBeDefined()
  })
})
