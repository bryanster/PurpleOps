import {
  useInfiniteQuery,
  useMutation,
  useQueryClient,
  type UseInfiniteQueryResult,
  type UseMutationResult,
} from '@tanstack/react-query'

import { api, unwrap } from '@/api/client'
import type { components } from '@/api/schema'
import { authKeys } from '@/features/auth/queries'

/**
 * User administration and the activity feed (M1-015, M1-016).
 *
 * Both collections are cursor-paginated by the same convention — `limit` and
 * `cursor` in, `{items, nextCursor}` out — so both are infinite queries, and
 * the filters live in the query key exactly as `src/api/README.md` requires: a
 * key that omitted one would serve the previous filter's rows to whoever asked
 * second.
 */

export type User = components['schemas']['User']
export type UserStatus = components['schemas']['UserStatus']
export type PlatformRole = components['schemas']['PlatformRole']
export type CreatedUser = components['schemas']['CreatedUser']
export type CreateUserRequest = components['schemas']['CreateUserRequest']
export type UpdateUserRequest = components['schemas']['UpdateUserRequest']
export type ActivityEntry = components['schemas']['ActivityEntry']

/** How many rows a page asks for. The server caps this at 200. */
const PAGE_SIZE = 50

export interface UserFilters {
  q?: string
  status?: UserStatus
  role?: PlatformRole
}

export interface ActivityFilters {
  actorId?: string
  verb?: string
  objectType?: string
  objectId?: string
}

export const userKeys = {
  all: ['users'] as const,
  list: (filters: UserFilters) => [...userKeys.all, 'list', filters] as const,
}

export const activityKeys = {
  all: ['activity'] as const,
  list: (filters: ActivityFilters) => [...activityKeys.all, 'list', filters] as const,
}

export function useUsers(
  filters: UserFilters,
): UseInfiniteQueryResult<{ pages: components['schemas']['UserPage'][] }> {
  return useInfiniteQuery({
    queryKey: userKeys.list(filters),
    queryFn: async ({ pageParam, signal }) =>
      unwrap(
        await api.GET('/users', {
          params: {
            query: {
              limit: PAGE_SIZE,
              ...(pageParam === undefined ? {} : { cursor: pageParam }),
              ...(filters.q === undefined || filters.q === '' ? {} : { q: filters.q }),
              ...(filters.status === undefined ? {} : { status: filters.status }),
              ...(filters.role === undefined ? {} : { role: filters.role }),
            },
          },
          signal,
        }),
      ),
    initialPageParam: undefined as string | undefined,
    // `?? undefined`: the schema allows an explicit null for "no more pages",
    // and TanStack treats null and undefined differently — null would look like
    // a real cursor to a naive reading and fetch the first page forever.
    getNextPageParam: (last) => last.nextCursor ?? undefined,
  })
}

export function useActivity(
  filters: ActivityFilters,
): UseInfiniteQueryResult<{ pages: components['schemas']['ActivityPage'][] }> {
  return useInfiniteQuery({
    queryKey: activityKeys.list(filters),
    queryFn: async ({ pageParam, signal }) =>
      unwrap(
        await api.GET('/activity', {
          params: {
            query: {
              limit: PAGE_SIZE,
              ...(pageParam === undefined ? {} : { cursor: pageParam }),
              ...(filters.actorId === undefined || filters.actorId === ''
                ? {}
                : { actor: filters.actorId }),
              ...(filters.verb === undefined || filters.verb === '' ? {} : { verb: filters.verb }),
              ...(filters.objectType === undefined || filters.objectType === ''
                ? {}
                : { objectType: filters.objectType }),
              ...(filters.objectId === undefined || filters.objectId === ''
                ? {}
                : { objectId: filters.objectId }),
            },
          },
          signal,
        }),
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.nextCursor ?? undefined,
  })
}

export function useCreateUser(): UseMutationResult<CreatedUser, Error, CreateUserRequest> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body) => unwrap(await api.POST('/users', { body })),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: userKeys.all })
    },
  })
}

/**
 * Edit an account.
 *
 * Nothing here is optimistic. A role change is a privilege change, and drawing
 * one before the server has agreed would show an administrator a promotion that
 * may have been refused — the last-admin guard answers 409 (M1-016), and that
 * is precisely the case an optimistic update would paper over.
 */
export function useUpdateUser(): UseMutationResult<
  User,
  Error,
  { userId: string; patch: UpdateUserRequest }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ userId, patch }) =>
      unwrap(await api.PATCH('/users/{userId}', { params: { path: { userId } }, body: patch })),
    onSuccess: async (_user, { userId }) => {
      await invalidateAccount(queryClient, userId)
    },
  })
}

export function useDisableUser(): UseMutationResult<User, Error, { userId: string }> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ userId }) =>
      unwrap(await api.POST('/users/{userId}/disable', { params: { path: { userId } } })),
    onSuccess: async (_user, { userId }) => {
      await invalidateAccount(queryClient, userId)
    },
  })
}

export function useEnableUser(): UseMutationResult<User, Error, { userId: string }> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ userId }) =>
      unwrap(await api.POST('/users/{userId}/enable', { params: { path: { userId } } })),
    onSuccess: async (_user, { userId }) => {
      await invalidateAccount(queryClient, userId)
    },
  })
}

export function useRevokeUserSessions(): UseMutationResult<
  components['schemas']['RevokedSessions'],
  Error,
  { userId: string }
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ userId }) =>
      unwrap(await api.POST('/users/{userId}/sessions/revoke', { params: { path: { userId } } })),
    onSuccess: async (_result, { userId }) => {
      await invalidateAccount(queryClient, userId)
    },
  })
}

/**
 * What to refetch after an account changed.
 *
 * The user list, obviously. The activity feed, because every one of these
 * writes a row to it and an administrator watching both expects them to agree.
 * And `auth/me` when the account is the caller's own — an administrator can
 * edit themselves, and a stale answer there is an interface showing controls
 * the server would now refuse.
 */
async function invalidateAccount(
  queryClient: ReturnType<typeof useQueryClient>,
  _userId: string,
): Promise<void> {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: userKeys.all }),
    queryClient.invalidateQueries({ queryKey: activityKeys.all }),
    queryClient.invalidateQueries({ queryKey: authKeys.me() }),
  ])
}
