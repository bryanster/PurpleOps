import { describe, expect, test } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import type { components } from '@/api/schema'
import {
  adminUserFixture,
  get,
  memberUserFixture,
} from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { EngagementCtx, type EngagementContextValue } from './engagement-layout'
import { engagementWorkbookPath } from './paths'
import { WorkbookPage } from './workbook-page'

const ENGAGEMENT_ID = '0192a000-0000-7000-8000-000000000001'

// ── Fixtures ──────────────────────────────────────────────────────────────────

const scenarioFixture: components['schemas']['Scenario'] = {
  id: '0192a000-0000-7000-8000-000000000002',
  engagementId: ENGAGEMENT_ID,
  ordinal: 1,
  name: 'Initial Access',
  narrative: '',
  source: 'manual',
  createdAt: '2025-12-02T00:00:00Z',
  updatedAt: '2025-12-02T00:00:00Z',
}

const revealedStep: components['schemas']['Step'] = {
  id: '0192a000-0000-7000-8000-000000000003',
  scenarioId: scenarioFixture.id,
  ordinal: 1,
  name: 'Phishing',
  objective: 'Get initial access',
  templateId: '',
  targetAsset: '',
  attackVersion: '15.1',
  revealedAt: '2025-12-03T00:00:00Z',
  createdAt: '2025-12-02T00:00:00Z',
  updatedAt: '2025-12-03T00:00:00Z',
}

const unrevealedStep: components['schemas']['Step'] = {
  id: '0192a000-0000-7000-8000-000000000004',
  scenarioId: scenarioFixture.id,
  ordinal: 2,
  name: 'Persistence',
  objective: 'Maintain access',
  templateId: '',
  targetAsset: '',
  attackVersion: '15.1',
  createdAt: '2025-12-02T00:00:00Z',
  updatedAt: '2025-12-02T00:00:00Z',
}

function executionFor(stepId: string, id: string): components['schemas']['Execution'] {
  return {
    id,
    stepId,
    version: 1,
    status: 'pending',
    executedBy: '',
    commandRun: '',
    sourceHost: '',
    targetHost: '',
    redNotes: '',
    detectionModifiers: [],
    detectingSource: '',
    detectingRuleRef: '',
    alertSeverity: '',
    blueNotes: '',
    scoredBy: '',
    createdAt: '2025-12-02T00:00:00Z',
    updatedAt: '2025-12-02T00:00:00Z',
  }
}

const blueUser: components['schemas']['CurrentUser'] = {
  ...memberUserFixture,
  id: '0192a000-0000-7000-8000-00000000000b',
  email: 'blue@example.com',
  displayName: 'Blue Op',
  memberships: [
    {
      engagementId: ENGAGEMENT_ID,
      role: 'blue',
      addedAt: '2025-12-01T00:00:00Z',
    },
  ],
}

const redUser: components['schemas']['CurrentUser'] = {
  ...memberUserFixture,
  id: '0192a000-0000-7000-8000-00000000000d',
  email: 'red@example.com',
  displayName: 'Red Op',
  memberships: [
    {
      engagementId: ENGAGEMENT_ID,
      role: 'red',
      addedAt: '2025-12-01T00:00:00Z',
    },
  ],
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function stubWorkbookData(): void {
  server.use(
    get('/engagements/{engagementId}/scenarios', () =>
      Response.json({ items: [scenarioFixture] }, { status: 200 }),
    ),
    get('/engagements/{engagementId}/steps', () =>
      Response.json(
        { items: [revealedStep, unrevealedStep] },
        { status: 200 },
      ),
    ),
    get('/engagements/{engagementId}/executions', () =>
      Response.json(
        {
          items: [
            executionFor(revealedStep.id, '0192a000-0000-7000-8000-000000000005'),
            executionFor(unrevealedStep.id, '0192a000-0000-7000-8000-000000000006'),
          ],
        },
        { status: 200 },
      ),
    ),
  )
}

function renderWorkbook(
  user: components['schemas']['CurrentUser'],
  contextRole: components['schemas']['EngagementRole'],
): ReturnType<typeof renderWithProviders> {
  const ctx: EngagementContextValue = {
    engagementId: ENGAGEMENT_ID,
    role: contextRole,
    closed: false,
  }
  return renderWithProviders(
    <EngagementCtx value={ctx}>
      <WorkbookPage />
    </EngagementCtx>,
    {
      user,
      route: engagementWorkbookPath(ENGAGEMENT_ID),
    },
  )
}

/** Click the scenario header to expand it and reveal its steps. */
async function expandFirstScenario(): Promise<void> {
  const header = await screen.findByText(/Initial Access/)
  await userEvent.click(header)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('WorkbookPage — red sees all steps', () => {
  test('shows revealed step name', async () => {
    stubWorkbookData()
    renderWorkbook(redUser, 'red')
    await expandFirstScenario()

    expect(screen.getByText('Phishing')).toBeDefined()
  })

  test('shows unrevealed step name for red', async () => {
    stubWorkbookData()
    renderWorkbook(redUser, 'red')
    await expandFirstScenario()

    expect(screen.getByText('Persistence')).toBeDefined()
  })
  test('shows Reveal button for unrevealed step in drawer', async () => {
    stubWorkbookData()
    renderWorkbook(redUser, 'red')
    await expandFirstScenario()

    // Click the unrevealed step to open the execution drawer
    await userEvent.click(screen.getByText('Persistence'))

    expect(screen.getByRole('button', { name: /reveal to blue/i })).toBeDefined()
  })

  test('shows Add Scenario button for red', async () => {
    stubWorkbookData()
    renderWorkbook(redUser, 'red')
    await expandFirstScenario()

    expect(screen.getByRole('button', { name: /add scenario/i })).toBeDefined()
  })
})

describe('WorkbookPage — blind blue', () => {
  test('shows revealed step name', async () => {
    stubWorkbookData()
    renderWorkbook(blueUser, 'blue')
    await expandFirstScenario()

    expect(screen.getByText('Phishing')).toBeDefined()
  })

  test('hides unrevealed step name — shows placeholder', async () => {
    stubWorkbookData()
    renderWorkbook(blueUser, 'blue')
    await expandFirstScenario()

    expect(screen.getByText(/Unrevealed/i)).toBeDefined()
    expect(screen.queryByText('Persistence')).toBeNull()
  })

  test('no Reveal button for blue', async () => {
    stubWorkbookData()
    renderWorkbook(blueUser, 'blue')
    await expandFirstScenario()

    expect(screen.queryByRole('button', { name: /reveal/i })).toBeNull()
  })

  test('no Add Scenario button for blue', async () => {
    stubWorkbookData()
    renderWorkbook(blueUser, 'blue')
    await expandFirstScenario()

    expect(screen.queryByRole('button', { name: /add scenario/i })).toBeNull()
  })
})
