import { SettingsIcon } from 'lucide-react'
import { type ReactNode, useState } from 'react'

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
import { toast } from 'sonner'

import { usePatchReport, type Report } from './queries'

/**
 * Side panel (dialog) for per-report branding overrides (M6-004, M6-013).
 *
 * Keyed by report id so form state resets when a different report opens.
 */
export function ReportSettingsPanel({
  report,
  engagementId,
}: {
  report: Report
  engagementId: string
}): ReactNode {
  const patchReport = usePatchReport()
  const [open, setOpen] = useState(false)

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button variant="ghost" size="sm" aria-label="Report settings">
          <SettingsIcon className="size-4" />
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-md">
        <SettingsForm
          key={report.id}
          report={report}
          engagementId={engagementId}
          patchReport={patchReport}
          onClose={() => {
            setOpen(false)
          }}
        />
      </DialogContent>
    </Dialog>
  )
}

function SettingsForm({
  report,
  engagementId,
  patchReport,
  onClose,
}: {
  report: Report
  engagementId: string
  patchReport: ReturnType<typeof usePatchReport>
  onClose: () => void
}): ReactNode {
  const [title, setTitle] = useState(report.title)
  const [clientName, setClientName] = useState(report.clientName ?? '')
  const rawColours = report.colours as { primary?: string; secondary?: string } | null | undefined
  const [primaryColor, setPrimaryColor] = useState(rawColours?.primary ?? '')
  const [secondaryColor, setSecondaryColor] = useState(rawColours?.secondary ?? '')

  const handleSave = async (): Promise<void> => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const body: Record<string, any> = {}

    if (title !== report.title) body.title = title
    if (clientName !== (report.clientName ?? '')) {
      body.clientName = clientName || null
    }

    const currentPrimary = rawColours?.primary ?? ''
    const currentSecondary = rawColours?.secondary ?? ''
    const coloursChanged = primaryColor !== currentPrimary || secondaryColor !== currentSecondary

    if (coloursChanged) {
      if (!primaryColor && !secondaryColor) {
        body.colours = null
      } else {
        body.colours = {
          primary: primaryColor || currentPrimary,
          secondary: secondaryColor || currentSecondary,
        }
      }
    }

    if (Object.keys(body).length === 0) {
      onClose()
      return
    }

    try {
      await patchReport.mutateAsync({
        engagementId,
        reportId: report.id,
        body: body as Parameters<typeof patchReport.mutateAsync>[0]['body'],
      })
      onClose()
      toast.success('Report updated')
    } catch {
      toast.error('Failed to update report')
    }
  }

  return (
    <>
      <DialogHeader>
        <DialogTitle>Report settings</DialogTitle>
      </DialogHeader>

      <div className="space-y-4">
        <div className="space-y-2">
          <Label htmlFor="rs-title">Title</Label>
          <Input
            id="rs-title"
            value={title}
            onChange={(e) => {
              setTitle(e.target.value)
            }}
          />
        </div>

        <div className="space-y-2">
          <Label htmlFor="rs-client">Client name</Label>
          <Input
            id="rs-client"
            value={clientName}
            onChange={(e) => {
              setClientName(e.target.value)
            }}
            placeholder="Overrides engagement client"
          />
        </div>

        <div className="space-y-2">
          <Label>Colours</Label>
          <div className="flex gap-2">
            <div className="flex-1 space-y-1">
              <Label htmlFor="rs-primary" className="text-xs">
                Primary
              </Label>
              <div className="flex items-center gap-2">
                <input
                  id="rs-primary"
                  type="color"
                  value={primaryColor || '#000000'}
                  onChange={(e) => {
                    setPrimaryColor(e.target.value)
                  }}
                  className="h-8 w-8 cursor-pointer rounded border"
                />
                <Input
                  value={primaryColor}
                  onChange={(e) => {
                    setPrimaryColor(e.target.value)
                  }}
                  placeholder="#1a1a2e"
                  className="font-mono text-xs"
                />
              </div>
            </div>
            <div className="flex-1 space-y-1">
              <Label htmlFor="rs-secondary" className="text-xs">
                Secondary
              </Label>
              <div className="flex items-center gap-2">
                <input
                  id="rs-secondary"
                  type="color"
                  value={secondaryColor || '#000000'}
                  onChange={(e) => {
                    setSecondaryColor(e.target.value)
                  }}
                  className="h-8 w-8 cursor-pointer rounded border"
                />
                <Input
                  value={secondaryColor}
                  onChange={(e) => {
                    setSecondaryColor(e.target.value)
                  }}
                  placeholder="#16213e"
                  className="font-mono text-xs"
                />
              </div>
            </div>
          </div>
        </div>
      </div>

      <DialogFooter>
        <Button variant="outline" onClick={onClose}>
          Cancel
        </Button>
        <Button
          onClick={() => {
            void handleSave()
          }}
          disabled={patchReport.isPending}
        >
          {patchReport.isPending ? 'Saving…' : 'Save'}
        </Button>
      </DialogFooter>
    </>
  )
}
