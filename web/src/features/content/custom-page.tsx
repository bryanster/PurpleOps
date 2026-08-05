import { type ReactNode, useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ConfirmDialog } from '@/features/admin/confirm-dialog'

import {
  useCustomDetections,
  useCustomNotes,
  useCustomProcedures,
  useDeleteCustomDetection,
  useDeleteCustomNote,
  useDeleteCustomProcedure,
  useExportCustomContent,
  type ContentDetectionRule,
  type ContentNote,
  type ContentProcedureTemplate,
} from './custom-queries'
import { DetectionFormDialog } from './detection-form-dialog'
import { ImportWizardDialog } from './import-wizard'
import { NoteFormDialog } from './note-form-dialog'
import { CONTENT_PATH } from './paths'
import { ProcedureFormDialog } from './procedure-form-dialog'
import { DetailDrawer, IdBadges, MetaRow, CopyBlock } from './shared'

type CustomTab = 'procedures' | 'detections' | 'notes'

/**
 * Custom content authoring + import/export (M2-015).
 *
 * Admin-only control plane. Members browse the same rows via the Content
 * library; they never reach this route (RequireAdmin + nav hide).
 */
export function CustomContentPage(): ReactNode {
  const [tab, setTab] = useState<CustomTab>('procedures')
  const [importOpen, setImportOpen] = useState(false)
  const exportMutation = useExportCustomContent()

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex flex-col gap-1">
          <h1 className="text-xl font-semibold">Custom content</h1>
          <p className="text-muted-foreground max-w-prose text-sm">
            Author procedure templates, detection rule references, and knowledge-base notes under
            the installation&apos;s custom source. Import v1 testcases or a prior export; browse
            results in the{' '}
            <Link to={CONTENT_PATH} className="underline-offset-4 hover:underline">
              content library
            </Link>
            .
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={exportMutation.isPending}
            onClick={() => {
              exportMutation.mutate(
                { format: 'yaml' },
                {
                  onSuccess: () => {
                    toast.success('Export downloaded.')
                  },
                  onError: (error) => {
                    toast.error(error.message)
                  },
                },
              )
            }}
          >
            {exportMutation.isPending ? 'Exporting…' : 'Export YAML'}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={exportMutation.isPending}
            onClick={() => {
              exportMutation.mutate(
                { format: 'json' },
                {
                  onSuccess: () => {
                    toast.success('Export downloaded.')
                  },
                  onError: (error) => {
                    toast.error(error.message)
                  },
                },
              )
            }}
          >
            Export JSON
          </Button>
          <Button
            type="button"
            size="sm"
            onClick={() => {
              setImportOpen(true)
            }}
          >
            Import…
          </Button>
        </div>
      </header>

      <Tabs
        value={tab}
        onValueChange={(value) => {
          setTab(value as CustomTab)
        }}
        className="gap-4"
      >
        <TabsList variant="line" className="w-full max-w-full flex-wrap justify-start">
          <TabsTrigger value="procedures">Procedures</TabsTrigger>
          <TabsTrigger value="detections">Detection rules</TabsTrigger>
          <TabsTrigger value="notes">Notes</TabsTrigger>
        </TabsList>
        <TabsContent value="procedures" className="outline-none">
          <ProceduresSection />
        </TabsContent>
        <TabsContent value="detections" className="outline-none">
          <DetectionsSection />
        </TabsContent>
        <TabsContent value="notes" className="outline-none">
          <NotesSection />
        </TabsContent>
      </Tabs>

      <ImportWizardDialog open={importOpen} onOpenChange={setImportOpen} />
    </div>
  )
}

