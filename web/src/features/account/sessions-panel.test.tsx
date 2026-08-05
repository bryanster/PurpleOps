import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import {
  adminUserFixture,
  currentSessionFixture,
  del,
  get,
  otherSessionFixture,
  post,
} from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { SessionsPanel } from './sessions-panel'

/**
 * "Where you are signed in" (M1-017).
 *
 * The two things worth pinning are both about not surprising anybody: the
 * session you are using is marked and cannot be revoked from here, and the
 * bulk control says how many browsers it is about to end.
 */
function renderSessions(): void {
  renderWithProviders(<SessionsPanel />, {
    user: adminUserFixture,
    route: '/settings/account',
  })
}

describe('SessionsPanel', () => {
  it('marks the current browser and offers no revoke for it', async () => {
    renderSessions()

    // Found by role and by what the row says, so the query fails loudly if the
    // row is missing rather than needing a non-null assertion to compile.
    const current = await screen.findByRole('row', { name: /This browser/ })
    const other = await screen.findByRole('row', { name: /hotel/ })

    expect(within(current).queryByRole('button', { name: 'Revoke' })).toBeNull()
    expect(within(other).getByRole('button', { name: 'Revoke' })).toBeEnabled()
  })

  it('shows what a session is, so an unfamiliar one can be spotted', async () => {
    renderSessions()

    const row = await screen.findByRole('row', { name: /hotel/ })
    expect(row).toHaveTextContent('203.0.113.7')
    // A session that never presented a second factor is worth noticing.
    expect(row).toHaveTextContent('No second factor')
  })

  it('revokes one session and leaves the rest alone', async () => {
    const user = userEvent.setup()
    let revokedId: string | undefined
    server.use(
      del('/auth/sessions/{sessionId}', ({ params }) => {
        revokedId = String(params.sessionId)
        return new Response(null, { status: 204 })
      }),
    )
    renderSessions()

    const row = await screen.findByRole('row', { name: /hotel/ })
    await user.click(within(row).getByRole('button', { name: 'Revoke' }))

    expect(revokedId).toBe(otherSessionFixture.id)
  })

  it('says how many browsers "everywhere else" means before ending them', async () => {
    const user = userEvent.setup()
    let revokedOthers = false
    server.use(
      post('/auth/sessions/revoke-others', () => {
        revokedOthers = true
        return Response.json({ revoked: 1 })
      }),
    )
    renderSessions()

    await user.click(await screen.findByRole('button', { name: 'Sign out everywhere else' }))

    const dialog = await screen.findByRole('alertdialog')
    expect(dialog).toHaveTextContent('1 other browser')
    expect(dialog).toHaveTextContent('keeps the one you are using')
    expect(revokedOthers).toBe(false)

    await user.click(within(dialog).getByRole('button', { name: 'Sign out everywhere else' }))
    expect(revokedOthers).toBe(true)
  })

  it('offers nothing to end when this is the only browser', async () => {
    server.use(get('/auth/sessions', () => Response.json({ items: [currentSessionFixture] })))
    renderSessions()

    await screen.findByText('This browser')
    expect(screen.getByRole('button', { name: 'Sign out everywhere else' })).toBeDisabled()
  })

  it('shows a failed read as an error with its request id, not an empty table', async () => {
    server.use(get('/auth/sessions', () => new Response('nope', { status: 502 })))
    renderSessions()

    expect(await screen.findByRole('alert')).toHaveTextContent('That request failed')
  })
})
