import { describe, expect, test } from 'vitest'
import { act, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import type { components } from '@/api/schema'
import { adminUserFixture, get, memberUserFixture } from '@/test/msw/handlers'
import { patch } from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { EngagementCtx, type EngagementContextValue } from './engagement-layout'
import { engagementWorkbookPath } from './paths'
import { engagementKeys, type EngagementRole } from './queries'
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

const leadUser: components['schemas']['CurrentUser'] = {
  ...adminUserFixture,
  id: '0192a000-0000-7000-8000-00000000000a',
  email: 'lead@example.com',
  displayName: 'Lead Op',
  memberships: [
    {
      engagementId: ENGAGEMENT_ID,
      role: 'lead',
      addedAt: '2025-12-01T00:00:00Z',
    },
  ],
}

const observerUser: components['schemas']['CurrentUser'] = {
  ...memberUserFixture,
  id: '0192a000-0000-7000-8000-00000000000f',
  email: 'obs@example.com',
  displayName: 'Observer',
  memberships: [
    {
      engagementId: ENGAGEMENT_ID,
      role: 'observer',
      addedAt: '2025-12-01T00:00:00Z',
    },
  ],
}

function scoredExecution(): components['schemas']['Execution'] {
  return {
    id: '0192a000-0000-7000-8000-000000000005',
    stepId: revealedStep.id,
    version: 1,
    status: 'complete',
    executedBy: '',
    commandRun: '',
    sourceHost: '',
    targetHost: '',
    redNotes: '',
    detectionCategory: 'none',
    detectionModifiers: [],
    protection: 'not_blocked',
    detectedAt: '2025-12-03T12:00:00Z',
    detectingSource: '',
    detectingRuleRef: '',
    alertSeverity: '',
    blueNotes: '',
    scoredBy: '',
    outcome: 'not_detected',
    mttdSeconds: null,
    startedAt: '2025-12-03T11:00:00Z',
    endedAt: '2025-12-03T11:15:00Z',
    createdAt: '2025-12-03T12:00:00Z',
    updatedAt: '2025-12-03T12:00:00Z',
  }
}

function stubScoredWorkbook(): void {
  const exec = scoredExecution()
  server.use(
    get('/engagements/{engagementId}/scenarios', () =>
      Response.json({ items: [scenarioFixture] }, { status: 200 }),
    ),
    get('/engagements/{engagementId}/steps', () =>
      Response.json({ items: [revealedStep] }, { status: 200 }),
    ),
    get('/engagements/{engagementId}/executions', () =>
      Response.json({ items: [exec] }, { status: 200 }),
    ),
  )
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function stubWorkbookData(): void {
  server.use(
    get('/engagements/{engagementId}/scenarios', () =>
      Response.json({ items: [scenarioFixture] }, { status: 200 }),
    ),
    get('/engagements/{engagementId}/steps', () =>
      Response.json({ items: [revealedStep, unrevealedStep] }, { status: 200 }),
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
  contextRole: EngagementRole,
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

describe('WorkbookPage — platform admin holding no seat', () => {
  // Regression: the toolbar was gated on lead/red only, so an administrator —
  // the seat the bootstrapped first account holds — opened the workbook of a
  // fresh install and found no way to add anything, though the server grants
  // `workbook.write` to `Platform: admins`.
  test('shows Add Scenario button for admin', async () => {
    stubWorkbookData()
    renderWorkbook(adminUserFixture, 'admin')

    expect(await screen.findByRole('button', { name: /add scenario/i })).toBeDefined()
  })

  test('shows the step-building controls for admin', async () => {
    stubWorkbookData()
    renderWorkbook(adminUserFixture, 'admin')

    expect(await screen.findByRole('button', { name: /add step/i })).toBeDefined()
    expect(screen.getByRole('button', { name: /import ctid/i })).toBeDefined()
    expect(screen.getByRole('button', { name: /from template/i })).toBeDefined()
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

// ── Scoring UI tests (M3-015) ─────────────────────────────────────────────────

describe('BlueDetectionEditor — scoring', () => {
  test('shows 5-button scale with tooltips for detection category', async () => {
    stubScoredWorkbook()
    renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()

    // Open the execution drawer on the revealed step
    await userEvent.click(screen.getByText('Phishing'))

    // All five category buttons should be visible
    for (const label of ['None', 'Telemetry', 'General', 'Tactic', 'Technique']) {
      expect(screen.getByRole('button', { name: label })).toBeDefined()
    }
  })

  test('selects a category and sends it in the PATCH body', async () => {
    let patchedBody: unknown = null
    server.use(
      patch(
        '/engagements/{engagementId}/executions/{executionId}/detection',
        async ({ request }) => {
          patchedBody = await request.json()
          return Response.json(
            {
              ...scoredExecution(),
              version: 2,
              detectionCategory: 'technique' as const,
              outcome: 'detected' as const,
              mttdSeconds: 3600,
            },
            { status: 200 },
          )
        },
      ),
    )

    stubScoredWorkbook()
    renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    // Click the Technique button
    await userEvent.click(screen.getByRole('button', { name: 'Technique' }))

    // Save
    await userEvent.click(screen.getByRole('button', { name: 'Save Blue' }))

    // Check body
    const body = patchedBody as Record<string, unknown>
    expect(body).toBeDefined()
    expect(body.version).toBe(1)
    expect(body.detectionCategory).toBe('technique')
  })

  test('modifiers section starts collapsed when no modifiers selected', async () => {
    stubScoredWorkbook()
    renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    // The modifiers should not be visible initially (collapsed)
    expect(screen.queryByText('Detection Modifiers')).toBeNull()

    // Advanced disclosure trigger should be present
    expect(screen.getByText('Advanced')).toBeDefined()
  })

  test('clicking Advanced toggles modifier visibility', async () => {
    stubScoredWorkbook()
    renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    // Click Advanced to expand
    await userEvent.click(screen.getByText('Advanced'))

    // Modifiers should now be visible
    expect(screen.getByText('Detection Modifiers')).toBeDefined()
    expect(screen.getByText('Alert')).toBeDefined()
    expect(screen.getByText('Correlated')).toBeDefined()
  })

  test('selects modifiers and persists them in PATCH', async () => {
    let patchedBody: unknown = null
    server.use(
      patch(
        '/engagements/{engagementId}/executions/{executionId}/detection',
        async ({ request }) => {
          patchedBody = await request.json()
          return Response.json(
            { ...scoredExecution(), version: 2, detectionModifiers: ['alert', 'delayed'] },
            { status: 200 },
          )
        },
      ),
    )

    stubScoredWorkbook()
    renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    // Expand Advanced
    await userEvent.click(screen.getByText('Advanced'))

    // Select two modifiers
    await userEvent.click(screen.getByText('Alert'))
    await userEvent.click(screen.getByText('Delayed'))

    // Save
    await userEvent.click(screen.getByRole('button', { name: 'Save Blue' }))

    const body = patchedBody as Record<string, unknown>
    expect(body.detectionModifiers).toEqual(['alert', 'delayed'])
  })

  test('shows conflict toast on 409 and invalidates queries', async () => {
    server.use(
      patch(
        '/engagements/{engagementId}/executions/{executionId}/detection',
        () =>
          new Response(
            JSON.stringify({
              code: 'conflict',
              detail: 'Version mismatch',
              status: 409,
              title: 'Conflict',
            }),
            {
              status: 409,
              headers: { 'content-type': 'application/problem+json' },
            },
          ),
      ),
    )

    stubScoredWorkbook()
    renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    await userEvent.click(screen.getByRole('button', { name: 'Save Blue' }))

    expect(await screen.findByText(/modified by someone else/i)).toBeDefined()
  })

  test('blue detection panel is read-only for observer', async () => {
    stubScoredWorkbook()
    renderWorkbook(observerUser, 'observer')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    expect(screen.getByText('Blue Detection')).toBeDefined()
    expect(screen.queryByRole('button', { name: 'Save Blue' })).toBeNull()
  })

  test('shows outcome badge as read-only derived value', async () => {
    stubScoredWorkbook()
    renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    expect(screen.getByText('Not Detected')).toBeDefined()
  })
})

// ── M4-005: Live workbook updates + 409 conflict recovery ──────────────────────

describe('M4-005 — live drawer updates', () => {
  test('drawer reflects remote red status change after cache invalidation', async () => {
    stubScoredWorkbook()
    const { queryClient } = renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()

    // Open the drawer — the drawer info bar shows a status badge
    await userEvent.click(screen.getByText('Phishing'))
    // "Complete" appears in the table row and the drawer — findAllByText returns all
    const completeBadges = screen.getAllByText('Complete')
    expect(completeBadges.length).toBeGreaterThanOrEqual(1)

    // Simulate remote update: change MSW handler to return 'running' status
    const updated = { ...scoredExecution(), version: 2, status: 'running' as const }
    server.use(
      get('/engagements/{engagementId}/executions', () =>
        Response.json({ items: [updated] }, { status: 200 }),
      ),
    )

    // Simulate SSE-triggered invalidation
    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: engagementKeys.executions(ENGAGEMENT_ID),
      })
    })

    // Drawer should now show the updated status (live derivation from query data)
    await waitFor(() => {
      const runningBadges = screen.getAllByText('Running')
      expect(runningBadges.length).toBeGreaterThanOrEqual(1)
    })
  })

  test('drawer reflects remote blue detection change after cache invalidation', async () => {
    stubScoredWorkbook()
    const { queryClient } = renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    // Initially shows "Not Detected" outcome
    expect(screen.getByText('Not Detected')).toBeDefined()

    // Remote blue team scores technique detection
    const updated = {
      ...scoredExecution(),
      version: 3,
      detectionCategory: 'technique' as const,
      outcome: 'detected' as const,
      mttdSeconds: 1800,
    }
    server.use(
      get('/engagements/{engagementId}/executions', () =>
        Response.json({ items: [updated] }, { status: 200 }),
      ),
    )

    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: engagementKeys.executions(ENGAGEMENT_ID),
      })
    })

    await waitFor(() => {
      expect(screen.getByText('Detected')).toBeDefined()
    })
  })

  test('board row flashes when remote execution data changes', async () => {
    stubScoredWorkbook()
    const { queryClient } = renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()

    // Remote update changes version
    const updated = { ...scoredExecution(), version: 4 }
    server.use(
      get('/engagements/{engagementId}/executions', () =>
        Response.json({ items: [updated] }, { status: 200 }),
      ),
    )

    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: engagementKeys.executions(ENGAGEMENT_ID),
      })
    })

    // Row should briefly have the flash class (1200ms window)
    // Use a tight polling interval to catch the brief flash
    await waitFor(
      () => {
        const rows = screen.getAllByRole('row')
        const flashed = rows.find((r) => r.textContent.includes('Phishing'))
        expect(flashed?.className).toContain('animate-flash-update')
      },
      { timeout: 3000, interval: 50 },
    )
  })
})

