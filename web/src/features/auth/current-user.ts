import { createContext, use } from 'react'

import type { CurrentUser } from './queries'

/**
 * The signed-in user, for everything rendered inside [RequireAuth].
 *
 * A context rather than a hook call per screen, for one reason: below the guard
 * the user is *known*, and a hook returning `CurrentUser | undefined` would make
 * every consumer handle a state the guard has already ruled out. Consumers that
 * genuinely might not have one — the login screen, the top bar before the first
 * answer — call `useCurrentUser()` instead and deal with the query states.
 *
 * The value is the query's data, so it is still one request and one cache
 * entry; this only removes the optionality.
 */
export const CurrentUserContext = createContext<CurrentUser | undefined>(undefined)

/**
 * The signed-in user. Throws when rendered outside [RequireAuth], which is a
 * programming error rather than a state to render: it means a screen that
 * assumes a session was routed somewhere a session is not guaranteed.
 */
export function useSignedInUser(): CurrentUser {
  const user = use(CurrentUserContext)
  if (user === undefined) {
    throw new Error('useSignedInUser was called outside RequireAuth; that route needs a guard')
  }
  return user
}
