import { type SyntheticEvent, type ReactNode, useId, useState } from 'react'
import { Navigate, useNavigate, useSearchParams } from 'react-router'

import { ApiError, isApiError } from '@/api/errors'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'

import { AuthLayout } from './auth-layout'
import { FormAlert } from './form-alert'
import { ENROLMENT_PATH, MFA_CHALLENGE_PATH } from './paths'
import { useAuthProviders, useCurrentUser, useLogin } from './queries'
import { RETURN_TO_PARAM, safeReturnTo } from './return-to'

/**
 * Sign in (M1-017).
 *
 * Three things here are security decisions rather than layout:
 *
 * 1. **One sentence for every refusal.** A wrong password, an address nobody
 *    holds, and a disabled account are one answer from the server (M1-003), and
 *    this screen writes that answer itself rather than rendering whatever came
 *    back — so the copy cannot start varying because somebody reworded a
 *    problem detail.
 * 2. **The lockout says when.** A 429 carries `Retry-After` (M1-004), and
 *    "too many attempts" without a number is a dead end that generates a
 *    support ticket.
 * 3. **`return_to` is checked before it is used.** See `return-to.ts`.
 */
export function LoginPage(): ReactNode {
  const [params] = useSearchParams()
  const returnTo = safeReturnTo(params.get(RETURN_TO_PARAM))

  const navigate = useNavigate()
  const login = useLogin()
  const providers = useAuthProviders()

  // Somebody who is already signed in and lands here — a bookmark, a back
  // button — is sent on rather than shown a form that would confuse them.
  // `retry: false` on this query means one request decides it.
  const { data: currentUser } = useCurrentUser()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')

  const emailId = useId()
  const passwordId = useId()
  const errorId = useId()

  if (currentUser !== undefined) {
    return <Navigate to={returnTo} replace />
  }

  function onSubmit(event: SyntheticEvent): void {
    event.preventDefault()
    login.mutate(
      { email, password },
      {
        onSuccess: (result) => {
          switch (result.status) {
            case 'authenticated':
              void navigate(returnTo, { replace: true })
              return
            case 'mfa_required':
              // No session yet: the pending state is in a short-lived cookie
              // the server set, and the challenge screen is the only thing that
              // can spend it. The destination travels in the router's state so
              // it survives the second step without going back through a query
              // string that would have to be re-checked.
              void navigate(MFA_CHALLENGE_PATH, { replace: true, state: { returnTo } })
              return
            case 'mfa_enrolment_required':
              void navigate(ENROLMENT_PATH, { replace: true })
              return
          }
        },
      },
    )
  }

  const failure = describeFailure(login.error)
  const passwordOffered = providers.data?.password ?? true
  const ssoProviders = providers.data?.sso ?? []

  return (
    <AuthLayout
      title="Sign in"
      description="This installation is private. Sessions end on their own, so signing in again is normal."
    >
      <div className="flex flex-col gap-6">
        {failure !== undefined && <FormAlert message={failure} error={login.error} />}

        {passwordOffered && (
          <form className="flex flex-col gap-4" onSubmit={onSubmit} noValidate>
            <div className="flex flex-col gap-2">
              <Label htmlFor={emailId}>Email address</Label>
              <Input
                id={emailId}
                name="email"
                type="email"
                autoComplete="username"
                autoFocus
                required
                value={email}
                onChange={(event) => {
                  setEmail(event.target.value)
                }}
                // Both fields point at the one alert, because the server does
                // not say which of them was wrong and this screen must not
                // imply that it did.
                aria-describedby={failure === undefined ? undefined : errorId}
              />
            </div>

            <div className="flex flex-col gap-2">
              <Label htmlFor={passwordId}>Password</Label>
              <Input
                id={passwordId}
                name="password"
                type="password"
                autoComplete="current-password"
                required
                value={password}
                onChange={(event) => {
                  setPassword(event.target.value)
                }}
                aria-describedby={failure === undefined ? undefined : errorId}
              />
            </div>

            {/* The alert above is what a screen reader announces; this is the
                element the two fields point at, kept empty of its own text so
                the message is not read twice. */}
            <span id={errorId} className="sr-only">
              {failure}
            </span>

            <Button type="submit" disabled={login.isPending}>
              {login.isPending ? 'Signing in…' : 'Sign in'}
            </Button>
          </form>
        )}

        {ssoProviders.length > 0 && (
          <>
            {passwordOffered && (
              <div className="flex items-center gap-3">
                <Separator className="flex-1" />
                <span className="text-muted-foreground text-xs uppercase">or</span>
                <Separator className="flex-1" />
              </div>
            )}

            <div className="flex flex-col gap-2">
              {ssoProviders.map((provider) => (
                <Button key={provider.id} variant="outline" asChild>
                  {/* A navigation, not a fetch: the endpoint answers with a
                      redirect to the identity provider and sets the cookie that
                      the callback needs (M1-009). An <a> rather than a router
                      link for the same reason — this leaves the application. */}
                  <a href={ssoHref(provider.startUrl, returnTo)}>{provider.label}</a>
                </Button>
              ))}
            </div>
          </>
        )}

        {!passwordOffered && ssoProviders.length === 0 && (
          <p className="text-muted-foreground text-sm" role="status">
            {providers.isPending
              ? 'Checking how this installation can be signed in to…'
              : 'This installation is not offering a way to sign in right now. If single sign-on is ' +
                'configured, its provider may be unreachable — an administrator can check the server log.'}
          </p>
        )}
      </div>
    </AuthLayout>
  )
}

/**
 * What to tell somebody about a failed attempt.
 *
 * The 401 wording is deliberately incurious: it does not say whether the
 * address exists, because the server has gone to some trouble to make those two
 * cases indistinguishable and a client that guessed would undo that.
 */
function describeFailure(error: unknown): string | undefined {
  if (error === null || error === undefined) {
    return undefined
  }
  if (isApiError(error, 'rate_limited')) {
    const wait = describeWait(error.retryAfterSeconds)
    return `Too many sign-in attempts. Try again ${wait}. The right password will not shorten this, and it applies to this account and this address rather than to you personally.`
  }
  if (isApiError(error, 'unauthenticated')) {
    return 'That email address and password do not match an account here.'
  }
  if (error instanceof ApiError && error.status >= 500) {
    return 'The server could not answer that. Try again in a moment.'
  }
  return 'That sign-in could not be completed.'
}

/** "in 4 minutes", or a vaguer phrase when the server sent no header. */
function describeWait(seconds: number | undefined): string {
  if (seconds === undefined || seconds <= 0) {
    return 'in a few minutes'
  }
  if (seconds < 60) {
    return `in ${String(seconds)} seconds`
  }
  const minutes = Math.ceil(seconds / 60)
  return minutes === 1 ? 'in about a minute' : `in about ${String(minutes)} minutes`
}

/**
 * Where an SSO button sends the browser.
 *
 * `startUrl` comes from the server, which builds it from its own base path — no
 * URL is written here, which is what `src/api/README.md` requires. It is still
 * checked for shape before being used as an href: it is the one value on this
 * screen that becomes a navigation target, and a path is all it is ever meant
 * to be.
 */
function ssoHref(startUrl: string, returnTo: string): string {
  if (!startUrl.startsWith('/') || startUrl.startsWith('//')) {
    return '#'
  }
  return `${startUrl}?${RETURN_TO_PARAM}=${encodeURIComponent(returnTo)}`
}
