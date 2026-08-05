import { Loader2Icon } from 'lucide-react'
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { useQueryClient } from '@tanstack/react-query'

import { Badge } from '@/components/ui/badge'
import { useEventSource, type ServerEvent } from '@/lib/use-event-source'
import { cn } from '@/lib/utils'

import {
  findActiveJob,
  isActiveJobStatus,
  jobKindLabel,
  sourceKeys,
  useContentJob,
  useContentJobs,
  type ContentSyncJob,
} from './sources-queries'

/**
 * Live view of the global content job slot (M2-014).
 *
 * Prefers SSE (`content.jobs`). When EventSource is unavailable or stays in
 * error, falls back to the jobs query's own refetch interval. On every
 * reconnect after a gap, reconciles the watched job from REST once — M2 does
 * not replay Last-Event-ID.
 *
 * REST is the durable source of truth for "is a job active"; SSE ticks only
 * refine phase/counters while connected. Terminal failures stay visible until
 * the next active job arrives.
 */

interface JobTick {
  jobId: string
  phase: string
  current: number
  total: number
  message: string
  status: ContentSyncJob['status']
}

function parseJobTick(data: unknown): JobTick | undefined {
  if (typeof data !== 'object' || data === null) {
    return undefined
  }
  if (!('jobId' in data) || typeof data.jobId !== 'string' || data.jobId === '') {
    return undefined
  }
  const status = 'status' in data && typeof data.status === 'string' ? data.status : ''
  if (
    status !== 'queued' &&
    status !== 'running' &&
    status !== 'cancelling' &&
    status !== 'cancelled' &&
    status !== 'succeeded' &&
    status !== 'failed' &&
    status !== 'interrupted'
  ) {
    return undefined
  }
  return {
    jobId: data.jobId,
    phase: 'phase' in data && typeof data.phase === 'string' ? data.phase : '',
    current: 'current' in data && typeof data.current === 'number' ? data.current : 0,
    total: 'total' in data && typeof data.total === 'number' ? data.total : 0,
    message: 'message' in data && typeof data.message === 'string' ? data.message : '',
    status,
  }
}

function tickFromJob(job: ContentSyncJob): JobTick {
  return {
    jobId: job.id,
    phase: job.phase,
    current: job.progressCurrent,
    total: job.progressTotal,
    message: job.message,
    status: job.status,
  }
}

