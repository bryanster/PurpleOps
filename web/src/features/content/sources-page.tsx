import { type ChangeEvent, type ReactNode, useId, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
import { ConfirmDialog } from '@/features/admin/confirm-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatMoment } from '@/lib/time'

import { JobProgressPanel } from './job-progress'
import {
  isCustomSource,
  kindLabel,
  statusLabel,
  useContentSource,
  useContentSources,
  useContentSourceVersions,
  useDeleteContentAttackVersion,
  useDeleteContentSource,
  useDisableContentSource,
  useEnableContentSource,
  useJobSlotHeld,
  useReprocessContentSource,
  useStartContentSourceSync,
  useUpdateContentSource,
  useUploadContentBundle,
  type ContentSource,
  type ContentSourceVersion,
} from './sources-queries'

/**
 * Sources admin control plane (M2-014).
 *
 * Enable, sync, offline bundle, reprocess, edit URL/ref, delete — with a live
 * job panel and a global slot gate so two admins cannot start overlapping work.
 * Custom is a permanent home for user content: delete is hidden, not offered
 * and then 409'd.
 */
export function SourcesPage(): ReactNode {
  const sources = useContentSources()
  const slot = useJobSlotHeld()
  const [detailId, setDetailId] = useState<string | undefined>(undefined)

  const sourceNames = useMemo(() => {
    const map = new Map<string, string>()
    for (const source of sources.data?.items ?? []) {
      map.set(source.id, source.name)
    }
    return map
  }, [sources.data?.items])

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="text-xl font-semibold">Content sources</h1>
        <p className="text-muted-foreground max-w-prose text-sm">
          Install and refresh reference content. Online sync and offline bundle import share one job
          slot for the whole installation — start one, wait for it to finish, then start the next.
        </p>
      </header>

      <JobProgressPanel sourceNames={sourceNames} />

      {slot.held && (
        <p className="text-muted-foreground text-sm" role="status">
          Sync, bundle upload, and reprocess are paused while a job holds the slot. {slot.reason}
        </p>
      )}

      {sources.isPending && <PageLoading label="Reading content sources…" />}

      {sources.error && (
        <PageError
          error={sources.error}
          onRetry={() => {
            void sources.refetch()
          }}
        />
      )}

      {sources.data &&
        (sources.data.items.length === 0 ? (
          <PageEmpty
            title="No content sources"
            description="The installation seed should create the upstream rows. Check migrations if this list is empty."
          />
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Source</TableHead>
                  <TableHead>Kind</TableHead>
                  <TableHead>Enabled</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Last synced</TableHead>
                  <TableHead className="text-right">Items</TableHead>
                  <TableHead>Error</TableHead>
                  <TableHead className="w-0" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {sources.data.items.map((source) => (
                  <SourceRow
                    key={source.id}
                    source={source}
                    slotHeld={slot.held}
                    slotReason={slot.reason}
                    onOpenDetail={() => {
                      setDetailId(source.id)
                    }}
                  />
                ))}
              </TableBody>
            </Table>
          </div>
        ))}

      <SourceDetailDialog
        sourceId={detailId}
        slotHeld={slot.held}
        slotReason={slot.reason}
        onOpenChange={(open) => {
          if (!open) {
            setDetailId(undefined)
          }
        }}
      />
    </div>
  )
}

