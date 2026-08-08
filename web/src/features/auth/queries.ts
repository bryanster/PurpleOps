import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query'

import { api, unwrap } from '@/api/client'
import { SESSION_QUERY_KEY } from '@/api/query-client'
import type { components } from '@/api/schema'

/**
 * Every call the identity screens make (M1-017). Components import a hook from
 * here and never touch `api` themselves — see `src/api/README.md`.
 *
 * Two things in this file are security decisions rather than plumbing, and both
 * are commented where they happen: nothing here is optimistic, and everything
 * that changes what a session may do invalidates the whole `auth` root.
 */

export type CurrentUser = components['schemas']['CurrentUser']
export type LoginResult = components['schemas']['LoginResult']
export type LoginStatus = components['schemas']['LoginStatus']
export type AuthProviders = components['schemas']['AuthProviders']
export type SSOProvider = components['schemas']['SSOProvider']
export type MFAState = components['schemas']['MFAState']
export type RecoveryCodes = components['schemas']['RecoveryCodes']
export type TOTPEnrolment = components['schemas']['TOTPEnrolment']
export type Session = components['schemas']['Session']
export type PlatformRole = components['schemas']['PlatformRole']

export const authKeys = {
  all: ['auth'] as const,
  // Not spelled out again: `api/query-client.ts` has to recognise this exact
  // key to tell "nobody is signed in" from "a session just died", and two
  // spellings of it would be a bug that only appears when a session expires.
  me: () => SESSION_QUERY_KEY,
  providers: () => [...authKeys.all, 'providers'] as const,
  sessions: () => [...authKeys.all, 'sessions'] as const,
}

/**
 * Who the caller is, and the query the route guards are built on.
 *
 * `retry: false` because the interesting failure is a 401, which is an answer
 * and not a hiccup — `shouldRetryQuery` already declines to repeat a 4xx, and
 * saying so here as well documents that the guard is not waiting on a backoff
 * before it redirects.
 *
 * `staleTime: 0`: this is the answer that decides what the whole interface
 * shows, and it is invalidated by every mutation that could change it. Serving
 * a cached role after a demotion would be showing somebody controls they no
 * longer hold — the API refuses them, but an interface should not offer them.
 */
export function currentUserQueryOptions() {
  return queryOptions({
    queryKey: authKeys.me(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/auth/me', { signal })),
    staleTime: 0,
    retry: false,
  })
}

/**
 * The sign-in methods on offer. Public, and read before anybody is signed in.
 *
 * A provider that is configured but unreachable is absent from the answer
 * rather than listed (M1-009), so this is refetched on mount rather than served
 * indefinitely from cache: a provider that has come back deserves its button
 * back without a reload.
 */
export function authProvidersQueryOptions() {
  return queryOptions({
    queryKey: authKeys.providers(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/auth/providers', { signal })),
    staleTime: 10_000,
  })
}

/** The browsers the caller is signed in on (M1-017). */
export function sessionsQueryOptions() {
  return queryOptions({
    queryKey: authKeys.sessions(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/auth/sessions', { signal })),
  })
}

export function useCurrentUser(): UseQueryResult<CurrentUser> {
  return useQuery(currentUserQueryOptions())
}

export function useAuthProviders(): UseQueryResult<AuthProviders> {
  return useQuery(authProvidersQueryOptions())
}

export function useSessions(): UseQueryResult<components['schemas']['Sessions']> {
  return useQuery(sessionsQueryOptions())
}

/**
 * Sign in with an email address and a password.
 *
 * The result is handed back rather than acted on: a 200 here is not necessarily
 * a session, and which of the three outcomes it was decides which screen comes
 * next (`LoginStatus`). A hook that navigated on its own would have to know
 * about routing, and the screen would still have to branch.
 */
export function useLogin(): UseMutationResult<
  LoginResult,
  Error,
  { email: string; password: string }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body) => unwrap(await api.POST('/auth/login', { body })),
    onSuccess: async () => {
      // Everything under `auth` — anything cached from a previous session in
      // this tab is now somebody else's answer.
      await queryClient.invalidateQueries({ queryKey: authKeys.all })
    },
  })
}

/** The second half of a sign-in: a code from the authenticator. */
export function useVerifyTotp(): UseMutationResult<LoginResult, Error, { code: string }> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body) => unwrap(await api.POST('/auth/mfa/totp/verify', { body })),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: authKeys.all })
    },
  })
}

/** The same half of the same sign-in, with a printed code instead (M1-007). */
export function useVerifyRecoveryCode(): UseMutationResult<LoginResult, Error, { code: string }> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body) => unwrap(await api.POST('/auth/mfa/recovery/verify', { body })),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: authKeys.all })
    },
  })
}

