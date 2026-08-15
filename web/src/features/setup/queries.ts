import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
  type UseMutationResult,
  type UseQueryResult,
} from '@tanstack/react-query'

import { api, unwrap } from '@/api/client'
import type { components } from '@/api/schema'
import { contentKeys } from '@/features/content/queries'
import { sourceKeys } from '@/features/content/sources-queries'

/**
 * Everything the first-run wizard reads and writes.
 *
 * Two of the three calls are borrowed rather than new — enabling the ATT&CK
 * source and starting a sync are the same operations the sources admin screen
 * uses, and the wizard is a friendlier path to them rather than a second
 * implementation of them. What is only here is the setup state itself and the
 * release catalog the picker is built on.
 */

export type SetupState = components['schemas']['SetupState']
export type ContentAttackRelease = components['schemas']['ContentAttackRelease']
export type ContentAttackReleaseList = components['schemas']['ContentAttackReleaseList']

export const setupKeys = {
  all: ['setup'] as const,
  state: () => [...setupKeys.all, 'state'] as const,
  releases: () => [...contentKeys.all, 'attack-releases'] as const,
}

/**
 * Whether this installation has been set up.
 *
 * `staleTime: 0` and no retry, for the same reason `GET /auth/me` has them:
 * this answer decides which screen the whole interface shows, and a cached
 * "not yet" after somebody finished the wizard is a wizard that will not go
 * away. A 403 is an answer too — a member has no business reading it and the
 * guard treats "cannot tell" as "do not redirect".
 */
export function setupStateQueryOptions() {
  return queryOptions({
    queryKey: setupKeys.state(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/setup', { signal })),
    staleTime: 0,
    retry: false,
  })
}

export function useSetupState(enabled = true): UseQueryResult<SetupState> {
  return useQuery({ ...setupStateQueryOptions(), enabled })
}

/**
 * The ATT&CK releases upstream offers, next to what is installed.
 *
 * This reaches the internet while the request is open, so it is slow in the
 * way a network call is slow and it is not refetched on a whim. An unreachable
 * upstream comes back as a 200 saying so — see the operation description — so
 * `error` here means the request itself failed, not that the picker is empty.
 */
export function attackReleasesQueryOptions() {
  return queryOptions({
    queryKey: setupKeys.releases(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/content/attack/releases', { signal })),
    staleTime: 60_000,
    retry: false,
  })
}

export function useAttackReleases(): UseQueryResult<ContentAttackReleaseList> {
  return useQuery(attackReleasesQueryOptions())
}

/**
 * Finish the wizard.
 *
 * Invalidates the setup state so the guard stops redirecting, and the content
 * queries with it: whoever is about to land in the application should see the
 * library the wizard just filled rather than the empty one it started from.
 */
export function useCompleteSetup(): UseMutationResult<SetupState, Error, void> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async () => unwrap(await api.POST('/setup/complete', {})),
    onSuccess: async (state) => {
      queryClient.setQueryData(setupKeys.state(), state)
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: setupKeys.state() }),
        queryClient.invalidateQueries({ queryKey: contentKeys.all }),
        queryClient.invalidateQueries({ queryKey: sourceKeys.all }),
      ])
    },
  })
}