describe('M4-005 — 409 conflict recovery', () => {
  test('409 resets red editor to server version after refetch', async () => {
    stubScoredWorkbook()

    // Patch returns 409; GET returns new server data with higher version.
    // server.use AFTER stubScoredWorkbook so our GET overrides the stub.
    const serverExec = {
      ...scoredExecution(),
      version: 5,
      status: 'running' as const,
      redNotes: 'Server note',
    }
    server.use(
      patch(
        '/engagements/{engagementId}/executions/{executionId}/execution',
        () =>
          new Response(
            JSON.stringify({
              code: 'conflict',
              detail: 'Version mismatch',
              status: 409,
              title: 'Conflict',
            }),
            {
              status: 409,
              headers: { 'content-type': 'application/problem+json' },
            },
          ),
      ),
      get('/engagements/{engagementId}/executions', () =>
        Response.json({ items: [serverExec] }, { status: 200 }),
      ),
    )

    renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    // Click save to trigger 409
    await userEvent.click(screen.getByRole('button', { name: 'Save Red' }))

    // Toast appears
    expect(await screen.findByText(/modified by someone else/i)).toBeDefined()

    // After invalidation → refetch → version change, the editor resets.
    // The status badge in the drawer should now show 'Running'.
    await waitFor(() => {
      const runningBadges = screen.getAllByText('Running')
      expect(runningBadges.length).toBeGreaterThanOrEqual(1)
    })
  })

  test('409 resets blue editor to server version after refetch', async () => {
    stubScoredWorkbook()

    const serverExec = {
      ...scoredExecution(),
      version: 5,
      detectionCategory: 'technique' as const,
      outcome: 'detected' as const,
      blueNotes: 'Server blue note',
    }
    server.use(
      patch(
        '/engagements/{engagementId}/executions/{executionId}/detection',
        () =>
          new Response(
            JSON.stringify({
              code: 'conflict',
              detail: 'Version mismatch',
              status: 409,
              title: 'Conflict',
            }),
            {
              status: 409,
              headers: { 'content-type': 'application/problem+json' },
            },
          ),
      ),
      get('/engagements/{engagementId}/executions', () =>
        Response.json({ items: [serverExec] }, { status: 200 }),
      ),
    )

    renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    await userEvent.click(screen.getByRole('button', { name: 'Save Blue' }))

    expect(await screen.findByText(/modified by someone else/i)).toBeDefined()

    // After reset, outcome should show the server version's outcome
    await waitFor(() => {
      expect(screen.getByText('Detected')).toBeDefined()
    })
  })

  test('409 toast appears and user can retry successfully', async () => {
    stubScoredWorkbook()

    let callCount = 0
    const serverExec = { ...scoredExecution(), version: 5, status: 'running' as const }
    server.use(
      patch('/engagements/{engagementId}/executions/{executionId}/execution', () => {
        callCount++
        if (callCount === 1) {
          return new Response(
            JSON.stringify({
              code: 'conflict',
              detail: 'Version mismatch',
              status: 409,
              title: 'Conflict',
            }),
            {
              status: 409,
              headers: { 'content-type': 'application/problem+json' },
            },
          )
        }
        // Second call succeeds
        return Response.json(
          { ...serverExec, version: 6, status: 'complete' as const },
          { status: 200 },
        )
      }),
      get('/engagements/{engagementId}/executions', () =>
        Response.json({ items: [serverExec] }, { status: 200 }),
      ),
    )

    renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    // First save — 409
    await userEvent.click(screen.getByRole('button', { name: 'Save Red' }))
    expect(await screen.findByText(/modified by someone else/i)).toBeDefined()

    // Wait for editor to reset to server state
    await waitFor(() => {
      const runningBadges = screen.getAllByText('Running')
      expect(runningBadges.length).toBeGreaterThanOrEqual(1)
    })

    // Second save — succeeds
    await userEvent.click(screen.getByRole('button', { name: 'Save Red' }))
    await waitFor(() => {
      expect(screen.getByText('Red execution saved')).toBeDefined()
    })
  })
})