function SourceRow({
  source,
  slotHeld,
  slotReason,
  onOpenDetail,
}: {
  source: ContentSource
  slotHeld: boolean
  slotReason: string | undefined
  onOpenDetail: () => void
}): ReactNode {
  const enable = useEnableContentSource()
  const disable = useDisableContentSource()
  const sync = useStartContentSourceSync()
  const reprocess = useReprocessContentSource()
  const upload = useUploadContentBundle()
  const remove = useDeleteContentSource()

  const [confirmDisable, setConfirmDisable] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [confirmReprocess, setConfirmReprocess] = useState(false)
  const [bundleOpen, setBundleOpen] = useState(false)
  const [syncVersionOpen, setSyncVersionOpen] = useState(false)

  const custom = isCustomSource(source)
  const busy =
    enable.isPending ||
    disable.isPending ||
    sync.isPending ||
    reprocess.isPending ||
    upload.isPending ||
    remove.isPending
  const jobActionsDisabled = slotHeld || busy || custom
  const jobActionsTitle = custom
    ? 'Custom content is authored in-app, not synced from an upstream.'
    : slotHeld
      ? (slotReason ?? 'A content job is already running.')
      : undefined

  return (
    <TableRow>
      <TableCell>
        <button
          type="button"
          className="text-left font-medium underline-offset-4 hover:underline"
          onClick={onOpenDetail}
        >
          {source.name}
        </button>
      </TableCell>
      <TableCell>
        <Badge variant="outline">{kindLabel(source.kind)}</Badge>
      </TableCell>
      <TableCell>
        <Badge variant={source.enabled ? 'secondary' : 'outline'}>
          {source.enabled ? 'Enabled' : 'Disabled'}
        </Badge>
      </TableCell>
      <TableCell>
        <Badge variant={source.status === 'error' ? 'destructive' : 'outline'}>
          {statusLabel(source.status)}
        </Badge>
      </TableCell>
      <TableCell className="text-sm whitespace-nowrap">
        {source.lastSyncedAt === undefined ? 'Never' : formatMoment(source.lastSyncedAt)}
      </TableCell>
      <TableCell className="text-right font-mono text-sm">{source.itemCount}</TableCell>
      <TableCell className="max-w-56 truncate text-sm" title={source.error || undefined}>
        {source.error === '' ? (
          <span className="text-muted-foreground">—</span>
        ) : (
          <span className="text-destructive">{source.error}</span>
        )}
      </TableCell>
      <TableCell>
        <div className="flex flex-wrap justify-end gap-1">
          <Button variant="ghost" size="sm" onClick={onOpenDetail} disabled={busy}>
            Details
          </Button>
          {source.enabled ? (
            <Button
              variant="ghost"
              size="sm"
              disabled={busy}
              onClick={() => {
                setConfirmDisable(true)
              }}
            >
              Disable
            </Button>
          ) : (
            <Button
              variant="ghost"
              size="sm"
              disabled={busy}
              onClick={() => {
                enable.mutate(
                  { sourceId: source.id },
                  {
                    onSuccess: () => {
                      toast.success(`${source.name} is enabled.`)
                    },
                    onError: (error) => {
                      toast.error(error.message)
                    },
                  },
                )
              }}
            >
              Enable
            </Button>
          )}
          {source.kind === 'attack' ? (
            <Button
              variant="ghost"
              size="sm"
              disabled={jobActionsDisabled}
              title={jobActionsTitle}
              onClick={() => {
                setSyncVersionOpen(true)
              }}
            >
              Sync
            </Button>
          ) : (
            <Button
              variant="ghost"
              size="sm"
              disabled={jobActionsDisabled}
              title={jobActionsTitle}
              onClick={() => {
                sync.mutate(
                  { sourceId: source.id },
                  {
                    onSuccess: (job) => {
                      toast.success(`Sync queued (${job.id}).`)
                    },
                    onError: (error) => {
                      toast.error(error.message)
                    },
                  },
                )
              }}
            >
              Sync
            </Button>
          )}
          <Button
            variant="ghost"
            size="sm"
            disabled={jobActionsDisabled}
            title={jobActionsTitle}
            onClick={() => {
              setBundleOpen(true)
            }}
          >
            Upload bundle
          </Button>
          <Button
            variant="ghost"
            size="sm"
            disabled={jobActionsDisabled}
            title={jobActionsTitle}
            onClick={() => {
              setConfirmReprocess(true)
            }}
          >
            Reprocess
          </Button>
          {!custom && (
            <Button
              variant="ghost"
              size="sm"
              disabled={busy}
              onClick={() => {
                setConfirmDelete(true)
              }}
            >
              Delete
            </Button>
          )}
        </div>

        <ConfirmDialog
          open={confirmDisable}
          onOpenChange={setConfirmDisable}
          title={`Disable ${source.name}?`}
          description="Installed objects stay on disk but leave browse, search, and pickers. New references to this source are refused until it is enabled again. Running jobs are not cancelled."
          confirmLabel="Disable source"
          pending={disable.isPending}
          onConfirm={() => {
            disable.mutate(
              { sourceId: source.id },
              {
                onSuccess: () => {
                  setConfirmDisable(false)
                  toast.success(`${source.name} is disabled.`)
                },
                onError: (error) => {
                  toast.error(error.message)
                },
              },
            )
          }}
        />

        <ConfirmDialog
          open={confirmReprocess}
          onOpenChange={setConfirmReprocess}
          title={`Reprocess ${source.name}?`}
          description={
            source.kind === 'attack'
              ? 'Reprocess walks every stored raw snapshot for this source through Parse → Normalize → Apply again. Prefer the per-version control in Details when you only need one release. Requires at least one raw snapshot.'
              : 'Reprocess opens the last raw snapshot and runs Parse → Normalize → Apply with no network fetch. Fails with a conflict if no raw snapshot exists yet.'
          }
          confirmLabel="Reprocess"
          destructive={false}
          pending={reprocess.isPending}
          onConfirm={() => {
            reprocess.mutate(
              { sourceId: source.id },
              {
                onSuccess: (job) => {
                  setConfirmReprocess(false)
                  toast.success(`Reprocess queued (${job.id}).`)
                },
                onError: (error) => {
                  toast.error(error.message)
                },
              },
            )
          }}
        />

        <ConfirmDialog
          open={confirmDelete}
          onOpenChange={setConfirmDelete}
          title={`Delete ${source.name}?`}
          description="This permanently removes the source row and every version, raw snapshot, and normalized object under it. Engagements that already reference those objects keep their pins but can no longer resolve library detail. This cannot be undone from the UI."
          confirmLabel="Delete source"
          pending={remove.isPending}
          onConfirm={() => {
            remove.mutate(
              { sourceId: source.id },
              {
                onSuccess: () => {
                  setConfirmDelete(false)
                  toast.success(`${source.name} was deleted.`)
                },
                onError: (error) => {
                  toast.error(error.message)
                },
              },
            )
          }}
        />

        <BundleUploadDialog
          open={bundleOpen}
          onOpenChange={setBundleOpen}
          source={source}
          pending={upload.isPending}
          onUpload={(file, version) => {
            upload.mutate(
              { sourceId: source.id, file, version },
              {
                onSuccess: (job) => {
                  setBundleOpen(false)
                  toast.success(`Bundle import queued (${job.id}).`)
                },
                onError: (error) => {
                  toast.error(error.message)
                },
              },
            )
          }}
        />

        {source.kind === 'attack' && (
          <AttackSyncDialog
            open={syncVersionOpen}
            onOpenChange={setSyncVersionOpen}
            sourceName={source.name}
            pending={sync.isPending}
            onSync={(version) => {
              sync.mutate(
                { sourceId: source.id, version: version === '' ? undefined : version },
                {
                  onSuccess: (job) => {
                    setSyncVersionOpen(false)
                    toast.success(`Sync queued (${job.id}).`)
                  },
                  onError: (error) => {
                    toast.error(error.message)
                  },
                },
              )
            }}
          />
        )}
      </TableCell>
    </TableRow>
  )
}

