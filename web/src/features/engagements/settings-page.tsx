import { type ReactNode, useState } from 'react'
import { useNavigate } from 'react-router'
import { toast } from 'sonner'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { ConfirmDialog } from '@/features/admin/confirm-dialog'

import { useEngagementContext } from './engagement-layout'
import {
  useDeleteEngagement,
  useEngagement,
  usePatchEngagement,
  useSetEngagementStatus,
  type Engagement,
  type EngagementMode,
  type EngagementStatus,
} from './queries'
import { canManage } from './roles'

const MODE_OPTIONS: { value: EngagementMode; label: string }[] = [
  { value: 'standard', label: 'Standard' },
  { value: 'blind', label: 'Blind' },
]

// ── Page ───────────────────────────────────────────────────────────────────────

export function SettingsPage(): ReactNode {
  const { engagementId, role } = useEngagementContext()
  const engagement = useEngagement(engagementId)

  if (engagement.isPending) {
    return <PageLoading label="Loading engagement…" />
  }

  if (engagement.error) {
    return (
      <PageError
        error={engagement.error}
        onRetry={() => {
          void engagement.refetch()
        }}
      />
    )
  }

  if (!canManage(role)) {
    return (
      <PageEmpty
        title="You do not have permission to manage this engagement."
        description="Only the engagement lead or a platform admin can change settings."
      />
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <EngagementSettings engagement={engagement.data} engagementId={engagementId} />
    </div>
  )
}

// ── Settings Sections ──────────────────────────────────────────────────────────

function EngagementSettings({
  engagement,
  engagementId,
}: {
  engagement: Engagement
  engagementId: string
}): ReactNode {
  const closed = engagement.status === 'closed' || engagement.status === 'archived'

  return (
    <>
      <ModeSection engagementId={engagementId} mode={engagement.mode} disabled={closed} />
      <AutoRevealSection
        engagementId={engagementId}
        autoReveal={engagement.autoRevealOnStart}
        disabled={closed}
      />
      <StatusSection engagementId={engagementId} current={engagement.status} />
      <DangerSection engagementId={engagementId} engagementName={engagement.name} />
    </>
  )
}

// ── Mode ──────────────────────────────────────────────────────────────────────

function ModeSection({
  engagementId,
  mode,
  disabled,
}: {
  engagementId: string
  mode: EngagementMode
  disabled: boolean
}): ReactNode {
  const patchEngagement = usePatchEngagement()

  return (
    <section className="rounded-lg border p-5">
      <h2 className="mb-3 text-lg font-semibold">Mode</h2>
      <p className="text-muted-foreground mb-3 text-sm">
        Standard shows the workbook to both sides. Blind hides step details from blue until red
        manually reveals them.
      </p>
      {disabled && (
        <p className="text-muted-foreground mb-3 text-xs">
          Mode cannot be changed on a closed or archived engagement.
        </p>
      )}
      <Select
        value={mode}
        disabled={disabled || patchEngagement.isPending}
        onValueChange={(v) => {
          const newMode = v as EngagementMode
          patchEngagement.mutate(
            { engagementId, patch: { mode: newMode } },
            {
              onSuccess: () => {
                toast.success(`Mode set to ${newMode === 'blind' ? 'Blind' : 'Standard'}.`)
              },
              onError: (error) => {
                toast.error(error.message)
              },
            },
          )
        }}
      >
        <SelectTrigger className="w-44" aria-label="Engagement mode">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {MODE_OPTIONS.map((opt) => (
            <SelectItem key={opt.value} value={opt.value}>
              {opt.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </section>
  )
}

// ── Auto-reveal ────────────────────────────────────────────────────────────────

function AutoRevealSection({
  engagementId,
  autoReveal,
  disabled,
}: {
  engagementId: string
  autoReveal: boolean
  disabled: boolean
}): ReactNode {
  const patchEngagement = usePatchEngagement()

  return (
    <section className="rounded-lg border p-5">
      <h2 className="mb-3 text-lg font-semibold">Auto-reveal</h2>
      <p className="text-muted-foreground mb-3 text-sm">
        When enabled, the first red execution on each step automatically reveals it to blue. When
        disabled, steps must be manually revealed.
      </p>
      <div className="flex items-center gap-2">
        <Checkbox
          id="auto-reveal"
          checked={autoReveal}
          disabled={disabled || patchEngagement.isPending}
          onCheckedChange={(checked) => {
            patchEngagement.mutate(
              {
                engagementId,
                patch: { autoRevealOnStart: checked === true },
              },
              {
                onSuccess: () => {
                  toast.success(checked === true ? 'Auto-reveal enabled.' : 'Auto-reveal disabled.')
                },
                onError: (error) => {
                  toast.error(error.message)
                },
              },
            )
          }}
        />
        <Label htmlFor="auto-reveal" className="cursor-pointer text-sm">
          Auto-reveal steps on start
        </Label>
      </div>
    </section>
  )
}

// ── Status ─────────────────────────────────────────────────────────────────────

function StatusSection({
  engagementId,
  current,
}: {
  engagementId: string
  current: EngagementStatus
}): ReactNode {
  const setStatus = useSetEngagementStatus()
  const transitions = validTransitions(current)

  if (transitions.length === 0) return null

  return (
    <section className="rounded-lg border p-5">
      <h2 className="mb-3 text-lg font-semibold">Status</h2>
      <p className="text-muted-foreground mb-3 text-sm">
        Current status:{' '}
        <Badge
          variant={current === 'active' ? 'default' : current === 'draft' ? 'secondary' : 'outline'}
        >
          {current.charAt(0).toUpperCase() + current.slice(1)}
        </Badge>
      </p>
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-muted-foreground text-sm">Transition to:</span>
        {transitions.map((t) => (
          <Button
            key={t.to}
            variant="outline"
            size="sm"
            disabled={setStatus.isPending}
            onClick={() => {
              setStatus.mutate(
                { engagementId, status: { status: t.to } },
                {
                  onSuccess: () => {
                    toast.success(`Engagement ${t.label.toLowerCase()}d.`)
                  },
                  onError: (error) => {
                    toast.error(error.message)
                  },
                },
              )
            }}
          >
            {t.label}
          </Button>
        ))}
      </div>
    </section>
  )
}

function validTransitions(current: EngagementStatus): { to: EngagementStatus; label: string }[] {
  switch (current) {
    case 'draft':
      return [
        { to: 'active', label: 'Activate' },
        { to: 'closed', label: 'Close' },
      ]
    case 'active':
      return [{ to: 'closed', label: 'Close' }]
    case 'closed':
      return [{ to: 'archived', label: 'Archive' }]
    default:
      return []
  }
}

// ── Danger Zone ────────────────────────────────────────────────────────────────

function DangerSection({
  engagementId,
  engagementName,
}: {
  engagementId: string
  engagementName: string
}): ReactNode {
  const navigate = useNavigate()
  const deleteEngagement = useDeleteEngagement()
  const [confirmOpen, setConfirmOpen] = useState(false)

  return (
    <section className="border-destructive/30 rounded-lg border p-5">
      <h2 className="text-destructive mb-3 text-lg font-semibold">Danger zone</h2>
      <p className="text-muted-foreground mb-3 text-sm">
        Permanently delete this engagement and all its scenarios, steps, executions, findings, and
        evidence. This cannot be undone.
      </p>

      <Button
        variant="destructive"
        size="sm"
        onClick={() => {
          setConfirmOpen(true)
        }}
      >
        Delete engagement
      </Button>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title="Delete engagement"
        description={
          <>
            Permanently delete <strong>{engagementName}</strong> and all of its data? This action
            cannot be undone.
          </>
        }
        confirmLabel="Delete"
        destructive
        pending={deleteEngagement.isPending}
        onConfirm={() => {
          deleteEngagement.mutate(
            { engagementId },
            {
              onSuccess: () => {
                toast.success(`"${engagementName}" deleted.`)
                void navigate('/engagements')
              },
              onError: (error) => {
                toast.error(error.message)
              },
            },
          )
        }}
      />
    </section>
  )
}
