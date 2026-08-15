import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { type ReactNode } from 'react'
import { useLocation } from 'react-router'
import { describe, expect, it } from 'vitest'

import type { components } from '@/api/schema'
import { get, memberUserFixture } from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { UseInScenarioButton } from './use-in-scenario'

const PLAN_ID = '0192f1a0-0000-7000-8000-0000000plan1'

function engagement(
  overrides: Partial<components['schemas']['Engagement']>,
): components['schemas']['Engagement'] {
  return {
    id: '0192f1a0-0000-7000-8000-00000000eng1',
    name: 'Spring Purple',
    client: 'Acme',
    description: '',
    status: 'active',
    startsOn: '2026-03-01',
    endsOn: '2026-03-31',
    attackVersion: '15.1',
    mode: 'standard',
    autoRevealOnStart: false,
    createdBy: memberUserFixture.id,
    createdAt: '2026-02-01T09:00:00Z',
    updatedAt: '2026-02-01T09:00:00Z',
    ...overrides,
  }
}

const activeEngagement = engagement({})
const closedEngagement = engagement({
  id: '0192f1a0-0000-7000-8000-00000000eng2',
  name: 'Last Year Retest',
  status: 'closed',
})

function engagementsHandler(items: components['schemas']['Engagement'][]) {
  return get('/engagements', () =>
    Response.json({ items, nextCursor: null } satisfies components['schemas']['EngagementPage'], {
      status: 200,
    }),
  )
}

/** Renders the button beside the current location so navigation is observable. */
function Harness(): ReactNode {
  const location = useLocation()
  return (
    <>
      <UseInScenarioButton kind="plan" id={PLAN_ID} />
      <span data-testid="location">{`${location.pathname}${location.search}`}</span>
    </>
  )
}

describe('UseInScenarioButton', () => {
  it('hands the plan off to the chosen engagement workbook', async () => {
    server.use(engagementsHandler([activeEngagement, closedEngagement]))
    const user = userEvent.setup()
    renderWithProviders(<Harness />, { user: memberUserFixture })

    await user.click(screen.getByRole('button', { name: 'Use in scenario' }))

    const dialog = await screen.findByRole('dialog')
    await user.click(await within(dialog).findByRole('combobox'))
    await user.click(await screen.findByRole('option', { name: /Spring Purple/ }))
    await user.click(within(dialog).getByRole('button', { name: 'Continue' }))

    await waitFor(() => {
      expect(screen.getByTestId('location')).toHaveTextContent(
        `/engagements/${activeEngagement.id}/workbook?use=plan&useId=${PLAN_ID}`,
      )
    })
  })

  it('does not offer engagements that refuse new content', async () => {
    server.use(engagementsHandler([activeEngagement, closedEngagement]))
    const user = userEvent.setup()
    renderWithProviders(<Harness />, { user: memberUserFixture })

    await user.click(screen.getByRole('button', { name: 'Use in scenario' }))
    const dialog = await screen.findByRole('dialog')
    await user.click(await within(dialog).findByRole('combobox'))

    expect(await screen.findByRole('option', { name: /Spring Purple/ })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: /Last Year Retest/ })).not.toBeInTheDocument()
  })

  it('says so when there is nowhere to send the plan', async () => {
    server.use(engagementsHandler([closedEngagement]))
    const user = userEvent.setup()
    renderWithProviders(<Harness />, { user: memberUserFixture })

    await user.click(screen.getByRole('button', { name: 'Use in scenario' }))

    expect(await screen.findByText(/No open engagements/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Continue' })).toBeDisabled()
  })
})
