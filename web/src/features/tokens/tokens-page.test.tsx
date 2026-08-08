import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import type { components } from '@/api/schema'
import { adminUserFixture, del, get, post } from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { TokensPage } from './tokens-page'

/**
 * Service tokens (M1-011, M1-017).
 *
 * The case this file exists for is the secret: it arrives in one response and
 * never again, so the screen has to put it in front of the person and refuse to
 * be dismissed by reflex.
 */

const SECRET = 'blk_live_abcd1234_thisisthesecretpart'

const tokenFixture: components['schemas']['ServiceToken'] = {
  id: '0192f1a0-0000-7000-8000-00000000c001',
  name: 'nightly-report-export',
  prefix: 'blk_live_abcd1234',
  scopes: ['reports:read'],
  status: 'active',
  createdAt: '2026-01-15T09:00:00Z',
  expiresAt: '2026-04-15T09:00:00Z',
  lastUsedAt: '2026-02-01T03:00:00Z',
}

function renderTokens(items: components['schemas']['ServiceToken'][] = []): void {
  server.use(get('/auth/tokens', () => Response.json({ items })))
  renderWithProviders(<TokensPage />, { user: adminUserFixture, route: '/settings/tokens' })
}

describe('TokensPage', () => {
  it('says what to do when there are none, rather than showing an empty table', async () => {
    renderTokens()

    expect(await screen.findByText('No tokens yet')).toBeInTheDocument()
  })

  it('lists a token by its prefix and never by a secret', async () => {
    renderTokens([tokenFixture])

    const row = await screen.findByRole('row', { name: /nightly-report-export/ })
    expect(within(row).getByText(tokenFixture.prefix)).toBeInTheDocument()
    expect(within(row).getByText('reports:read')).toBeInTheDocument()
    expect(document.body).not.toHaveTextContent(SECRET)
  })

  it('shows the secret once, with a warning, behind an acknowledgement', async () => {
    const user = userEvent.setup()
    // Seeded with a row so that the empty state's own "New token" button is not
    // also on screen: two buttons with one name is an ambiguous query, not a
    // bug in the screen.
    renderTokens([tokenFixture])
    server.use(
      post('/auth/tokens', () =>
        Response.json({ serviceToken: tokenFixture, token: SECRET }, { status: 201 }),
      ),
    )

    await user.click(await screen.findByRole('button', { name: 'New token' }))
    await user.type(screen.getByLabelText('Name'), 'nightly-report-export')
    await user.click(screen.getByRole('checkbox', { name: /Read reports/ }))
    await user.click(screen.getByRole('button', { name: 'Create token' }))

    // The secret, and a warning that says why this moment matters.
    expect(await screen.findByText(SECRET)).toBeInTheDocument()
    expect(screen.getByRole('alert')).toHaveTextContent('stored as a hash')

    const done = screen.getByRole('button', { name: 'Done' })
    expect(done).toBeDisabled()

    await user.click(screen.getByRole('checkbox', { name: /I have saved this token/ }))
    expect(done).toBeEnabled()

    await user.click(done)

    // Gone from the document once the dialog closes: it lives in component
    // state, so unmounting is what removes it.
    expect(document.body).not.toHaveTextContent(SECRET)
  })

  it('does not show the secret again if the dialog is reopened', async () => {
    const user = userEvent.setup()
    renderTokens([tokenFixture])
    server.use(
      post('/auth/tokens', () =>
        Response.json({ serviceToken: tokenFixture, token: SECRET }, { status: 201 }),
      ),
    )

    await user.click(await screen.findByRole('button', { name: 'New token' }))
    await user.type(screen.getByLabelText('Name'), 'nightly-report-export')
    await user.click(screen.getByRole('checkbox', { name: /Read reports/ }))
    await user.click(screen.getByRole('button', { name: 'Create token' }))
    await screen.findByText(SECRET)

    await user.keyboard('{Escape}')
    await user.click(screen.getByRole('button', { name: 'New token' }))

    // A fresh form, and no trace of the token that was minted a moment ago.
    expect(await screen.findByLabelText('Name')).toHaveValue('')
    expect(document.body).not.toHaveTextContent(SECRET)
  })

  it('says what revoking will break before it does it', async () => {
    const user = userEvent.setup()
    let revoked = false
    renderTokens([tokenFixture])
    server.use(
      del('/auth/tokens/{tokenId}', () => {
        revoked = true
        return new Response(null, { status: 204 })
      }),
    )

    await user.click(await screen.findByRole('button', { name: 'Revoke' }))

    const dialog = await screen.findByRole('alertdialog')
    expect(dialog).toHaveTextContent('stops working at its next request')
    expect(dialog).toHaveTextContent(tokenFixture.prefix)
    expect(revoked).toBe(false)

    await user.click(within(dialog).getByRole('button', { name: 'Revoke token' }))
    expect(revoked).toBe(true)
  })
})
