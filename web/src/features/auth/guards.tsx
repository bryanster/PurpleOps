import type { ReactNode } from 'react'
import { Navigate, Outlet, useLocation } from 'react-router'

import { isApiError } from '@/api/errors'
import { PageError, PageLoading } from '@/app/shell/page-state'

import { CurrentUserContext } from './current-user'
import { isAdmin, mustEnrol, useCurrentUser } from './queries'
import { ENROLMENT_PATH } from './paths'
import { loginUrlFor, returnToFor } from './return-to'

/**
 * Route guarding, driven by `GET /auth/me` and nothing else (M1-017).
 *
 * Two things this is *not*. It is not access control — the server decides that,
 * on every request, from the session rather than from anything the client says
 * (M1-013). And it is not a set of `if`s scattered through screens: one query
 * answers who the caller is, and these three components are the only places
 * that act on it.
 *
 * What it is for is honesty. A member who is shown an admin link and then told
 * "forbidden" has been lied to by the interface; a member who never sees the
 * link, and gets a page explaining the refusal if they type the address, has
 * not.
 */

/**
 * Everything below this needs a session.
 *
 * The 401 case redirects to the login screen carrying where the user was trying
 * to go, as a relative path — see `return-to.ts` for why that check exists.
 * `replace` so that the browser's back button does not land on the guard again
 * and bounce.
 */
export function RequireAuth(): ReactNode {
  const location = useLocation()
  const { data: user, error, isPending, refetch } = useCurrentUser()

  if (isPending) {
    // A full-height container rather than a bare spinner in the corner: at this
    // point there is no shell around it, and the alternative is a blank page
    // that looks broken while the request is in flight.
    return (
      <div className="flex h-dvh items-center justify-center">
        <PageLoading label="Checking your session…" />
      </div>
    )
  }

  if (error) {
    if (isApiError(error, 'unauthenticated')) {
      return <Navigate to={loginUrlFor(returnToFor(location))} replace />
    }
    // Anything else — the server is down, the network went away — is not a
    // reason to send somebody to a login form they cannot use either.
    return (
      <div className="flex h-dvh items-center justify-center p-6">
        <PageError
          error={error}
          onRetry={() => {
            void refetch()
          }}
        />
      </div>
    )
  }

  // The enrolment gate (M1-008). A session in this state may reach exactly one
  // screen, and the server agrees: every other endpoint answers 403 with
  // `mfa_enrolment_required` until an authenticator is confirmed. Redirecting
  // rather than rendering is what makes the address bar useless as a way past
  // it — every in-app path lands back here.
  if (mustEnrol(user) && location.pathname !== ENROLMENT_PATH) {
    return <Navigate to={ENROLMENT_PATH} replace />
  }
  // And the other direction: somebody who has enrolled has no business on the
  // blocking screen, however they got the address.
  if (!mustEnrol(user) && location.pathname === ENROLMENT_PATH) {
    return <Navigate to="/" replace />
  }

  return (
    <CurrentUserContext value={user}>
      <Outlet />
    </CurrentUserContext>
  )
}

/**
 * Everything below this needs the `admin` platform role.
 *
 * Hiding the nav entry is not access control and this is not either — the
 * server refuses a member's request whatever the client renders. This exists so
 * that a member who follows a bookmark gets a page that explains itself instead
 * of a screen of failed requests.
 */
export function RequireAdmin(): ReactNode {
  const location = useLocation()
  const { data: user, error, isPending } = useCurrentUser()

  if (isPending) {
    return <PageLoading label="Checking your permissions…" />
  }
  if (error) {
    if (isApiError(error, 'unauthenticated')) {
      return <Navigate to={loginUrlFor(returnToFor(location))} replace />
    }
    return <PageError error={error} />
  }
  if (!isAdmin(user)) {
    return <ForbiddenPage />
  }
  return <Outlet />
}

/**
 * What a member sees if they reach an administrator's address.
 *
 * It says what happened and does not offer a way to try again, because there
 * isn't one: the answer will not change until somebody with the admin role
 * changes theirs.
 */
export function ForbiddenPage(): ReactNode {
  return (
    <section className="flex max-w-prose flex-col items-start gap-3" role="alert">
      <h1 className="text-xl font-semibold">Not yours to see</h1>
      <p className="text-muted-foreground">
        This screen is for administrators of this installation, and your account is not one. Nothing
        was changed and nobody was notified — if you need access, ask an administrator to change
        your platform role.
      </p>
    </section>
  )
}
