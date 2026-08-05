import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it } from 'vitest'

import type { components } from '@/api/schema'
import {
  TEST_REQUEST_ID,
  adminUserFixture,
  get,
  internalError,
  memberUserFixture,
} from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { REFERENCE_ONLY_MESSAGE } from './detections-panel'
import { LibraryPage } from './library-page'
import { CONTENT_PATH, CONTENT_SOURCES_PATH } from './paths'

const attackVersion15: components['schemas']['ContentAttackVersion'] = {
  version: '15.1',
  status: 'ready',
  itemCount: 2,
  sourceEnabled: true,
  syncedAt: '2026-01-02T09:00:00Z',
}

const attackVersion14: components['schemas']['ContentAttackVersion'] = {
  version: '14.1',
  status: 'ready',
  itemCount: 1,
  sourceEnabled: true,
  syncedAt: '2026-01-01T09:00:00Z',
}

const techniqueT1059: components['schemas']['ContentTechnique'] = {
  id: '0192f1a0-0000-7000-8000-00000000t059',
  sourceId: '0192f1a0-0000-7000-8000-00000000src1',
  version: '15.1',
  externalId: 'T1059',
  name: 'Command and Scripting Interpreter',
  description: 'Adversaries may abuse command and script interpreters.',
  isSubtechnique: false,
  parentExternalId: '',
  createdAt: '2026-01-02T09:00:00Z',
  updatedAt: '2026-01-02T09:00:00Z',
}

const techniqueT1059v14: components['schemas']['ContentTechnique'] = {
  ...techniqueT1059,
  id: '0192f1a0-0000-7000-8000-00000000t014',
  version: '14.1',
  name: 'Command and Scripting Interpreter (14.1)',
  description: 'Older description for the 14.1 pin.',
}

const techniqueDetail15: components['schemas']['ContentTechniqueDetail'] = {
  ...techniqueT1059,
  tactics: ['TA0002'],
  mitigations: ['M1049'],
}

const techniqueDetail14: components['schemas']['ContentTechniqueDetail'] = {
  ...techniqueT1059v14,
  tactics: ['TA0002'],
  mitigations: [],
}

const procedure: components['schemas']['ContentProcedureTemplate'] = {
  id: '0192f1a0-0000-7000-8000-00000000p001',
  sourceId: '0192f1a0-0000-7000-8000-00000000src2',
  version: 'current',
  externalId: '11111111-1111-4111-8111-111111111111',
  name: 'PowerShell Echo Input',
  description: 'Windows PowerShell test that takes an input argument and has cleanup.',
  platforms: ['windows'],
  executor: 'powershell',
  elevationRequired: false,
  command: 'Write-Host "#{message}"',
  cleanup: 'Remove-Item -Path "#{output_file}" -ErrorAction Ignore',
  inputArgs: [
    {
      name: 'message',
      description: 'Text to echo',
      type: 'string',
      default: 'hello-atomic',
    },
  ],
  techniqueExternalIds: ['T1059.001'],
  dependencyExecutorName: '',
  dependencies: '',
  createdAt: '2026-01-02T09:00:00Z',
  updatedAt: '2026-01-02T09:00:00Z',
}

const detection: components['schemas']['ContentDetectionRule'] = {
  id: '0192f1a0-0000-7000-8000-00000000d001',
  sourceId: '0192f1a0-0000-7000-8000-00000000src3',
  version: 'current',
  externalId: '5d2c185a-6d5a-4f3e-9c1a-0b7e6f4d2a11',
  name: 'Encoded PowerShell',
  description: 'Detects encoded PowerShell.',
  techniqueExternalIds: ['T1059.001'],
  level: 'high',
  status: 'test',
  logsource: { product: 'windows', category: 'process_creation' },
  ruleYaml: 'title: Encoded PowerShell\nlevel: high\n',
  createdAt: '2026-01-02T09:00:00Z',
  updatedAt: '2026-01-02T09:00:00Z',
}

function stubEmptyLibrary(): void {
  server.use(get('/content/attack/versions', () => Response.json({ items: [] })))
}

function stubLibraryWithContent(): void {
  server.use(
    get('/content/attack/versions', () =>
      Response.json({ items: [attackVersion14, attackVersion15] }),
    ),
    get('/content/tactics', () =>
      Response.json({
        items: [
          {
            id: '0192f1a0-0000-7000-8000-00000000ta02',
            sourceId: techniqueT1059.sourceId,
            version: '15.1',
            externalId: 'TA0002',
            name: 'Execution',
            description: 'Run code.',
            createdAt: '2026-01-02T09:00:00Z',
            updatedAt: '2026-01-02T09:00:00Z',
          },
        ],
      }),
    ),
    get('/content/techniques', ({ request }) => {
      const version = new URL(request.url).searchParams.get('version')
      const q = new URL(request.url).searchParams.get('q') ?? ''
      const items =
        version === '14.1'
          ? [techniqueT1059v14]
          : [techniqueT1059].filter(
              (t) =>
                q === '' ||
                t.externalId.toLowerCase().includes(q.toLowerCase()) ||
                t.name.toLowerCase().includes(q.toLowerCase()),
            )
      return Response.json({ items })
    }),
    get('/content/techniques/{techniqueId}', ({ params }) => {
      if (params.techniqueId === techniqueT1059v14.id) {
        return Response.json(techniqueDetail14)
      }
      return Response.json(techniqueDetail15)
    }),
    get('/content/procedure-templates', () => Response.json({ items: [procedure] })),
    get('/content/procedure-templates/{templateId}', () => Response.json(procedure)),
    get('/content/detection-rules', () => Response.json({ items: [detection] })),
    get('/content/detection-rules/{ruleId}', () => Response.json(detection)),
    get('/content/emulation-plans', () => Response.json({ items: [] })),
    get('/content/custom/notes', () => Response.json({ items: [] })),
  )
}

