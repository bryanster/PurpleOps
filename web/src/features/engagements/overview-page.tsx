import { type ReactNode, useState } from 'react'
import { toast } from 'sonner'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { ConfirmDialog } from '@/features/admin/confirm-dialog'
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
import { useUsers } from '@/features/admin/queries'
import { formatMoment } from '@/lib/time'
import { cn } from '@/lib/utils'

import { useEngagementContext } from './engagement-layout'
import {
  useAddMember,
  useEngagement,
  useEngagementMembers,
  usePatchMember,
  useRemoveMember,
  useSetEngagementStatus,
  type Engagement,
  type EngagementMember,
  type EngagementRole,
  type EngagementStatus,
  type MemberRole,
} from './queries'
import { canManage } from './roles'

const ROLE_OPTIONS: { value: MemberRole; label: string }[] = [
  { value: 'lead', label: 'Lead' },
  { value: 'red', label: 'Red' },
  { value: 'blue', label: 'Blue' },
  { value: 'observer', label: 'Observer' },
]

function roleLabel(role: EngagementRole): string {
  switch (role) {
    case 'lead':
      return 'Lead'
    case 'red':
      return 'Red'
    case 'blue':
      return 'Blue'
    case 'observer':
      return 'Observer'
    case 'admin':
      return 'Admin'
  }
}

// ── Page ───────────────────────────────────────────────────────────────────────

export function OverviewPage(): ReactNode {
  const { engagementId, role, closed } = useEngagementContext()
  const engagement = useEngagement(engagementId)
  const members = useEngagementMembers(engagementId)

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

  return (
    <div className="flex flex-col gap-6">
      <MetadataCard engagement={engagement.data} role={role} closed={closed} />
      <MembersCard
        members={members.data ?? []}
        isPending={members.isPending}
        error={members.error ?? undefined}
        refetchMembers={() => {
          void members.refetch()
        }}
        engagementId={engagementId}
        role={role}
      />
    </div>
  )
}

// ── Metadata Card ──────────────────────────────────────────────────────────────

