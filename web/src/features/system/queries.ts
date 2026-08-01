import { queryOptions, useQuery, type UseQueryResult } from '@tanstack/react-query'

import { api, unwrap } from '@/api/client'
import type { components } from '@/api/schema'

export type Version = components['schemas']['Version']
export type Health = components['schemas']['Health']
export type HealthState = components['schemas']['HealthState']

/** See `src/api/README.md`: one root per feature, built in one place. */
export const systemKeys = {
  all: ['system'] as const,
  version: () => [...systemKeys.all, 'version'] as const,
  health: () => [...systemKeys.all, 'health'] as const,
}

/**
 * Build identity of the server that served this page.
 *
 * It cannot change without the process restarting, which also reloads the SPA,
 * so it is never stale — hence `staleTime: Infinity` rather than the 30s
 * default.
 */
export function versionQueryOptions() {
  return queryOptions({
    queryKey: systemKeys.version(),
    queryFn: async ({ signal }) => unwrap(await api.GET('/version', { signal })),
    staleTime: Infinity,
  })
}

/**
 * Health of the server and its dependencies.
 *
 * A 503 is a health *report*, not a failed request: the body is the same
 * `Health` shape as the 200 and names the dependency that is down, which is the
 * only useful thing about it. `client.ts` leaves it alone for exactly this
 * reason, and it arrives here as `error` because it is not a 2xx.
 *
 * Short stale time: this is the screen someone opens when they suspect the
 * server is unwell, and a cached "everything is fine" from a minute ago is
 * worse than no answer.
 */
export function healthQueryOptions() {
  return queryOptions({
    queryKey: systemKeys.health(),
    queryFn: async ({ signal }) => {
      const result = await api.GET('/healthz', { signal })
      if (result.error && 'checks' in result.error) {
        return result.error
      }
      return unwrap(result)
    },
    staleTime: 5_000,
  })
}

export function useVersion(): UseQueryResult<Version> {
  return useQuery(versionQueryOptions())
}

export function useHealth(): UseQueryResult<Health> {
  return useQuery(healthQueryOptions())
}
