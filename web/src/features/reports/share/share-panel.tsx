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
import { PageLoading } from '@/app/shell/page-state'
import { formatMoment } from '@/lib/time'
import { CopyIcon, LinkIcon, PlusIcon, Trash2Icon, XIcon } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { toast } from 'sonner'

import {
  useShares,
  useCreateShare,
  useRevokeShare,
  useRevokeGrant,
  type ReportShare,
} from '../queries'

/**
 * Share panel for a published version — create, list, revoke (M6-014).
 *
 * The share token is shown once after creation with a copy button and
 * a warning that it cannot be retrieved again.
 */
export function SharePanel({ versionId }: { versionId: string }): ReactNode {
  const shares = useShares(versionId)
  const createShare = useCreateShare()
  const revokeShare = useRevokeShare()
  const revokeGrant = useRevokeGrant()

  const [createOpen, setCreateOpen] = useState(false)
  const [password, setPassword] = useState('')
  const [expiresAt, setExpiresAt] = useState('')
  const [label, setLabel] = useState('')
  const [maxGrants, setMaxGrants] = useState('')

  // Token display state — shown once after creation
  const [resultToken, setResultToken] = useState<string | undefined>(undefined)
  const [resultUrl, setResultUrl] = useState<string | undefined>(undefined)

  if (shares.isPending) return <PageLoading label="Loading shares…" />

  return (
    <div className="space-y-3">
      {/* Token result — shown once */}
      {resultToken && (
        <div className="rounded-md border border-primary/30 bg-primary/5 px-3 py-2">
          <div className="mb-1 text-xs font-medium text-primary">Share created</div>
          <div className="flex items-center gap-1">
            <code className="flex-1 truncate rounded bg-muted px-1.5 py-0.5 text-[10px]">
              {resultUrl}
            </code>
            <Button
              size="sm"
              variant="ghost"
              className="size-6"
              title="Copy URL"
              onClick={() => {
                void navigator.clipboard.writeText(resultUrl)
              }}
            >
              <CopyIcon className="size-3" />
            </Button>
          </div>
          <p className="mt-1 text-[10px] text-destructive">
            This link cannot be retrieved again. Save it now.
          </p>
          <Button
            size="sm"
            variant="ghost"
            className="mt-1 h-5 text-[10px]"
            onClick={() => {
              setResultToken(undefined)
              setResultUrl(undefined)
            }}
          >
            <XIcon className="size-3" />
            Dismiss
          </Button>
        </div>
      )}

      {/* Share list */}
      {shares.data && shares.data.length > 0 && (
        <div className="space-y-2">
          {shares.data.map((share) => (
            <ShareRow
              key={share.id}
              share={share}
              versionId={versionId}
              onRevokeShare={(sid) => {
                revokeShare.mutate(
                  { shareId: sid, versionId },
                  {
                    onSuccess: () => {
                      toast.success('Share revoked')
                    },
                    onError: () => {
                      toast.error('Failed to revoke share')
                    },
                  },
                )
              }}
              onRevokeGrant={(shareId, grantId) => {
                revokeGrant.mutate(
                  { shareId, grantId, versionId },
                  {
                    onSuccess: () => {
                      toast.success('Grant revoked')
                    },
                    onError: () => {
                      toast.error('Failed to revoke grant')
                    },
                  },
                )
              }}
            />
          ))}
        </div>
      )}

      {(!shares.data || shares.data.length === 0) && !resultToken && (
        <p className="text-xs text-muted-foreground">No share links yet.</p>
      )}

      {/* Create share dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogTrigger asChild>
          <Button size="sm" variant="outline" className="w-full">
            <PlusIcon className="size-3" />
            Create share link
          </Button>
        </DialogTrigger>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Create share link</DialogTitle>
          </DialogHeader>

          <div className="space-y-3">
            <div className="space-y-1">
              <Label htmlFor="share-label">Label (optional)</Label>
              <Input
                id="share-label"
                placeholder="e.g. Client review"
                value={label}
                onChange={(e) => {
                  setLabel(e.target.value)
                }}
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="share-password">Password (optional)</Label>
              <Input
                id="share-password"
                type="password"
                placeholder="Min 8 characters"
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value)
                }}
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="share-expiry">Expiry (optional)</Label>
              <Input
                id="share-expiry"
                type="datetime-local"
                value={expiresAt}
                onChange={(e) => {
                  setExpiresAt(e.target.value)
                }}
              />
            </div>
            <div className="space-y-1">
              <Label htmlFor="share-max-grants">Max grants (optional)</Label>
              <Input
                id="share-max-grants"
                type="number"
                min={1}
                placeholder="Unlimited"
                value={maxGrants}
                onChange={(e) => {
                  setMaxGrants(e.target.value)
                }}
              />
            </div>
          </div>

          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => {
                setCreateOpen(false)
              }}
            >
              Cancel
            </Button>
            <Button
              onClick={() => {
                const body: Record<string, unknown> = {}
                if (label.trim()) body.label = label.trim()
                if (password.length >= 8) body.password = password
                else if (password.length > 0) {
                  toast.error('Password must be at least 8 characters')
                  return
                }
                if (expiresAt) body.expiresAt = new Date(expiresAt).toISOString()
                if (maxGrants) body.maxGrants = parseInt(maxGrants, 10)

                createShare.mutate(
                  { versionId, body },
                  {
                    onSuccess: (result) => {
                      setCreateOpen(false)
                      setPassword('')
                      setExpiresAt('')
                      setLabel('')
                      setMaxGrants('')
                      setResultToken(result.token)
                      setResultUrl(result.claimUrl)
                      toast.success('Share created')
                    },
                    onError: () => {
                      toast.error('Failed to create share')
                    },
                  },
                )
              }}
              disabled={createShare.isPending}
            >
              {createShare.isPending ? 'Creating…' : 'Create'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

function ShareRow({
  share,
  versionId: _versionId,
  onRevokeShare,
  onRevokeGrant,
}: {
  share: ReportShare
  versionId: string
  onRevokeShare: (shareId: string) => void
  onRevokeGrant: (shareId: string, grantId: string) => void
}): ReactNode {
  const [showGrants, setShowGrants] = useState(false)

  const status =
    share.revokedAt
      ? ('Revoked' as const)
      : share.expiresAt && new Date(share.expiresAt) < new Date()
        ? ('Expired' as const)
        : ('Active' as const)

  return (
    <div className="rounded border bg-background px-3 py-2">
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-1.5">
            <LinkIcon className="size-3 text-muted-foreground shrink-0" />
            <span className="truncate text-xs font-medium">
              {share.label ?? 'Untitled share'}
            </span>
          </div>
          <div className="mt-0.5 flex items-center gap-1.5 text-[10px] text-muted-foreground">
            <span>{status}</span>
            {share.passwordProtected && (
              <>
                <span>&middot;</span>
                <span>Password protected</span>
              </>
            )}
            {share.grantCount !== undefined && (
              <>
                <span>&middot;</span>
                <span>
                  {share.grantCount} grant{share.grantCount !== 1 ? 's' : ''}
                </span>
              </>
            )}
          </div>
        </div>
        <div className="flex items-center gap-1 shrink-0">
          {status === 'Active' && share.grants && share.grants.length > 0 && (
            <Button
              size="sm"
              variant="ghost"
              className="h-6 text-[10px]"
              onClick={() => {
                setShowGrants(!showGrants)
              }}
            >
              {showGrants ? 'Hide' : 'Grants'}
            </Button>
          )}
          {status === 'Active' && (
            <Button
              size="sm"
              variant="ghost"
              className="size-6 text-destructive"
              title="Revoke share"
              onClick={() => {
                onRevokeShare(share.id)
              }}
            >
              <Trash2Icon className="size-3" />
            </Button>
          )}
        </div>
      </div>

      {showGrants && share.grants && share.grants.length > 0 && (
        <div className="mt-2 space-y-1 border-t pt-2">
          {share.grants.map((grant) => {
            const grantStatus =
              grant.revokedAt
                ? ('Revoked' as const)
                : grant.claimedAt
                  ? ('Claimed' as const)
                  : ('Unclaimed' as const)
            return (
              <div
                key={grant.id}
                className="flex items-center justify-between gap-2 text-[10px]"
              >
                <span className="text-muted-foreground">
                  {grant.userId ? (
                    <>
                      User <code className="text-[10px]">{grant.userId.slice(0, 8)}</code>
                    </>
                  ) : (
                    'Unclaimed invite'
                  )}
                  {grant.claimedAt && (
                    <span className="ml-1"> &middot; {formatMoment(grant.claimedAt)}</span>
                  )}
                </span>
                <span className="flex items-center gap-1">
                  <span
                    className={
                      grantStatus === 'Revoked'
                        ? 'text-destructive'
                        : grantStatus === 'Claimed'
                          ? 'text-green-600'
                          : 'text-muted-foreground'
                    }
                  >
                    {grantStatus}
                  </span>
                  {grantStatus !== 'Revoked' && (
                    <Button
                      size="sm"
                      variant="ghost"
                      className="size-4 text-destructive"
                      title="Revoke grant"
                      onClick={() => {
                        onRevokeGrant(share.id, grant.id)
                      }}
                    >
                      <XIcon className="size-3" />
                    </Button>
                  )}
                </span>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
