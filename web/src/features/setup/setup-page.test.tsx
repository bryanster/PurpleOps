import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HttpResponse } from 'msw'
import { Route, Routes } from 'react-router'
import { describe, expect, it, vi } from 'vitest'

import type { components } from '@/api/schema'
import { adminUserFixture, get, post } from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { SetupPage } from './setup-page'

/**
 * The first-run wizard.
 *
 * The behaviour worth pinning down is not the layout — it is that the picker
 * offers what upstream offers, that installing means enable-then-sync in that
 * order, that an air-gapped installation is given a way through instead of a
 * dead end, and that every exit finishes setup. The last one matters most:
 * `RequireAuth` sends an administrator back here until setup is finished, so a
 * button that navigated away without completing would be a loop.
 */

const attackSourceId = '01900000-0000-7000-8000-000000000001'

const releasesFixture: components['schemas']['ContentAttackReleaseList'] = {
  reachable: true,
  sourceEnabled: false,
  items: [
    { version: '17.1', latest: true, installed: false, released: '2026-04-22T00:00:00Z' },
    { version: '16.1', latest: false, installed: false },
    { version: '15.1', latest: false, installed: true, status: 'ready' },
  ],
}

const jobFixture: components['schemas']['ContentSyncJob'] = {
  id: '0192f1a0-0000-7000-8000-00000000c001',
  sourceId: attackSourceId,
  version: '17.1',
  kind: 'sync',
  status: 'running',
  phase: 'fetch',
  progressCurrent: 0,
  progressTotal: 0,
  message: 'fetching',
  error: '',
  createdBy: adminUserFixture.id,
  createdAt: '2026-04-23T09:00:00Z',
}

