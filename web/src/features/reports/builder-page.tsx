import {
  closestCenter,
  DndContext,
  type DragEndEvent,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from '@dnd-kit/core'
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import {
  BookTemplateIcon,
  FileTextIcon,
  GripVerticalIcon,
  PanelRightCloseIcon,
  PanelRightIcon,
  PlusIcon,
  SaveIcon,
  ShieldAlertIcon,
  Trash2Icon,
} from 'lucide-react'
import { type ReactNode, useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router'

import { isApiError } from '@/api/errors'
import { PageError, PageLoading } from '@/app/shell/page-state'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useEngagements } from '@/features/engagements/queries'
import { cn } from '@/lib/utils'
import { toast } from 'sonner'

import { getBlockEntry, getCatalog, type BlockParamField, type BlockParams } from './block-catalog'
import { RichTextEditor } from './rich-text-editor'
import {
  useApplyTemplate,
  useCreateTemplateFromReport,
  usePreviewHtml,
  usePutReportBlocks,
  useReport,
  useTemplates,
  type Report,
  type ReportBlockInput,
  type ReportVersion,
} from './queries'
import { ReportSettingsPanel } from './report-settings-panel'
import { PublishDialog } from './publish/publish-dialog'
import { VersionsPanel } from './publish/versions-panel'

/**
 * The generated `params` type is `Record<string, never>` — openapi-typescript's
 * rendering of a free-form JSON object, which no real value satisfies. These
 * two are the only places that reconcile it with the object the block schemas
 * actually describe, rather than every editor doing it with a cast.
 */
function asParams(params: ReportBlockInput['params']): BlockParams {
  return params ?? {}
}

function toWireParams(params: BlockParams): ReportBlockInput['params'] {
  return params as ReportBlockInput['params']
}

/**
 * A one-line description of why a save was refused, naming the block rather
 * than the wire path.
 *
 * The server reports a rejected parameter as `blocks[2].params`, which is the
 * right thing to send and the wrong thing to show: the user sees a column of
 * named blocks, not an array. Anything that is not a field-level rejection
 * returns undefined, and the toast's own title carries it.
 */
function saveErrorDetail(error: unknown, blocks: ReportBlockInput[]): string | undefined {
  if (!isApiError(error) || error.errors.length === 0) return undefined

  return error.errors
    .map((entry) => {
      const index = /^blocks\[(\d+)\]/.exec(entry.field)?.[1]
      if (index === undefined) return entry.message
      const block = blocks[Number(index)]
      const title = block ? (getBlockEntry(block.blockId)?.title ?? block.blockId) : 'Block'
      return `${title}: ${entry.message}`
    })
    .join('; ')
}

export function BuilderPage(): ReactNode {
  const { engagementId, reportId } = useParams<{
    engagementId: string
    reportId: string
  }>()
  const eid = engagementId ?? ''
  const rid = reportId ?? ''

  const report = useReport(engagementId, reportId)
  const putBlocks = usePutReportBlocks()
  const preview = usePreviewHtml(engagementId, reportId)

  const [showPreview, setShowPreview] = useState(false)
  const [localBlocks, setLocalBlocks] = useState<ReportBlockInput[] | null>(null)

  const serverBlocks = useMemo<ReportBlockInput[]>(
    () =>
      (report.data?.blocks ?? []).map((b) => ({
        blockId: b.blockId,
        params: b.params,
      })),
    [report.data?.blocks],
  )

  const blocks: ReportBlockInput[] = localBlocks ?? serverBlocks

  const prevReportIdRef = useRef(report.data?.id)
  useEffect(() => {
    const currentId = report.data?.id
    if (currentId !== undefined && currentId !== prevReportIdRef.current) {
      prevReportIdRef.current = currentId
      setLocalBlocks(null)
    }
  }, [report.data?.id])

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    }),
  )

  const handleDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active, over } = event
      if (!over || active.id === over.id) return
      setLocalBlocks((prev) => {
        const base = prev ?? serverBlocks
        const oldIndex = base.findIndex((_, i) => active.id === String(i))
        const newIndex = base.findIndex((_, i) => over.id === String(i))
        if (oldIndex === -1 || newIndex === -1) return base
        return arrayMove(base, oldIndex, newIndex)
      })
    },
    [serverBlocks],
  )

  const handleAddBlock = useCallback(
    (blockId: string) => {
      setLocalBlocks((prev) => [...(prev ?? serverBlocks), { blockId, params: {} }])
    },
    [serverBlocks],
  )

  const handleRemoveBlock = useCallback(
    (index: number) => {
      setLocalBlocks((prev) => (prev ?? serverBlocks).filter((_, i) => i !== index))
    },
    [serverBlocks],
  )

  const handleUpdateBlockParams = useCallback(
    (index: number, params: BlockParams) => {
      setLocalBlocks((prev) => {
        const base = prev ?? serverBlocks
        const next = [...base]
        // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
        next[index] = { ...next[index]!, params: toWireParams(params) }
        return next
      })
    },
    [serverBlocks],
  )

  const handleSave = useCallback(async () => {
    // `?? serverBlocks`, never `?? []`: PUT replaces the whole list, so a save
    // with nothing edited must send back what is already there rather than
    // clearing the report.
    const toSave = localBlocks ?? serverBlocks
    try {
      await putBlocks.mutateAsync({
        engagementId: eid,
        reportId: rid,
        body: { blocks: toSave },
      })
      setLocalBlocks(null)
      toast.success('Blocks saved')
    } catch (err) {
      // The server names the offending block and field on a 400 — a rejected
      // param is the difference between "try again" and "fix this field", and
      // a fixed sentence hides which one it is.
      toast.error('Failed to save blocks', { description: saveErrorDetail(err, toSave) })
    }
  }, [eid, rid, localBlocks, serverBlocks, putBlocks])

  const handlePublished = useCallback(
    (_version: ReportVersion) => {
      void report.refetch()
    },
    [report],
  )

  if (report.isPending) return <PageLoading label="Loading report…" />
  if (report.error) {
    return (
      <PageError
        error={report.error}
        onRetry={() => {
          void report.refetch()
        }}
      />
    )
  }

  const reportData = report.data
  const isSaving = putBlocks.isPending
  const hasUnsaved = localBlocks !== null
  const sortableIds = blocks.map((_, i) => String(i))

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <h2 className="text-lg font-semibold">{reportData.title}</h2>
          <ReportSettingsPanel report={reportData} engagementId={eid} />
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setShowPreview((p) => !p)
            }}
          >
            {showPreview ? (
              <PanelRightCloseIcon className="size-4" />
            ) : (
              <PanelRightIcon className="size-4" />
            )}
            {showPreview ? 'Hide preview' : 'Preview'}
          </Button>
          <PublishDialog report={reportData} engagementId={eid} onPublished={handlePublished} />
          <VersionsPanel engagementId={eid} reportId={rid} />
          <Button
            size="sm"
            onClick={() => {
              void handleSave()
            }}
            disabled={isSaving || !hasUnsaved}
          >
            <SaveIcon className="size-4" />
            {isSaving ? 'Saving…' : 'Save'}
          </Button>
        </div>
      </div>

      {showPreview && (
        <div className="bg-muted/20 text-muted-foreground flex items-center gap-1.5 rounded-lg border px-3 py-2 text-xs">
          <ShieldAlertIcon className="size-3 shrink-0" />
          Preview reflects your current seat scope — blind blue seats may see less than the final
          published report.
        </div>
      )}

      <div className="flex min-h-0 flex-1 gap-4">
        {/* The palette folds away while the preview is open: three columns in
            the width this page gets squeezes the block editors to a sliver.
            The inline palette below takes over. */}
        <div className={cn('hidden w-56 shrink-0 overflow-y-auto', !showPreview && 'lg:block')}>
          <BlockPalette existingBlocks={blocks.map((b) => b.blockId)} onAdd={handleAddBlock} />
        </div>

        <div className="flex min-w-0 flex-1 flex-col gap-2 overflow-y-auto">
          {blocks.length === 0 ? (
            <EmptyBlocks />
          ) : (
            <DndContext
              sensors={sensors}
              collisionDetection={closestCenter}
              onDragEnd={handleDragEnd}
            >
              <SortableContext items={sortableIds} strategy={verticalListSortingStrategy}>
                {blocks.map((block, i) => (
                  <SortableBlock
                    key={sortableIds[i]}
                    block={block}
                    index={i}
                    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
                    sortableId={sortableIds[i]!}
                    onRemove={() => {
                      handleRemoveBlock(i)
                    }}
                    onParamsChange={(params) => {
                      handleUpdateBlockParams(i, params)
                    }}
                  />
                ))}
              </SortableContext>
            </DndContext>
          )}
          <div className={cn(!showPreview && 'lg:hidden')}>
            <MobileBlockPalette
              existingBlocks={blocks.map((b) => b.blockId)}
              onAdd={handleAddBlock}
            />
          </div>
        </div>

        {showPreview && (
          <div className="hidden min-w-0 flex-1 lg:block">
            <PreviewPane
              html={preview.data ?? ''}
              isLoading={preview.isPending || preview.isFetching}
              error={preview.error}
              savedBlockCount={serverBlocks.length}
              hasUnsaved={hasUnsaved}
            />
          </div>
        )}
      </div>

      <div className="flex items-center gap-2 border-t pt-3">
        <TemplateActions engagementId={eid} reportId={rid} report={reportData} />
      </div>
    </div>
  )
}