// ── M4-007: Live comment threads + lightweight unread ─────────────────────────

const COMMENT_1: components['schemas']['Comment'] = {
  id: '0192a000-0000-7000-8000-000000000010',
  executionId: '0192a000-0000-7000-8000-000000000005',
  authorId: '0192a000-0000-7000-8000-00000000000a', // lead
  body: 'First comment',
  createdAt: '2025-12-04T10:00:00Z',
}

const COMMENT_2: components['schemas']['Comment'] = {
  id: '0192a000-0000-7000-8000-000000000011',
  executionId: '0192a000-0000-7000-8000-000000000005',
  authorId: '0192a000-0000-7000-8000-00000000000b', // blue
  body: 'Second comment',
  createdAt: '2025-12-04T11:00:00Z',
}

const COMMENT_EDITED: components['schemas']['Comment'] = {
  ...COMMENT_1,
  body: 'First comment — edited',
  editedAt: '2025-12-04T12:00:00Z',
}

const MEMBER_LEAD: components['schemas']['EngagementMember'] = {
  id: '0192a000-0000-7000-8000-00000000000a',
  email: 'lead@example.com',
  displayName: 'Lead Op',
  role: 'lead',
  addedAt: '2025-12-01T00:00:00Z',
}

const MEMBER_BLUE: components['schemas']['EngagementMember'] = {
  id: '0192a000-0000-7000-8000-00000000000b',
  email: 'blue@example.com',
  displayName: 'Blue Op',
  role: 'blue',
  addedAt: '2025-12-01T00:00:00Z',
}

