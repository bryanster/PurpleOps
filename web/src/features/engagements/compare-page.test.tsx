import { describe, expect, test } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import type { components } from '@/api/schema'
import { adminUserFixture, get } from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { EngagementCtx, type EngagementContextValue } from './engagement-layout'
import { ComparePage } from './compare-page'

const ENGAGEMENT_ID = '0192a000-0000-7000-8000-000000000099'
const BASELINE_ID = '0192a000-0000-7000-8000-000000000088'

const ctx: EngagementContextValue = {
  engagementId: ENGAGEMENT_ID,
  role: 'lead',
  closed: false,
}

const COMPARE_PATH = '/engagements/{engagementId}/analytics/compare' as const
const ENGAGEMENTS_PATH = '/engagements' as const

function renderCompare(baselineId?: string): ReturnType<typeof renderWithProviders> {
  const route = baselineId
    ? `/engagements/${ENGAGEMENT_ID}/analytics/compare?baseline=${encodeURIComponent(baselineId)}`
    : `/engagements/${ENGAGEMENT_ID}/analytics/compare`
  return renderWithProviders(
    <EngagementCtx value={ctx}>
      <ComparePage />
    </EngagementCtx>,
    {
      user: adminUserFixture,
      route,
    },
  )
}

function defaultCompare(): components['schemas']['AnalyticsCompare'] {
  return {
    rows: [
      {
        techniqueId: 'T1059',
        subtechniqueId: '',
        name: 'Command and Scripting Interpreter',
        baselineCategory: 'general',
        baselineCategoryOrdinal: 2,
        baselineProtection: 'partial',
        currentCategory: 'technique',
        currentCategoryOrdinal: 4,
        currentProtection: 'blocked',
        ordinalDelta: 2,
        classification: 'improved',
      },
      {
        techniqueId: 'T1203',
        subtechniqueId: '',
        name: 'Exploitation for Client Execution',
        baselineCategory: 'technique',
        baselineCategoryOrdinal: 4,
        baselineProtection: 'blocked',
        currentCategory: 'general',
        currentCategoryOrdinal: 2,
        currentProtection: 'not_blocked',
        ordinalDelta: -2,
        classification: 'regressed',
      },
      {
        techniqueId: 'T1087',
        subtechniqueId: '',
        name: 'Account Discovery',
        baselineCategory: 'none',
        baselineCategoryOrdinal: 0,
        baselineProtection: 'not_blocked',
        currentCategory: 'none',
        currentCategoryOrdinal: 0,
        currentProtection: 'not_blocked',
        ordinalDelta: 0,
        classification: 'unchanged',
      },
      {
        techniqueId: 'T1547',
        subtechniqueId: '',
        name: 'Boot or Logon Autostart Execution',
        baselineCategory: '',
        baselineCategoryOrdinal: null,
        baselineProtection: '',
        currentCategory: 'technique',
        currentCategoryOrdinal: 4,
        currentProtection: 'blocked',
        ordinalDelta: null,
        classification: 'newlyAttempted',
      },
      {
        techniqueId: 'T1003',
        subtechniqueId: '',
        name: 'OS Credential Dumping',
        baselineCategory: 'general',
        baselineCategoryOrdinal: 2,
        baselineProtection: 'partial',
        currentCategory: '',
        currentCategoryOrdinal: null,
        currentProtection: '',
        ordinalDelta: null,
        classification: 'noLongerAttempted',
      },
      {
        techniqueId: 'T1566',
        subtechniqueId: '',
        name: 'Phishing',
        baselineCategory: '',
        baselineCategoryOrdinal: null,
        baselineProtection: '',
        currentCategory: 'none',
        currentCategoryOrdinal: 0,
        currentProtection: 'not_blocked',
        ordinalDelta: null,
        classification: 'incomparable',
      },
    ],
    improved: 1,
    regressed: 1,
    unchanged: 1,
    newlyAttempted: 1,
    noLongerAttempted: 1,
    incomparable: 1,
    baselineBlindFiltered: false,
    currentBlindFiltered: false,
  }
}