function BlockPalette({
  onAdd,
}: {
  existingBlocks: string[]
  onAdd: (blockId: string) => void
}): ReactNode {
  const catalog = getCatalog()
  return (
    <div className="rounded-lg border p-3">
      <h3 className="text-muted-foreground mb-2 text-xs font-semibold tracking-wider uppercase">
        Add block
      </h3>
      <div className="flex flex-col gap-1">
        {catalog.map((entry) => (
          <Button
            key={entry.id}
            variant="ghost"
            size="sm"
            className="justify-start text-xs"
            onClick={() => {
              onAdd(entry.id)
            }}
          >
            <PlusIcon className="size-3" />
            {entry.title}
          </Button>
        ))}
      </div>
    </div>
  )
}

function MobileBlockPalette({
  onAdd,
}: {
  existingBlocks: string[]
  onAdd: (blockId: string) => void
}): ReactNode {
  const catalog = getCatalog()
  return (
    <div className="rounded-lg border p-3">
      <h3 className="mb-2 text-sm font-medium">Add block</h3>
      <div className="flex flex-wrap gap-1">
        {catalog.map((entry) => (
          <Button
            key={entry.id}
            variant="outline"
            size="sm"
            className="text-xs"
            onClick={() => {
              onAdd(entry.id)
            }}
          >
            <PlusIcon className="size-3" />
            {entry.title}
          </Button>
        ))}
      </div>
    </div>
  )
}

