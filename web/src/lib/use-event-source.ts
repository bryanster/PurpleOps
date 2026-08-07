import { useEffect, useRef, useState } from 'react'

import { apiUrl } from '@/api/client'

/**
 * Shape of the hub Event envelope (`events.Event` in Go).
 *
 * The SSE `data:` field carries the full envelope.  This interface describes the
 * parsed form when it looks like a hub Event (has top-level `id`, `type`,
 * `data`).  When the payload does not match this shape the raw parsed JSON
 * is passed through as `data` and `id`/`type` come from the transport layer.
 */
interface HubEnvelope {
  id: string
  topic: string
  type: string
  at: string
  data: unknown
}

/** @internal — exported for tests */
export function isHubEnvelope(v: unknown): v is HubEnvelope {
  if (typeof v !== 'object' || v === null) return false
  const o = v as Record<string, unknown>
  return typeof o.id === 'string' && typeof o.type === 'string' && 'data' in o
}

/**
 * One parsed SSE frame from `GET /api/v1/events`.
 *
 * When the `data:` payload is a hub Event envelope, `data` is the **inner**
 * payload (`envelope.data`) and `id`/`type` come from the envelope.
 * Otherwise the raw parsed JSON is passed as `data`.
 */
export interface ServerEvent {
  /** Envelope `id` (activity row UUID), or SSE `lastEventId`, or undefined. */
  id?: string
  /** Envelope `type` (verb), or the SSE event type, or 'message'. */
  type: string
  /** Inner payload from the envelope, or raw parsed JSON. */
  data: unknown
}

export interface UseEventSourceOptions {
  /** Topic names (`content.jobs`, `content.jobs.{id}`, `engagement.{id}`, …). Empty disables. */
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
  /** The last event id received, for `Last-Event-ID` catch-up (M4-004). */
  lastEventId: string | undefined
}

/**
 * Subscribe to the shared SSE hub (`M2-004`, extended in M4-001).
 *
 * Session cookie only — EventSource cannot set Authorization, which matches
 * the server's session-only gate. On drop the browser reconnects with its
 * built-in backoff.
 *
 * The hook parses the hub Event envelope: when the `data:` payload has
 * top-level `id`, `type` and `data`, those are unwrapped so `event.type`
 * comes from the envelope and `event.data` is the inner payload.  This is
 * the stable contract for both content job ticks and engagement events (M4).
 *
 * `lastEventId` tracks the most recent envelope `id` for M4-004 replay.
 *
 * Prefer this over hand-rolled `EventSource` so all M2/M4 consumers share
 * one reconnect and teardown path.
 */
export function useEventSource(options: UseEventSourceOptions): UseEventSourceState {
  const { topics, onEvent, enabled = true } = options
  const onEventRef = useRef(onEvent)
  useEffect(() => {
    onEventRef.current = onEvent
  }, [onEvent])

  const [connected, setConnected] = useState(false)
  const [error, setError] = useState<Event | null>(null)
  const lastEventIdRef = useRef<string | undefined>(undefined)

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

    // All events are unnamed (no `event:` field since M4-003).  The `type` is
    // in the hub Event envelope, which we parse here.
    const handleMessage = (ev: MessageEvent) => {
      const raw = typeof ev.data === 'string' ? ev.data : String(ev.data)
      let parsed: unknown = raw
      try {
        parsed = JSON.parse(raw) as unknown
      } catch {
        // Leave the raw string; the server always sends JSON, but a proxy
        // inserting noise should not crash the hook.
      }

      if (isHubEnvelope(parsed)) {
        lastEventIdRef.current = parsed.id
        onEventRef.current?.({
          id: parsed.id,
          type: parsed.type,
          data: parsed.data,
        })
      } else {
        onEventRef.current?.({
          id: (ev as MessageEvent).lastEventId || undefined,
          type: (ev as MessageEvent).type || 'message',
          data: parsed,
        })
      }
    }

    source.onmessage = handleMessage

    return () => {
      source.close()
      setConnected(false)
    }
    // topicsKey captures topics by value; onEvent is a ref.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- topicsKey is the intentional dep
  }, [active, topicsKey])

  return {
    connected: active ? connected : false,
    error: active ? error : null,
    lastEventId: lastEventIdRef.current,
  }
}

/** Build the absolute events URL — useful for tests and non-hook callers. */
export function eventsUrl(topics: string[]): string {
  const params = new URLSearchParams()
  for (const topic of topics) {
    params.append('topics', topic)
  }
  return `${apiUrl('/events')}?${params.toString()}`
}