function ProceduresSection(): ReactNode {
  const list = useCustomProcedures()
  const remove = useDeleteCustomProcedure()
  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<ContentProcedureTemplate | undefined>(undefined)
  const [detailId, setDetailId] = useState<string | undefined>(undefined)
  const [deleteTarget, setDeleteTarget] = useState<ContentProcedureTemplate | undefined>(undefined)

  const rows = list.data?.items ?? []
  const detail = rows.find((row) => row.id === detailId) ?? editTarget

  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-end">
        <Button
          type="button"
          size="sm"
          onClick={() => {
            setCreateOpen(true)
          }}
        >
          New procedure
        </Button>
      </div>

      {list.isPending && <PageLoading label="Reading custom procedures…" />}
      {list.error && (
        <PageError
          error={list.error}
          onRetry={() => {
            void list.refetch()
          }}
        />
      )}
      {list.data &&
        (rows.length === 0 ? (
          <PageEmpty
            title="No custom procedures yet"
            description="Create one, or import a v1 testcases file."
            action={
              <Button type="button" size="sm" variant="outline" onClick={() => setCreateOpen(true)}>
                New procedure
              </Button>
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead className="w-40">Techniques</TableHead>
                <TableHead className="w-36">Platforms</TableHead>
                <TableHead className="w-28">Executor</TableHead>
                <TableHead className="w-0" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <TableRow key={row.id}>
                  <TableCell>
                    <button
                      type="button"
                      className="hover:text-foreground focus-visible:ring-ring/50 rounded-sm text-left font-medium outline-none focus-visible:ring-3"
                      onClick={() => {
                        setDetailId(row.id)
                      }}
                    >
                      {row.name}
                    </button>
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    {row.techniqueExternalIds.length === 0
                      ? '—'
                      : row.techniqueExternalIds.join(', ')}
                  </TableCell>
                  <TableCell className="text-sm">
                    {row.platforms.length === 0 ? '—' : row.platforms.join(', ')}
                  </TableCell>
                  <TableCell className="font-mono text-xs">{row.executor || '—'}</TableCell>
                  <TableCell>
                    <div className="flex flex-wrap justify-end gap-1">
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setDetailId(row.id)
                        }}
                      >
                        Open
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setEditTarget(row)
                        }}
                      >
                        Edit
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setDeleteTarget(row)
                        }}
                      >
                        Delete
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ))}

      <ProcedureFormDialog open={createOpen} onOpenChange={setCreateOpen} />
      <ProcedureFormDialog
        open={editTarget !== undefined}
        initial={editTarget}
        onOpenChange={(open) => {
          if (!open) {
            setEditTarget(undefined)
          }
        }}
      />

      <ProcedureDetailDrawer
        row={detailId !== undefined ? (rows.find((r) => r.id === detailId) ?? detail) : undefined}
        open={detailId !== undefined}
        onOpenChange={(open) => {
          if (!open) {
            setDetailId(undefined)
          }
        }}
        onEdit={() => {
          const row = rows.find((r) => r.id === detailId)
          if (row !== undefined) {
            setDetailId(undefined)
            setEditTarget(row)
          }
        }}
        onDelete={() => {
          const row = rows.find((r) => r.id === detailId)
          if (row !== undefined) {
            setDeleteTarget(row)
          }
        }}
      />

      <ConfirmDialog
        open={deleteTarget !== undefined}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTarget(undefined)
          }
        }}
        title="Delete procedure template"
        description={
          deleteTarget === undefined
            ? ''
            : `Delete “${deleteTarget.name}”? This removes the custom template permanently.`
        }
        confirmLabel="Delete procedure"
        pending={remove.isPending}
        onConfirm={() => {
          if (deleteTarget === undefined) {
            return
          }
          remove.mutate(
            { templateId: deleteTarget.id },
            {
              onSuccess: () => {
                toast.success(`Deleted “${deleteTarget.name}”.`)
                if (detailId === deleteTarget.id) {
                  setDetailId(undefined)
                }
                setDeleteTarget(undefined)
              },
              onError: (error) => {
                toast.error(error.message)
              },
            },
          )
        }}
      />
    </div>
  )
}

