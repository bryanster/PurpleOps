import { queryOptions, useQuery, type UseQueryResult } from '@tanstack/react-query'

import { api, unwrap } from '@/api/client'
import type { components } from '@/api/schema'

import { engagementKeys } from './queries'

// ── Types ────────────────────────────────────────────────────────────────────

export type AnalyticsCoverage = components['schemas']['AnalyticsCoverage']
export type AnalyticsDistribution = components['schemas']['AnalyticsDistribution']
export type AnalyticsMttd = components['schemas']['AnalyticsMttd']
export type AnalyticsBurndown = components['schemas']['AnalyticsBurndown']
export type TechniqueCoverage = components['schemas']['TechniqueCoverage']
export type TechniqueCoverageRow = components['schemas']['TechniqueCoverageRow']
export type TacticCoverageRow = components['schemas']['TacticCoverageRow']
export type DistributionBucket = components['schemas']['DistributionBucket']
export type DistributionResult = components['schemas']['DistributionResult']
export type BurndownPoint = components['schemas']['BurndownPoint']
export type SeverityBucket = components['schemas']['SeverityBucket']
export type AnalyticsCompare = components['schemas']['AnalyticsCompare']
export type CompareRow = components['schemas']['CompareRow']
export type PinMismatch = components['schemas']['PinMismatch']

// ── Colour ramp ──────────────────────────────────────────────────────────────

/**
 * Detection-category colour ramp matching
 * [internal/analytics.NavigatorColourRamp] and `docs/analytics.md`.
 * One ramp, two renderers — heatmap and Navigator layer must not disagree.
 */
export const COLOUR_RAMP = [
  '#aeb3bf', // 0 — none: grey
  '#ffee58', // 1 — telemetry: amber
  '#fca128', // 2 — general: orange
  '#d13c3c', // 3 — tactic: red
  '#862121', // 4 — technique: dark red
] as const

export const LEGEND_LABELS = ['None', 'Telemetry', 'General', 'Tactic', 'Technique'] as const

/** Colour for not-attempted cells. Unscored and not-attempted are visually distinct from `none`. */
export const NOT_ATTEMPTED_COLOUR = '#e8e8e8'
export const UNSCORED_COLOUR = '#f5f5f5'
export const NOT_ATTEMPTED_LABEL = 'Not attempted'
export const UNSCORED_LABEL = 'Unscored'

// ── Query options (importable by tests) ──────────────────────────────────────

export function analyticsCoverageQueryOptions(engagementId: string | undefined) {
  return queryOptions({
    queryKey: engagementKeys.analyticsCoverage(engagementId ?? ''),
    queryFn: async ({ signal }) => {
      if (!engagementId) throw new Error('engagementId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/analytics/coverage', {
          params: { path: { engagementId } },
          signal,
        }),
      )
    },
    enabled: engagementId !== undefined && engagementId !== '',
  })
}

export function analyticsDistributionQueryOptions(engagementId: string | undefined) {
  return queryOptions({
    queryKey: engagementKeys.analyticsDistribution(engagementId ?? ''),
    queryFn: async ({ signal }) => {
      if (!engagementId) throw new Error('engagementId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/analytics/distribution', {
          params: { path: { engagementId } },
          signal,
        }),
      )
    },
    enabled: engagementId !== undefined && engagementId !== '',
  })
}

export function analyticsMttdQueryOptions(engagementId: string | undefined) {
  return queryOptions({
    queryKey: engagementKeys.analyticsMttd(engagementId ?? ''),
    queryFn: async ({ signal }) => {
      if (!engagementId) throw new Error('engagementId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/analytics/mttd', {
          params: { path: { engagementId } },
          signal,
        }),
      )
    },
    enabled: engagementId !== undefined && engagementId !== '',
  })
}

export function analyticsBurndownQueryOptions(engagementId: string | undefined) {
  return queryOptions({
    queryKey: engagementKeys.analyticsBurndown(engagementId ?? ''),
    queryFn: async ({ signal }) => {
      if (!engagementId) throw new Error('engagementId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/analytics/burndown', {
          params: { path: { engagementId } },
          signal,
        }),
      )
    },
    enabled: engagementId !== undefined && engagementId !== '',
  })
}

// ── Hooks ────────────────────────────────────────────────────────────────────

export function useAnalyticsCoverage(
  engagementId: string | undefined,
): UseQueryResult<AnalyticsCoverage> {
  return useQuery(analyticsCoverageQueryOptions(engagementId))
}

export function useAnalyticsDistribution(
  engagementId: string | undefined,
): UseQueryResult<AnalyticsDistribution> {
  return useQuery(analyticsDistributionQueryOptions(engagementId))
}

export function useAnalyticsMttd(engagementId: string | undefined): UseQueryResult<AnalyticsMttd> {
  return useQuery(analyticsMttdQueryOptions(engagementId))
}

export function useAnalyticsBurndown(
  engagementId: string | undefined,
): UseQueryResult<AnalyticsBurndown> {
  return useQuery(analyticsBurndownQueryOptions(engagementId))
}

export function analyticsCompareQueryOptions(
  engagementId: string | undefined,
  baselineId: string | undefined,
) {
  return queryOptions({
    queryKey: engagementKeys.analyticsCompare(engagementId ?? '', baselineId ?? ''),
    queryFn: async ({ signal }) => {
      if (!engagementId || !baselineId) throw new Error('engagementId and baselineId required')
      return unwrap(
        await api.GET('/engagements/{engagementId}/analytics/compare', {
          params: {
            path: { engagementId },
            query: { baseline: baselineId },
          },
          signal,
        }),
      )
    },
    enabled: engagementId !== undefined && baselineId !== undefined,
  })
}

export function useAnalyticsCompare(
  engagementId: string | undefined,
  baselineId: string | undefined,
): UseQueryResult<AnalyticsCompare> {
  return useQuery(analyticsCompareQueryOptions(engagementId, baselineId))
}
