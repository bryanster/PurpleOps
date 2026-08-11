import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { PageLoading } from '@/app/shell/page-state'
import { useEngagementContext } from '@/features/engagements/engagement-layout'
import { canManage } from '@/features/engagements/roles'
import { formatMoment } from '@/lib/time'
import { FileTextIcon, HistoryIcon, DownloadIcon, ExternalLinkIcon } from 'lucide-react'
import { API_BASE_URL } from '@/api/client'
import { type ReactNode, useState } from 'react'

import type { ReportVersion } from '../queries'
import { useVersions } from '../queries'
import { SharePanel } from '../share/share-panel'

/**
 * Versions panel --- lists published versions with open HTML / download PDF
 * actions and a share panel per version (M6-014).
 */
export function VersionsPanel({
  engagementId,
  reportId,
}: {
  engagementId: string
  reportId: string
}): ReactNode {
  const { role } = useEngagementContext()
  const isLead = canManage(role)
  const versions = useVersions(engagementId, reportId)
  const [open, setOpen] = useState(false)
  const [selectedVersion, setSelectedVersion] = useState<ReportVersion | undefined>(undefined)

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button size="sm" variant="outline">
          <HistoryIcon className="size-4" />
          Versions
        </Button>
      </DialogTrigger>
      <DialogContent className="sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Published versions</DialogTitle>
        </DialogHeader>

        {versions.isPending ? (
          <PageLoading label="Loading versions…" />
        ) : versions.error ? (
          <p className="text-destructive text-sm">Failed to load versions.</p>
        ) : versions.data.length > 0 ? (
          <div className="max-h-[60vh] space-y-3 overflow-y-auto">
            {versions.data.map((v) => (
              <VersionRow
                key={v.id}
                version={v}
                engagementId={engagementId}
                reportId={reportId}
                isLead={isLead}
                selectedVersionId={selectedVersion?.id}
                onSelectShare={setSelectedVersion}
              />
            ))}
          </div>
        ) : (
          <p className="text-muted-foreground text-sm">No published versions yet.</p>
        )}
      </DialogContent>
    </Dialog>
  )
}

function VersionRow({
  version,
  engagementId,
  reportId,
  isLead,
  selectedVersionId,
  onSelectShare,
}: {
  version: ReportVersion
  engagementId: string
  reportId: string
  isLead: boolean
  selectedVersionId?: string
  onSelectShare: (v: ReportVersion) => void
}): ReactNode {
  const htmlUrl = `${API_BASE_URL}/engagements/${engagementId}/reports/${reportId}/versions/${version.id}/html`
  const pdfUrl = `${API_BASE_URL}/engagements/${engagementId}/reports/${reportId}/versions/${version.id}/pdf`
  const showShare = selectedVersionId === version.id

  return (
    <div className="rounded-lg border">
      <div className="flex items-center justify-between px-4 py-3">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="text-sm font-medium">v{version.ordinal}</span>
            <span className="text-muted-foreground truncate text-sm">{version.title}</span>
          </div>
          <div className="text-muted-foreground mt-0.5 flex items-center gap-2 text-xs">
            <span>{version.publishedBy}</span>
            <span>&middot;</span>
            <span>{formatMoment(version.publishedAt)}</span>
            {version.includeEvidence && (
              <>
                <span>&middot;</span>
                <span>Evidence included</span>
              </>
            )}
            {version.contentSha256 && (
              <>
                <span>&middot;</span>
                <code className="text-[10px]">{version.contentSha256.slice(0, 8)}</code>
              </>
            )}
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1">
          <Button asChild size="sm" variant="ghost" title="Open HTML in new tab">
            <a href={htmlUrl} target="_blank" rel="noopener noreferrer">
              <FileTextIcon className="size-4" />
            </a>
          </Button>
          <Button asChild size="sm" variant="ghost" title="Download PDF">
            <a href={pdfUrl} target="_blank" rel="noopener noreferrer">
              <DownloadIcon className="size-4" />
            </a>
          </Button>
          {isLead && (
            <Button
              size="sm"
              variant="ghost"
              title="Share"
              onClick={() => {
                onSelectShare(showShare ? (undefined as never) : version)
              }}
            >
              <ExternalLinkIcon className="size-4" />
              Share
            </Button>
          )}
        </div>
      </div>
      {showShare && (
        <div className="border-t px-4 py-3">
          <SharePanel versionId={version.id} />
        </div>
      )}
    </div>
  )
}
