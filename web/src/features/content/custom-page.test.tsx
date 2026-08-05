import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { components } from '@/api/schema'
import {
  TEST_REQUEST_ID,
  adminUserFixture,
  del,
  get,
  memberUserFixture,
  patch,
  post,
  problem,
} from '@/test/msw/handlers'
import { server } from '@/test/msw/server'
import { renderWithProviders } from '@/test/render'

import { CustomContentPage } from './custom-page'
import { CONTENT_CUSTOM_PATH } from './paths'

/**
 * Custom content editor + import wizard (M2-015).
 *
 * Pins form validation field errors, dry-run-then-confirm import, export
 * download, and list/detail flows for authored procedures.
 */

const procedureFixture: components['schemas']['ContentProcedureTemplate'] = {
  id: '0192f1a0-0000-7000-8000-00000000c001',
  sourceId: '01900000-0000-7000-8000-000000000005',
  version: 'current',
  externalId: 'custom-proc-1',
  name: 'Echo twice',
  description: 'Custom template with two args.',
  platforms: ['linux'],
  executor: 'sh',
  elevationRequired: false,
  command: 'echo "#{one}" "#{two}"',
  cleanup: '',
  inputArgs: [
    { name: 'one', description: 'First', type: 'string', default: 'a' },
    { name: 'two', description: 'Second', type: 'string', default: 'b' },
  ],
  techniqueExternalIds: ['T1059.004'],
  dependencyExecutorName: '',
  dependencies: '',
  createdAt: '2026-01-02T09:00:00Z',
  updatedAt: '2026-01-02T09:00:00Z',
}

const emptyImportReport: components['schemas']['ContentImportReport'] = {
  dryRun: true,
  format: 'testcases_json',
  proceduresCreated: 2,
  proceduresUpdated: 0,
  notesCreated: 0,
  notesUpdated: 0,
  detectionsCreated: 0,
  detectionsUpdated: 0,
  warnings: [{ path: '-', message: 'legacy actions field flattened into command' }],
  errors: [],
}

const committedImportReport: components['schemas']['ContentImportReport'] = {
  ...emptyImportReport,
  dryRun: false,
}

function stubEmptyCustom(): void {
  server.use(
    get('/content/custom/procedure-templates', () => Response.json({ items: [] })),
    get('/content/custom/detection-rules', () => Response.json({ items: [] })),
    get('/content/custom/notes', () => Response.json({ items: [] })),
  )
}

function stubWithProcedure(
  items: components['schemas']['ContentProcedureTemplate'][] = [procedureFixture],
): void {
  server.use(
    get('/content/custom/procedure-templates', () => Response.json({ items })),
    get('/content/custom/detection-rules', () => Response.json({ items: [] })),
    get('/content/custom/notes', () => Response.json({ items: [] })),
    get('/content/custom/procedure-templates/{templateId}', ({ params }) => {
      const row = items.find((item) => item.id === String(params.templateId))
      if (row === undefined) {
        return problem({
          status: 404,
          code: 'not_found',
          title: 'Not Found',
          detail: 'missing',
        })
      }
      return Response.json(row)
    }),
  )
}

function renderCustom(user: components['schemas']['CurrentUser'] = adminUserFixture): void {
  renderWithProviders(<CustomContentPage />, { user, route: CONTENT_CUSTOM_PATH })
}

