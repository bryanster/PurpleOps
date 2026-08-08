import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import type { components } from '@/api/schema'
import { adminUserFixture, get, patch, post, problem } from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { UsersPage } from './users-page'

/**
 * Administering accounts (M1-016, M1-017).
 *
 * The ticket's requirement is that a destructive action says what will happen,
 * so most of what is asserted here is the wording of a confirmation — and that
 * nothing is sent until it is confirmed.
 */
const memberRow: components['schemas']['User'] = {
  id: '0192f1a0-0000-7000-8000-00000000a002',
  email: 'mel@example.test',
  displayName: 'Mel Chen',
  platformRole: 'member',
  status: 'active',
  mfaEnforced: false,
  createdAt: '2026-01-02T09:00:00Z',
  updatedAt: '2026-01-02T09:00:00Z',
  lastLoginAt: '2026-02-01T08:00:00Z',
}

function renderUsers(items: components['schemas']['User'][] = [memberRow]): void {
  server.use(get('/users', () => Response.json({ items })))
  renderWithProviders(<UsersPage />, { user: adminUserFixture, route: '/admin/users' })
}

describe('UsersPage', () => {
  it('lists accounts with the facts an administrator acts on', async () => {
    renderUsers()

    const row = await screen.findByRole('row', { name: /Mel Chen/ })
    expect(row).toHaveTextContent('mel@example.test')
    expect(row).toHaveTextContent('Member')
    expect(row).toHaveTextContent('active')
  })

  it('narrows the list through the API rather than in the browser', async () => {
    const user = userEvent.setup()
    let asked: URLSearchParams | undefined
    server.use(
      get('/users', ({ request }) => {
        asked = new URL(request.url).searchParams
        return Response.json({ items: [memberRow] })
      }),
    )
    // Rendered directly rather than through renderUsers, whose own handler
    // would be registered later and take precedence over this one.
    renderWithProviders(<UsersPage />, { user: adminUserFixture, route: '/admin/users' })
    await screen.findByRole('row', { name: /Mel Chen/ })

    await user.type(screen.getByLabelText('Search'), 'mel')

    // A filter that was applied client-side would page wrongly the moment there
    // were more accounts than one page.
    await waitFor(() => {
      expect(asked?.get('q')).toBe('mel')
    })
  })

  it('says how a disable will land before doing it', async () => {
    const user = userEvent.setup()
    let disabled = false
    server.use(
      post('/users/{userId}/disable', () => {
        disabled = true
        return Response.json({ ...memberRow, status: 'disabled' })
      }),
    )
    renderUsers()

    const row = await screen.findByRole('row', { name: /Mel Chen/ })
    await user.click(within(row).getByRole('button', { name: 'Disable' }))

    const dialog = await screen.findByRole('alertdialog')
    expect(dialog).toHaveTextContent('signed out of every browser')
    expect(dialog).toHaveTextContent('every service token they own stops working')
    expect(disabled).toBe(false)

    await user.click(within(dialog).getByRole('button', { name: 'Disable account' }))
    expect(disabled).toBe(true)
  })

  it('quotes how many sessions were ended, including none', async () => {
    const user = userEvent.setup()
    server.use(post('/users/{userId}/sessions/revoke', () => Response.json({ revoked: 3 })))
    renderUsers()

    const row = await screen.findByRole('row', { name: /Mel Chen/ })
    await user.click(within(row).getByRole('button', { name: 'Sign out' }))

    const dialog = await screen.findByRole('alertdialog')
    expect(dialog).toHaveTextContent('Service tokens they own are not sessions')
    await user.click(within(dialog).getByRole('button', { name: 'Sign them out' }))

    expect(await screen.findByText('Ended 3 sessions.')).toBeInTheDocument()
  })

  it('sends only the fields that changed when editing', async () => {
    const user = userEvent.setup()
    let body: unknown
    server.use(
      patch('/users/{userId}', async ({ request }) => {
        body = await request.json()
        return Response.json({ ...memberRow, platformRole: 'admin' })
      }),
    )
    renderUsers()

    const row = await screen.findByRole('row', { name: /Mel Chen/ })
    await user.click(within(row).getByRole('button', { name: 'Edit' }))

    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('checkbox', { name: /Require a second factor/ }))
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }))

    // A patch, not a replacement: two administrators editing different things
    // must not overwrite each other (M1-016).
    expect(body).toEqual({ mfaEnforced: true })
  })

  it('passes the last-administrator refusal through in the server’s own words', async () => {
    const user = userEvent.setup()
    server.use(
      post('/users/{userId}/disable', () =>
        problem({
          status: 409,
          code: 'conflict',
          title: 'Conflict',
          detail: 'the last active administrator cannot be disabled',
        }),
      ),
    )
    renderUsers([{ ...memberRow, platformRole: 'admin' }])

    const row = await screen.findByRole('row', { name: /Mel Chen/ })
    await user.click(within(row).getByRole('button', { name: 'Disable' }))
    const dialog = await screen.findByRole('alertdialog')
    await user.click(within(dialog).getByRole('button', { name: 'Disable account' }))

    expect(
      await screen.findByText('the last active administrator cannot be disabled'),
    ).toBeInTheDocument()
  })

  it('says so when nothing matches, rather than showing an empty table', async () => {
    renderUsers([])

    expect(await screen.findByText('No accounts yet')).toBeInTheDocument()
  })
})
