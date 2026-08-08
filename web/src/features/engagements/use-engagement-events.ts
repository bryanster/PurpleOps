import { useCallback, useRef } from 'react'

import { useQueryClient } from '@tanstack/react-query'

import { useEventSource, type ServerEvent } from '@/lib/use-event-source'

import { engagementKeys } from './queries'
import { queryKeysForVerb, type EngagementEventPayload } from './event-invalidation'

const CURSOR_PREFIX = 'bl_event_cursor:'

function cursorKey(engagementId: string): string {
  return CURSOR_PREFIX + engagementId
}

function loadCursor(engagementId: string): string | undefined {
  try {
    return sessionStorage.getItem(cursorKey(engagementId)) ?? undefined
  } catch {
    return undefined
  }
}

function saveCursor(engagementId: string, eventId: string): void {
  try {
    sessionStorage.setItem(cursorKey(engagementId), eventId)
  } catch {
    // sessionStorage unavailable — best effort.
  }
}

/**
 * Subscribe to live engagement events for the duration an engagement layout
 * is mounted.  Parses the hub Event envelope, maps the verb to precise
 * TanStack Query key invalidations, and tears down on unmount.
 *
 * M4-004: persists the last event id per engagement in sessionStorage so
 * reconnects and page reloads get catch-up replay.  `stream.gap` and
 * `sync.required` events trigger broad engagement invalidation.
 *
 * Unknown verbs produce a one-shot `console.debug` in dev and are otherwise
 * ignored.
 */
export function useEngagementEvents(engagementId: string | undefined): void {
  const qc = useQueryClient()
  const warnedRef = useRef(new Set<string>())
  const initialCursor = engagementId ? loadCursor(engagementId) : undefined

  const onEvent = useCallback(
    (event: ServerEvent) => {
      // Handle synthetic gap events (M4-004).
      if (event.type === 'stream.gap' || event.type === 'sync.required') {
        const gapPayload = event.data as { engagementId?: string } | undefined
        const eid = gapPayload?.engagementId ?? engagementId
        if (eid) {
          void qc.invalidateQueries({ queryKey: engagementKeys.all, exact: false })
        }
        return
      }

      const payload = event.data as EngagementEventPayload | undefined
      if (!payload?.engagementId) return
      if (payload.engagementId !== engagementId) return

      // Persist cursor for catch-up on reconnect/reload.
      if (event.id && engagementId) {
        saveCursor(engagementId, event.id)
      }

      const parents = {
        executionId: payload.executionId,
        scenarioId: payload.scenarioId,
        stepId: payload.stepId,
      }

      const keys = queryKeysForVerb(event.type, payload.engagementId, parents)

      if (keys.length === 0) {
        if (import.meta.env.DEV && !warnedRef.current.has(event.type)) {
          warnedRef.current.add(event.type)
          console.debug(
            `[useEngagementEvents] unknown verb "${event.type}" — ignored (add to event-invalidation.ts if needed)`,
          )
        }
        return
      }

      for (const key of keys) {
        void qc.invalidateQueries({ queryKey: key })
      }
    },
    [qc, engagementId],
  )

  useEventSource({
    topics: engagementId ? [`engagement.${engagementId}`] : [],
    onEvent,
    enabled: engagementId !== undefined,
    initialLastEventId: initialCursor,
  })
}
