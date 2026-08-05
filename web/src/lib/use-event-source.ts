import { useEffect, useRef, useState } from 'react'

import { apiUrl } from '@/api/client'

/**
 * One parsed SSE frame from `GET /api/v1/events`.
 *
 * `data` is the JSON object the server put in the `data:` field — for content
 * job ticks that is the full hub Event (`id`, `topic`, `type`, `at`, `data`).
 */
export interface ServerEvent {
  id?: string
  type: string
  data: unknown
}

export interface UseEventSourceOptions {
  /** Topic names (`content.jobs`, `content.jobs.{id}`, …). Empty disables. */
  topics: string[]
  /** Called for every non-comment frame. */
  onEvent?: (event: ServerEvent) => void
  /** When false the connection is not opened. Default true. */
  enabled?: boolean
}

export interface UseEventSourceState {
  /** true while the browser holds an open EventSource. */
  connected: boolean
  /** Last transport-level error, if any. Cleared on a successful open. */
  error: Event | null
}

/**
 * Subscribe to the shared SSE hub (`M2-004`).
 *
 * Session cookie only — EventSource cannot set Authorization, which matches
 * the server's session-only gate. On drop the browser reconnects with its
 * built-in backoff; callers that need catch-up after a gap should reconcile
 * from REST (`GET /content/jobs/{id}`) because M2 does not replay
 * `Last-Event-ID`.
 *
 * Prefer this over hand-rolled `EventSource` so M2-014 and M4 share one
 * reconnect and teardown path.
 */
export function useEventSource(options: UseEventSourceOptions): UseEventSourceState {
  const { topics, onEvent, enabled = true } = options
  const onEventRef = useRef(onEvent)
  useEffect(() => {
    onEventRef.current = onEvent
  }, [onEvent])

  const [connected, setConnected] = useState(false)
  const [error, setError] = useState<Event | null>(null)

  // Stable key so topic-array identity changes that are value-equal do not
  // thrash the socket.
  const topicsKey = topics.slice().sort().join('\0')
  const active = enabled && topics.length > 0

  useEffect(() => {
    if (!active) {
      return
    }

    const source = new EventSource(eventsUrl(topics), { withCredentials: true })

    source.onopen = () => {
      setConnected(true)
      setError(null)
    }
    source.onerror = (ev) => {
      setConnected(source.readyState === EventSource.OPEN)
      setError(ev)
    }

    // Named content job types. The browser only routes `event:` frames to a
    // named listener; `onmessage` covers frames with no event name.
    const deliver = (type: string) => (ev: MessageEvent) => {
      const raw = typeof ev.data === 'string' ? ev.data : String(ev.data)
      let data: unknown = raw
      try {
        data = JSON.parse(raw) as unknown
      } catch {
        // Leave the raw string; the server always sends JSON, but a proxy
        // inserting noise should not crash the hook.
      }
      onEventRef.current?.({ id: ev.lastEventId || undefined, type, data })
    }

    source.addEventListener('content.job.progress', deliver('content.job.progress'))
    source.addEventListener('content.job.terminal', deliver('content.job.terminal'))
    source.onmessage = (ev) => {
      deliver(ev.type || 'message')(ev)
    }

    return () => {
      source.close()
      setConnected(false)
    }
    // topicsKey captures topics by value; onEvent is a ref.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- topicsKey is the intentional dep
  }, [active, topicsKey])

  return { connected: active ? connected : false, error: active ? error : null }
}

/** Build the absolute events URL — useful for tests and non-hook callers. */
export function eventsUrl(topics: string[]): string {
  const params = new URLSearchParams()
  for (const topic of topics) {
    params.append('topics', topic)
  }
  return `${apiUrl('/events')}?${params.toString()}`
}