function BundleUploadDialog({
  open,
  onOpenChange,
  source,
  pending,
  onUpload,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  source: ContentSource
  pending: boolean
  onUpload: (file: File, version?: string) => void
}): ReactNode {
  const fileId = useId()
  const versionId = useId()
  const inputRef = useRef<HTMLInputElement>(null)
  const [version, setVersion] = useState('')
  const [chosenName, setChosenName] = useState<string | undefined>(undefined)

  function reset(): void {
    setVersion('')
    setChosenName(undefined)
    if (inputRef.current !== null) {
      inputRef.current.value = ''
    }
  }

  function handleFileChange(event: ChangeEvent<HTMLInputElement>): void {
    const file = event.target.files?.[0]
    setChosenName(file?.name)
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          reset()
        }
        onOpenChange(next)
      }}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Upload bundle — {source.name}</DialogTitle>
          <DialogDescription>
            Offline install uses the same parse path as online sync. Choose a release archive (zip /
            tar.gz) that matches what the upstream fetch would have produced.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4 py-2">
          <div className="flex flex-col gap-2">
            <Label htmlFor={fileId}>Archive file</Label>
            <Input
              id={fileId}
              ref={inputRef}
              type="file"
              accept=".zip,.tar.gz,.tgz,application/zip,application/gzip"
              disabled={pending}
              onChange={handleFileChange}
            />
            {chosenName !== undefined && (
              <p className="text-muted-foreground text-xs">{chosenName}</p>
            )}
          </div>
          {source.kind === 'attack' && (
            <div className="flex flex-col gap-2">
              <Label htmlFor={versionId}>ATT&amp;CK version (optional)</Label>
              <Input
                id={versionId}
                value={version}
                placeholder="e.g. 15.1 — omit to use the archive label"
                disabled={pending}
                onChange={(event) => {
                  setVersion(event.target.value)
                }}
              />
            </div>
          )}
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() => {
              onOpenChange(false)
            }}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={pending || chosenName === undefined}
            onClick={() => {
              const file = inputRef.current?.files?.[0]
              if (file === undefined) {
                toast.error('Choose an archive file first.')
                return
              }
              // File is read from the input at submit time — never stored in
              // query cache or long-lived component state.
              onUpload(file, version.trim() === '' ? undefined : version.trim())
            }}
          >
            {pending ? 'Uploading…' : 'Upload and import'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function AttackSyncDialog({
  open,
  onOpenChange,
  sourceName,
  pending,
  onSync,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  sourceName: string
  pending: boolean
  onSync: (version: string) => void
}): ReactNode {
  const versionId = useId()
  const [version, setVersion] = useState('')

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          setVersion('')
        }
        onOpenChange(next)
      }}
    >
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Sync {sourceName}</DialogTitle>
          <DialogDescription>
            Pin an ATT&amp;CK release label, or leave blank to fetch the latest discoverable
            enterprise release.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-2 py-2">
          <Label htmlFor={versionId}>Version</Label>
          <Input
            id={versionId}
            value={version}
            placeholder="e.g. 15.1"
            disabled={pending}
            onChange={(event) => {
              setVersion(event.target.value)
            }}
          />
        </div>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            disabled={pending}
            onClick={() => {
              onOpenChange(false)
            }}
          >
            Cancel
          </Button>
          <Button
            type="button"
            disabled={pending}
            onClick={() => {
              onSync(version.trim())
            }}
          >
            {pending ? 'Starting…' : 'Start sync'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function SourceDetailDialog({
  sourceId,
  slotHeld,
  slotReason,
  onOpenChange,
}: {
  sourceId: string | undefined
  slotHeld: boolean
  slotReason: string | undefined
  onOpenChange: (open: boolean) => void
}): ReactNode {
  const open = sourceId !== undefined
  const detail = useContentSource(sourceId)
  const versions = useContentSourceVersions(sourceId)
  const update = useUpdateContentSource()
  const sync = useStartContentSourceSync()
  const reprocess = useReprocessContentSource()
  const deleteVersion = useDeleteContentAttackVersion()

  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [refValue, setRefValue] = useState('')
  const [hydratedFor, setHydratedFor] = useState<string | undefined>(undefined)
  const [confirmVersionDelete, setConfirmVersionDelete] = useState<string | undefined>(undefined)
  const [confirmVersionReprocess, setConfirmVersionReprocess] = useState<string | undefined>(
    undefined,
  )

  // Hydrate the edit form when a new source opens — not on every keystroke refetch.
  if (detail.data !== undefined && detail.data.id !== hydratedFor) {
    setName(detail.data.name)
    setUrl(detail.data.url)
    setRefValue(detail.data.ref)
    setHydratedFor(detail.data.id)
  }

  const source = detail.data
  const custom = source !== undefined && isCustomSource(source)
  const nameId = useId()
  const urlId = useId()
  const refId = useId()

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          setHydratedFor(undefined)
          setConfirmVersionDelete(undefined)
          setConfirmVersionReprocess(undefined)
        }
        onOpenChange(next)
      }}
    >
      <DialogContent className="flex max-h-[min(90dvh,48rem)] max-w-2xl flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
        <DialogHeader className="border-b px-6 py-4 text-left">
          <DialogTitle className="pr-8 text-base leading-snug">
            {source?.name ?? 'Source'}
          </DialogTitle>
          <DialogDescription className="text-left">
            License, upstream location, and installed versions.
          </DialogDescription>
        </DialogHeader>

        <div className="min-h-0 flex-1 overflow-y-auto px-6 py-4">
          {detail.isPending && <PageLoading label="Reading source…" />}
          {detail.error && (
            <PageError
              error={detail.error}
              onRetry={() => {
                void detail.refetch()
              }}
            />
          )}
          {source !== undefined && (
            <div className="flex flex-col gap-6">
              <LicenseBlock source={source} />

              {source.lastJob !== undefined && (
                <section className="flex flex-col gap-2">
                  <h3 className="text-sm font-medium">Last job</h3>
                  <dl className="grid gap-2 text-sm sm:grid-cols-2">
                    <div>
                      <dt className="text-muted-foreground">Status</dt>
                      <dd>{source.lastJob.status}</dd>
                    </div>
                    <div>
                      <dt className="text-muted-foreground">Kind</dt>
                      <dd>{source.lastJob.kind}</dd>
                    </div>
                    <div className="sm:col-span-2">
                      <dt className="text-muted-foreground">Job id</dt>
                      <dd className="font-mono text-xs break-all">{source.lastJob.id}</dd>
                    </div>
                    {source.lastJob.error !== undefined && source.lastJob.error !== '' && (
                      <div className="sm:col-span-2">
                        <dt className="text-muted-foreground">Error</dt>
                        <dd className="text-destructive whitespace-pre-wrap">
                          {source.lastJob.error}
                        </dd>
                      </div>
                    )}
                  </dl>
                </section>
              )}

              {!custom && (
                <section className="flex flex-col gap-3">
                  <h3 className="text-sm font-medium">Upstream (advanced)</h3>
                  <p className="text-muted-foreground text-xs">
                    Change these only when you know the adapter expects a different archive URL or
                    ref pattern. A bad URL fails the next sync; it does not rewrite installed
                    objects by itself.
                  </p>
                  <div className="flex flex-col gap-3">
                    <div className="flex flex-col gap-2">
                      <Label htmlFor={nameId}>Name</Label>
                      <Input
                        id={nameId}
                        value={name}
                        onChange={(event) => {
                          setName(event.target.value)
                        }}
                      />
                    </div>
                    <div className="flex flex-col gap-2">
                      <Label htmlFor={urlId}>URL</Label>
                      <Input
                        id={urlId}
                        value={url}
                        onChange={(event) => {
                          setUrl(event.target.value)
                        }}
                      />
                    </div>
                    <div className="flex flex-col gap-2">
                      <Label htmlFor={refId}>Ref</Label>
                      <Input
                        id={refId}
                        value={refValue}
                        onChange={(event) => {
                          setRefValue(event.target.value)
                        }}
                      />
                    </div>
                    <Button
                      type="button"
                      variant="outline"
                      className="self-start"
                      disabled={update.isPending}
                      onClick={() => {
                        const patch: {
                          name?: string
                          url?: string
                          ref?: string
                        } = {}
                        if (name.trim() !== source.name) {
                          patch.name = name.trim()
                        }
                        if (url !== source.url) {
                          patch.url = url
                        }
                        if (refValue !== source.ref) {
                          patch.ref = refValue
                        }
                        if (Object.keys(patch).length === 0) {
                          toast.message('Nothing to save.')
                          return
                        }
                        update.mutate(
                          { sourceId: source.id, patch },
                          {
                            onSuccess: () => {
                              toast.success('Source updated.')
                            },
                            onError: (error) => {
                              toast.error(error.message)
                            },
                          },
                        )
                      }}
                    >
                      {update.isPending ? 'Saving…' : 'Save upstream'}
                    </Button>
                  </div>
                </section>
              )}

              {custom && (
                <p className="text-muted-foreground text-sm">
                  Custom content cannot be deleted — it is the permanent home for user-authored
                  templates, rules, and notes. Author and import it from the custom content screens
                  (M2-015).
                </p>
              )}

              <section className="flex flex-col gap-3">
                <h3 className="text-sm font-medium">Installed versions</h3>
                {versions.isPending && <PageLoading label="Reading versions…" />}
                {versions.error && (
                  <PageError
                    error={versions.error}
                    onRetry={() => {
                      void versions.refetch()
                    }}
                  />
                )}
                {versions.data &&
                  (versions.data.items.length === 0 ? (
                    <p className="text-muted-foreground text-sm">
                      No versions installed yet. Enable the source and sync or upload a bundle.
                    </p>
                  ) : (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Version</TableHead>
                          <TableHead>Status</TableHead>
                          <TableHead className="text-right">Items</TableHead>
                          <TableHead>Synced</TableHead>
                          <TableHead className="w-0" />
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {versions.data.items.map((version) => (
                          <VersionRow
                            key={version.id}
                            version={version}
                            source={source}
                            slotHeld={slotHeld}
                            slotReason={slotReason}
                            syncPending={sync.isPending}
                            reprocessPending={reprocess.isPending}
                            deletePending={deleteVersion.isPending}
                            onSync={() => {
                              sync.mutate(
                                { sourceId: source.id, version: version.version },
                                {
                                  onSuccess: (job) => {
                                    toast.success(`Sync queued for ${version.version} (${job.id}).`)
                                  },
                                  onError: (error) => {
                                    toast.error(error.message)
                                  },
                                },
                              )
                            }}
                            onReprocess={() => {
                              setConfirmVersionReprocess(version.version)
                            }}
                            onDelete={
                              source.kind === 'attack'
                                ? () => {
                                    setConfirmVersionDelete(version.version)
                                  }
                                : undefined
                            }
                          />
                        ))}
                      </TableBody>
                    </Table>
                  ))}
              </section>
            </div>
          )}
        </div>
      </DialogContent>

      <ConfirmDialog
        open={confirmVersionDelete !== undefined}
        onOpenChange={(next) => {
          if (!next) {
            setConfirmVersionDelete(undefined)
          }
        }}
        title={`Delete ATT&CK ${confirmVersionDelete ?? ''}?`}
        description="Removes this release's normalized objects and raw snapshot. Other installed ATT&CK versions are left alone. Engagements pinned to this label will no longer resolve library detail for it."
        confirmLabel="Delete version"
        pending={deleteVersion.isPending}
        onConfirm={() => {
          if (confirmVersionDelete === undefined) {
            return
          }
          deleteVersion.mutate(
            { version: confirmVersionDelete },
            {
              onSuccess: () => {
                toast.success(`ATT&CK ${confirmVersionDelete} was deleted.`)
                setConfirmVersionDelete(undefined)
              },
              onError: (error) => {
                toast.error(error.message)
              },
            },
          )
        }}
      />

      <ConfirmDialog
        open={confirmVersionReprocess !== undefined}
        onOpenChange={(next) => {
          if (!next) {
            setConfirmVersionReprocess(undefined)
          }
        }}
        title={`Reprocess ${confirmVersionReprocess ?? ''}?`}
        description="Runs Parse → Normalize → Apply against the stored raw snapshot for this version only. No network fetch."
        confirmLabel="Reprocess version"
        destructive={false}
        pending={reprocess.isPending}
        onConfirm={() => {
          if (source === undefined || confirmVersionReprocess === undefined) {
            return
          }
          reprocess.mutate(
            { sourceId: source.id, version: confirmVersionReprocess },
            {
              onSuccess: (job) => {
                toast.success(`Reprocess queued (${job.id}).`)
                setConfirmVersionReprocess(undefined)
              },
              onError: (error) => {
                toast.error(error.message)
              },
            },
          )
        }}
      />
    </Dialog>
  )
}

