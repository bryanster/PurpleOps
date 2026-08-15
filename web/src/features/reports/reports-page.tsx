import { PlusIcon, Trash2Icon } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { Link, useParams } from 'react-router'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
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
import { ConfirmDialog } from '@/features/admin/confirm-dialog'
import { useEngagementContext } from '@/features/engagements/engagement-layout'
import { engagementReportPath } from '@/features/reports/paths'
import { formatMoment } from '@/lib/time'

import { useCreateReport, useDeleteReport, useReports, type Report } from './queries'

/**
 * Lists every report in the engagement (M6-013).
 */
export function ReportsPage(): ReactNode {
  const { engagementId } = useParams<{ engagementId: string }>()
  const { closed } = useEngagementContext()
  const reports = useReports(engagementId)
  const createReport = useCreateReport()
  const deleteReport = useDeleteReport()

  const [createOpen, setCreateOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Report | undefined>(undefined)
  const [title, setTitle] = useState('')

  if (reports.isPending) return <PageLoading label="Loading reports…" />
  if (reports.error) {
    return (
      <PageError
        error={reports.error}
        onRetry={() => {
          void reports.refetch()
        }}
      />
    )
  }

  const items = reports.data

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold">Reports</h2>
        {!closed && (
          <Dialog open={createOpen} onOpenChange={setCreateOpen}>
            <DialogTrigger asChild>
              <Button size="sm">
                <PlusIcon className="size-4" />
                New report
              </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-sm">
              <DialogHeader>
                <DialogTitle>New report</DialogTitle>
              </DialogHeader>
              <div className="space-y-2">
                <Label htmlFor="report-title">Title</Label>
                <Input
                  id="report-title"
                  placeholder="Q3 assessment report"
                  value={title}
                  onChange={(e) => {
                    setTitle(e.target.value)
                  }}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') {
                      void handleCreate()
                    }
                  }}
                />
              </div>
              <DialogFooter>
                <Button
                  variant="outline"
                  onClick={() => {
                    setCreateOpen(false)
                    setTitle('')
                  }}
                >
                  Cancel
                </Button>
                <Button
                  onClick={() => {
                    void handleCreate()
                  }}
                  disabled={createReport.isPending}
                >
                  Create
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        )}
      </div>

      {items.length === 0 ? (
        <PageEmpty
          title="No reports yet"
          description="Create a report draft to start building your deliverable."
          action={
            !closed && (
              <Button
                size="sm"
                onClick={() => {
                  setCreateOpen(true)
                }}
              >
                <PlusIcon className="size-4" />
                New report
              </Button>
            )
          }
        />
      ) : (
        <div className="divide-y rounded-lg border">
          {items.map((report) => (
            <ReportRow
              key={report.id}
              report={report}
              engagementId={engagementId ?? ''}
              onDelete={() => {
                setDeleteTarget(report)
              }}
            />
          ))}
        </div>
      )}

      <ConfirmDialog
        open={deleteTarget !== undefined}
        onOpenChange={(open) => {
          if (!open) {
            setDeleteTarget(undefined)
          }
        }}
        title="Delete report"
        confirmLabel="Delete"
        description={
          <>
            Delete <strong>{deleteTarget?.title ?? 'this report'}</strong>? Its draft blocks,
            published versions, and any share links to them go with it. This cannot be undone.
          </>
        }
        onConfirm={() => {
          if (!deleteTarget || !engagementId) return
          deleteReport.mutate(
            { engagementId, reportId: deleteTarget.id },
            {
              onSuccess: () => {
                setDeleteTarget(undefined)
              },
            },
          )
        }}
        pending={deleteReport.isPending}
      />
    </div>
  )

  async function handleCreate(): Promise<void> {
    if (!engagementId || !title.trim()) return
    try {
      const report = await createReport.mutateAsync({
        engagementId,
        body: { title: title.trim() },
      })
      setCreateOpen(false)
      setTitle('')
      window.location.href = engagementReportPath(engagementId, report.id)
    } catch {
      // Error surfaced by mutation state
    }
  }
}

function ReportRow({
  report,
  engagementId,
  onDelete,
}: {
  report: Report
  engagementId: string
  onDelete: () => void
}): ReactNode {
  // `blocks` is absent from the list response — the count is its own field.
  const blockCount = report.blockCount
  return (
    <div className="flex items-center justify-between px-4 py-3 first:rounded-t-lg last:rounded-b-lg">
      <Link
        to={engagementReportPath(engagementId, report.id)}
        className="min-w-0 flex-1 text-sm font-medium hover:underline"
      >
        {report.title}
      </Link>
      <div className="text-muted-foreground flex shrink-0 items-center gap-3 text-xs">
        <span>
          {blockCount} block{blockCount === 1 ? '' : 's'}
        </span>
        <span>{formatMoment(report.updatedAt)}</span>
        <Button
          variant="ghost"
          size="sm"
          aria-label={`Delete ${report.title}`}
          onClick={(e) => {
            e.preventDefault()
            onDelete()
          }}
        >
          <Trash2Icon className="size-4" />
        </Button>
      </div>
    </div>
  )
}