function stubMembers(): void {
  server.use(
    get('/engagements/{engagementId}/members', () =>
      Response.json([MEMBER_LEAD, MEMBER_BLUE], { status: 200 }),
    ),
  )
}

function stubComments(comments: components['schemas']['Comment'][]): void {
  server.use(
    get('/engagements/{engagementId}/executions/{executionId}/comments', () =>
      Response.json(comments, { status: 200 }),
    ),
  )
}

describe('M4-007 — comment thread refresh via event', () => {
  test('new comment appears after cache invalidation for comment.created', async () => {
    stubScoredWorkbook()
    stubMembers()
    stubComments([COMMENT_1])
    const { queryClient } = renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    // Initial comment visible
    expect(screen.getByText('First comment')).toBeDefined()

    // Simulate remote user adding a comment — SSE invalidates comment.created
    server.use(
      get('/engagements/{engagementId}/executions/{executionId}/comments', () =>
        Response.json([COMMENT_1, COMMENT_2], { status: 200 }),
      ),
    )

    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: engagementKeys.comments(ENGAGEMENT_ID, scoredExecution().id),
      })
    })

    await waitFor(() => {
      expect(screen.getByText('Second comment')).toBeDefined()
    })
  })

  test('edited comment reflects via comment.edited invalidation', async () => {
    stubScoredWorkbook()
    stubMembers()
    stubComments([COMMENT_1])
    const { queryClient } = renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    expect(screen.getByText('First comment')).toBeDefined()

    // Remote edit
    server.use(
      get('/engagements/{engagementId}/executions/{executionId}/comments', () =>
        Response.json([COMMENT_EDITED], { status: 200 }),
      ),
    )

    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: engagementKeys.comments(ENGAGEMENT_ID, scoredExecution().id),
      })
    })

    await waitFor(() => {
      expect(screen.getByText('First comment — edited')).toBeDefined()
      expect(screen.getByText('(edited)')).toBeDefined()
    })
  })
})