function SortableBlock({
  block,
  sortableId,
  onRemove,
  onParamsChange,
}: {
  block: ReportBlockInput
  index: number
  sortableId: string
  onRemove: () => void
  onParamsChange: (params: BlockParams) => void
}): ReactNode {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: sortableId,
  })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  }

  const entry = getBlockEntry(block.blockId)

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn('bg-card rounded-lg border', isDragging && 'z-10 shadow-lg')}
    >
      <div className="flex items-center gap-2 border-b px-3 py-2">
        <button
          className="text-muted-foreground hover:text-foreground cursor-grab touch-none"
          {...attributes}
          {...listeners}
          aria-label={`Drag to reorder ${entry?.title ?? block.blockId}`}
        >
          <GripVerticalIcon className="size-4" />
        </button>
        <span className="flex-1 text-sm font-medium">{entry?.title ?? block.blockId}</span>
        <Button
          variant="ghost"
          size="sm"
          aria-label={`Remove ${entry?.title ?? block.blockId}`}
          onClick={onRemove}
        >
          <Trash2Icon className="size-4" />
        </Button>
      </div>
      <BlockParamsForm
        blockId={block.blockId}
        params={asParams(block.params)}
        onChange={onParamsChange}
      />
    </div>
  )
}

/**
 * The editors for one block's parameters, driven by the catalogue's field list
 * rather than a per-block component.
 *
 * The field names are the server's: `ValidateParams` rejects a key its schema
 * does not declare, so a form that invents one — as the old rich-text editor
 * did, writing `body` where the block declares `html` — fails the whole save
 * with a 400 rather than dropping that one value.
 */
function BlockParamsForm({
  blockId,
  params,
  onChange,
}: {
  blockId: string
  params: BlockParams
  onChange: (params: BlockParams) => void
}): ReactNode {
  const fields = getBlockEntry(blockId)?.params
  if (!fields || fields.length === 0) return null

  return (
    <div className="flex flex-col gap-3 px-3 py-3">
      {fields.map((field) => (
        <BlockParamField
          key={field.name}
          field={field}
          value={params[field.name]}
          onChange={(value) => {
            onChange({ ...params, [field.name]: value })
          }}
        />
      ))}
    </div>
  )
}