function VersionRow({
  version,
  source,
  slotHeld,
  slotReason,
  syncPending,
  reprocessPending,
  deletePending,
  onSync,
  onReprocess,
  onDelete,
}: {
  version: ContentSourceVersion
  source: ContentSource
  slotHeld: boolean
  slotReason: string | undefined
  syncPending: boolean
  reprocessPending: boolean
  deletePending: boolean
  onSync: () => void
  onReprocess: () => void
  onDelete: (() => void) | undefined
}): ReactNode {
  const custom = isCustomSource(source)
  const jobDisabled = slotHeld || custom || syncPending || reprocessPending

  return (
    <TableRow>
      <TableCell className="font-mono text-sm">{version.version}</TableCell>
      <TableCell>
        <Badge variant="outline">{version.status}</Badge>
      </TableCell>
      <TableCell className="text-right font-mono text-sm">{version.itemCount}</TableCell>
      <TableCell className="text-sm whitespace-nowrap">
        {version.syncedAt === undefined ? '—' : formatMoment(version.syncedAt)}
      </TableCell>
      <TableCell>
        <div className="flex justify-end gap-1">
          {!custom && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              disabled={jobDisabled}
              title={slotHeld ? slotReason : undefined}
              onClick={onSync}
            >
              Sync
            </Button>
          )}
          {!custom && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              disabled={jobDisabled || version.rawSha256 === ''}
              title={
                version.rawSha256 === ''
                  ? 'No raw snapshot to reprocess.'
                  : slotHeld
                    ? slotReason
                    : undefined
              }
              onClick={onReprocess}
            >
              Reprocess
            </Button>
          )}
          {onDelete !== undefined && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              disabled={deletePending}
              onClick={onDelete}
            >
              Delete
            </Button>
          )}
        </div>
      </TableCell>
    </TableRow>
  )
}

