import { describe, expect, test } from 'vitest'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import type { components } from '@/api/schema'
import { adminUserFixture, del, get, problem } from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { EngagementCtx, type EngagementContextValue } from './engagement-layout'
import { engagementSettingsPath } from './paths'
import { SettingsPage } from './settings-page'

const ENGAGEMENT_ID = '0192a000-0000-7000-8000-000000000001'

const engagementFixture: components['schemas']['Engagement'] = {
  id: ENGAGEMENT_ID,
  name: 'Q4 Assessment',
  client: 'Acme Corp',
  description: 'Q4 purple team',
  status: 'active',
  startsOn: '2026-10-01',
  endsOn: '2026-10-15',
  attackVersion: '15.1',
  mode: 'standard',
  autoRevealOnStart: false,
  createdBy: adminUserFixture.id,
  createdAt: '2026-09-20T10:00:00Z',
  updatedAt: '2026-09-20T10:00:00Z',
}

const leadContext: EngagementContextValue = {
  engagementId: ENGAGEMENT_ID,
  role: 'lead',
  closed: false,
}

function renderSettings(): ReturnType<typeof renderWithProviders> {
  server.use(get('/engagements/{engagementId}', () => Response.json(engagementFixture)))
  return renderWithProviders(
    <EngagementCtx value={leadContext}>
      <SettingsPage />
    </EngagementCtx>,
    { user: adminUserFixture, route: engagementSettingsPath(ENGAGEMENT_ID) },
  )
}

/** Open the danger zone's confirmation and press its Delete. */
async function confirmDelete(): Promise<void> {
  await userEvent.click(await screen.findByRole('button', { name: 'Delete engagement' }))
  const dialog = await screen.findByRole('alertdialog')
  await userEvent.click(await within(dialog).findByRole('button', { name: 'Delete' }))
}

describe('SettingsPage danger zone', () => {
  test('deleting sends DELETE for this engagement and returns to the list', async () => {
    let deletedId: string | undefined
    server.use(
      del('/engagements/{engagementId}', ({ params }) => {
        deletedId = params.engagementId as string
        return new Response(null, { status: 204 })
      }),
    )

    renderSettings()
    await confirmDelete()

    await waitFor(() => {
      expect(deletedId).toBe(ENGAGEMENT_ID)
    })
    expect(await screen.findByText('"Q4 Assessment" deleted.')).toBeDefined()
  })

  // The failure this test exists for: the mutation used to `await` the DELETE
  // and throw the result away, so a refused delete ran the success path — the
  // toast said the engagement was gone and the page navigated to the list,
  // while the server had refused and the engagement was still there.
  test('a refused delete is reported as an error, not a success', async () => {
    server.use(
      del('/engagements/{engagementId}', () =>
        problem({
          status: 403,
          code: 'forbidden',
          title: 'Forbidden',
          detail: 'only the lead may delete this engagement',
        }),
      ),
    )

    renderSettings()
    await confirmDelete()

    expect(await screen.findByText(/only the lead may delete this engagement/)).toBeDefined()
    expect(screen.queryByText('"Q4 Assessment" deleted.')).toBeNull()
  })

  // A non-2xx whose body is plain `application/json` rather than a problem
  // document slips past the response middleware. Before `unwrapVoid` that
  // response reached the mutation as a discarded result and was read as
  // success.
  test('a non-problem error body is still not treated as success', async () => {
    server.use(
      del('/engagements/{engagementId}', () => Response.json({ message: 'nope' }, { status: 500 })),
    )

    renderSettings()
    await confirmDelete()

    await waitFor(() => {
      expect(screen.queryByText('"Q4 Assessment" deleted.')).toBeNull()
    })
  })
})
