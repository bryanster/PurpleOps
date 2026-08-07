import { useCallback, useRef } from 'react'

import { useQueryClient } from '@tanstack/react-query'

import { useEventSource, type ServerEvent } from '@/lib/use-event-source'

import {
  queryKeysForVerb,
  type EngagementEventPayload,
} from './event-invalidation'

/**
 * Subscribe to live engagement events for the duration an engagement layout
 * is mounted.  Parses the hub Event envelope, maps the verb to precise
 * TanStack Query key invalidations, and tears down on unmount.
 *
 * Unknown verbs produce a one-shot `console.debug` in dev and are otherwise
 * ignored.
 */
export function useEngagementEvents(engagementId: string | undefined): void {
  const qc = useQueryClient()
  const warnedRef = useRef(new Set<string>())

  const onEvent = useCallback(
    (event: ServerEvent) => {
      const payload = event.data as EngagementEventPayload | undefined
      if (!payload?.engagementId) return
      if (payload.engagementId !== engagementId) return

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
  })
}