/**
 * Sign out.
 *
 * The cache is cleared rather than invalidated: invalidating would refetch
 * every active query as the signed-out user, producing a screen full of 401s on
 * the way to the login page. `clear()` drops the lot, which is the honest state
 * of a tab that no longer has a session.
 */
export function useLogout(): UseMutationResult<void, Error, void> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      await api.POST('/auth/logout')
    },
    onSuccess: () => {
      queryClient.clear()
    },
  })
}

/** Start enrolling an authenticator. The response is the only one with a secret. */
export function useEnrollTotp(): UseMutationResult<TOTPEnrolment, Error, void> {
  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/auth/mfa/totp/enroll')),
  })
}

/**
 * Confirm an enrolment, which mints the recovery codes.
 *
 * Its answer is the only copy of those codes that will ever exist, so the
 * caller — not this hook — decides when they have been shown; nothing here
 * stores them anywhere they could be read back.
 *
 * **It deliberately does not invalidate anything.** Confirming changes what
 * `GET /auth/me` says — the session now satisfies MFA — and the forced-enrolment
 * screen is mounted *under a route guard that reads exactly that*. Refetching
 * here would flip `mfa.enrolled` to true and the guard would redirect to the
 * application, throwing away the recovery codes on the way past. They exist in
 * one response and are gone the moment that component unmounts, so the refetch
 * belongs at the point where the person says they have saved them:
 * [markEnrolmentComplete].
 */
export function useConfirmTotp(): UseMutationResult<RecoveryCodes, Error, { code: string }> {
  return useMutation({
    mutationFn: async (body) => unwrap(await api.POST('/auth/mfa/totp/confirm', { body })),
  })
}

/**
 * Pick up what confirming an enrolment changed, once the recovery codes have
 * been dealt with. See [useConfirmTotp] for why this is a separate step.
 */
export function useMarkEnrolmentComplete(): () => Promise<void> {
  const queryClient = useQueryClient()
  return async () => {
    await queryClient.invalidateQueries({ queryKey: authKeys.all })
  }
}

/** Replace the recovery codes, invalidating every previous one (M1-007). */
export function useRegenerateRecoveryCodes(): UseMutationResult<
  RecoveryCodes,
  Error,
  { currentPassword: string }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body) => unwrap(await api.POST('/auth/mfa/recovery/regenerate', { body })),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: authKeys.me() })
    },
  })
}

/** Remove the authenticator, and the recovery codes with it. */
export function useDisableTotp(): UseMutationResult<void, Error, { currentPassword: string }> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body) => {
      await api.DELETE('/auth/mfa/totp', { body })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: authKeys.me() })
    },
  })
}

/**
 * Change your own password. Every other session of yours ends, so the session
 * list is invalidated alongside the profile.
 */
export function useChangePassword(): UseMutationResult<
  void,
  Error,
  { currentPassword: string; newPassword: string }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body) => {
      await api.POST('/auth/password', { body })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: authKeys.all })
    },
  })
}

/** Change your own display name. The one field `PATCH /users/me` has. */
export function useUpdateDisplayName(): UseMutationResult<
  components['schemas']['User'],
  Error,
  { displayName: string }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body) => unwrap(await api.PATCH('/users/me', { body })),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: authKeys.me() })
    },
  })
}

/**
 * Revoke one of your own sessions.
 *
 * Deliberately not optimistic. An optimistic update would take the row out of
 * the table before the server had agreed, which on this screen means showing
 * somebody that a browser they do not recognise has been signed out when it may
 * not have been. Nothing security-relevant is drawn ahead of the server.
 */
export function useRevokeSession(): UseMutationResult<void, Error, { sessionId: string }> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ sessionId }) => {
      await api.DELETE('/auth/sessions/{sessionId}', { params: { path: { sessionId } } })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: authKeys.sessions() })
    },
  })
}

/** Sign out of every browser but this one. */
export function useRevokeOtherSessions(): UseMutationResult<
  components['schemas']['RevokedSessions'],
  Error,
  void
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/auth/sessions/revoke-others')),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: authKeys.sessions() })
    },
  })
}

/**
 * Whether this person must hold a second factor and has not enrolled one — the
 * state that confines a session to enrolling and nothing else (M1-008).
 *
 * One function, used by the route guard and by the account screen, so that
 * "blocked" and "we are telling you why you are blocked" cannot disagree.
 */
export function mustEnrol(user: CurrentUser): boolean {
  return user.mfa.required && !user.mfa.enrolled
}

/** Whether this person administers the installation. */
export function isAdmin(user: CurrentUser): boolean {
  return user.platformRole === 'admin'
}
