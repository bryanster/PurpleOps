import { useInfiniteQuery, type UseInfiniteQueryResult } from '@tanstack/react-query'

import { api, unwrap } from '@/api/client'
import type { components } from '@/api/schema'

import { engagementKeys } from './queries'

export type ActivityEntry = components['schemas']['ActivityEntry']

const PAGE_SIZE = 30

export interface ActivityFilters {
  verb?: string
}

export function useEngagementActivity(
  engagementId: string | undefined,
  filters: ActivityFilters = {},
): UseInfiniteQueryResult<{ pages: components['schemas']['ActivityPage'][] }> {
  return useInfiniteQuery({
    queryKey: engagementKeys.activity(engagementId ?? '', filters),
    queryFn: async ({ pageParam, signal }) => {
      if (!engagementId) throw new Error('engagementId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/activity', {
          params: {
            path: { engagementId },
            query: {
              limit: PAGE_SIZE,
              ...(pageParam === undefined ? {} : { cursor: pageParam }),
              ...(filters.verb === undefined || filters.verb === '' ? {} : { verb: filters.verb }),
            },
          },
          signal,
        }),
      )
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.nextCursor ?? undefined,
    enabled: engagementId !== undefined,
  })
}