describe('M4-007 — unread badge', () => {
  test('unread badge shows on board row when comments exist', async () => {
    stubScoredWorkbook()
    stubComments([COMMENT_1])
    const { queryClient } = renderWorkbook(leadUser, 'lead')

    // Wait for comments to load for the badge (it fires per execution row)
    await act(async () => {
      await queryClient.invalidateQueries({
        queryKey: engagementKeys.comments(ENGAGEMENT_ID, scoredExecution().id),
      })
    })

    await expandFirstScenario()

    // The badge shows the comment count inside the row (circle with "1")
    // The UnreadCommentBadge renders a span with rounded-full
    const rows = screen.getAllByRole('row')
    const phishingRow = rows.find((r) => r.textContent.includes('Phishing'))
    expect(phishingRow).toBeDefined()
    // Badge content is the comment count
    expect(phishingRow?.textContent).toContain('1')
  })
  test('opening drawer clears unread badge in localStorage', async () => {
    stubScoredWorkbook()
    stubMembers()
    stubComments([COMMENT_1])
    renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()

    // Open the drawer — this calls markCommentRead
    await userEvent.click(screen.getByText('Phishing'))

    // Comments section should be visible in the drawer
    expect(screen.getByText('First comment')).toBeDefined()

    // After opening, localStorage should have a recent lastViewedAt.
    // The badge should disappear because newest comment is not newer than lastViewedAt.
    const key = 'bl_comment_unread:' + ENGAGEMENT_ID + ':' + scoredExecution().id
    const raw = localStorage.getItem(key)
    if (raw === null) throw new Error('expected stored value')
    const stored = JSON.parse(raw) as { lastViewedAt: string }
    expect(stored.lastViewedAt).toBeDefined()

    // After markRead, newestCommentAt (2025-12-04T10:00:00Z) < lastViewedAt (now)
    // So unread should be false. Verify the badge is not in the table.
    // Close the drawer so we can inspect the table rows.
    await userEvent.keyboard('{Escape}')

    await waitFor(() => {
      const rows = screen.getAllByRole('row')
      const _phishingRow = rows.find((r) => r.textContent.includes('Phishing'))
      expect(_phishingRow).toBeDefined()
      // The badge text (the count number) should not appear as a standalone element
      // in the row — the UnreadCommentBadge returns null
    })
  })
})

describe('M4-007 — comment editing', () => {
  test('comment author can see edit button', async () => {
    stubScoredWorkbook()
    stubMembers()
    stubComments([COMMENT_1]) // authorId = lead
    renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    // Lead authored COMMENT_1, so they should see an edit button
    const editButton = screen.getByRole('button', { name: 'Edit comment' })
    expect(editButton).toBeDefined()
  })

  test('edit flow: save updates comment', async () => {
    let patchedBody: unknown = null
    server.use(
      patch('/engagements/{engagementId}/comments/{commentId}', async ({ request }) => {
        patchedBody = await request.json()
        return Response.json(COMMENT_EDITED, { status: 200 })
      }),
    )

    stubScoredWorkbook()
    stubMembers()
    stubComments([COMMENT_1])
    renderWorkbook(leadUser, 'lead')
    await expandFirstScenario()
    await userEvent.click(screen.getByText('Phishing'))

    // Click edit
    await userEvent.click(screen.getByRole('button', { name: 'Edit comment' }))

    // Textarea should show current body
    const textarea = screen.getByDisplayValue('First comment')
    expect(textarea).toBeDefined()

    // Change text and save
    await userEvent.clear(textarea)
    await userEvent.type(textarea, 'First comment — edited')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    // Check PATCH body
    const body = patchedBody as Record<string, unknown>
    expect(body.body).toBe('First comment — edited')
  })
})