export function JobProgressPanel({
  sourceNames,
}: {
  /** sourceId → display name, so the banner can name the source without another fetch. */
  sourceNames: ReadonlyMap<string, string>
}): ReactNode {
  const queryClient = useQueryClient()
  const jobs = useContentJobs()
  const restActive = findActiveJob(jobs.data?.items)

  /** Live SSE overlay for the active job; cleared on terminal. */
  const [sseTick, setSseTick] = useState<JobTick | undefined>(undefined)
  /** Last terminal event, kept until a new active job appears. */
  const [terminal, setTerminal] = useState<JobTick | undefined>(undefined)
  const wasConnected = useRef(false)

  const onEvent = useCallback(
    (event: ServerEvent) => {
      const body = parseJobTick(event.data)
      if (body === undefined) {
        return
      }

      if (event.type === 'content.job.terminal' || !isActiveJobStatus(body.status)) {
        setSseTick(undefined)
        setTerminal(body)
        void queryClient.invalidateQueries({ queryKey: sourceKeys.all })
        void queryClient.invalidateQueries({ queryKey: sourceKeys.jobs() })
        void queryClient.invalidateQueries({ queryKey: sourceKeys.job(body.jobId) })
        return
      }

      setTerminal(undefined)
      setSseTick(body)
      queryClient.setQueryData(sourceKeys.job(body.jobId), (prev: ContentSyncJob | undefined) => {
        if (prev === undefined) {
          return prev
        }
        return {
          ...prev,
          phase: body.phase,
          progressCurrent: body.current,
          progressTotal: body.total,
          message: body.message,
          status: body.status,
        }
      })
    },
    [queryClient],
  )

  const sseAvailable = typeof globalThis.EventSource === 'function'

  const { connected, error: sseError } = useEventSource({
    topics: ['content.jobs'],
    onEvent,
    enabled: sseAvailable,
  })

  const watchedId =
    (sseTick !== undefined && isActiveJobStatus(sseTick.status) ? sseTick.jobId : undefined) ??
    restActive?.id ??
    terminal?.jobId

  // Reconcile from REST once when the socket reopens after a drop.
  useEffect(() => {
    if (connected) {
      if (!wasConnected.current && watchedId !== undefined) {
        void queryClient.invalidateQueries({ queryKey: sourceKeys.job(watchedId) })
        void queryClient.invalidateQueries({ queryKey: sourceKeys.jobs() })
      }
      wasConnected.current = true
      return
    }
    if (sseError !== null) {
      wasConnected.current = false
    }
  }, [connected, sseError, watchedId, queryClient])

  const jobQuery = useContentJob(watchedId)

  // Prefer live SSE, then REST active job, then a remembered terminal failure.
  let display:
    | { kind: 'active'; tick: JobTick; job: ContentSyncJob | undefined }
    | { kind: 'terminal'; tick: JobTick; job: ContentSyncJob | undefined }
    | undefined

  if (sseTick !== undefined && isActiveJobStatus(sseTick.status)) {
    display = {
      kind: 'active',
      tick: sseTick,
      job: jobQuery.data ?? (restActive?.id === sseTick.jobId ? restActive : undefined),
    }
  } else if (restActive !== undefined) {
    display = { kind: 'active', tick: tickFromJob(restActive), job: restActive }
  } else if (terminal !== undefined) {
    display = {
      kind: 'terminal',
      tick: terminal,
      job: jobQuery.data?.id === terminal.jobId ? jobQuery.data : undefined,
    }
  }

  if (display === undefined) {
    return null
  }

  const { tick: shown, job, kind } = display
  const sourceName =
    job !== undefined ? (sourceNames.get(job.sourceId) ?? job.sourceId) : shown.jobId
  const pct =
    shown.total > 0 ? Math.min(100, Math.round((shown.current / shown.total) * 100)) : undefined
  const usingFallback = !sseAvailable || (sseError !== null && !connected)
  const failed = kind === 'terminal' && shown.status === 'failed'
  const succeeded = kind === 'terminal' && shown.status === 'succeeded'

  return (
    <section
      aria-live="polite"
      className={cn(
        'flex flex-col gap-3 rounded-lg border p-4',
        failed && 'border-destructive/40 bg-destructive/5',
        succeeded && 'border-emerald-600/30 bg-emerald-500/5',
        kind === 'active' && 'bg-muted/40',
      )}
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <div className="flex flex-wrap items-center gap-2">
            {kind === 'active' && (
              <Loader2Icon className="text-muted-foreground size-4 animate-spin" aria-hidden />
            )}
            <h2 className="text-sm font-semibold">
              {kind === 'active'
                ? 'A content job is running'
                : failed
                  ? 'Content job failed'
                  : shown.status === 'cancelled'
                    ? 'Content job cancelled'
                    : 'Content job finished'}
            </h2>
            <Badge variant="outline">{job !== undefined ? jobKindLabel(job.kind) : 'Job'}</Badge>
            <Badge variant={failed ? 'destructive' : 'secondary'}>{shown.status}</Badge>
          </div>
          <p className="text-muted-foreground text-sm">
            {sourceName}
            {job?.version !== undefined && job.version !== '' ? ` · ${job.version}` : ''}
            {shown.phase !== '' ? ` · ${shown.phase}` : ''}
          </p>
        </div>
        <p className="text-muted-foreground font-mono text-xs">
          job <span className="select-all">{shown.jobId}</span>
        </p>
      </div>

      {kind === 'active' && (
        <div className="flex flex-col gap-1.5">
          <div
            className="bg-muted h-2 overflow-hidden rounded-full"
            role="progressbar"
            aria-valuemin={0}
            aria-valuemax={shown.total > 0 ? shown.total : 100}
            aria-valuenow={shown.total > 0 ? shown.current : undefined}
            aria-label="Job progress"
          >
            <div
              className="bg-primary h-full transition-[width] duration-300"
              style={{ width: pct === undefined ? '15%' : `${String(pct)}%` }}
            />
          </div>
          <p className="text-muted-foreground text-xs">
            {shown.message !== ''
              ? shown.message
              : pct === undefined
                ? 'Working…'
                : `${String(shown.current)} / ${String(shown.total)} (${String(pct)}%)`}
          </p>
        </div>
      )}

      {failed && (
        <div role="alert" className="flex flex-col gap-1 text-sm">
          <p className="font-medium">The job reported an error.</p>
          <p className="text-destructive whitespace-pre-wrap">
            {(job?.error !== undefined && job.error !== '' ? job.error : undefined) ??
              (shown.message !== '' ? shown.message : 'No error detail was returned.')}
          </p>
          <p className="text-muted-foreground text-xs">
            Quote job <code className="font-mono">{shown.jobId}</code> when reporting this.
          </p>
        </div>
      )}

      {succeeded && shown.message !== '' && (
        <p className="text-muted-foreground text-sm">{shown.message}</p>
      )}

      {usingFallback && kind === 'active' && (
        <p className="text-muted-foreground text-xs">
          Live updates unavailable — refreshing job status every few seconds instead.
        </p>
      )}
    </section>
  )
}