function BlockParamField({
  field,
  value,
  onChange,
}: {
  field: BlockParamField
  value: unknown
  onChange: (value: unknown) => void
}): ReactNode {
  const id = `param-${field.name}`

  if (field.kind === 'toggle') {
    return (
      <label className="flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          className="border-input size-4 rounded border"
          checked={value === true}
          onChange={(e) => {
            onChange(e.target.checked)
          }}
        />
        {field.label}
      </label>
    )
  }

  return (
    <div className="flex flex-col gap-1">
      <Label htmlFor={id} className="text-xs">
        {field.label}
      </Label>
      <BlockParamControl id={id} field={field} value={value} onChange={onChange} />
      {field.help !== undefined && <p className="text-muted-foreground text-xs">{field.help}</p>}
    </div>
  )
}

function BlockParamControl({
  id,
  field,
  value,
  onChange,
}: {
  id: string
  field: BlockParamField
  value: unknown
  onChange: (value: unknown) => void
}): ReactNode {
  switch (field.kind) {
    case 'html':
      return (
        <RichTextEditor
          content={typeof value === 'string' ? value : ''}
          onChange={(html) => {
            // TipTap emits "<p></p>" for an empty document; store nothing so an
            // untouched block renders as absent rather than as a blank section.
            onChange(html === '<p></p>' ? '' : html)
          }}
        />
      )
    case 'textarea':
      return (
        <textarea
          id={id}
          className="bg-background min-h-[80px] w-full rounded-md border px-3 py-2 text-sm"
          placeholder={field.placeholder}
          value={typeof value === 'string' ? value : ''}
          onChange={(e) => {
            onChange(e.target.value)
          }}
        />
      )
    case 'integer':
      return (
        <Input
          id={id}
          type="number"
          min={1}
          step={1}
          placeholder={field.placeholder}
          value={typeof value === 'number' ? String(value) : ''}
          onChange={(e) => {
            const parsed = Number.parseInt(e.target.value, 10)
            onChange(Number.isNaN(parsed) ? undefined : parsed)
          }}
        />
      )
    case 'select':
      return (
        <Select
          value={typeof value === 'string' ? value : ''}
          onValueChange={(next) => {
            onChange(next)
          }}
        >
          <SelectTrigger id={id}>
            <SelectValue placeholder="Choose…" />
          </SelectTrigger>
          <SelectContent>
            {(field.options ?? []).map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )
    case 'engagement':
      return <EngagementParamSelect id={id} value={value} onChange={onChange} />
    default:
      return (
        <Input
          id={id}
          placeholder={field.placeholder}
          value={typeof value === 'string' ? value : ''}
          onChange={(e) => {
            onChange(e.target.value)
          }}
        />
      )
  }
}

function EngagementParamSelect({
  id,
  value,
  onChange,
}: {
  id: string
  value: unknown
  onChange: (value: unknown) => void
}): ReactNode {
  const engagements = useEngagements({})

  const activeEngagements = useMemo(() => {
    if (!engagements.data?.pages) return []
    return engagements.data.pages.flatMap((p) => p.items).filter((e) => e.status !== 'archived')
  }, [engagements.data])

  return (
    <Select
      value={typeof value === 'string' ? value : ''}
      onValueChange={(next) => {
        onChange(next)
      }}
    >
      <SelectTrigger id={id}>
        <SelectValue placeholder="Select an engagement…" />
      </SelectTrigger>
      <SelectContent>
        {activeEngagements.map((e) => (
          <SelectItem key={e.id} value={e.id}>
            {e.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

function EmptyBlocks(): ReactNode {
  return (
    <div className="text-muted-foreground flex flex-col items-center justify-center gap-2 py-12">
      <FileTextIcon className="size-8" />
      <p className="text-sm font-medium">No blocks yet</p>
      <p className="max-w-xs text-center text-xs">
        Add blocks from the palette on the left to build your report.
      </p>
    </div>
  )
}

/**
 * The rendered draft, in an iframe.
 *
 * The server renders the *saved* draft, and always returns a complete HTML
 * document — a report with no blocks is a valid page with an empty body. So the
 * empty state is decided from the saved block count rather than from whether
 * there is any HTML: keying it off the HTML meant a report with nothing saved
 * showed a blank white rectangle and no explanation of why.
 */
function PreviewPane({
  html,
  isLoading,
  error,
  savedBlockCount,
  hasUnsaved,
}: {
  html: string
  isLoading: boolean
  error: Error | null
  savedBlockCount: number
  hasUnsaved: boolean
}): ReactNode {
  return (
    <div className="sticky top-0 flex h-full min-h-0 flex-col">
      <div className="mb-2 flex items-baseline justify-between gap-2">
        <h3 className="text-muted-foreground text-xs font-semibold tracking-wider uppercase">
          Preview
        </h3>
        {hasUnsaved && (
          <span className="text-muted-foreground text-xs">
            Showing the last save — save to update.
          </span>
        )}
      </div>
      <div className="flex-1 overflow-hidden rounded-lg border bg-white">
        <PreviewBody
          html={html}
          isLoading={isLoading}
          error={error}
          savedBlockCount={savedBlockCount}
        />
      </div>
    </div>
  )
}

function PreviewBody({
  html,
  isLoading,
  error,
  savedBlockCount,
}: {
  html: string
  isLoading: boolean
  error: Error | null
  savedBlockCount: number
}): ReactNode {
  const message = (text: string): ReactNode => (
    <div className="text-muted-foreground flex h-full items-center justify-center p-8 text-center text-sm">
      {text}
    </div>
  )

  if (isLoading) return message('Rendering…')
  if (error) return message('The preview could not be rendered. Try saving again.')
  if (savedBlockCount === 0) return message('Add a block and save to see the report here.')
  if (!html) return message('Nothing to preview yet.')

  return (
    <iframe
      className="h-full w-full"
      srcDoc={html}
      title="Report preview"
      sandbox="allow-same-origin"
    />
  )
}

function TemplateActions({
  engagementId,
  reportId,
}: {
  engagementId: string
  reportId: string
  report: Report
}): ReactNode {
  const templates = useTemplates(engagementId)
  const applyTemplate = useApplyTemplate()
  const createFromReport = useCreateTemplateFromReport()
  const [applyOpen, setApplyOpen] = useState(false)
  const [selectedTemplate, setSelectedTemplate] = useState<string>('')
  const [saveAsName, setSaveAsName] = useState('')
  const [saveAsOpen, setSaveAsOpen] = useState(false)

  const templateItems = templates.data ?? []

  return (
    <>
      <Dialog open={applyOpen} onOpenChange={setApplyOpen}>
        <DialogTrigger asChild>
          <Button variant="outline" size="sm">
            <BookTemplateIcon className="size-4" />
            Apply template
          </Button>
        </DialogTrigger>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Apply template</DialogTitle>
          </DialogHeader>
          <p className="text-muted-foreground text-sm">
            Replace the current draft blocks with a copy of the template blocks.
          </p>
          <Select value={selectedTemplate} onValueChange={setSelectedTemplate}>
            <SelectTrigger>
              <SelectValue placeholder="Choose a template…" />
            </SelectTrigger>
            <SelectContent>
              {templateItems.map((t) => (
                <SelectItem key={t.id} value={t.id}>
                  {t.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setApplyOpen(false)
                setSelectedTemplate('')
              }}
            >
              Cancel
            </Button>
            <Button
              disabled={!selectedTemplate || applyTemplate.isPending}
              onClick={() => {
                applyTemplate.mutate(
                  {
                    engagementId,
                    reportId,
                    body: { templateId: selectedTemplate },
                  },
                  {
                    onSuccess: () => {
                      setApplyOpen(false)
                      setSelectedTemplate('')
                      toast.success('Template applied')
                    },
                    onError: () => {
                      toast.error('Failed to apply template')
                    },
                  },
                )
              }}
            >
              Apply
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={saveAsOpen} onOpenChange={setSaveAsOpen}>
        <DialogTrigger asChild>
          <Button variant="outline" size="sm">
            <SaveIcon className="size-4" />
            Save as template
          </Button>
        </DialogTrigger>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Save as template</DialogTitle>
          </DialogHeader>
          <p className="text-muted-foreground text-sm">
            Create a reusable template from the current block arrangement.
          </p>
          <div className="space-y-2">
            <Label htmlFor="template-name">Template name</Label>
            <Input
              id="template-name"
              placeholder="Standard assessment"
              value={saveAsName}
              onChange={(e) => {
                setSaveAsName(e.target.value)
              }}
            />
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setSaveAsOpen(false)
                setSaveAsName('')
              }}
            >
              Cancel
            </Button>
            <Button
              disabled={!saveAsName.trim() || createFromReport.isPending}
              onClick={() => {
                createFromReport.mutate(
                  {
                    engagementId,
                    body: {
                      reportId,
                      name: saveAsName.trim(),
                    },
                  },
                  {
                    onSuccess: () => {
                      setSaveAsOpen(false)
                      setSaveAsName('')
                      toast.success('Template saved')
                    },
                    onError: () => {
                      toast.error('Failed to save template')
                    },
                  },
                )
              }}
            >
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
