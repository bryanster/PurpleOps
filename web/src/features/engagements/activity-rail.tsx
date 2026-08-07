import { type ReactNode, useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router'

import { PageEmpty, PageError } from '@/app/shell/page-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Separator } from '@/components/ui/separator'
import { cn } from '@/lib/utils'

import { useEngagementActivity, type ActivityEntry } from './activity-queries'
import {
  familyLabel,
  verbFamily,
  verbLabel,
  VERB_FAMILIES,
  type VerbFamily,
} from './activity-verbs'
import { engagementWorkbookPath } from './paths'
import { useEngagementContext } from './engagement-layout'

/** How long ago an ISO 8601 timestamp was, in compact English. */
function relativeTime(iso: string): string {
  const then = new Date(iso).getTime()
  const now = Date.now()
  const seconds = Math.floor((now - then) / 1000)
  if (seconds < 0) return 'just now'
  if (seconds < 60) return `${seconds}s ago`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`
  const days = Math.floor(hours / 24)
  if (days < 7) return `${days}d ago`
  return new Date(iso).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
  })
}

/**
 * Engagement activity rail (M4-008).
 *
 * A collapsible right-side panel that shows a live timeline of everything
 * that happened in this engagement — who did what to what, and when.
 * Filterable by verb family, navigable by clicking object references,
 * and updated in real time via SSE-driven query invalidation.
 */
export function ActivityRail(): ReactNode {
  const { engagementId } = useEngagementContext()
  const [open, setOpen] = useState(true)
  const [family, setFamily] = useState<VerbFamily | null>(null)
  const navigate = useNavigate()

  const activity = useEngagementActivity(engagementId)

  const rows = useMemo(
    () => activity.data?.pages.flatMap((page) => page.items) ?? [],
    [activity.data],
  )

  const filtered = useMemo(() => {
    if (!family) return rows
    return rows.filter((row) => verbFamily(row.verb) === family)
  }, [rows, family])

  const handleRowClick = useCallback(
    (_entry: ActivityEntry) => {
      // Navigate to the workbook — execution/step rows live there.
      void navigate(engagementWorkbookPath(engagementId))
    },
    [engagementId, navigate],
  )

  const hasMore = activity.hasNextPage && !activity.isFetchingNextPage

  const header = (
    <div className="flex items-center justify-between px-3 py-2">
      <h2 className="text-sm font-semibold">Activity</h2>
      <CollapsibleTrigger asChild>
        <Button variant="ghost" size="icon" className="size-7" aria-label="Toggle activity rail">
          <svg
            xmlns="http://www.w3.org/2000/svg"
            width="16"
            height="16"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            className={cn('transition-transform', open ? '' : 'rotate-180')}
          >
            <path d="M9 18l6-6-6-6" />
          </svg>
        </Button>
      </CollapsibleTrigger>
    </div>
  )

  const filters = (
    <div className="flex flex-wrap gap-1 px-3 pb-2">
      <Badge
        variant={family === null ? 'default' : 'outline'}
        className="cursor-pointer"
        onClick={() => setFamily(null)}
      >
        All
      </Badge>
      {VERB_FAMILIES.map((f) => (
        <Badge
          key={f}
          variant={family === f ? 'default' : 'outline'}
          className="cursor-pointer"
          onClick={() => setFamily(family === f ? null : f)}
        >
          {familyLabel(f)}
        </Badge>
      ))}
    </div>
  )

  let body: ReactNode

  if (activity.isPending) {
    body = (
      <div className="flex items-center justify-center py-8 text-sm text-muted-foreground">
        Loading…
      </div>
    )
  } else if (activity.error) {
    body = (
      <PageError
        error={activity.error}
        onRetry={() => {
          void activity.refetch()
        }}
      />
    )
  } else if (filtered.length === 0) {
    body = (
      <PageEmpty
        title={family ? `No ${familyLabel(family)} activity` : 'No activity yet'}
        description="Events will appear here as the engagement progresses."
      />
    )
  } else {
    body = (
      <div className="flex flex-col">
        {filtered.map((entry, i) => (
          <div key={entry.id}>
            {i > 0 && <Separator />}
            <ActivityRow entry={entry} onClick={handleRowClick} />
          </div>
        ))}
        {hasMore && (
          <div className="px-3 py-2">
            <Button
              variant="ghost"
              size="sm"
              className="w-full text-xs"
              disabled={activity.isFetchingNextPage}
              onClick={() => {
                void activity.fetchNextPage()
              }}
            >
              {activity.isFetchingNextPage ? 'Loading…' : 'Load older'}
            </Button>
          </div>
        )}
      </div>
    )
  }

  return (
    <Collapsible open={open} onOpenChange={setOpen} className="flex flex-col border-l w-72 shrink-0">
      {header}
      <CollapsibleContent className="flex flex-col flex-1 min-h-0">
        {filters}
        <Separator />
        <div className="flex-1 overflow-y-auto">{body}</div>
      </CollapsibleContent>
    </Collapsible>
  )
}

function ActivityRow({
  entry,
  onClick,
}: {
  entry: ActivityEntry
  onClick: (entry: ActivityEntry) => void
}): ReactNode {
  const family = verbFamily(entry.verb)

  return (
    <button
      type="button"
      className={cn(
        'flex flex-col gap-0.5 px-3 py-2 text-left w-full',
        'hover:bg-accent hover:text-accent-foreground',
        'focus-visible:bg-accent focus-visible:outline-none',
      )}
      onClick={() => onClick(entry)}
    >
      <div className="flex items-center gap-1.5">
        <Badge variant="secondary" className="px-1 py-0 text-[10px] leading-normal font-normal">
          {familyLabel(family)}
        </Badge>
        <span className="text-xs text-muted-foreground tabular-nums">
          {relativeTime(entry.at)}
        </span>
      </div>
      <span className="text-sm">{verbLabel(entry.verb)}</span>
      <span className="text-xs text-muted-foreground truncate">
        {entry.objectType}/{entry.objectId.slice(0, 8)}…
      </span>
    </button>
  )
}
