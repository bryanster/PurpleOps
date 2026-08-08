import { type ReactNode, useId, useState } from 'react'

import { PageEmpty, PageError, PageLoading } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { formatMoment } from '@/lib/time'

import { useActivity, type ActivityEntry, type ActivityFilters } from './queries'

/**
 * The installation-wide activity log (M1-015).
 *
 * Append-only and newest first. The filters are the ones an incident review
 * actually uses — who, what, and what to — and they are free text rather than
 * dropdowns because the vocabulary grows with every milestone and a fixed list
 * would be wrong by M3. The placeholder carries the spelling convention
 * (`object.past_tense_verb`) so nobody has to guess it.
 *
 * `delta` is rendered as text and never as markup. It is redacted before it is
 * stored — no password hash, token secret, TOTP secret or recovery code ever
 * reaches it — but it still contains values people typed, and this screen is
 * read by the person with the most authority on the installation.
 */
export function ActivityPage(): ReactNode {
  const [actorId, setActorId] = useState('')
  const [verb, setVerb] = useState('')
  const [objectType, setObjectType] = useState('')

  const filters: ActivityFilters = {
    ...(actorId.trim() === '' ? {} : { actorId: actorId.trim() }),
    ...(verb.trim() === '' ? {} : { verb: verb.trim() }),
    ...(objectType.trim() === '' ? {} : { objectType: objectType.trim() }),
  }
  const activity = useActivity(filters)

  const actorFilterId = useId()
  const verbFilterId = useId()
  const objectFilterId = useId()

  const rows = activity.data?.pages.flatMap((page) => page.items) ?? []
  const filtered = Object.keys(filters).length > 0

  return (
    <div className="flex flex-col gap-6">
      <header className="flex flex-col gap-1">
        <h1 className="text-xl font-semibold">Activity</h1>
        <p className="text-muted-foreground max-w-prose text-sm">
          Platform events — sign-ins, lockouts, token lifecycle, MFA and role changes — newest
          first. Nothing here can be edited or deleted, by anybody.
        </p>
      </header>

      <div className="flex flex-wrap items-end gap-3">
        <div className="flex min-w-64 flex-col gap-2">
          <Label htmlFor={actorFilterId}>Actor</Label>
          <Input
            id={actorFilterId}
            placeholder="User id"
            className="font-mono text-xs"
            value={actorId}
            onChange={(event) => {
              setActorId(event.target.value)
            }}
          />
        </div>
        <div className="flex min-w-48 flex-col gap-2">
          <Label htmlFor={verbFilterId}>Verb</Label>
          <Input
            id={verbFilterId}
            placeholder="session.login"
            className="font-mono text-xs"
            value={verb}
            onChange={(event) => {
              setVerb(event.target.value)
            }}
          />
        </div>
        <div className="flex min-w-40 flex-col gap-2">
          <Label htmlFor={objectFilterId}>Object type</Label>
          <Input
            id={objectFilterId}
            placeholder="user"
            className="font-mono text-xs"
            value={objectType}
            onChange={(event) => {
              setObjectType(event.target.value)
            }}
          />
        </div>
        {filtered && (
          <Button
            variant="ghost"
            onClick={() => {
              setActorId('')
              setVerb('')
              setObjectType('')
            }}
          >
            Clear filters
          </Button>
        )}
      </div>

      {activity.isPending && <PageLoading label="Reading the log…" />}

      {activity.error && (
        <PageError
          error={activity.error}
          onRetry={() => {
            void activity.refetch()
          }}
        />
      )}

      {activity.data &&
        (rows.length === 0 ? (
          <PageEmpty
            title={filtered ? 'Nothing matches those filters' : 'Nothing recorded yet'}
            description={
              filtered
                ? 'Check the spelling — a verb is `object.past_tense_verb`, and an actor is a user id rather than a name.'
                : 'Events appear here as people sign in and administer the installation.'
            }
          />
        ) : (
          <div className="flex flex-col gap-4">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>When</TableHead>
                  <TableHead>Verb</TableHead>
                  <TableHead>Actor</TableHead>
                  <TableHead>Object</TableHead>
                  <TableHead>Detail</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((entry) => (
                  <ActivityRow key={entry.id} entry={entry} />
                ))}
              </TableBody>
            </Table>

            {activity.hasNextPage && (
              <Button
                variant="outline"
                className="self-start"
                disabled={activity.isFetchingNextPage}
                onClick={() => {
                  void activity.fetchNextPage()
                }}
              >
                {activity.isFetchingNextPage ? 'Loading…' : 'Load more'}
              </Button>
            )}
          </div>
        ))}
    </div>
  )
}

function ActivityRow({ entry }: { entry: ActivityEntry }): ReactNode {
  return (
    <TableRow>
      <TableCell className="text-sm whitespace-nowrap">{formatMoment(entry.at)}</TableCell>
      <TableCell>
        <Badge variant="secondary" className="font-mono text-xs">
          {entry.verb}
        </Badge>
      </TableCell>
      <TableCell className="font-mono text-xs">
        {/* Absent when nobody could be identified — a failed sign-in that named
            no account. "Unknown" rather than an empty cell, because which of
            the two it is matters when reading a lockout. */}
        {entry.actorId ?? <span className="text-muted-foreground font-sans">Unknown</span>}
      </TableCell>
      <TableCell className="font-mono text-xs">
        {entry.objectType}
        <span className="text-muted-foreground"> {entry.objectId}</span>
      </TableCell>
      <TableCell className="max-w-96">
        {entry.delta === undefined ? (
          <span className="text-muted-foreground">—</span>
        ) : (
          <code className="text-xs break-all">{JSON.stringify(entry.delta)}</code>
        )}
      </TableCell>
    </TableRow>
  )
}