function LicenseBlock({ source }: { source: ContentSource }): ReactNode {
  const hasLicense =
    source.licenseSpdx !== '' ||
    source.licenseName !== '' ||
    source.licenseUrl !== '' ||
    source.attribution !== ''

  if (!hasLicense) {
    return (
      <section className="flex flex-col gap-1">
        <h3 className="text-sm font-medium">License</h3>
        <p className="text-muted-foreground text-sm">No license metadata on this source.</p>
      </section>
    )
  }

  return (
    <section className="flex flex-col gap-2" data-testid="source-license">
      <h3 className="text-sm font-medium">License</h3>
      <dl className="grid gap-2 text-sm">
        {source.licenseSpdx !== '' && (
          <div className="grid gap-1 sm:grid-cols-[8rem_1fr] sm:gap-3">
            <dt className="text-muted-foreground">SPDX</dt>
            <dd className="font-mono">{source.licenseSpdx}</dd>
          </div>
        )}
        {source.licenseName !== '' && (
          <div className="grid gap-1 sm:grid-cols-[8rem_1fr] sm:gap-3">
            <dt className="text-muted-foreground">Name</dt>
            <dd>{source.licenseName}</dd>
          </div>
        )}
        {source.licenseUrl !== '' && (
          <div className="grid gap-1 sm:grid-cols-[8rem_1fr] sm:gap-3">
            <dt className="text-muted-foreground">Link</dt>
            <dd>
              <a
                href={source.licenseUrl}
                target="_blank"
                rel="noreferrer"
                className="text-primary underline-offset-4 hover:underline"
              >
                {source.licenseUrl}
              </a>
            </dd>
          </div>
        )}
        {source.attribution !== '' && (
          <div className="grid gap-1 sm:grid-cols-[8rem_1fr] sm:gap-3">
            <dt className="text-muted-foreground">Attribution</dt>
            <dd className="whitespace-pre-wrap">{source.attribution}</dd>
          </div>
        )}
      </dl>
    </section>
  )
}