function ProcedureDetailDrawer({
  row,
  open,
  onOpenChange,
  onEdit,
  onDelete,
}: {
  row: ContentProcedureTemplate | undefined
  open: boolean
  onOpenChange: (open: boolean) => void
  onEdit: () => void
  onDelete: () => void
}): ReactNode {
  return (
    <DetailDrawer
      open={open}
      onOpenChange={onOpenChange}
      title={row?.name ?? 'Procedure'}
      description={
        row !== undefined
          ? row.techniqueExternalIds.join(', ') || 'No technique mapping'
          : undefined
      }
    >
      {row && (
        <div className="flex flex-col gap-5">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <dl className="flex flex-col gap-3">
              <MetaRow label="Executor">
                <span className="font-mono">{row.executor || '—'}</span>
              </MetaRow>
              <MetaRow label="Platforms">
                {row.platforms.length === 0 ? '—' : row.platforms.join(', ')}
              </MetaRow>
              <MetaRow label="Elevation">
                {row.elevationRequired ? 'Required' : 'Not required'}
              </MetaRow>
              <MetaRow label="Techniques">
                <IdBadges ids={row.techniqueExternalIds} />
              </MetaRow>
            </dl>
            <div className="flex flex-wrap gap-2">
              <Button type="button" variant="outline" size="sm" onClick={onEdit}>
                Edit
              </Button>
              <Button type="button" variant="outline" size="sm" onClick={onDelete}>
                Delete
              </Button>
            </div>
          </div>

          {row.description !== '' && (
            <section className="flex flex-col gap-2">
              <h3 className="text-sm font-medium">Description</h3>
              <p className="text-muted-foreground text-sm whitespace-pre-wrap">{row.description}</p>
            </section>
          )}

          <CopyBlock label="Command" value={row.command} />
          {row.cleanup !== '' && <CopyBlock label="Cleanup" value={row.cleanup} />}

          {row.inputArgs.length > 0 && (
            <section className="flex flex-col gap-2">
              <h3 className="text-sm font-medium">Input arguments</h3>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Default</TableHead>
                    <TableHead>Description</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {row.inputArgs.map((arg) => (
                    <TableRow key={arg.name}>
                      <TableCell className="font-mono text-xs">{arg.name}</TableCell>
                      <TableCell className="font-mono text-xs">{arg.type}</TableCell>
                      <TableCell className="font-mono text-xs">{arg.default || '—'}</TableCell>
                      <TableCell className="text-muted-foreground text-sm">
                        {arg.description || '—'}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </section>
          )}
        </div>
      )}
    </DetailDrawer>
  )
}

function DetectionsSection(): ReactNode {
  const list = useCustomDetections()
  const remove = useDeleteCustomDetection()
  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<ContentDetectionRule | undefined>(undefined)
  const [detailId, setDetailId] = useState<string | undefined>(undefined)
  const [deleteTarget, setDeleteTarget] = useState<ContentDetectionRule | undefined>(undefined)
  const rows = list.data?.items ?? []

  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-end">
        <Button type="button" size="sm" onClick={() => setCreateOpen(true)}>
          New detection
        </Button>
      </div>

      {list.isPending && <PageLoading label="Reading custom detections…" />}
      {list.error && (
        <PageError
          error={list.error}
          onRetry={() => {
            void list.refetch()
          }}
        />
      )}
      {list.data &&
        (rows.length === 0 ? (
          <PageEmpty
            title="No custom detection rules yet"
            description="Create a reference rule, or import from a custom export."
            action={
              <Button type="button" size="sm" variant="outline" onClick={() => setCreateOpen(true)}>
                New detection
              </Button>
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Title</TableHead>
                <TableHead className="w-40">Techniques</TableHead>
                <TableHead className="w-24">Level</TableHead>
                <TableHead className="w-0" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <TableRow key={row.id}>
                  <TableCell>
                    <button
                      type="button"
                      className="hover:text-foreground focus-visible:ring-ring/50 rounded-sm text-left font-medium outline-none focus-visible:ring-3"
                      onClick={() => setDetailId(row.id)}
                    >
                      {row.name}
                    </button>
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    {row.techniqueExternalIds.length === 0
                      ? '—'
                      : row.techniqueExternalIds.join(', ')}
                  </TableCell>
                  <TableCell>
                    {row.level === '' ? '—' : <Badge variant="outline">{row.level}</Badge>}
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap justify-end gap-1">
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => setDetailId(row.id)}
                      >
                        Open
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => setEditTarget(row)}
                      >
                        Edit
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => setDeleteTarget(row)}
                      >
                        Delete
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ))}

      <DetectionFormDialog open={createOpen} onOpenChange={setCreateOpen} />
      <DetectionFormDialog
        open={editTarget !== undefined}
        initial={editTarget}
        onOpenChange={(open) => {
          if (!open) setEditTarget(undefined)
        }}
      />

      <DetailDrawer
        open={detailId !== undefined}
        onOpenChange={(open) => {
          if (!open) setDetailId(undefined)
        }}
        title={rows.find((r) => r.id === detailId)?.name ?? 'Detection rule'}
      >
        {(() => {
          const row = rows.find((r) => r.id === detailId)
          if (row === undefined) return null
          return (
            <div className="flex flex-col gap-5">
              <div className="flex flex-wrap justify-end gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setDetailId(undefined)
                    setEditTarget(row)
                  }}
                >
                  Edit
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setDeleteTarget(row)}
                >
                  Delete
                </Button>
              </div>
              <MetaRow label="Techniques">
                <IdBadges ids={row.techniqueExternalIds} />
              </MetaRow>
              <MetaRow label="Level">{row.level || '—'}</MetaRow>
              <CopyBlock label="Rule body" value={row.ruleYaml} />
            </div>
          )
        })()}
      </DetailDrawer>

      <ConfirmDialog
        open={deleteTarget !== undefined}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(undefined)
        }}
        title="Delete detection rule"
        description={
          deleteTarget === undefined
            ? ''
            : `Delete “${deleteTarget.name}”? This removes the custom rule reference permanently.`
        }
        confirmLabel="Delete detection"
        pending={remove.isPending}
        onConfirm={() => {
          if (deleteTarget === undefined) return
          remove.mutate(
            { ruleId: deleteTarget.id },
            {
              onSuccess: () => {
                toast.success(`Deleted “${deleteTarget.name}”.`)
                if (detailId === deleteTarget.id) setDetailId(undefined)
                setDeleteTarget(undefined)
              },
              onError: (error) => {
                toast.error(error.message)
              },
            },
          )
        }}
      />
    </div>
  )
}