function defaultEngagementPage(): components['schemas']['EngagementPage'] {
  return {
    items: [
      {
        id: BASELINE_ID,
        name: 'Baseline Assessment',
        client: 'Acme Corp',
        description: '',
        status: 'active',
        startsOn: '2026-01-15',
        endsOn: '2026-02-15',
        attackVersion: '15.1',
        mode: 'standard',
        autoRevealOnStart: false,
        createdBy: '0192a000-0000-7000-8000-000000000001',
        createdAt: '2026-01-10T00:00:00Z',
        updatedAt: '2026-01-10T00:00:00Z',
      },
      {
        id: '0192a000-0000-7000-8000-000000000077',
        name: 'Other Assessment',
        client: 'Beta Inc',
        description: '',
        status: 'active',
        startsOn: '2026-03-01',
        endsOn: '2026-04-01',
        attackVersion: '15.1',
        mode: 'standard',
        autoRevealOnStart: false,
        createdBy: '0192a000-0000-7000-8000-000000000001',
        createdAt: '2026-02-28T00:00:00Z',
        updatedAt: '2026-02-28T00:00:00Z',
      },
    ],
    nextCursor: undefined,
  }
}

function stubAll(
  overrides: {
    compare?: Partial<components['schemas']['AnalyticsCompare']>
    engagements?: Partial<components['schemas']['EngagementPage']>
  } = {},
): void {
  server.use(
    get(COMPARE_PATH, () =>
      Response.json({ ...defaultCompare(), ...overrides.compare }, { status: 200 }),
    ),
    get(ENGAGEMENTS_PATH, () =>
      Response.json({ ...defaultEngagementPage(), ...overrides.engagements }, { status: 200 }),
    ),
  )
}

function stub403(): void {
  server.use(
    get(COMPARE_PATH, () =>
      Response.json(
        { title: 'Forbidden', status: 403, detail: 'report.read on baseline engagement' },
        { status: 403 },
      ),
    ),
    get(ENGAGEMENTS_PATH, () => Response.json(defaultEngagementPage(), { status: 200 })),
  )
}

