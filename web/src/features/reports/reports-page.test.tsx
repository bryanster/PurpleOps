import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Route, Routes } from 'react-router'
import { describe, expect, it } from 'vitest'

import type { components } from '@/api/schema'
import {
  EngagementCtx,
  type EngagementContextValue,
} from '@/features/engagements/engagement-layout'
import { adminUserFixture, del, get } from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { ReportsPage } from './reports-page'

const ENGAGEMENT_ID = '019385a2-9100-7000-8cf0-ef0123456789'

function report(overrides: Partial<components['schemas']['Report']>) {
  return {
    id: '019385a2-9100-7000-8cf0-ef0123450001',
    engagementId: ENGAGEMENT_ID,
    title: 'Q3 Assessment',
    clientName: null,
    logoBlobRef: null,
    colours: null,
    createdBy: adminUserFixture.id,
    createdAt: '2026-02-01T09:00:00Z',
    updatedBy: null,
    updatedAt: '2026-02-02T09:00:00Z',
    blockCount: 0,
    ...overrides,
  } satisfies components['schemas']['Report']
}

/** The list rows the page renders, as the list endpoint sends them: no blocks. */
const threeBlocks = report({ title: 'Q3 Assessment', blockCount: 3 })
const oneBlock = report({
  id: '019385a2-9100-7000-8cf0-ef0123450002',
  title: 'Retest Summary',
  blockCount: 1,
})
const noBlocks = report({
  id: '019385a2-9100-7000-8cf0-ef0123450003',
  title: 'Empty Draft',
  blockCount: 0,
})

function renderReports(items: components['schemas']['Report'][]): void {
  const ctx: EngagementContextValue = {
    engagementId: ENGAGEMENT_ID,
    role: 'lead',
    closed: false,
  }
  server.use(get('/engagements/{engagementId}/reports', () => Response.json(items)))
  renderWithProviders(
    <EngagementCtx value={ctx}>
      <Routes>
        <Route path="/engagements/:engagementId/reports" element={<ReportsPage />} />
      </Routes>
    </EngagementCtx>,
    { user: adminUserFixture, route: `/engagements/${ENGAGEMENT_ID}/reports` },
  )
}

describe('ReportsPage', () => {
  // The list response carries no `blocks` array, so the row counted an absent
  // field and every report read "0 blocks" however many it held.
  it('shows each row its own block count', async () => {
    renderReports([threeBlocks, oneBlock, noBlocks])

    expect(await screen.findByText('3 blocks')).toBeInTheDocument()
    // Singular for one, so the count is clearly per-row and not a constant.
    expect(screen.getByText('1 block')).toBeInTheDocument()
    expect(screen.getByText('0 blocks')).toBeInTheDocument()
  })

  it('deletes a report and drops it from the list', async () => {
    let deleted: string | undefined
    let listed = [threeBlocks, oneBlock]

    server.use(
      get('/engagements/{engagementId}/reports', () => Response.json(listed)),
      del('/engagements/{engagementId}/reports/{reportId}', ({ params }) => {
        deleted = String(params.reportId)
        listed = listed.filter((r) => r.id !== deleted)
        return new Response(null, { status: 204 })
      }),
    )

    const ctx: EngagementContextValue = {
      engagementId: ENGAGEMENT_ID,
      role: 'lead',
      closed: false,
    }
    renderWithProviders(
      <EngagementCtx value={ctx}>
        <Routes>
          <Route path="/engagements/:engagementId/reports" element={<ReportsPage />} />
        </Routes>
      </EngagementCtx>,
      { user: adminUserFixture, route: `/engagements/${ENGAGEMENT_ID}/reports` },
    )

    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Delete Q3 Assessment' }))

    const confirm = await screen.findByRole('alertdialog')
    await user.click(within(confirm).getByRole('button', { name: 'Delete' }))

    await waitFor(() => {
      expect(deleted).toBe(threeBlocks.id)
    })
    // The confirm dialog closes only from the mutation's onSuccess, so a
    // success wrongly read as a failure leaves it sitting open.
    await waitFor(() => {
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    })
    // By row link, not by text: the confirm dialog names the report too.
    await waitFor(() => {
      expect(screen.queryByRole('link', { name: 'Q3 Assessment' })).not.toBeInTheDocument()
    })
    // Awaited: the list re-fetches after the delete, so the surviving row
    // reappears a tick later rather than never having gone.
    expect(await screen.findByRole('link', { name: 'Retest Summary' })).toBeInTheDocument()
  })

  it('names what the delete takes with it', async () => {
    renderReports([threeBlocks])

    const user = userEvent.setup()
    await user.click(await screen.findByRole('button', { name: 'Delete Q3 Assessment' }))

    const confirm = await screen.findByRole('alertdialog')
    // The copy used to promise published versions survived. They cannot:
    // report_version has a RESTRICT foreign key to report.
    expect(within(confirm).getByText(/published versions/)).toBeInTheDocument()
    expect(within(confirm).queryByText(/unaffected/)).not.toBeInTheDocument()
  })
})