function NotesSection(): ReactNode {
  const list = useCustomNotes()
  const remove = useDeleteCustomNote()
  const [createOpen, setCreateOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<ContentNote | undefined>(undefined)
  const [detailId, setDetailId] = useState<string | undefined>(undefined)
  const [deleteTarget, setDeleteTarget] = useState<ContentNote | undefined>(undefined)
  const rows = list.data?.items ?? []

  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-end">
        <Button type="button" size="sm" onClick={() => setCreateOpen(true)}>
          New note
        </Button>
      </div>

      {list.isPending && <PageLoading label="Reading custom notes…" />}
      {list.error && (
        <PageError
          error={list.error}
          onRetry={() => {
            void list.refetch()
          }}
        />
      )}
      {list.data &&
        (rows.length === 0 ? (
          <PageEmpty
            title="No custom notes yet"
            description="Create a note, or import knowledgebase YAML."
            action={
              <Button type="button" size="sm" variant="outline" onClick={() => setCreateOpen(true)}>
                New note
              </Button>
            }
          />
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Title</TableHead>
                <TableHead className="w-32">Technique</TableHead>
                <TableHead>Tags</TableHead>
                <TableHead className="w-0" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {rows.map((row) => (
                <TableRow key={row.id}>
                  <TableCell>
                    <button
                      type="button"
                      className="hover:text-foreground focus-visible:ring-ring/50 rounded-sm text-left font-medium outline-none focus-visible:ring-3"
                      onClick={() => setDetailId(row.id)}
                    >
                      {row.title}
                    </button>
                  </TableCell>
                  <TableCell className="font-mono text-xs">
                    {row.techniqueExternalId === '' ? '—' : row.techniqueExternalId}
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {row.tags.length === 0
                        ? '—'
                        : row.tags.map((tag) => (
                            <Badge key={tag} variant="secondary">
                              {tag}
                            </Badge>
                          ))}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-wrap justify-end gap-1">
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => setDetailId(row.id)}
                      >
                        Open
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => setEditTarget(row)}
                      >
                        Edit
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => setDeleteTarget(row)}
                      >
                        Delete
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        ))}

      <NoteFormDialog open={createOpen} onOpenChange={setCreateOpen} />
      <NoteFormDialog
        open={editTarget !== undefined}
        initial={editTarget}
        onOpenChange={(open) => {
          if (!open) setEditTarget(undefined)
        }}
      />

      <DetailDrawer
        open={detailId !== undefined}
        onOpenChange={(open) => {
          if (!open) setDetailId(undefined)
        }}
        title={rows.find((r) => r.id === detailId)?.title ?? 'Note'}
      >
        {(() => {
          const row = rows.find((r) => r.id === detailId)
          if (row === undefined) return null
          return (
            <div className="flex flex-col gap-5">
              <div className="flex flex-wrap justify-end gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setDetailId(undefined)
                    setEditTarget(row)
                  }}
                >
                  Edit
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => setDeleteTarget(row)}
                >
                  Delete
                </Button>
              </div>
              {row.techniqueExternalId !== '' && (
                <MetaRow label="Technique">
                  <span className="font-mono">{row.techniqueExternalId}</span>
                </MetaRow>
              )}
              {row.tags.length > 0 && (
                <MetaRow label="Tags">
                  <div className="flex flex-wrap gap-1">
                    {row.tags.map((tag) => (
                      <Badge key={tag} variant="secondary">
                        {tag}
                      </Badge>
                    ))}
                  </div>
                </MetaRow>
              )}
              <section className="flex flex-col gap-2">
                <h3 className="text-sm font-medium">Body</h3>
                <pre className="bg-muted max-h-96 overflow-auto rounded-md border p-3 font-mono text-xs whitespace-pre-wrap">
                  {row.bodyMarkdown === '' ? '—' : row.bodyMarkdown}
                </pre>
              </section>
            </div>
          )
        })()}
      </DetailDrawer>

      <ConfirmDialog
        open={deleteTarget !== undefined}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(undefined)
        }}
        title="Delete note"
        description={
          deleteTarget === undefined
            ? ''
            : `Delete “${deleteTarget.title}”? This removes the note permanently.`
        }
        confirmLabel="Delete note"
        pending={remove.isPending}
        onConfirm={() => {
          if (deleteTarget === undefined) return
          remove.mutate(
            { noteId: deleteTarget.id },
            {
              onSuccess: () => {
                toast.success(`Deleted “${deleteTarget.title}”.`)
                if (detailId === deleteTarget.id) setDetailId(undefined)
                setDeleteTarget(undefined)
              },
              onError: (error) => {
                toast.error(error.message)
              },
            },
          )
        }}
      />
    </div>
  )
}
