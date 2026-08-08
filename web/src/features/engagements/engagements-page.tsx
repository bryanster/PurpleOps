import { type ReactNode, useState } from 'react'
import { Link } from 'react-router'
import { toast } from 'sonner'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { isBrowsableAttackVersion, useAttackVersions } from '@/features/content/queries'
import { formatMoment } from '@/lib/time'

import { engagementPath } from './paths'
import {
  useCreateEngagement,
  useEngagements,
  type CreateEngagement,
  type Engagement,
  type EngagementStatus,
} from './queries'

const STATUS_FILTER_ANY = 'any'
const STATUSES: { value: string; label: string }[] = [
  { value: 'draft', label: 'Draft' },
  { value: 'active', label: 'Active' },
  { value: 'closed', label: 'Closed' },
  { value: 'archived', label: 'Archived' },
]

const MODES: { value: string; label: string }[] = [
  { value: 'standard', label: 'Standard' },
  { value: 'blind', label: 'Blind' },
]

export function EngagementsPage(): ReactNode {
  const [statusFilter, setStatusFilter] = useState(STATUS_FILTER_ANY)
  const [createOpen, setCreateOpen] = useState(false)
  const engagements = useEngagements(
    statusFilter === STATUS_FILTER_ANY ? {} : { status: statusFilter },
  )

  const allItems = engagements.data?.pages.flatMap((p) => p.items) ?? []

  return (
    <div className="flex flex-col gap-6">
      <header className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-semibold">Engagements</h1>
          <p className="text-muted-foreground text-sm">
            Purple-team assessments you are a member of
          </p>
        </div>
        <Button
          size="sm"
          onClick={() => {
            setCreateOpen(true)
          }}
        >
          New engagement
        </Button>
      </header>

      <div className="flex flex-wrap items-end gap-3">
        <div className="flex flex-col gap-2">
          <Label htmlFor="status-filter">Status</Label>
          <Select
            value={statusFilter}
            onValueChange={(v) => {
              setStatusFilter(v)
            }}
          >
            <SelectTrigger id="status-filter" className="w-36">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={STATUS_FILTER_ANY}>All</SelectItem>
              {STATUSES.map((s) => (
                <SelectItem key={s.value} value={s.value}>
                  {s.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      {engagements.isPending && <PageLoading label="Loading engagements…" />}

      {engagements.error && (
        <PageError
          error={engagements.error}
          onRetry={() => {
            void engagements.refetch()
          }}
        />
      )}

      {engagements.data && allItems.length === 0 && (
        <PageEmpty
          title="No engagements"
          description={
            statusFilter === STATUS_FILTER_ANY
              ? 'Create one to get started.'
              : 'No engagements match this filter.'
          }
          action={
            statusFilter === STATUS_FILTER_ANY ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => {
                  setCreateOpen(true)
                }}
              >
                New engagement
              </Button>
            ) : undefined
          }
        />
      )}

      {engagements.data && allItems.length > 0 && (
        <div className="overflow-x-auto">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Client</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Mode</TableHead>
                <TableHead>ATT&amp;CK</TableHead>
                <TableHead>Dates</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {allItems.map((eng) => (
                <EngagementRow key={eng.id} engagement={eng} />
              ))}
            </TableBody>
          </Table>
        </div>
      )}

      {engagements.hasNextPage && (
        <div className="flex justify-center">
          <Button
            variant="outline"
            size="sm"
            disabled={engagements.isFetchingNextPage}
            onClick={() => {
              void engagements.fetchNextPage()
            }}
          >
            {engagements.isFetchingNextPage ? 'Loading…' : 'Load more'}
          </Button>
        </div>
      )}

      <CreateEngagementDialog open={createOpen} onOpenChange={setCreateOpen} />
    </div>
  )
}

function EngagementRow({ engagement }: { engagement: Engagement }): ReactNode {
  return (
    <TableRow>
      <TableCell>
        <Link
          to={engagementPath(engagement.id)}
          className="font-medium underline-offset-4 hover:underline"
        >
          {engagement.name}
        </Link>
      </TableCell>
      <TableCell className="text-muted-foreground text-sm">{engagement.client || '—'}</TableCell>
      <TableCell>
        <StatusBadge status={engagement.status} />
      </TableCell>
      <TableCell>
        <Badge variant="outline">{engagement.mode === 'blind' ? 'Blind' : 'Standard'}</Badge>
      </TableCell>
      <TableCell className="font-mono text-sm">{engagement.attackVersion}</TableCell>
      <TableCell className="text-muted-foreground text-sm whitespace-nowrap">
        {formatMoment(engagement.startsOn)} – {formatMoment(engagement.endsOn)}
      </TableCell>
    </TableRow>
  )
}

function StatusBadge({ status }: { status: EngagementStatus }): ReactNode {
  const variant =
    status === 'active'
      ? 'default'
      : status === 'draft'
        ? 'secondary'
        : status === 'closed'
          ? 'outline'
          : 'secondary'
  return <Badge variant={variant}>{statusLabel(status)}</Badge>
}

function statusLabel(status: EngagementStatus): string {
  return status.charAt(0).toUpperCase() + status.slice(1)
}

function CreateEngagementDialog({
  open,
  onOpenChange,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}): ReactNode {
  const versions = useAttackVersions()
  const createEngagement = useCreateEngagement()
  const [name, setName] = useState('')
  const [client, setClient] = useState('')
  const [attackVersion, setAttackVersion] = useState('')
  const [mode, setMode] = useState('standard')
  const [autoReveal, setAutoReveal] = useState(false)
  const [submitted, setSubmitted] = useState(false)

  const browsable = (versions.data?.items ?? []).filter(isBrowsableAttackVersion)

  function reset(): void {
    setName('')
    setClient('')
    setAttackVersion('')
    setMode('standard')
    setAutoReveal(false)
    setSubmitted(false)
  }

  function handleSubmit(event: React.SyntheticEvent): void {
    event.preventDefault()
    if (name.trim().length === 0 || attackVersion === '') return
    setSubmitted(true)
    const body: CreateEngagement = {
      name: name.trim(),
      client: client.trim(),
      description: '',
      attackVersion,
      mode: mode as 'standard' | 'blind',
      autoRevealOnStart: autoReveal,
    }

    createEngagement.mutate(body, {
      onSuccess: () => {
        toast.success(`Engagement "${name}" created.`)
        reset()
        onOpenChange(false)
      },
      onError: (error) => {
        toast.error(error.message)
        setSubmitted(false)
      },
    })
  }

  function handleOpenChange(next: boolean): void {
    if (!next) reset()
    onOpenChange(next)
  }

  const canSubmit = name.trim().length > 0 && attackVersion !== '' && !versions.isPending

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>New engagement</DialogTitle>
          <DialogDescription>
            You will be the lead. Add members from the engagement overview after creation.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="create-name">Name</Label>
            <Input
              id="create-name"
              required
              value={name}
              onChange={(e) => {
                setName(e.target.value)
              }}
              placeholder="Q4 Purple Team Assessment"
              maxLength={255}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="create-client">Client</Label>
            <Input
              id="create-client"
              value={client}
              onChange={(e) => {
                setClient(e.target.value)
              }}
              placeholder="Acme Corp"
              maxLength={255}
            />
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="create-attack">ATT&amp;CK version</Label>
            {versions.isPending ? (
              <p className="text-muted-foreground text-sm">Loading versions…</p>
            ) : browsable.length === 0 ? (
              <p className="text-muted-foreground text-sm">
                No ATT&amp;CK content installed. Sync a source first.
              </p>
            ) : (
              <Select value={attackVersion} onValueChange={setAttackVersion} required>
                <SelectTrigger id="create-attack" className="w-full">
                  <SelectValue placeholder="Select a version" />
                </SelectTrigger>
                <SelectContent>
                  {browsable.map((v) => (
                    <SelectItem key={v.version} value={v.version}>
                      {v.version}
                      <span className="text-muted-foreground ml-2 text-xs">({v.itemCount})</span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
          <div className="flex flex-col gap-2">
            <Label htmlFor="create-mode">Mode</Label>
            <Select value={mode} onValueChange={setMode}>
              <SelectTrigger id="create-mode" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {MODES.map((m) => (
                  <SelectItem key={m.value} value={m.value}>
                    {m.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex items-center gap-2">
            <Checkbox
              id="create-autoreveal"
              checked={autoReveal}
              onCheckedChange={(checked) => {
                setAutoReveal(checked === true)
              }}
            />
            <Label htmlFor="create-autoreveal" className="cursor-pointer text-sm">
              Auto-reveal steps on start
            </Label>
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
              {submitted ? 'Creating…' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
