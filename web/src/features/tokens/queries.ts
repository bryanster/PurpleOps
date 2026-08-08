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

/** Your own service tokens (M1-011). Never anybody else's — there is no parameter for that. */
export type ServiceToken = components['schemas']['ServiceToken']
export type ServiceTokens = components['schemas']['ServiceTokens']
export type CreatedServiceToken = components['schemas']['CreatedServiceToken']
export type CreateServiceTokenRequest = components['schemas']['CreateServiceTokenRequest']
export type TokenScope = components['schemas']['TokenScope']

export const tokenKeys = {
  all: ['tokens'] as const,
  list: () => [...tokenKeys.all, 'list'] as const,
}

export function serviceTokensQueryOptions() {
  return queryOptions({
    queryKey: tokenKeys.list(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/auth/tokens', { signal })),
  })
}

export function useServiceTokens(): UseQueryResult<ServiceTokens> {
  return useQuery(serviceTokensQueryOptions())
}

/**
 * Mint a token.
 *
 * The response carries the only copy of the secret that will ever exist, and it
 * is returned to the caller rather than cached: putting it in the query cache
 * would leave a credential sitting in memory long after the dialog that showed
 * it had closed, and a devtools panel is not where a token should be findable.
 */
export function useCreateServiceToken(): UseMutationResult<
  CreatedServiceToken,
  Error,
  CreateServiceTokenRequest
> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body) => unwrap(await api.POST('/auth/tokens', { body })),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: tokenKeys.list() })
    },
  })
}

/** Revoke one of your own. Takes effect on that token's next request. */
export function useRevokeServiceToken(): UseMutationResult<void, Error, { tokenId: string }> {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ tokenId }) => {
      await api.DELETE('/auth/tokens/{tokenId}', { params: { path: { tokenId } } })
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: tokenKeys.list() })
    },
  })
}

/**
 * The scopes a token may carry, with what each one actually permits.
 *
 * The list is deliberately written out rather than derived from the generated
 * enum: the enum gives the words, and what a person choosing between them needs
 * is the sentence. `TokenScope` typing the keys is what keeps the two in step —
 * a scope added to `api/openapi.yaml` fails to compile here until it is
 * described.
 */
export const TOKEN_SCOPES: { scope: TokenScope; label: string; description: string }[] = [
  {
    scope: 'content:read',
    label: 'Read content',
    description: 'Read the shared technique and test-case library.',
  },
  {
    scope: 'content:sync',
    label: 'Sync content',
    description: 'Pull the library from its upstream sources. Administrators only.',
  },
  {
    scope: 'engagements:read',
    label: 'Read engagements',
    description: 'Read engagements you are a member of, and what is in them.',
  },
  {
    scope: 'engagements:write',
    label: 'Write engagements',
    description: 'Create engagements and record executions, within your own seat.',
  },
  {
    scope: 'reports:read',
    label: 'Read reports',
    description: 'Read and export reports for engagements you can see.',
  },
  {
    scope: 'reports:write',
    label: 'Publish reports',
    description: 'Publish a report, which is what makes it somebody else’s evidence.',
  },
  {
    scope: 'admin:read',
    label: 'Read administration',
    description: 'Read accounts, settings and the activity log. Administrators only.',
  },
  {
    scope: 'admin:write',
    label: 'Write administration',
    description: 'Change accounts and settings. Administrators only.',
  },
]