function renderLibrary(user: components['schemas']['CurrentUser'] = memberUserFixture): void {
  renderWithProviders(<LibraryPage />, { user, route: CONTENT_PATH })
}

describe('LibraryPage', () => {
  it('tells a member to ask an admin when ATT&CK is not installed', async () => {
    stubEmptyLibrary()
    renderLibrary(memberUserFixture)

    expect(await screen.findByText('Ask an admin to install ATT&CK.')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Open sources admin' })).not.toBeInTheDocument()
    // No enable/sync chrome on this surface for anyone.
    expect(screen.queryByRole('button', { name: /sync/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /enable/i })).not.toBeInTheDocument()
  })

  it('links an administrator to sources admin from the empty library', async () => {
    stubEmptyLibrary()
    renderLibrary(adminUserFixture)

    const link = await screen.findByRole('link', { name: 'Open sources admin' })
    expect(link).toHaveAttribute('href', CONTENT_SOURCES_PATH)
    expect(screen.queryByRole('button', { name: /sync/i })).not.toBeInTheDocument()
  })

  it('searches T1059 and opens the technique detail for the selected version', async () => {
    const user = userEvent.setup()
    let askedVersion: string | null = null
    stubLibraryWithContent()
    server.use(
      get('/content/techniques', ({ request }) => {
        const params = new URL(request.url).searchParams
        askedVersion = params.get('version')
        const q = params.get('q') ?? ''
        const items = [techniqueT1059].filter(
          (t) =>
            q === '' ||
            t.externalId.toLowerCase().includes(q.toLowerCase()) ||
            t.name.toLowerCase().includes(q.toLowerCase()),
        )
        return Response.json({ items })
      }),
    )
    renderLibrary()

    await screen.findByRole('row', { name: /T1059/ })

    await user.type(screen.getByLabelText('Search'), 'T1059')
    await waitFor(() => {
      expect(askedVersion).toBe('15.1')
    })

    await user.click(screen.getByRole('button', { name: 'Command and Scripting Interpreter' }))

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(/ATT&CK 15\.1/)).toBeInTheDocument()
    expect(within(dialog).getByText('TA0002')).toBeInTheDocument()
    expect(within(dialog).getByText('M1049')).toBeInTheDocument()
  })

  it('clears technique detail identity when the ATT&CK version changes', async () => {
    const user = userEvent.setup()
    stubLibraryWithContent()
    renderLibrary()

    await screen.findByRole('row', { name: /T1059/ })
    await user.click(screen.getByRole('button', { name: 'Command and Scripting Interpreter' }))
    expect(await screen.findByRole('dialog')).toHaveTextContent('15.1')

    // Close and switch version — the open drawer must not keep 15.1 under 14.1.
    await user.keyboard('{Escape}')
    await waitFor(() => {
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    })

    await user.click(screen.getByRole('combobox', { name: /ATT&CK version/i }))
    await user.click(await screen.findByRole('option', { name: /14\.1/ }))

    await screen.findByRole('row', { name: /14\.1/ })
    await user.click(
      screen.getByRole('button', { name: 'Command and Scripting Interpreter (14.1)' }),
    )

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText(/ATT&CK 14\.1/)).toBeInTheDocument()
    expect(within(dialog).getByText('Older description for the 14.1 pin.')).toBeInTheDocument()
    expect(within(dialog).queryByText(techniqueDetail15.description)).not.toBeInTheDocument()
  })

  it('shows separate command and cleanup sections on a procedure', async () => {
    const user = userEvent.setup()
    stubLibraryWithContent()
    renderLibrary()

    await screen.findByRole('tab', { name: 'Procedures' })
    await user.click(screen.getByRole('tab', { name: 'Procedures' }))

    await user.click(await screen.findByRole('button', { name: 'PowerShell Echo Input' }))

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByRole('heading', { name: 'Command' })).toBeInTheDocument()
    expect(within(dialog).getByRole('heading', { name: 'Cleanup' })).toBeInTheDocument()
    expect(within(dialog).getByText('Write-Host "#{message}"')).toBeInTheDocument()
    expect(
      within(dialog).getByText('Remove-Item -Path "#{output_file}" -ErrorAction Ignore'),
    ).toBeInTheDocument()
    expect(within(dialog).getByText('message')).toBeInTheDocument()
  })

  it('labels detection rules as reference only', async () => {
    const user = userEvent.setup()
    stubLibraryWithContent()
    renderLibrary()

    await user.click(await screen.findByRole('tab', { name: 'Detection rules' }))
    await user.click(await screen.findByRole('button', { name: 'Encoded PowerShell' }))

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getAllByText(REFERENCE_ONLY_MESSAGE).length).toBeGreaterThan(0)
    expect(within(dialog).getByText(/title: Encoded PowerShell/)).toBeInTheDocument()
  })

  it('surfaces a failed library request with its request id', async () => {
    server.use(get('/content/attack/versions', () => internalError()))
    renderLibrary()

    expect(await screen.findByRole('alert')).toHaveTextContent('That request failed.')
    expect(screen.getByText(TEST_REQUEST_ID)).toBeInTheDocument()
  })

  it('says so when filters match nothing', async () => {
    const user = userEvent.setup()
    stubLibraryWithContent()
    server.use(get('/content/techniques', () => Response.json({ items: [] })))
    renderLibrary()

    await screen.findByLabelText('Search')
    await user.type(screen.getByLabelText('Search'), 'nope')

    expect(await screen.findByText('No techniques match those filters')).toBeInTheDocument()
  })
})
