import { screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { Route, Routes, useSearchParams } from 'react-router'
import { describe, expect, it } from 'vitest'

import {
  adminUserFixture,
  get,
  memberUserFixture,
  mustEnrolUserFixture,
  unauthenticated,
} from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { RequireAdmin, RequireAuth } from './guards'

/**
 * Route guarding (M1-017).
 *
 * None of this is access control — the server decides that on every request
 * (M1-013) — and the tests are written to say so: what is asserted is where the
 * browser ends up and what the person is told, not whether anything was
 * permitted.
 *
 * The enrolment case is the one with teeth. M1-008's hole was an interface that
 * let somebody skip enrolment, so "every in-app path lands back on the
 * enrolment screen" is asserted from three different addresses.
 */
function renderGuardedApp(route: string): ReturnType<typeof renderWithProviders> {
  return renderWithProviders(
    <Routes>
      <Route path="/login" element={<LoginStandIn />} />

      <Route element={<RequireAuth />}>
        <Route path="/login/enrol" element={<p>enrolment screen</p>} />
        <Route path="/" element={<p>landing screen</p>} />
        <Route path="/settings/account" element={<p>account screen</p>} />

        <Route element={<RequireAdmin />}>
          <Route path="/admin/users" element={<p>users screen</p>} />
        </Route>
      </Route>
    </Routes>,
    { route },
  )
}

/**
 * A stand-in for the sign-in screen that renders the destination it was handed,
 * so a test can assert the redirect *carried* something rather than only that
 * it happened. That is the half of the requirement worth checking: a guard that
 * dropped `return_to` would still land somebody on the login form.
 */
function LoginStandIn(): ReactNode {
  const [params] = useSearchParams()
  return <p>login screen: {params.get('return_to') ?? 'no destination'}</p>
}

describe('RequireAuth', () => {
  it('sends an unauthenticated browser to the login screen, keeping where it was going', async () => {
    server.use(get('/auth/me', () => unauthenticated()))

    renderGuardedApp('/settings/account')

    // The destination survives as a relative in-app path, which is what
    // `safeReturnTo` will accept on the way back in.
    expect(await screen.findByText('login screen: /settings/account')).toBeInTheDocument()
  })

  it('lets a signed-in browser through', async () => {
    server.use(get('/auth/me', () => Response.json(adminUserFixture)))

    renderGuardedApp('/settings/account')

    expect(await screen.findByText('account screen')).toBeInTheDocument()
  })

  it('shows a failure that is not a 401 rather than sending anybody to a login form', async () => {
    server.use(get('/auth/me', () => new Response('gateway is unwell', { status: 502 })))

    renderGuardedApp('/settings/account')

    expect(await screen.findByRole('alert')).toHaveTextContent('That request failed')
    expect(screen.queryByText(/^login screen/)).not.toBeInTheDocument()
  })

  it('cannot be escaped by typing another address while enrolment is required', async () => {
    server.use(get('/auth/me', () => Response.json(mustEnrolUserFixture)))

    for (const address of ['/', '/settings/account', '/admin/users']) {
      const { unmount } = renderGuardedApp(address)
      expect(await screen.findByText('enrolment screen')).toBeInTheDocument()
      unmount()
    }
  })

  it('keeps somebody who has enrolled off the blocking screen', async () => {
    server.use(get('/auth/me', () => Response.json(adminUserFixture)))

    renderGuardedApp('/login/enrol')

    expect(await screen.findByText('landing screen')).toBeInTheDocument()
  })
})

describe('RequireAdmin', () => {
  it('lets an administrator through', async () => {
    server.use(get('/auth/me', () => Response.json(adminUserFixture)))

    renderGuardedApp('/admin/users')

    expect(await screen.findByText('users screen')).toBeInTheDocument()
  })

  it('answers a member with an explanation rather than a screen of failures', async () => {
    server.use(get('/auth/me', () => Response.json(memberUserFixture)))

    renderGuardedApp('/admin/users')

    expect(await screen.findByRole('heading', { name: 'Not yours to see' })).toBeInTheDocument()
    expect(screen.queryByText('users screen')).not.toBeInTheDocument()
  })
})