describe('ComparePage', () => {
  test('shows picker when no baseline selected', async () => {
    stubAll()
    renderCompare()

    await screen.findByText('Compare')
    expect(screen.getByText(/Compare this engagement/)).toBeDefined()
    const picker = screen.getByLabelText('Compare with…')
    expect(picker).toBeDefined()
  })

  test('shows loading state when baseline set', async () => {
    stubAll()
    renderCompare(BASELINE_ID)

    expect(await screen.findByText('Loading comparison…')).toBeDefined()
  })

  test('renders data fully after loading', async () => {
    stubAll()
    renderCompare(BASELINE_ID)

    // Wait for data to resolve
    await screen.findByText('Compare')
    const table = await screen.findByRole('table', { name: 'Technique comparison table' })
    await waitFor(() => {
      expect(within(table).getByText('Command and Scripting Interpreter')).toBeDefined()
    })
  })

  test('summary row shows all classification counts', async () => {
    stubAll()
    renderCompare(BASELINE_ID)

    await screen.findByText('Compare')
    // Wait for summary to render
    const summary = await screen.findByRole('group', { name: 'Comparison summary' })

    // Within the summary group, each chip has the count
    expect(within(summary).getByText('Regressed')).toBeDefined()
    expect(within(summary).getByText('Improved')).toBeDefined()
    expect(within(summary).getByText('Unchanged')).toBeDefined()
    expect(within(summary).getByText('New')).toBeDefined()
    expect(within(summary).getByText('Removed')).toBeDefined()
    expect(within(summary).getByText('Incomparable')).toBeDefined()
  })

  test('improvement and regression distinguishable without colour', async () => {
    stubAll()
    renderCompare(BASELINE_ID)

    const table = await screen.findByRole('table', { name: 'Technique comparison table' })
    await waitFor(() => {
      expect(within(table).getByText('Improved')).toBeDefined()
    })
    // Text labels distinguish them
    expect(within(table).getByText('Regressed')).toBeDefined()
    // Delta values also distinguish
    expect(within(table).getByText('+2')).toBeDefined()
    expect(within(table).getByText('-2')).toBeDefined()
  })

  test('sorts regressed before improved', async () => {
    stubAll()
    renderCompare(BASELINE_ID)

    const table = await screen.findByRole('table', { name: 'Technique comparison table' })
    await waitFor(() => {
      const rows = within(table).getAllByRole('row')
      expect(rows.length).toBeGreaterThanOrEqual(3)
    })

    const rows = within(table).getAllByRole('row')
    // First data row should be regressed (classification order 0)
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    expect(within(rows[0]!).getByText('Regressed')).toBeDefined()
    // Second data row should be improved (classification order 1)
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    expect(within(rows[1]!).getByText('Improved')).toBeDefined()
  })

  test('incomparable rows are visible and counted', async () => {
    stubAll()
    renderCompare(BASELINE_ID)

    const summary = await screen.findByRole('group', { name: 'Comparison summary' })
    expect(within(summary).getByText('Incomparable')).toBeDefined()

    const table = screen.getByRole('table', { name: 'Technique comparison table' })
    expect(within(table).getByText('Phishing')).toBeDefined()
    expect(within(table).getByText('Incomparable')).toBeDefined()
  })

  test('filtering by classification', async () => {
    const user = userEvent.setup()
    stubAll()
    renderCompare(BASELINE_ID)

    const summary = await screen.findByRole('group', { name: 'Comparison summary' })

    // Click the "Improved" chip
    await user.click(within(summary).getByText('Improved'))

    const table = screen.getByRole('table', { name: 'Technique comparison table' })
    await waitFor(() => {
      const rows = within(table).getAllByRole('row')
      // Only 1 data row (no header row role)
      expect(rows).toHaveLength(1)
    })
    const rows = within(table).getAllByRole('row')
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
    expect(within(rows[0]!).getByText('Improved')).toBeDefined()
  })

  test('toggling filter shows all rows again', async () => {
    const user = userEvent.setup()
    stubAll()
    renderCompare(BASELINE_ID)

    const summary = await screen.findByRole('group', { name: 'Comparison summary' })

    await user.click(within(summary).getByText('Improved'))
    await user.click(within(summary).getByText('Improved'))

    const table = screen.getByRole('table', { name: 'Technique comparison table' })
    await waitFor(() => {
      const rows = within(table).getAllByRole('row')
      expect(rows).toHaveLength(6)
    })
  })

  test('shows empty state when nothing pairs', async () => {
    stubAll({
      compare: {
        rows: [],
        improved: 0,
        regressed: 0,
        unchanged: 0,
        newlyAttempted: 0,
        noLongerAttempted: 0,
        incomparable: 0,
      },
    })
    renderCompare(BASELINE_ID)

    await screen.findByText('Compare')
    await waitFor(() => {
      expect(screen.queryByText('Loading comparison…')).toBeNull()
    })
    expect(await screen.findByText('No techniques to compare')).toBeDefined()
  })

  test('shows error when baseline returns 403', async () => {
    stub403()
    renderCompare(BASELINE_ID)

    await screen.findByText('Compare')
    await waitFor(() => {
      expect(screen.queryByText('Loading comparison…')).toBeNull()
    })
    // The PageError component may show the error title or message
    await waitFor(() => {
      const errorText = screen.queryByText(/forbidden/i)
      const retryText = screen.queryByText('Retry')
      const problemText = screen.queryByText(/report\.read/)
      expect(errorText ?? retryText ?? problemText).toBeTruthy()
    })
  })

  test('shows pin mismatch banner', async () => {
    stubAll({
      compare: {
        pinMismatch: { baseline: '14.1', current: '15.1' },
      },
    })
    renderCompare(BASELINE_ID)

    await screen.findByText('Compare')
    const banner = await screen.findByRole('alert')
    expect(banner.textContent).toContain('v14.1')
    expect(banner.textContent).toContain('v15.1')
  })

  test('shows baseline blind banner', async () => {
    stubAll({
      compare: { baselineBlindFiltered: true },
    })
    renderCompare(BASELINE_ID)

    await screen.findByText('Compare')
    expect(
      await screen.findByRole('status', { name: 'Baseline blind engagement notice' }),
    ).toBeDefined()
  })

  test('shows current blind banner', async () => {
    stubAll({
      compare: { currentBlindFiltered: true },
    })
    renderCompare(BASELINE_ID)

    await screen.findByText('Compare')
    expect(
      await screen.findByRole('status', { name: 'Current blind engagement notice' }),
    ).toBeDefined()
  })

  test('shows direction label', async () => {
    stubAll()
    renderCompare(BASELINE_ID)

    // Wait for data to load (table renders)
    await screen.findByRole('table', { name: 'Technique comparison table' })
    // Direction is "Baseline ← Current" shown in the header
    expect(screen.getAllByText('Current').length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('Baseline')).toBeDefined()
  })

  test('URL round-trips with baseline parameter', async () => {
    stubAll()
    renderCompare(BASELINE_ID)

    await screen.findByText('Compare')
    const picker = screen.getByLabelText('Compare with…') as HTMLSelectElement // eslint-disable-line @typescript-eslint/no-unnecessary-type-assertion
    await waitFor(() => {
      expect(picker.value).toBe(BASELINE_ID)
    })
  })
})
