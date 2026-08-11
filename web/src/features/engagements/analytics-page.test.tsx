import { describe, expect, test } from 'vitest'
import { screen, waitFor } from '@testing-library/react'

import type { components } from '@/api/schema'
import { adminUserFixture, get } from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { EngagementCtx, type EngagementContextValue } from './engagement-layout'
import { AnalyticsPage } from './analytics-page'
import { engagementAnalyticsPath } from './paths'
import {
  COLOUR_RAMP,
  LEGEND_LABELS,
  NOT_ATTEMPTED_COLOUR,
  UNSCORED_COLOUR,
} from './analytics-queries'

const ENGAGEMENT_ID = '0192a000-0000-7000-8000-000000000099'

const ctx: EngagementContextValue = {
  engagementId: ENGAGEMENT_ID,
  role: 'lead',
  closed: false,
}

function renderAnalytics(): ReturnType<typeof renderWithProviders> {
  return renderWithProviders(
    <EngagementCtx value={ctx}>
      <AnalyticsPage />
    </EngagementCtx>,
    {
      user: adminUserFixture,
      route: engagementAnalyticsPath(ENGAGEMENT_ID),
    },
  )
}

const COVERAGE_PATH = '/engagements/{engagementId}/analytics/coverage'
const DISTRIBUTION_PATH = '/engagements/{engagementId}/analytics/distribution'
const MTTD_PATH = '/engagements/{engagementId}/analytics/mttd'
const BURNDOWN_PATH = '/engagements/{engagementId}/analytics/burndown'

function defaultCoverage(): components['schemas']['AnalyticsCoverage'] {
  return {
    techniques: {
      rows: [
        {
          techniqueId: 'T1059',
          name: 'Command and Scripting Interpreter',
          isSubtechnique: false,
          parentTechniqueId: '',
          matched: true,
          attempted: true,
          bestCategory: 'technique',
          bestCategoryOrdinal: 4,
          bestProtection: 'blocked',
          stepCount: 2,
        },
        {
          techniqueId: 'T1059.001',
          name: 'PowerShell',
          isSubtechnique: true,
          parentTechniqueId: 'T1059',
          matched: true,
          attempted: true,
          bestCategory: 'general',
          bestCategoryOrdinal: 2,
          bestProtection: 'partial',
          stepCount: 1,
        },
        {
          techniqueId: 'T1203',
          name: 'Exploitation for Client Execution',
          isSubtechnique: false,
          parentTechniqueId: '',
          matched: true,
          attempted: false,
          bestCategory: '',
          bestCategoryOrdinal: null,
          bestProtection: '',
          stepCount: 0,
        },
      ],
      attempted: 2,
      notAttempted: 1,
      matrix: 200,
      unmatched: 0,
    },
    tactics: {
      rows: [
        {
          tacticId: 'execution',
          tacticName: 'Execution',
          attemptedTechniques: 2,
          matrixTechniques: 14,
          categories: [
            { category: 'technique', count: 1 },
            { category: 'general', count: 1 },
          ],
        },
      ],
    },
    blindFiltered: false,
  }
}

function defaultDistribution(): components['schemas']['AnalyticsDistribution'] {
  return {
    category: {
      attempted: 3,
      buckets: [
        { label: 'technique', count: 1 },
        { label: 'general', count: 1 },
        { label: 'none', count: 1 },
      ],
    },
    protection: {
      attempted: 3,
      buckets: [
        { label: 'blocked', count: 1 },
        { label: 'partial', count: 1 },
        { label: 'not_blocked', count: 1 },
      ],
    },
    outcome: { attempted: 3, buckets: [] },
    modifier: { attempted: 3, buckets: [] },
    blindFiltered: false,
  }
}

function defaultMttd(): components['schemas']['AnalyticsMttd'] {
  return {
    p50: 120,
    p90: 360,
    max: 720,
    detectedCount: 3,
    undetectedCount: 1,
    unscoredCount: 0,
    unmeasurableCount: 0,
    attemptedCount: 4,
    blindFiltered: false,
  }
}

function defaultBurndown(): components['schemas']['AnalyticsBurndown'] {
  return {
    interval: 'daily',
    points: [
      { date: '2026-10-01', open: 3, inProgress: 1, resolved: 0, acceptedRisk: 0, totalOpen: 4 },
      { date: '2026-10-02', open: 2, inProgress: 1, resolved: 1, acceptedRisk: 0, totalOpen: 3 },
      { date: '2026-10-03', open: 1, inProgress: 0, resolved: 2, acceptedRisk: 1, totalOpen: 1 },
    ],
    severity: {
      buckets: [
        { severity: 'high', open: 1, inProgress: 0, resolved: 0, acceptedRisk: 0, totalOpen: 1 },
        { severity: 'medium', open: 0, inProgress: 0, resolved: 1, acceptedRisk: 1, totalOpen: 0 },
      ],
    },
    blindFiltered: false,
  }
}

function stubAll(
  overrides: {
    coverage?: Partial<components['schemas']['AnalyticsCoverage']>
    distribution?: Partial<components['schemas']['AnalyticsDistribution']>
    mttd?: Partial<components['schemas']['AnalyticsMttd']>
    burndown?: Partial<components['schemas']['AnalyticsBurndown']>
  } = {},
): void {
  server.use(
    get(COVERAGE_PATH, () =>
      Response.json({ ...defaultCoverage(), ...overrides.coverage }, { status: 200 }),
    ),
    get(DISTRIBUTION_PATH, () =>
      Response.json({ ...defaultDistribution(), ...overrides.distribution }, { status: 200 }),
    ),
    get(MTTD_PATH, () => Response.json({ ...defaultMttd(), ...overrides.mttd }, { status: 200 })),
    get(BURNDOWN_PATH, () =>
      Response.json({ ...defaultBurndown(), ...overrides.burndown }, { status: 200 }),
    ),
  )
}

