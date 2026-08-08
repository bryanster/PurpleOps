import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router'
import { describe, expect, it } from 'vitest'

import { get, post, unauthenticated } from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { MfaChallengePage } from './mfa-challenge-page'

/**
 * The second half of a sign-in (M1-006, M1-007).
 *
 * The two behaviours worth pinning are the ones a person notices: six digits
 * submit themselves, and the recovery-code path is one click away rather than
 * buried.
 */
function renderChallenge(): void {
  server.use(get('/auth/me', () => unauthenticated()))
  renderWithProviders(
    <Routes>
      <Route path="/login/mfa" element={<MfaChallengePage />} />
      <Route path="/" element={<p>landing screen</p>} />
    </Routes>,
    { route: '/login/mfa' },
  )
}

describe('MfaChallengePage', () => {
  it('submits as soon as six digits are present, without a click', async () => {
    const user = userEvent.setup()
    server.use(
      post('/auth/mfa/totp/verify', () =>
        Response.json({ status: 'authenticated' }, { headers: { 'content-type': 'text/json' } }),
      ),
    )
    renderChallenge()

    await user.type(screen.getByLabelText('Authenticator code'), '492817')

    expect(await screen.findByText('landing screen')).toBeInTheDocument()
  })

  it('accepts a code pasted with the space some apps put in it', async () => {
    const user = userEvent.setup()
    let submitted: string | undefined
    server.use(
      post('/auth/mfa/totp/verify', async ({ request }) => {
        submitted = ((await request.json()) as { code: string }).code
        return Response.json(
          { status: 'authenticated' },
          { headers: { 'content-type': 'text/json' } },
        )
      }),
    )
    renderChallenge()

    const field = screen.getByLabelText('Authenticator code')
    field.focus()
    await user.paste('492 817')

    expect(await screen.findByText('landing screen')).toBeInTheDocument()
    expect(submitted).toBe('492817')
  })

  it('is focused on mount, so a code can be typed straight away', () => {
    renderChallenge()

    expect(screen.getByLabelText('Authenticator code')).toHaveFocus()
  })

  it('clears the field and explains when a code is refused', async () => {
    const user = userEvent.setup()
    server.use(post('/auth/mfa/totp/verify', () => unauthenticated()))
    renderChallenge()

    const field = screen.getByLabelText('Authenticator code')
    await user.type(field, '000000')

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('That code was not accepted')
    // Cleared, because a refused code has usually rolled over.
    expect(field).toHaveValue('')
  })

  it('offers the recovery-code path and signs in with one', async () => {
    const user = userEvent.setup()
    server.use(
      post('/auth/mfa/recovery/verify', () =>
        Response.json({ status: 'authenticated' }, { headers: { 'content-type': 'text/json' } }),
      ),
    )
    renderChallenge()

    await user.click(screen.getByRole('button', { name: 'Use a recovery code instead' }))
    await user.type(screen.getByLabelText('Recovery code'), '3K9M-2PTV-XA47-QRJH-58WY')
    await user.click(screen.getByRole('button', { name: 'Use recovery code' }))

    expect(await screen.findByText('landing screen')).toBeInTheDocument()
  })

  it('says a recovery code is spent rather than blaming the authenticator', async () => {
    const user = userEvent.setup()
    server.use(post('/auth/mfa/recovery/verify', () => unauthenticated()))
    renderChallenge()

    await user.click(screen.getByRole('button', { name: 'Use a recovery code instead' }))
    await user.type(screen.getByLabelText('Recovery code'), '3K9M-2PTV-XA47-QRJH-58WY')
    await user.click(screen.getByRole('button', { name: 'Use recovery code' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('Each code works once')
  })
})