const sourceFixture: components['schemas']['ContentSource'] = {
  id: attackSourceId,
  kind: 'attack',
  name: 'MITRE ATT&CK Enterprise',
  url: 'https://raw.githubusercontent.com/mitre-attack/attack-stix-data/master',
  ref: 'enterprise-attack/enterprise-attack-{version}.json',
  enabled: false,
  status: 'idle',
  itemCount: 0,
  error: '',
  licenseSpdx: 'Apache-2.0',
  licenseName: 'Apache License 2.0',
  licenseUrl: 'https://www.apache.org/licenses/LICENSE-2.0',
  attribution: 'ATT&CK content is © MITRE Corporation, used under the Apache License 2.0.',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

const incompleteSetup: components['schemas']['SetupState'] = { completed: false }
const completeSetup: components['schemas']['SetupState'] = {
  completed: true,
  completedAt: '2026-04-23T09:05:00Z',
  completedBy: adminUserFixture.id,
}

function seedWizard(overrides: {
  releases?: components['schemas']['ContentAttackReleaseList']
  onEnable?: () => void
  onSync?: (body: unknown) => void
  onComplete?: () => void
}): void {
  server.use(
    get('/setup', () => HttpResponse.json(incompleteSetup)),
    get('/content/attack/releases', () => HttpResponse.json(overrides.releases ?? releasesFixture)),
    get('/content/sources', () => HttpResponse.json({ items: [sourceFixture] })),
    post('/content/sources/{sourceId}/enable', () => {
      overrides.onEnable?.()
      return HttpResponse.json({ ...sourceFixture, enabled: true })
    }),
    post('/content/sources/{sourceId}/sync', async ({ request }) => {
      const body: unknown = await request.json().catch(() => undefined)
      overrides.onSync?.(body)
      return HttpResponse.json(jobFixture, { status: 202 })
    }),
    get('/content/jobs/{jobId}', () => HttpResponse.json(jobFixture)),
    post('/setup/complete', () => {
      overrides.onComplete?.()
      return HttpResponse.json(completeSetup)
    }),
  )
}

function renderWizard(): void {
  renderWithProviders(
    <Routes>
      <Route path="/setup" element={<SetupPage />} />
      <Route path="/" element={<p>the application</p>} />
      <Route path="/admin/content/sources" element={<p>content sources screen</p>} />
    </Routes>,
    { route: '/setup', user: adminUserFixture },
  )
}

describe('the first-run wizard', () => {
  it('offers every release upstream published, marking the newest and what is already here', async () => {
    seedWizard({})
    renderWizard()

    expect(await screen.findByRole('radio', { name: /17\.1/ })).toBeChecked()
    expect(screen.getByRole('radio', { name: /16\.1/ })).not.toBeChecked()

    // The badges are the two facts a person choosing needs: which is newest,
    // and which they would be reinstalling.
    expect(screen.getByRole('radio', { name: /17\.1.*Latest/ })).toBeInTheDocument()
    expect(screen.getByRole('radio', { name: /15\.1.*Installed/ })).toBeInTheDocument()

    // MITRE's licence terms travel with their content.
    expect(screen.getByText(/MITRE Corporation/)).toBeInTheDocument()
  })

  it('enables the source and then syncs the chosen version', async () => {
    const user = userEvent.setup()
    const enabled = vi.fn()
    const synced = vi.fn()
    seedWizard({ onEnable: enabled, onSync: synced })
    renderWizard()

    await user.click(await screen.findByRole('radio', { name: /16\.1/ }))
    await user.click(screen.getByRole('button', { name: 'Install and continue' }))

    // Enable first: the seeded source is disabled, and a sync against a
    // disabled source is refused. Order is the whole assertion.
    await waitFor(() => {
      expect(synced).toHaveBeenCalledWith({ version: '16.1' })
    })
    expect(enabled).toHaveBeenCalledOnce()
    const enabledAt = enabled.mock.invocationCallOrder[0] ?? Number.POSITIVE_INFINITY
    const syncedAt = synced.mock.invocationCallOrder[0] ?? Number.NEGATIVE_INFINITY
    expect(enabledAt).toBeLessThan(syncedAt)

    // And the screen moves to watching the job it just started.
    expect(await screen.findByText('fetch')).toBeInTheDocument()
  })

  it('finishes setup and leaves for the application', async () => {
    const user = userEvent.setup()
    const completed = vi.fn()
    seedWizard({ onComplete: completed })
    renderWizard()

    await user.click(await screen.findByRole('button', { name: 'Skip for now' }))

    expect(await screen.findByText('the application')).toBeInTheDocument()
    expect(completed).toHaveBeenCalledOnce()
  })

  it('finishes setup on the way to the content sources screen, rather than leaving it unfinished', async () => {
    const user = userEvent.setup()
    const completed = vi.fn()
    seedWizard({ onComplete: completed })
    renderWizard()

    await user.click(await screen.findByRole('button', { name: 'Install and continue' }))
    await user.click(await screen.findByRole('button', { name: 'Finish and open Content sources' }))

    expect(await screen.findByText('content sources screen')).toBeInTheDocument()
    expect(completed).toHaveBeenCalledOnce()
  })

  it('gives an air-gapped installation the offline path and a way to name a release anyway', async () => {
    const user = userEvent.setup()
    const synced = vi.fn()
    seedWizard({
      releases: {
        reachable: false,
        unreachable: 'attack: read the release index https://…/index.json: dial tcp: i/o timeout',
        sourceEnabled: false,
        items: [{ version: '15.1', latest: false, installed: true, status: 'ready' }],
      },
      onSync: synced,
    })
    renderWizard()

    expect(await screen.findByText(/could not be reached/)).toBeInTheDocument()
    expect(screen.getByText(/i\/o timeout/)).toBeInTheDocument()
    expect(screen.getAllByText(/offline bundle/).length).toBeGreaterThan(0)
    // Nothing to pick from, so nothing to install until a label is typed.
    expect(screen.queryByRole('radio')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Install and continue' })).toBeDisabled()

    await user.type(screen.getByLabelText('Install a release by label'), '17.1')
    await user.click(screen.getByRole('button', { name: 'Install and continue' }))

    // A named release is fetched directly; only the index was out of reach.
    await waitFor(() => {
      expect(synced).toHaveBeenCalledWith({ version: '17.1' })
    })
  })

  it('says what went wrong when the install could not be started, and stays put', async () => {
    const user = userEvent.setup()
    seedWizard({})
    server.use(
      post('/content/sources/{sourceId}/sync', () =>
        HttpResponse.json(
          {
            type: 'about:blank',
            title: 'Conflict',
            status: 409,
            code: 'conflict',
            detail: 'a content job is already active (jobId: 0192…)',
          },
          { status: 409, headers: { 'Content-Type': 'application/problem+json' } },
        ),
      ),
    )
    renderWizard()

    await user.click(await screen.findByRole('button', { name: 'Install and continue' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(/already active/)
    // Still on the picker, with the choice intact rather than a half-started
    // install to reason about.
    expect(screen.getByRole('radio', { name: /17\.1/ })).toBeChecked()
  })
})
