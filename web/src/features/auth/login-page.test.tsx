import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router'
import { describe, expect, it } from 'vitest'

import { API_BASE_PATH } from '@/api/client'
import { get, post, problem, rateLimited, unauthenticated } from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { LoginPage } from './login-page'

/**
 * The sign-in screen (M1-017).
 *
 * The cases here are the ones the ticket names, and two of them are about what
 * the screen must *not* say: a refusal that varies with whether the account
 * exists, and a lockout that does not say when it lifts.
 */

/**
 * Where the server says a sign-on begins. Built from the client's own base path
 * rather than typed out, because no module but `api/client.ts` writes an API
 * URL — the lint rule that enforces that applies to tests as well, and it is
 * right to: a fixture with a stale path would pass while the real button led
 * nowhere.
 */
const SSO_START_URL = `${API_BASE_PATH}/auth/oidc/start`

/** Nobody is signed in, which is the state this screen is written for. */
function anonymous(): void {
  server.use(get('/auth/me', () => unauthenticated()))
}

/**
 * The screen inside a route table, because what a successful sign-in *does* is
 * navigate — and a component rendered on its own would stay on screen however
 * many times it called `navigate`. The stand-in screens are what the assertions
 * look for.
 */
function renderLogin(route: string): ReturnType<typeof renderWithProviders> {
  return renderWithProviders(
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/login/mfa" element={<p>mfa challenge</p>} />
      <Route path="/login/enrol" element={<p>enrolment screen</p>} />
      <Route path="/settings/account" element={<p>account screen</p>} />
      <Route path="/" element={<p>landing screen</p>} />
    </Routes>,
    { route },
  )
}

async function signIn(email: string, password: string): Promise<void> {
  const user = userEvent.setup()
  await user.type(screen.getByLabelText('Email address'), email)
  await user.type(screen.getByLabelText('Password'), password)
  await user.click(screen.getByRole('button', { name: 'Sign in' }))
}

describe('LoginPage', () => {
  it('signs in and sends the browser to the intended destination', async () => {
    anonymous()
    server.use(
      post('/auth/login', () =>
        Response.json({ status: 'authenticated' }, { headers: { 'content-type': 'text/json' } }),
      ),
    )

    renderLogin('/login?return_to=%2Fsettings%2Faccount')

    await signIn('ada@example.test', 'correct horse battery staple')

    expect(await screen.findByText('account screen')).toBeInTheDocument()
  })

  it('says the same thing about a wrong password and an address nobody holds', async () => {
    anonymous()
    server.use(post('/auth/login', () => unauthenticated()))

    const { unmount } = renderLogin('/login')
    await signIn('ada@example.test', 'not the right password')
    const wrongPassword = (await screen.findByRole('alert')).textContent

    unmount()

    renderLogin('/login')
    await signIn('nobody@example.test', 'correct horse battery staple')
    const unknownAddress = (await screen.findByRole('alert')).textContent

    // If these ever differ, the login form has become a way to find out who has
    // an account here — which is the thing M1-003 spends extra work preventing
    // on the server side.
    expect(unknownAddress).toBe(wrongPassword)
    expect(wrongPassword).toContain('do not match an account here')
  })

  it('says when a lockout lifts rather than only that there is one', async () => {
    anonymous()
    server.use(post('/auth/login', () => rateLimited(240)))

    renderLogin('/login')
    await signIn('ada@example.test', 'correct horse battery staple')

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('Too many sign-in attempts')
    expect(alert).toHaveTextContent('in about 4 minutes')
    // And the request ID, so the person can quote it (M0B-007).
    expect(alert).toHaveTextContent(/Request/)
  })

  it('sends somebody with a second factor to the challenge rather than to the app', async () => {
    anonymous()
    server.use(
      post('/auth/login', () =>
        Response.json({ status: 'mfa_required' }, { headers: { 'content-type': 'text/json' } }),
      ),
    )

    renderLogin('/login')
    await signIn('ada@example.test', 'correct horse battery staple')

    // No session was issued, so this must not land in the application.
    expect(await screen.findByText('mfa challenge')).toBeInTheDocument()
  })

  it('draws a button for a healthy single sign-on provider and carries the destination', async () => {
    anonymous()
    server.use(
      get('/auth/providers', () =>
        Response.json(
          {
            password: true,
            sso: [{ id: 'oidc', label: 'Single sign-on', startUrl: SSO_START_URL }],
          },
          { headers: { 'content-type': 'text/json' } },
        ),
      ),
    )

    renderLogin('/login?return_to=%2Fsettings%2Faccount')

    const button = await screen.findByRole('link', { name: 'Single sign-on' })
    expect(button).toHaveAttribute('href', `${SSO_START_URL}?return_to=%2Fsettings%2Faccount`)
  })

  it('offers nothing but an explanation when no provider is reachable', async () => {
    anonymous()
    server.use(
      get('/auth/providers', () =>
        Response.json({ password: false, sso: [] }, { headers: { 'content-type': 'text/json' } }),
      ),
    )

    renderLogin('/login')

    expect(await screen.findByText(/not offering a way to sign in/)).toBeInTheDocument()
    expect(screen.queryByLabelText('Password')).not.toBeInTheDocument()
  })

  it('refuses to be redirected off this origin by return_to', async () => {
    anonymous()
    server.use(
      get('/auth/providers', () =>
        Response.json(
          {
            password: true,
            sso: [{ id: 'oidc', label: 'Single sign-on', startUrl: SSO_START_URL }],
          },
          { headers: { 'content-type': 'text/json' } },
        ),
      ),
    )

    renderLogin(`/login?return_to=${encodeURIComponent('https://evil.example/phish')}`)

    // Fell back to the landing path rather than carrying the absolute URL into
    // the sign-on flow. `return-to.test.ts` covers the rule itself.
    const button = await screen.findByRole('link', { name: 'Single sign-on' })
    expect(button).toHaveAttribute('href', `${SSO_START_URL}?return_to=%2F`)
  })

  it('shows a server failure as something to retry, with its request id', async () => {
    anonymous()
    server.use(
      post('/auth/login', () =>
        problem({ status: 500, code: 'internal', title: 'Internal Server Error' }),
      ),
    )

    renderLogin('/login')
    await signIn('ada@example.test', 'correct horse battery staple')

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent('The server could not answer that')
  })
})