function MetadataCard({
  engagement,
  role,
  closed,
}: {
  engagement: Engagement
  role: EngagementRole
  closed: boolean
}): ReactNode {
  const canManageEng = canManage(role)
  const setStatus = useSetEngagementStatus()
  const transitions = canManageEng && !closed ? validTransitions(engagement.status) : []

  return (
    <section className="rounded-lg border p-5">
      <h2 className="mb-4 text-lg font-semibold">Overview</h2>

      <dl className="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div>
          <dt className="text-muted-foreground text-sm">Name</dt>
          <dd className="text-sm font-medium">{engagement.name}</dd>
        </div>

        <div>
          <dt className="text-muted-foreground text-sm">Client</dt>
          <dd className="text-sm">{engagement.client || '—'}</dd>
        </div>

        <div>
          <dt className="text-muted-foreground text-sm">Status</dt>
          <dd>
            <Badge
              variant={
                engagement.status === 'active'
                  ? 'default'
                  : engagement.status === 'draft'
                    ? 'secondary'
                    : 'outline'
              }
            >
              {engagement.status.charAt(0).toUpperCase() + engagement.status.slice(1)}
            </Badge>
          </dd>
        </div>

        <div>
          <dt className="text-muted-foreground text-sm">Mode</dt>
          <dd>
            <Badge variant="outline">{engagement.mode === 'blind' ? 'Blind' : 'Standard'}</Badge>
          </dd>
        </div>

        <div>
          <dt className="text-muted-foreground text-sm">ATT&amp;CK version</dt>
          <dd className="text-sm">{engagement.attackVersion}</dd>
        </div>

        <div>
          <dt className="text-muted-foreground text-sm">Dates</dt>
          <dd className="text-sm">
            {engagement.startsOn ? formatMoment(engagement.startsOn) : '—'} –{' '}
            {engagement.endsOn ? formatMoment(engagement.endsOn) : '—'}
          </dd>
        </div>

        {engagement.autoRevealOnStart && (
          <div>
            <dt className="text-muted-foreground text-sm">Auto-reveal</dt>
            <dd className="text-sm">Enabled</dd>
          </div>
        )}
      </dl>

      {engagement.description && (
        <div className="mt-4">
          <dt className="text-muted-foreground text-sm">Description</dt>
          <dd className="text-muted-foreground mt-1 text-sm leading-relaxed whitespace-pre-wrap">
            {engagement.description}
          </dd>
        </div>
      )}

      {transitions.length > 0 && (
        <div className="mt-5 flex flex-wrap items-center gap-2 border-t pt-4">
          <span className="text-muted-foreground text-sm">Transition to:</span>
          {transitions.map((t) => (
            <Button
              key={t.to}
              variant="outline"
              size="sm"
              disabled={setStatus.isPending}
              onClick={() => {
                setStatus.mutate(
                  { engagementId: engagement.id, status: { status: t.to } },
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
      )}
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

// ── Members Card ───────────────────────────────────────────────────────────────

function MembersCard({
  members,
  isPending,
  error,
  refetchMembers,
  engagementId,
  role,
}: {
  members: EngagementMember[]
  isPending: boolean
  error: Error | undefined
  refetchMembers: () => void
  engagementId: string
  role: EngagementRole
}): ReactNode {
  const canManageMembers = canManage(role)

  return (
    <section className="rounded-lg border p-5">
      <div className="mb-4 flex items-center justify-between gap-3">
        <h2 className="text-lg font-semibold">Members</h2>
        {canManageMembers && <AddMemberDialog engagementId={engagementId} />}
      </div>

      {isPending && <PageLoading label="Loading members…" />}

      {error && <PageError error={error} onRetry={refetchMembers} />}

      {!isPending && !error && members.length === 0 && (
        <PageEmpty title="No members" description="Add team members to this engagement." />
      )}

      {!isPending && !error && members.length > 0 && (
        <ul className="divide-y">
          {members.map((member) => (
            <MemberRow
              key={member.id}
              member={member}
              canManage={canManageMembers}
              engagementId={engagementId}
            />
          ))}
        </ul>
      )}
    </section>
  )
}

// ── Member Row ─────────────────────────────────────────────────────────────────

function MemberRow({
  member,
  canManage,
  engagementId,
}: {
  member: EngagementMember
  canManage: boolean
  engagementId: string
}): ReactNode {
  const patchMember = usePatchMember()
  const removeMember = useRemoveMember()
  const [removeOpen, setRemoveOpen] = useState(false)

  return (
    <li className="flex items-center justify-between gap-3 py-3 first:pt-0 last:pb-0">
      <div className="flex min-w-0 flex-col gap-0.5">
        <span className="truncate text-sm font-medium">{member.displayName}</span>
        <span className="text-muted-foreground truncate text-xs">{member.email}</span>
      </div>

      <div className="flex items-center gap-2">
        {canManage ? (
          <Select
            value={member.role}
            onValueChange={(v) => {
              const newRole = v as MemberRole
              if (newRole === member.role) return
              patchMember.mutate(
                {
                  engagementId,
                  userId: member.id,
                  body: { role: newRole },
                },
                {
                  onSuccess: () => {
                    toast.success(`${member.displayName} is now ${roleLabel(newRole)}.`)
                  },
                  onError: (error) => {
                    toast.error(error.message)
                  },
                },
              )
            }}
          >
            <SelectTrigger className="h-7 w-28" aria-label="Change role">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ROLE_OPTIONS.map((opt) => (
                <SelectItem key={opt.value} value={opt.value}>
                  {opt.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : (
          <Badge variant="secondary">{roleLabel(member.role)}</Badge>
        )}

        {canManage && (
          <>
            <Button
              variant="ghost"
              size="icon-xs"
              aria-label={`Remove ${member.displayName}`}
              onClick={() => {
                setRemoveOpen(true)
              }}
            >
              ×
            </Button>

            <ConfirmDialog
              open={removeOpen}
              onOpenChange={setRemoveOpen}
              title="Remove member"
              description={
                <>
                  Remove <strong>{member.displayName}</strong> from this engagement? They will lose
                  access to the workbook and findings.
                </>
              }
              confirmLabel="Remove"
              destructive
              pending={removeMember.isPending}
              onConfirm={() => {
                removeMember.mutate(
                  { engagementId, userId: member.id },
                  {
                    onSuccess: () => {
                      toast.success(`${member.displayName} removed.`)
                      setRemoveOpen(false)
                    },
                    onError: (error) => {
                      toast.error(error.message)
                    },
                  },
                )
              }}
            />
          </>
        )}
      </div>
    </li>
  )
}

// ── Add Member Dialog ──────────────────────────────────────────────────────────

function AddMemberDialog({ engagementId }: { engagementId: string }): ReactNode {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [selectedUserId, setSelectedUserId] = useState('')
  const [selectedRole, setSelectedRole] = useState<MemberRole>('observer')
  const addMember = useAddMember()
  const users = useUsers(search.trim() ? { q: search.trim() } : {})

  const allUsers = users.data?.pages.flatMap((p) => p.items) ?? []

  function handleSubmit(event: React.SyntheticEvent): void {
    event.preventDefault()
    if (!selectedUserId) return

    addMember.mutate(
      {
        engagementId,
        body: { userId: selectedUserId, role: selectedRole },
      },
      {
        onSuccess: () => {
          toast.success('Member added.')
          reset()
          setOpen(false)
        },
        onError: (error) => {
          toast.error(error.message)
        },
      },
    )
  }

  function reset(): void {
    setSearch('')
    setSelectedUserId('')
    setSelectedRole('observer')
  }

  function handleOpenChange(next: boolean): void {
    if (!next) reset()
    setOpen(next)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <Button
        size="sm"
        variant="outline"
        onClick={() => {
          setOpen(true)
        }}
      >
        Add member
      </Button>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Add member</DialogTitle>
          <DialogDescription>
            Search for a platform user and assign them an engagement role.
          </DialogDescription>
        </DialogHeader>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          {/* User search */}
          <div className="flex flex-col gap-2">
            <Label htmlFor="member-search">User</Label>
            <Input
              id="member-search"
              placeholder="Search by name or email…"
              value={search}
              onChange={(e) => {
                setSearch(e.target.value)
                setSelectedUserId('')
              }}
            />
            {users.isPending && <p className="text-muted-foreground text-xs">Searching…</p>}
            {allUsers.length > 0 && (
              <div className="max-h-40 overflow-y-auto rounded-md border">
                {allUsers.map((user) => {
                  const isSelected = user.id === selectedUserId
                  return (
                    <button
                      key={user.id}
                      type="button"
                      className={cn(
                        'w-full px-3 py-2 text-left text-sm transition-colors',
                        'hover:bg-muted',
                        isSelected && 'bg-accent',
                      )}
                      onClick={() => {
                        setSelectedUserId(user.id)
                      }}
                    >
                      <span className="font-medium">{user.displayName}</span>
                      <span className="text-muted-foreground ml-2 text-xs">{user.email}</span>
                    </button>
                  )
                })}
              </div>
            )}
            {!users.isPending && search.trim() && allUsers.length === 0 && (
              <p className="text-muted-foreground text-xs">No users found.</p>
            )}
          </div>

          {/* Role picker */}
          <div className="flex flex-col gap-2">
            <Label htmlFor="member-role">Role</Label>
            <Select
              value={selectedRole}
              onValueChange={(v) => {
                setSelectedRole(v as MemberRole)
              }}
            >
              <SelectTrigger id="member-role" className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ROLE_OPTIONS.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value}>
                    {opt.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                handleOpenChange(false)
              }}
              disabled={addMember.isPending}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={!selectedUserId || addMember.isPending}>
              {addMember.isPending ? 'Adding…' : 'Add'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
