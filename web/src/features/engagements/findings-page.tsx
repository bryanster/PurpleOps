import { type FormEvent, type ReactNode, useState } from 'react'
import { toast } from 'sonner'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatMoment } from '@/lib/time'

import { useEngagementContext } from './engagement-layout'
import {
  canWriteFindings,
  canManage,
} from './roles'
import {
  useCreateFinding,
  useFindings,
  type Finding,
  type NewFinding,
} from './queries'

const SEVERITIES = [
  { value: 'critical', label: 'Critical' },
  { value: 'high', label: 'High' },
  { value: 'medium', label: 'Medium' },
  { value: 'low', label: 'Low' },
  { value: 'info', label: 'Info' },
]

const STATUSES = [
  { value: 'open', label: 'Open' },
  { value: 'in_progress', label: 'In Progress' },
  { value: 'resolved', label: 'Resolved' },
  { value: 'wont_fix', label: "Won't Fix" },
]

export function FindingsPage(): ReactNode {
  const { engagementId, role, closed } = useEngagementContext()
  const findings = useFindings(engagementId)
  const [createOpen, setCreateOpen] = useState(false)
  const [detailId, setDetailId] = useState<string | undefined>(undefined)

  const detailFinding = detailId
    ? (findings.data ?? []).find((f) => f.id === detailId)
    : undefined

  if (findings.isPending) {
    return <PageLoading label="Loading findings…" />
  }

  if (findings.error) {
    return (
      <PageError
        error={findings.error}
        onRetry={() => {
          void findings.refetch()
        }}
      />
    )
  }

  const items = findings.data ?? []

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-3">
        <p className="text-muted-foreground text-sm">
          {items.length} finding{items.length !== 1 ? 's' : ''}
        </p>
        {canWriteFindings(role) && !closed && (
          <Button
            size="sm"
            onClick={() => {
              setCreateOpen(true)
            }}
          >
            New finding
          </Button>
        )}
      </div>

      {items.length === 0 ? (
        <PageEmpty
          title="No findings"
          description="Findings raised during this engagement will appear here."
        />
      ) : (
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Title</TableHead>
                <TableHead>Severity</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Owner</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((finding) => (
                <TableRow key={finding.id}>
                  <TableCell>
                    <button
                      type="button"
                      className="text-left font-medium underline-offset-4 hover:underline"
                      onClick={() => {
                        setDetailId(finding.id)
                      }}
                    >
                      {finding.title}
                    </button>
                  </TableCell>
                  <TableCell>
                    <SeverityBadge severity={finding.severity} />
                  </TableCell>
                  <TableCell>
                    <StatusBadge status={finding.status} />
                  </TableCell>
                  <TableCell className="text-muted-foreground text-sm">
                    {finding.owner || '—'}
                  </TableCell>
                  <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
                    {formatMoment(finding.createdAt)}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      <CreateFindingDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
      />

      {detailFinding !== undefined && (
        <FindingDetailDialog
          finding={detailFinding}
          open
          onOpenChange={(open) => {
            if (!open) setDetailId(undefined)
          }}
        />
      )}
    </div>
  )
}

function SeverityBadge({
  severity,
}: {
  severity: string
}): ReactNode {
  const variant =
    severity === 'critical' || severity === 'high'
      ? 'destructive'
      : severity === 'medium'
        ? 'default'
        : 'secondary'
  return (
    <Badge variant={variant as never}>
      {severity.charAt(0).toUpperCase() + severity.slice(1).replace('_', ' ')}
    </Badge>
  )
}

function StatusBadge({ status }: { status: string }): ReactNode {
  const variant =
    status === 'open'
      ? 'default'
      : status === 'in_progress'
        ? 'default'
        : status === 'resolved'
          ? 'secondary'
          : 'outline'
  return (
    <Badge variant={variant as never}>
      {status === 'in_progress'
        ? 'In Progress'
        : status === 'wont_fix'
          ? "Won't Fix"
          : status.charAt(0).toUpperCase() + status.slice(1)}
    </Badge>
  )
}

function CreateFindingDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}): ReactNode {
  const { engagementId } = useEngagementContext()
  const createFinding = useCreateFinding()
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [severity, setSeverity] = useState('medium')
  const [recommendation, setRecommendation] = useState('')
  const [submitted, setSubmitted] = useState(false)

  function reset(): void {
    setTitle('')
    setDescription('')
    setSeverity('medium')
    setRecommendation('')
    setSubmitted(false)
  }

  function handleSubmit(event: FormEvent): void {
    event.preventDefault()
    if (title.trim().length === 0 || description.trim().length === 0) return
    setSubmitted(true)
    const body: NewFinding = {
      title: title.trim(),
      description: description.trim(),
      severity: severity as NewFinding['severity'],
    }
    if (recommendation.trim()) body.recommendation = recommendation.trim()
    createFinding.mutate(
      { engagementId, body },
      {
        onSuccess: () => {
          toast.success(`Finding "${title}" raised.`)
          reset()
          onOpenChange(false)
        },
        onError: (error) => {
          toast.error(error.message)
          setSubmitted(false)
        },
      },
    )
  }

  function handleOpenChange(next: boolean): void {
    if (!next) reset()
    onOpenChange(next)
  }

  const canSubmit = title.trim().length > 0 && description.trim().length > 0

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>New finding</DialogTitle>
          <DialogDescription>
            Raise a remediation finding for this engagement.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="finding-title">Title</Label>
            <Input
              id="finding-title"
              required
              value={title}
              onChange={(e) => {
                setTitle(e.target.value)
              }}
              placeholder="Missing multi-factor on admin portal"
              maxLength={500}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="finding-severity">Severity</Label>
            <Select value={severity} onValueChange={setSeverity}>
              <SelectTrigger id="finding-severity">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {SEVERITIES.map((s) => (
                  <SelectItem key={s.value} value={s.value}>
                    {s.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="finding-description">Description</Label>
            <Textarea
              id="finding-description"
              required
              value={description}
              onChange={(e) => {
                setDescription(e.target.value)
              }}
              placeholder="Describe what was found and the impact…"
              rows={4}
              maxLength={16384}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="finding-recommendation">Recommendation</Label>
            <Textarea
              id="finding-recommendation"
              value={recommendation}
              onChange={(e) => {
                setRecommendation(e.target.value)
              }}
              placeholder="How to fix it…"
              rows={3}
              maxLength={16384}
            />
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                handleOpenChange(false)
              }}
              disabled={submitted}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={submitted || !canSubmit}>
              {submitted ? 'Creating…' : 'Create finding'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

function FindingDetailDialog({
  finding,
  open,
  onOpenChange,
}: {
  finding: Finding
  open: boolean
  onOpenChange: (open: boolean) => void
}): ReactNode {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex max-h-[min(90dvh,48rem)] max-w-2xl flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
        <DialogHeader className="border-b px-6 py-4 text-left">
          <DialogTitle className="pr-8 text-base leading-snug">
            <div className="flex items-center gap-2">
              {finding.title}
              <SeverityBadge severity={finding.severity} />
              <StatusBadge status={finding.status} />
            </div>
          </DialogTitle>
          <DialogDescription className="text-left">
            {finding.owner && (
              <span>
                Owner: {finding.owner} ·{' '}
              </span>
            )}
            Created {formatMoment(finding.createdAt)}
            {finding.updatedAt !== finding.createdAt &&
              ` · Updated ${formatMoment(finding.updatedAt)}`}
          </DialogDescription>
        </DialogHeader>
        <div className="min-h-0 flex-1 overflow-y-auto px-6 py-4">
          <div className="flex flex-col gap-4">
            <div>
              <h3 className="text-sm font-medium">Description</h3>
              <p className="text-muted-foreground mt-1 text-sm whitespace-pre-wrap">
                {finding.description}
              </p>
            </div>
            {finding.recommendation && (
              <div>
                <h3 className="text-sm font-medium">Recommendation</h3>
                <p className="text-muted-foreground mt-1 text-sm whitespace-pre-wrap">
                  {finding.recommendation}
                </p>
              </div>
            )}
            {finding.stepIds && finding.stepIds.length > 0 && (
              <div>
                <h3 className="text-sm font-medium">Related steps</h3>
                <div className="mt-1 flex flex-wrap gap-1">
                  {finding.stepIds.map((id) => (
                    <Badge key={id} variant="secondary" className="font-mono text-xs">
                      {id}
                    </Badge>
                  ))}
                </div>
              </div>
            )}
            {finding.createdFromExecution && (
              <div>
                <h3 className="text-sm font-medium">Source execution</h3>
                <p className="text-muted-foreground mt-1 font-mono text-xs">
                  {finding.createdFromExecution}
                </p>
              </div>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