describe('AnalyticsPage', () => {
  test('renders scorecards when all data loads', async () => {
    stubAll()
    renderAnalytics()

    await screen.findByText('Analytics')
    await screen.findByText('1.0%')
    expect(screen.getByText(/2 of 200/)).toBeDefined()
    await screen.findByText('technique: 1')
    expect(screen.getByText('general: 1')).toBeDefined()
    expect(screen.getByText('66.7%')).toBeDefined()
    expect(screen.getByText('2m 0s')).toBeDefined()
    expect(screen.getByText(/3 detected/)).toBeDefined()
    expect(screen.getByText(/1 undetected/)).toBeDefined()
    await waitFor(() => {
      expect(screen.getByText(/2 closed/)).toBeDefined()
    })
  })

  test('shows empty state when no techniques scored', async () => {
    stubAll({
      coverage: {
        techniques: { rows: [], attempted: 0, notAttempted: 3, matrix: 200, unmatched: 0 },
      },
    })
    renderAnalytics()

    await waitFor(() => {
      expect(screen.getAllByText('Nothing scored yet').length).toBeGreaterThan(0)
    })
  })

  test('panels fail independently', async () => {
    const cov = defaultCoverage()
    const mttd = defaultMttd()
    const burn = defaultBurndown()
    server.use(
      get(COVERAGE_PATH, () => Response.json(cov, { status: 200 })),
      get(DISTRIBUTION_PATH, () => Response.json({ title: 'Server Error' }, { status: 500 })),
      get(MTTD_PATH, () => Response.json(mttd, { status: 200 })),
      get(BURNDOWN_PATH, () => Response.json(burn, { status: 200 })),
    )

    renderAnalytics()

    await screen.findByText('1.0%')
    await waitFor(() => {
      expect(screen.getAllByText('That request failed.').length).toBeGreaterThanOrEqual(1)
    })
    expect(screen.getByText('2m 0s')).toBeDefined()
  })

  test('renders blind banner when blindFiltered', async () => {
    stubAll({
      coverage: { blindFiltered: true },
      distribution: { blindFiltered: true },
      mttd: { blindFiltered: true },
      burndown: { blindFiltered: true },
    })
    renderAnalytics()

    await screen.findByRole('status', { name: 'Blind engagement notice' })
    expect(screen.getByText(/revealed steps only/)).toBeDefined()
  })

  test('no blind banner when not filtered', async () => {
    stubAll()
    renderAnalytics()

    await screen.findByText('Analytics')
    expect(screen.queryByRole('status', { name: 'Blind engagement notice' })).toBeNull()
  })

  test('heatmap renders with legend and technique cells', async () => {
    stubAll()
    renderAnalytics()

    await screen.findByText('ATT&CK Heatmap')
    // Legend labels — find within the heatmap section
    for (const label of LEGEND_LABELS) {
      await waitFor(() => {
        expect(screen.getAllByText(label).length).toBeGreaterThanOrEqual(1)
      })
    }
    expect(screen.getByText('Not attempted')).toBeDefined()
    expect(screen.getByText('Unscored')).toBeDefined()
    expect(screen.getByText('T1059')).toBeDefined()
    expect(screen.getByText('PowerShell')).toBeDefined()
  })

  test('burndown renders with interval', async () => {
    stubAll()
    renderAnalytics()

    await screen.findByText('Findings Burndown')
    await waitFor(() => {
      expect(screen.getByText('Daily burndown')).toBeDefined()
    })
    expect(screen.getByRole('img', { name: /burndown chart/ })).toBeDefined()
  })

  test('burndown empty state', async () => {
    stubAll({ burndown: { points: [] } })
    renderAnalytics()

    await screen.findByText('No findings history')
  })
})

describe('Colour ramp agreement', () => {
  test('COLOUR_RAMP matches docs/analytics.md values', () => {
    expect(COLOUR_RAMP).toEqual(['#aeb3bf', '#ffee58', '#fca128', '#d13c3c', '#862121'])
  })

  test('LEGEND_LABELS values', () => {
    expect(LEGEND_LABELS).toEqual(['None', 'Telemetry', 'General', 'Tactic', 'Technique'])
  })

  test('not-attempted and unscored are visually distinct from none', () => {
    expect(NOT_ATTEMPTED_COLOUR).not.toBe(COLOUR_RAMP[0])
    expect(UNSCORED_COLOUR).not.toBe(COLOUR_RAMP[0])
    expect(UNSCORED_COLOUR).not.toBe(NOT_ATTEMPTED_COLOUR)
  })
})

describe('MTTD required denominator', () => {
  test('AnalyticsMttd carries undetectedCount and detectedCount', () => {
    const mttd: components['schemas']['AnalyticsMttd'] = {
      detectedCount: 1,
      undetectedCount: 2,
      unscoredCount: 0,
      unmeasurableCount: 0,
      attemptedCount: 3,
      blindFiltered: false,
    }
    expect(mttd.undetectedCount).toBe(2)
    expect(mttd.detectedCount).toBe(1)
    expect(mttd.attemptedCount).toBe(3)
  })
})
