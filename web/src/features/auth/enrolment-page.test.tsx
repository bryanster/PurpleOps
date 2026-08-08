import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router'
import { describe, expect, it } from 'vitest'

import {
  get,
  memberUserFixture,
  mustEnrolUserFixture,
  post,
  problem,
  recoveryCodesFixture,
  totpEnrolmentFixture,
} from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { EnrolmentPage } from './enrolment-page'
import { RequireAuth } from './guards'

/**
 * Forced enrolment (M1-008).
 *
 * The requirement this file exists for is the last case: the recovery codes are
 * shown exactly once, and the way past them is deliberate rather than a button
 * somebody hits on the way to the application.
 */
function renderEnrolment(): void {
  server.use(
    get('/auth/me', () => Response.json(mustEnrolUserFixture)),
    post('/auth/mfa/totp/enroll', () => Response.json(totpEnrolmentFixture)),
  )
  renderWithProviders(
    <Routes>
      <Route path="/login/enrol" element={<EnrolmentPage />} />
      <Route path="/" element={<p>landing screen</p>} />
    </Routes>,
    { route: '/login/enrol' },
  )
}

async function confirm(code: string): Promise<void> {
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText('Six-digit code from your authenticator'), code)
}

describe('EnrolmentPage', () => {
  it('offers the secret as a QR code and as text to type', async () => {
    renderEnrolment()

    const qr = await screen.findByAltText('QR code containing your authenticator secret')
    expect(qr).toHaveAttribute('src', totpEnrolmentFixture.qrCode)
    expect(screen.getByText(totpEnrolmentFixture.secret)).toBeInTheDocument()
  })

  it('explains a wrong code instead of restarting the enrolment', async () => {
    server.use(
      post('/auth/mfa/totp/confirm', () =>
        problem({
          status: 400,
          code: 'validation_failed',
          title: 'Bad Request',
          errors: [{ field: 'code', message: 'that code is not right' }],
        }),
      ),
    )
    renderEnrolment()

    await confirm('000000')

    expect(await screen.findByRole('alert')).toHaveTextContent('That code was not right')
    // Still on the enrolment screen, with the same secret: a refused code must
    // not mint a new one and invalidate what has already been scanned.
    expect(screen.getByText(totpEnrolmentFixture.secret)).toBeInTheDocument()
  })

  it('shows the recovery codes once, and will not continue until they are saved', async () => {
    const user = userEvent.setup()
    server.use(post('/auth/mfa/totp/confirm', () => Response.json(recoveryCodesFixture)))
    renderEnrolment()

    await confirm('492817')

    const list = await screen.findByRole('list', { name: 'Recovery codes' })
    for (const code of recoveryCodesFixture.codes) {
      expect(list).toHaveTextContent(code)
    }

    // The way out is a deliberate act, not the next button along.
    const proceed = screen.getByRole('button', { name: 'Continue to Blacklight' })
    expect(proceed).toBeDisabled()

    await user.click(screen.getByRole('checkbox'))
    expect(proceed).toBeEnabled()

    await user.click(proceed)
    expect(await screen.findByText('landing screen')).toBeInTheDocument()
  })

  it('keeps the codes on screen when the account stops needing to enrol', async () => {
    const user = userEvent.setup()

    // The server's answer changes the moment the enrolment is confirmed — which
    // is the whole hazard: the route guard is built on that answer, and a
    // refetch at the wrong moment would redirect away from the only screen
    // these codes will ever appear on. This renders the page *under the real
    // guard* so the regression is reachable.
    let confirmed = false
    server.use(
      get('/auth/me', () => Response.json(confirmed ? memberUserFixture : mustEnrolUserFixture)),
      post('/auth/mfa/totp/enroll', () => Response.json(totpEnrolmentFixture)),
      post('/auth/mfa/totp/confirm', () => {
        confirmed = true
        return Response.json(recoveryCodesFixture)
      }),
    )

    renderWithProviders(
      <Routes>
        <Route element={<RequireAuth />}>
          <Route path="/login/enrol" element={<EnrolmentPage />} />
          <Route path="/" element={<p>landing screen</p>} />
        </Route>
      </Routes>,
      { route: '/login/enrol' },
    )

    await confirm('492817')

    expect(await screen.findByRole('list', { name: 'Recovery codes' })).toBeVisible()
    expect(screen.queryByText('landing screen')).not.toBeInTheDocument()

    // And once they are saved, the way on opens.
    await user.click(screen.getByRole('checkbox'))
    await user.click(screen.getByRole('button', { name: 'Continue to Blacklight' }))
    expect(await screen.findByText('landing screen')).toBeInTheDocument()
  })

  it('offers a copy and a download, because one of them is always the wrong one', async () => {
    server.use(post('/auth/mfa/totp/confirm', () => Response.json(recoveryCodesFixture)))
    renderEnrolment()

    await confirm('492817')

    expect(await screen.findByRole('button', { name: 'Copy' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Download' })).toBeInTheDocument()
  })

  it('has no way out but signing out, which changes nothing about the account', async () => {
    renderEnrolment()

    await screen.findByAltText('QR code containing your authenticator secret')

    // No nav, no shell, no link into the application: the only control that
    // leaves this screen is the one that ends the session.
    expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sign out instead' })).toBeInTheDocument()
  })
})
