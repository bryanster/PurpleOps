import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import { EngagementCtx, type EngagementContextValue } from '@/features/engagements/engagement-layout'
import { renderWithProviders } from '@/test/render'

import { PublishDialog } from './publish-dialog'

const ENGAGEMENT_ID = '0192a000-0000-7000-8000-000000000001'

const REPORT = {
  id: '0192a000-0000-7000-8000-000000000002',
  engagementId: ENGAGEMENT_ID,
  title: 'Q3 Report',
  blocks: [],
  createdAt: '2025-12-01T00:00:00Z',
  updatedAt: '2025-12-01T00:00:00Z',
} as unknown as Parameters<typeof PublishDialog>[0]['report']

function renderDialog(role: EngagementContextValue['role'] = 'lead', closed = false) {
  return renderWithProviders(
    <EngagementCtx value={{ engagementId: ENGAGEMENT_ID, role, closed }}>
      <PublishDialog report={REPORT} engagementId={ENGAGEMENT_ID} />
    </EngagementCtx>,
  )
}

describe('PublishDialog', () => {
  it('renders a publish button', () => {
    renderDialog()
    expect(screen.getByRole('button', { name: /publish/i })).toBeInTheDocument()
  })

  it('shows evidence checkbox default off in dialog', async () => {
    renderDialog()
    const user = userEvent.setup()

    await user.click(screen.getByRole('button', { name: /publish/i }))

    const checkbox = screen.getByLabelText(/include evidence/i)
    expect(checkbox).not.toBeChecked()
  })

  it('disables publish button for non-leads', () => {
    renderDialog('observer')
    const button = screen.getByRole('button', { name: /publish/i })
    expect(button).toBeDisabled()
  })

  it('disables publish button when engagement is closed', () => {
    renderDialog('lead', true)
    const button = screen.getByRole('button', { name: /publish/i })
    expect(button).toBeDisabled()
  })

  it('shows the full-data statement in the dialog', async () => {
    renderDialog()
    const user = userEvent.setup()

    await user.click(screen.getByRole('button', { name: /publish/i }))

    expect(screen.getByText(/always use full engagement data/i)).toBeInTheDocument()
  })
})
