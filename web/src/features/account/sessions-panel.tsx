import { type ReactNode, useState } from 'react'
import { toast } from 'sonner'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { ConfirmDialog } from '@/features/admin/confirm-dialog'
import { formatMoment } from '@/lib/time'
import { useRevokeOtherSessions, useRevokeSession, useSessions } from '@/features/auth/queries'
import type { Session } from '@/features/auth/queries'

import { SettingsSection } from './section'

/**
 * Where you are signed in, and how to stop being (M1-017).
 *
 * Every row is a browser that could act right now — the server filters to what
 * would still be accepted on the next request, using the same function that
 * accepts it — so "revoke" here is never a no-op on a session that had already
 * ended. The current session is marked and has no revoke button: ending it is
 * signing out, which lives in the top bar and clears the cookie properly.
 *
 * Nothing is optimistic. The row disappears when the server says it did.
 */
export function SessionsPanel(): ReactNode {
  const sessions = useSessions()
  const revoke = useRevokeSession()
  const revokeOthers = useRevokeOtherSessions()
  const [confirmingAll, setConfirmingAll] = useState(false)

  const items = sessions.data?.items ?? []
  const others = items.filter((item) => !item.current)

  return (
    <SettingsSection
      title="Where you are signed in"
      description="Sessions end on their own after a period of inactivity, and at a fixed age however busy they are. Anything here you do not recognise is worth ending."
    >
      {sessions.isPending && <PageLoading label="Reading your sessions…" />}

      {sessions.error && (
        <PageError
          error={sessions.error}
          onRetry={() => {
            void sessions.refetch()
          }}
        />
      )}

      {sessions.data && items.length === 0 && (
        // Practically unreachable — the request that asked was made on a
        // session — but a table that renders nothing at all is worse than a
        // sentence saying so, and this costs one line.
        <PageEmpty title="No live sessions" description="Nothing to end." />
      )}

      {sessions.data && items.length > 0 && (
        <div className="flex flex-col gap-4">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Signed in</TableHead>
                <TableHead>Last used</TableHead>
                <TableHead>Address</TableHead>
                <TableHead>Browser</TableHead>
                <TableHead className="w-0" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.map((session) => (
                <SessionRow
                  key={session.id}
                  session={session}
                  disabled={revoke.isPending || revokeOthers.isPending}
                  onRevoke={() => {
                    revoke.mutate(
                      { sessionId: session.id },
                      {
                        onSuccess: () => {
                          toast.success('That session was ended.')
                        },
                        onError: () => {
                          toast.error('That session could not be ended.')
                        },
                      },
                    )
                  }}
                />
              ))}
            </TableBody>
          </Table>

          <Button
            variant="outline"
            className="self-start"
            disabled={others.length === 0 || revokeOthers.isPending}
            onClick={() => {
              setConfirmingAll(true)
            }}
          >
            Sign out everywhere else
          </Button>
        </div>
      )}

      <ConfirmDialog
        open={confirmingAll}
        onOpenChange={setConfirmingAll}
        title="Sign out everywhere else?"
        // The count is the point of the sentence: "everywhere else" is abstract,
        // "3 other browsers" is a thing somebody can picture and check.
        description={`This ends ${describeCount(others.length)} and keeps the one you are using. Service tokens are not sessions and are not affected.`}
        confirmLabel="Sign out everywhere else"
        pending={revokeOthers.isPending}
        onConfirm={() => {
          revokeOthers.mutate(undefined, {
            onSuccess: (result) => {
              setConfirmingAll(false)
              toast.success(`Signed out of ${describeCount(result.revoked)}.`)
            },
            onError: () => {
              toast.error('Those sessions could not be ended.')
            },
          })
        }}
      />
    </SettingsSection>
  )
}

function SessionRow({
  session,
  onRevoke,
  disabled,
}: {
  session: Session
  onRevoke: () => void
  disabled: boolean
}): ReactNode {
  return (
    <TableRow>
      <TableCell className="whitespace-nowrap">
        {formatMoment(session.createdAt)}
        {session.current && (
          <Badge variant="secondary" className="ml-2">
            This browser
          </Badge>
        )}
        {!session.mfaSatisfied && (
          <Badge variant="outline" className="ml-2">
            No second factor
          </Badge>
        )}
      </TableCell>
      <TableCell className="whitespace-nowrap">{formatMoment(session.lastSeenAt)}</TableCell>
      <TableCell className="font-mono text-xs">{session.ip ?? '—'}</TableCell>
      {/* Text, never markup: a User-Agent is whatever the client sent. React
          escapes it, and the `break-all` is so a long one wraps rather than
          stretching the table off the screen. */}
      <TableCell className="max-w-64 truncate text-xs" title={session.userAgent ?? undefined}>
        {session.userAgent ?? '—'}
      </TableCell>
      <TableCell>
        {session.current ? (
          <span className="text-muted-foreground text-xs">Sign out to end</span>
        ) : (
          <Button variant="ghost" size="sm" disabled={disabled} onClick={onRevoke}>
            Revoke
          </Button>
        )}
      </TableCell>
    </TableRow>
  )
}

/** "3 other browsers", or "1 other browser". */
function describeCount(count: number): string {
  return count === 1 ? '1 other browser' : `${String(count)} other browsers`
}
