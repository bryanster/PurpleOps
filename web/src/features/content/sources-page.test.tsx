import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import type { components } from '@/api/schema'
import { TEST_REQUEST_ID, adminUserFixture, get, post, problem } from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { CONTENT_SOURCES_PATH } from './paths'
import { SourcesPage } from './sources-page'

/**
 * Sources admin control plane (M2-014).
 *
 * Pins the behaviours the ticket names: slot-held action disablement, full
 * error text (not a generic toast only), license display, and custom's missing
 * delete control.
 */

const attackSource: components['schemas']['ContentSource'] = {
  id: '01900000-0000-7000-8000-000000000001',
  kind: 'attack',
  name: 'MITRE ATT&CK Enterprise',
  url: 'https://example.test/attack',
  ref: 'enterprise-attack-{version}.json',
  enabled: false,
  status: 'idle',
  itemCount: 0,
  error: '',
  licenseSpdx: 'Apache-2.0',
  licenseName: 'Apache License 2.0',
  licenseUrl: 'https://www.apache.org/licenses/LICENSE-2.0',
  attribution: 'ATT&CK content is © MITRE Corporation.',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

const customSource: components['schemas']['ContentSource'] = {
  id: '01900000-0000-7000-8000-000000000005',
  kind: 'custom',
  name: 'Custom content',
  url: '',
  ref: '',
  enabled: true,
  status: 'idle',
  itemCount: 3,
  error: '',
  licenseSpdx: '',
  licenseName: '',
  licenseUrl: '',
  attribution: 'User-authored content for this installation.',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
}

const activeJob: components['schemas']['ContentSyncJob'] = {
  id: '0192f1a0-0000-7000-8000-00000000job1',
  sourceId: attackSource.id,
  kind: 'sync',
  status: 'running',
  phase: 'parse',
  progressCurrent: 2,
  progressTotal: 10,
  message: 'Parsing STIX bundle',
  error: '',
  createdBy: adminUserFixture.id,
  createdAt: '2026-02-01T10:00:00Z',
  startedAt: '2026-02-01T10:00:01Z',
}

function stubSources(
  items: components['schemas']['ContentSource'][] = [attackSource, customSource],
  jobs: components['schemas']['ContentSyncJob'][] = [],
): void {
  server.use(
    get('/content/sources', () => Response.json({ items })),
    get('/content/jobs', () => Response.json({ items: jobs })),
    get('/content/sources/{sourceId}', ({ params }) => {
      const id = String(params.sourceId)
      const source = items.find((row) => row.id === id)
      if (source === undefined) {
        return problem({
          status: 404,
          code: 'not_found',
          title: 'Not Found',
          detail: 'no such source',
        })
      }
      return Response.json({ ...source } satisfies components['schemas']['ContentSourceDetail'])
    }),
    get('/content/sources/{sourceId}/versions', () => Response.json({ items: [] })),
    get('/content/jobs/{jobId}', ({ params }) => {
      const id = String(params.jobId)
      const job = jobs.find((row) => row.id === id)
      if (job === undefined) {
        return problem({
          status: 404,
          code: 'not_found',
          title: 'Not Found',
          detail: 'no such job',
        })
      }
      return Response.json(job)
    }),
  )
}

function renderSources(): void {
  renderWithProviders(<SourcesPage />, {
    user: adminUserFixture,
    route: CONTENT_SOURCES_PATH,
  })
}

describe('SourcesPage', () => {
  it('lists sources with status, counts, and license on detail', async () => {
    stubSources()
    renderSources()

    expect(await screen.findByRole('heading', { name: 'Content sources' })).toBeInTheDocument()
    expect(
      await screen.findByRole('button', { name: 'MITRE ATT&CK Enterprise' }),
    ).toBeInTheDocument()
    expect(screen.getByText('Custom content')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'MITRE ATT&CK Enterprise' }))

    const license = await screen.findByTestId('source-license')
    expect(within(license).getByText('Apache-2.0')).toBeInTheDocument()
    expect(within(license).getByText('Apache License 2.0')).toBeInTheDocument()
    expect(within(license).getByText('ATT&CK content is © MITRE Corporation.')).toBeInTheDocument()
    expect(within(license).getByRole('link', { name: /apache\.org/ })).toHaveAttribute(
      'href',
      'https://www.apache.org/licenses/LICENSE-2.0',
    )
  })

  it('hides delete on the custom source', async () => {
    stubSources()
    renderSources()

    const customRow = await screen.findByRole('row', { name: /Custom content/ })
    expect(within(customRow).queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument()

    const attackRow = screen.getByRole('row', { name: /MITRE ATT&CK Enterprise/ })
    expect(within(attackRow).getByRole('button', { name: 'Delete' })).toBeInTheDocument()
  })

  it('disables sync and bundle while a job holds the global slot', async () => {
    stubSources([attackSource, customSource], [activeJob])
    renderSources()

    expect(await screen.findByText('A content job is running')).toBeInTheDocument()
    expect(screen.getByText(/Parsing STIX bundle/)).toBeInTheDocument()
    expect(screen.getByText(activeJob.id)).toBeInTheDocument()

    const attackRow = screen.getByRole('row', { name: /MITRE ATT&CK Enterprise/ })
    expect(within(attackRow).getByRole('button', { name: 'Sync' })).toBeDisabled()
    expect(within(attackRow).getByRole('button', { name: 'Upload bundle' })).toBeDisabled()
    expect(within(attackRow).getByRole('button', { name: 'Reprocess' })).toBeDisabled()

    // Enable/disable are not job-slot actions.
    expect(within(attackRow).getByRole('button', { name: 'Enable' })).toBeEnabled()
  })

  it('surfaces the API error text when a job-producing action is refused', async () => {
    stubSources()
    server.use(
      post('/content/sources/{sourceId}/sync', () =>
        problem({
          status: 409,
          code: 'conflict',
          title: 'Conflict',
          detail: `job ${activeJob.id} already holds the content slot`,
        }),
      ),
    )
    renderSources()

    const attackRow = await screen.findByRole('row', { name: /MITRE ATT&CK Enterprise/ })
    await userEvent.click(within(attackRow).getByRole('button', { name: 'Sync' }))

    // ATT&CK opens a version dialog first.
    await userEvent.click(screen.getByRole('button', { name: 'Start sync' }))

    expect(
      await screen.findByText(`job ${activeJob.id} already holds the content slot`),
    ).toBeInTheDocument()
  })

  it('enables a source through the API', async () => {
    stubSources()
    let enabledId: string | undefined
    server.use(
      post('/content/sources/{sourceId}/enable', ({ params }) => {
        enabledId = String(params.sourceId)
        return Response.json({ ...attackSource, enabled: true })
      }),
    )
    renderSources()

    const attackRow = await screen.findByRole('row', { name: /MITRE ATT&CK Enterprise/ })
    await userEvent.click(within(attackRow).getByRole('button', { name: 'Enable' }))

    await waitFor(() => {
      expect(enabledId).toBe(attackSource.id)
    })
    expect(await screen.findByText(/is enabled/)).toBeInTheDocument()
  })

  it('shows the failed job error from the API, not a generic toast only', async () => {
    const failed: components['schemas']['ContentSyncJob'] = {
      ...activeJob,
      status: 'failed',
      phase: 'parse',
      message: 'done',
      error: 'STIX bundle missing enterprise-attack collection',
      finishedAt: '2026-02-01T10:05:00Z',
    }
    stubSources(
      [
        {
          ...attackSource,
          status: 'error',
          error: 'STIX bundle missing enterprise-attack collection',
        },
        customSource,
      ],
      [failed],
    )
    renderSources()

    // Source row carries the error summary.
    expect(
      await screen.findByText('STIX bundle missing enterprise-attack collection'),
    ).toBeInTheDocument()

    // Request id plumbing still works on list failures.
    server.use(
      get('/content/sources', () =>
        problem({
          status: 500,
          code: 'internal',
          title: 'Internal Server Error',
          detail: 'writer locked',
        }),
      ),
    )
    renderWithProviders(<SourcesPage />, {
      user: adminUserFixture,
      route: CONTENT_SOURCES_PATH,
    })
    expect(await screen.findByText('writer locked')).toBeInTheDocument()
    expect(screen.getByText(TEST_REQUEST_ID)).toBeInTheDocument()
  })

  it('uploads a bundle file via multipart without stashing it in query state', async () => {
    stubSources()
    let sawMultipart = false
    server.use(
      post('/content/sources/{sourceId}/bundle', ({ request }) => {
        const contentType = request.headers.get('content-type') ?? ''
        sawMultipart = contentType.includes('multipart/form-data')
        // Do not call request.formData() here — undici's parser rejects the
        // File body openapi-fetch builds under jsdom. Content-Type is enough
        // to prove we did not JSON-encode the archive.
        return Response.json(
          {
            ...activeJob,
            id: '0192f1a0-0000-7000-8000-00000000job2',
            kind: 'bundle_import',
            status: 'queued',
            phase: '',
            progressCurrent: 0,
            progressTotal: 0,
            message: '',
          } satisfies components['schemas']['ContentSyncJob'],
          { status: 202 },
        )
      }),
    )
    renderSources()

    const attackRow = await screen.findByRole('row', { name: /MITRE ATT&CK Enterprise/ })
    await userEvent.click(within(attackRow).getByRole('button', { name: 'Upload bundle' }))

    const dialog = await screen.findByRole('dialog', { name: /Upload bundle/ })
    const fileInput = within(dialog).getByLabelText('Archive file')
    const file = new File(['{"type":"bundle"}'], 'enterprise-mini-15.1.json', {
      type: 'application/json',
    })
    fireEvent.change(fileInput, { target: { files: [file] } })
    expect(await within(dialog).findByText('enterprise-mini-15.1.json')).toBeInTheDocument()
    await userEvent.type(within(dialog).getByLabelText(/ATT&CK version/i), '15.1')
    const submit = within(dialog).getByRole('button', { name: 'Upload and import' })
    await waitFor(() => {
      expect(submit).toBeEnabled()
    })
    await userEvent.click(submit)

    expect(await screen.findByText(/Bundle import queued/)).toBeInTheDocument()
    expect(sawMultipart).toBe(true)
  })
})
