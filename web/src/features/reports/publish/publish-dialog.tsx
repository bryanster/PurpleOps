import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { useEngagementContext } from '@/features/engagements/engagement-layout'
import { canManage } from '@/features/engagements/roles'
import { LockIcon, UploadIcon } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { toast } from 'sonner'

import { usePublishReport, type Report, type ReportVersion } from '../queries'

/**
 * Publish dialog — lead only (M6-014).
 *
 * Non-leads see a disabled button with a reason. The dialog shows the
 * include-evidence checkbox and a note about full engagement data.
 */
export function PublishDialog({
  report,
  engagementId,
  onPublished,
}: {
  report: Report
  engagementId: string
  onPublished?: (version: ReportVersion) => void
}): ReactNode {
  const { role, closed } = useEngagementContext()
  const canPublish = canManage(role) && !closed
  const publish = usePublishReport()

  const [open, setOpen] = useState(false)
  const [includeEvidence, setIncludeEvidence] = useState(false)

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button
          size="sm"
          variant="default"
          disabled={!canPublish || publish.isPending}
          title={!canPublish ? 'Only the engagement lead or a platform admin can publish' : undefined}
        >
          <UploadIcon className="size-4" />
          Publish
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Publish report</DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          <p className="text-sm text-muted-foreground">
            Publish the current draft as an immutable version. Published versions use your full lead
            scope — never blind-filtered data.
          </p>

          <div className="flex items-start gap-2">
            <Checkbox
              id="include-evidence"
              checked={includeEvidence}
              onCheckedChange={(v) => {
                setIncludeEvidence(v === true)
              }}
            />
            <div className="grid gap-1">
              <Label htmlFor="include-evidence" className="text-sm font-medium">
                Include evidence
              </Label>
              <p className="text-xs text-muted-foreground">
                Evidence files may contain client-sensitive screenshots. Keep this off unless the
                recipient needs raw evidence.
              </p>
            </div>
          </div>

          <div className="flex items-center gap-1.5 rounded-md border bg-muted/40 px-3 py-2 text-xs text-muted-foreground">
            <LockIcon className="size-3 shrink-0" />
            Published reports always use full engagement data (not blind-filtered).
          </div>
        </div>

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => {
              setOpen(false)
            }}
          >
            Cancel
          </Button>
          <Button
            onClick={() => {
              publish.mutate(
                { engagementId, reportId: report.id, body: { includeEvidence } },
                {
                  onSuccess: (version) => {
                    setOpen(false)
                    toast.success('Report published')
                    onPublished?.(version)
                  },
                  onError: () => {
                    toast.error('Failed to publish report')
                  },
                },
              )
            }}
            disabled={publish.isPending}
          >
            {publish.isPending ? 'Publishing…' : 'Publish'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