/** Toolbar "New procedure" — empty state also renders one with the same name. */
async function clickNewProcedure(user: ReturnType<typeof userEvent.setup>): Promise<void> {
  const buttons = await screen.findAllByRole('button', { name: 'New procedure' })
  await user.click(buttons[0]!)
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('CustomContentPage', () => {
  it('creates a procedure with two input args and shows both in the detail', async () => {
    const user = userEvent.setup()
    let createdBody: unknown
    stubEmptyCustom()
    server.use(
      post('/content/custom/procedure-templates', async ({ request }) => {
        createdBody = await request.json()
        return Response.json(procedureFixture, { status: 201 })
      }),
      get('/content/custom/procedure-templates', () =>
        Response.json({ items: createdBody === undefined ? [] : [procedureFixture] }),
      ),
    )

    renderCustom()

    await clickNewProcedure(user)
    const dialog = await screen.findByRole('dialog')

    await user.type(within(dialog).getAllByLabelText('Name')[0]!, 'Echo twice')
    await user.type(within(dialog).getByLabelText('Command'), 'echo "#{one}" "#{two}"')
    await user.click(within(dialog).getByRole('button', { name: 'Add argument' }))
    await user.click(within(dialog).getByRole('button', { name: 'Add argument' }))

    const nameFields = within(dialog).getAllByLabelText('Name')
    await user.clear(nameFields[1]!)
    await user.type(nameFields[1]!, 'one')
    await user.clear(nameFields[2]!)
    await user.type(nameFields[2]!, 'two')

    const defaultFields = within(dialog).getAllByLabelText('Default')
    await user.type(defaultFields[0]!, 'a')
    await user.type(defaultFields[1]!, 'b')

    await user.click(within(dialog).getByRole('button', { name: 'Create procedure' }))

    await waitFor(() => {
      expect(createdBody).toMatchObject({
        name: 'Echo twice',
        inputArgs: [
          expect.objectContaining({ name: 'one' }),
          expect.objectContaining({ name: 'two' }),
        ],
      })
    })

    stubWithProcedure([procedureFixture])
    await user.click(await screen.findByRole('button', { name: 'Echo twice' }))

    const detail = await screen.findByRole('dialog')
    expect(within(detail).getByText('one')).toBeInTheDocument()
    expect(within(detail).getByText('two')).toBeInTheDocument()
  })

  it('maps server field errors onto the procedure form', async () => {
    const user = userEvent.setup()
    stubEmptyCustom()
    server.use(
      post('/content/custom/procedure-templates', () =>
        problem({
          status: 400,
          code: 'validation_failed',
          title: 'Bad Request',
          detail: 'invalid technique',
          errors: [{ field: 'techniqueExternalIds', message: 'must look like T1059' }],
        }),
      ),
    )
    renderCustom()

    await clickNewProcedure(user)
    const dialog = await screen.findByRole('dialog')
    await user.type(within(dialog).getAllByLabelText('Name')[0]!, 'Bad tech')
    await user.type(within(dialog).getByLabelText('Technique ids'), 'not-a-tech')
    await user.click(within(dialog).getByRole('button', { name: 'Create procedure' }))

    expect(await within(dialog).findByText('must look like T1059')).toBeInTheDocument()
  })

  it('dry-runs an import then confirms a write', async () => {
    const user = userEvent.setup()
    const calls: { dryRun: string | null; multipart: boolean }[] = []
    stubEmptyCustom()
    server.use(
      post('/content/custom/import', ({ request }) => {
        const url = new URL(request.url)
        const contentType = request.headers.get('content-type') ?? ''
        // Do not call request.formData() — undici rejects File bodies under jsdom
        // (same constraint as the sources bundle upload test).
        calls.push({
          dryRun: url.searchParams.get('dryRun'),
          multipart: contentType.includes('multipart/form-data'),
        })
        const dry = url.searchParams.get('dryRun') === 'true'
        return Response.json(dry ? emptyImportReport : committedImportReport)
      }),
    )
    renderCustom()

    await user.click(await screen.findByRole('button', { name: 'Import…' }))
    const dialog = await screen.findByRole('dialog')

    const fileInput = within(dialog).getByLabelText('File')
    const file = new File(
      [JSON.stringify([{ name: 'a', mitreid: 'T1059', actions: 'echo' }])],
      'testcases.json',
      { type: 'application/json' },
    )
    fireEvent.change(fileInput, { target: { files: [file] } })

    await user.click(within(dialog).getByRole('button', { name: 'Dry-run preview' }))

    expect(await within(dialog).findByText('Procedures created')).toBeInTheDocument()
    expect(within(dialog).getByText(/legacy actions field flattened/)).toBeInTheDocument()
    // Count "2" appears as proceduresCreated.
    expect(within(dialog).getAllByText('2').length).toBeGreaterThan(0)

    await user.click(within(dialog).getByRole('button', { name: 'Confirm import' }))

    await waitFor(() => {
      expect(calls).toEqual([
        { dryRun: 'true', multipart: true },
        { dryRun: 'false', multipart: true },
      ])
    })
    expect(await within(dialog).findByText(/Imported 2 procedure/)).toBeInTheDocument()
  })

  it('downloads a non-empty export', async () => {
    const user = userEvent.setup()
    stubEmptyCustom()

    const createObjectURL = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:export')
    const revokeObjectURL = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => undefined)
    const clickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(() => undefined)

    server.use(
      get('/content/custom/export', () => {
        return new Response('meta:\n  sourceName: Custom\nprocedureTemplates: []\n', {
          status: 200,
          headers: { 'Content-Type': 'application/yaml' },
        })
      }),
    )
    renderCustom()

    await user.click(await screen.findByRole('button', { name: 'Export YAML' }))

    await waitFor(() => {
      expect(createObjectURL).toHaveBeenCalled()
    })
    const blob = createObjectURL.mock.calls[0]?.[0] as Blob
    expect(blob).toBeInstanceOf(Blob)
    expect(blob.size).toBeGreaterThan(0)
    expect(clickSpy).toHaveBeenCalled()
    expect(revokeObjectURL).toHaveBeenCalled()
  })

  it('surfaces request id on list failure', async () => {
    server.use(
      get('/content/custom/procedure-templates', () =>
        problem({
          status: 500,
          code: 'internal',
          title: 'Internal Server Error',
          detail: 'boom',
        }),
      ),
      get('/content/custom/detection-rules', () => Response.json({ items: [] })),
      get('/content/custom/notes', () => Response.json({ items: [] })),
    )
    renderCustom()

    expect(await screen.findByRole('alert')).toHaveTextContent('That request failed.')
    expect(screen.getByText(TEST_REQUEST_ID)).toBeInTheDocument()
  })

  it('deletes a procedure after confirm', async () => {
    const user = userEvent.setup()
    let deleted = false
    stubWithProcedure()
    server.use(
      del('/content/custom/procedure-templates/{templateId}', () => {
        deleted = true
        return new Response(null, { status: 204 })
      }),
    )
    renderCustom()

    await screen.findByRole('button', { name: 'Echo twice' })
    const row = screen.getByRole('row', { name: /Echo twice/ })
    await user.click(within(row).getByRole('button', { name: 'Delete' }))

    const confirm = await screen.findByRole('alertdialog')
    await user.click(within(confirm).getByRole('button', { name: 'Delete procedure' }))

    await waitFor(() => {
      expect(deleted).toBe(true)
    })
  })

  it('still lists for a member fixture but write chrome is present only as page chrome', async () => {
    // Route guard is the real gate; this asserts the page itself does not crash
    // when given a member context (e.g. during tests) and still loads lists.
    stubWithProcedure()
    renderCustom(memberUserFixture)

    expect(await screen.findByRole('heading', { name: 'Custom content' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Import…' })).toBeInTheDocument()
  })
})

describe('procedure form patch', () => {
  it('sends a patch on edit', async () => {
    const user = userEvent.setup()
    let patched: unknown
    stubWithProcedure()
    server.use(
      patch('/content/custom/procedure-templates/{templateId}', async ({ request }) => {
        patched = await request.json()
        return Response.json({
          ...procedureFixture,
          name: 'Echo thrice',
        })
      }),
    )
    renderCustom()

    const row = await screen.findByRole('row', { name: /Echo twice/ })
    await user.click(within(row).getByRole('button', { name: 'Edit' }))
    const dialog = await screen.findByRole('dialog')
    const nameInput = within(dialog).getAllByLabelText('Name')[0]!
    await user.clear(nameInput)
    await user.type(nameInput, 'Echo thrice')
    await user.click(within(dialog).getByRole('button', { name: 'Save changes' }))

    await waitFor(() => {
      expect(patched).toMatchObject({ name: 'Echo thrice' })
    })
  })
})
