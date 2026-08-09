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

import { getBlockEntry, getCatalog } from './block-catalog'
import {
  useApplyTemplate,
  useCreateTemplateFromReport,
  usePreviewHtml,
  usePutReportBlocks,
  useReport,
  useTemplates,
  type Report,
  type ReportBlockInput,
} from './queries'
import { ReportSettingsPanel } from './report-settings-panel'

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
  const [previewKey, setPreviewKey] = useState(0)
  const [localBlocks, setLocalBlocks] = useState<ReportBlockInput[] | null>(null)

  const blocks: ReportBlockInput[] =
    localBlocks ??
    (report.data?.blocks
      ? report.data.blocks.map((b) => ({
          blockId: b.blockId,
          params: b.params,
        }))
      : [])

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

  const handleDragEnd = useCallback((event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return
    setLocalBlocks((prev) => {
      if (!prev) return prev
      const oldIndex = prev.findIndex((_, i) => active.id === String(i))
      const newIndex = prev.findIndex((_, i) => over.id === String(i))
      if (oldIndex === -1 || newIndex === -1) return prev
      return arrayMove(prev, oldIndex, newIndex)
    })
  }, [])

  const handleAddBlock = useCallback((blockId: string) => {
    setLocalBlocks((prev) => [...(prev ?? []), { blockId, params: {} }])
  }, [])

  const handleRemoveBlock = useCallback((index: number) => {
    setLocalBlocks((prev) => {
      if (!prev) return prev
      return prev.filter((_, i) => i !== index)
    })
  }, [])

  const handleUpdateBlockParams = useCallback((index: number, params: Record<string, never>) => {
    setLocalBlocks((prev) => {
      if (!prev) return prev
      const next = [...prev]
      next[index] = { ...next[index], params }
      return next
    })
  }, [])

  const handleSave = useCallback(async () => {
    const toSave = localBlocks ?? []
    try {
      await putBlocks.mutateAsync({
        engagementId: eid,
        reportId: rid,
        body: { blocks: toSave },
      })
      setLocalBlocks(null)
      setPreviewKey((k) => k + 1)
      toast.success('Blocks saved')
    } catch {
      toast.error('Failed to save blocks')
    }
  }, [eid, rid, localBlocks, putBlocks])

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

      {preview.data && showPreview && (
        <div className="bg-muted/20 text-muted-foreground flex items-center gap-1.5 rounded-lg border px-3 py-2 text-xs">
          <ShieldAlertIcon className="size-3 shrink-0" />
          Preview reflects your current seat scope — blind blue seats may see less than the final
          published report.
        </div>
      )}

      <div className="flex min-h-0 flex-1 gap-4">
        <div className="hidden w-56 shrink-0 overflow-y-auto lg:block">
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
                    sortableId={sortableIds[i]}
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
          <div className="lg:hidden">
            <MobileBlockPalette
              existingBlocks={blocks.map((b) => b.blockId)}
              onAdd={handleAddBlock}
            />
          </div>
        </div>

        {showPreview && (
          <div className="hidden w-[420px] shrink-0 lg:block">
            <PreviewPane
              html={preview.data ?? ''}
              isLoading={preview.isFetching}
              previewKey={previewKey}
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
  onParamsChange: (params: Record<string, never>) => void
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
        params={block.params ?? {}}
        onChange={onParamsChange}
      />
    </div>
  )
}

function BlockParamsForm({
  blockId,
  params,
  onChange,
}: {
  blockId: string
  params: Record<string, never>
  onChange: (params: Record<string, never>) => void
}): ReactNode {
  if (blockId === 'rich_text') {
    return <RichTextParams params={params} onChange={onChange} />
  }
  if (blockId === 'engagement_compare') {
    return <CompareParams params={params} onChange={onChange} />
  }
  return null
}

function RichTextParams({
  params,
  onChange,
}: {
  params: Record<string, never>
  onChange: (params: Record<string, never>) => void
}): ReactNode {
  const body = (params as { body?: string }).body ?? ''

  return (
    <div className="px-3 py-2">
      <Label className="text-xs">Content</Label>
      <textarea
        className="bg-background mt-1 min-h-[100px] w-full rounded-md border px-3 py-2 text-sm"
        placeholder="Write your content…"
        value={body}
        onChange={(e) => {
          onChange({
            ...params,
            body: e.target.value,
          } as unknown as Record<string, never>)
        }}
      />
    </div>
  )
}

function CompareParams({
  params,
  onChange,
}: {
  params: Record<string, never>
  onChange: (params: Record<string, never>) => void
}): ReactNode {
  const compareParams = params as {
    baselineEngagementId?: string
  }
  const baselineId = compareParams.baselineEngagementId ?? ''

  const engagements = useEngagements({})

  const activeEngagements = useMemo(() => {
    if (!engagements.data?.pages) return []
    return engagements.data.pages.flatMap((p) => p.items).filter((e) => e.status !== 'archived')
  }, [engagements.data])

  return (
    <div className="px-3 py-2">
      <Label className="text-xs">Baseline engagement</Label>
      <Select
        value={baselineId}
        onValueChange={(value) => {
          onChange({
            ...params,
            baselineEngagementId: value,
          } as unknown as Record<string, never>)
        }}
      >
        <SelectTrigger className="mt-1">
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
    </div>
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

function PreviewPane({
  html,
  isLoading,
  previewKey: _previewKey,
}: {
  html: string
  isLoading: boolean
  previewKey: number
}): ReactNode {
  return (
    <div className="sticky top-0 flex h-full flex-col">
      <h3 className="text-muted-foreground mb-2 text-xs font-semibold tracking-wider uppercase">
        Preview
      </h3>
      <div className="flex-1 overflow-hidden rounded-lg border bg-white">
        {isLoading ? (
          <div className="text-muted-foreground flex items-center justify-center p-8 text-sm">
            Rendering…
          </div>
        ) : html ? (
          <iframe
            className="h-full w-full"
            srcDoc={html}
            title="Report preview"
            sandbox="allow-same-origin"
          />
        ) : (
          <div className="text-muted-foreground flex items-center justify-center p-8 text-sm">
            Save blocks to see preview.
          </div>
        )}
      </div>
    </div>
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
