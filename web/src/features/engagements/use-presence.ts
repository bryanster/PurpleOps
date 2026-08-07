import { useCallback, useEffect, useRef, useState } from 'react'

import { api } from '@/api/client'

const HEARTBEAT_INTERVAL = 15_000 // 15s (server TTL is 45s)

export interface PresenceFocus {
  stepId?: string
  executionId?: string
}

interface PresenceUser {
  userId: string
  displayName: string
  lastSeenAt: string
  tabCount: number
  focus?: PresenceFocus
}

export interface PresenceState {
  /** Every user currently present in this engagement. */
  users: PresenceUser[]
  /** Whether the heartbeat loop is active. */
  connected: boolean
  /** Set the user's current focus (step/execution). */
  setFocus: (focus: PresenceFocus) => void
}

/**
 * Manage presence heartbeat and snapshot for an engagement.
 *
 * On mount, generates a client presenceId, starts a heartbeat interval,
 * and begins tracking focus. On unmount, sends a DELETE via sendBeacon
 * and clears the interval.
 *
 * The server TTL (45s) is the source of truth for eviction; the
 * DELETE on unmount is best-effort.
 */
export function usePresence(
  engagementId: string | undefined,
  enabled: boolean,
): PresenceState {
  const [users, setUsers] = useState<PresenceUser[]>([])
  const [connected, setConnected] = useState(false)
  const presenceIdRef = useRef<string>(crypto.randomUUID())
  const focusRef = useRef<PresenceFocus>({})
  const intervalRef = useRef<ReturnType<typeof setInterval>>()
  const mountedRef = useRef(true)

  const heartbeat = useCallback(
    async (focus: PresenceFocus) => {
      if (!engagementId) return
      try {
        await api.PUT('/engagements/{engagementId}/presence', {
          params: {
            path: { engagementId },
          },
          body: {
            presenceId: presenceIdRef.current,
            ...(focus.stepId || focus.executionId
              ? {
                  focus: {
                    stepId: focus.stepId,
                    executionId: focus.executionId,
                  },
                }
              : {}),
          },
        })
      } catch {
        // Heartbeat is best-effort; TTL handles eviction.
      }
    },
    [engagementId],
  )

  const fetchSnapshot = useCallback(async () => {
    if (!engagementId) return
    try {
      const res = await api.GET(
        '/engagements/{engagementId}/presence',
        {
          params: { path: { engagementId } },
        },
      )
      if (res.data) {
        setUsers(res.data.entries ?? [])
      }
    } catch {
      // Snapshot is best-effort.
    }
  }, [engagementId])

  const setFocus = useCallback((focus: PresenceFocus) => {
    focusRef.current = focus
  }, [])

  useEffect(() => {
    if (!engagementId || !enabled) return

    mountedRef.current = true

    // Initial heartbeat + snapshot.
    heartbeat(focusRef.current)
    fetchSnapshot()
    setConnected(true)

    // Periodic heartbeat.
    intervalRef.current = setInterval(() => {
      if (!mountedRef.current) return
      heartbeat(focusRef.current)
    }, HEARTBEAT_INTERVAL)

    return () => {
      mountedRef.current = false
      setConnected(false)
      if (intervalRef.current) {
        clearInterval(intervalRef.current)
      }

      // Best-effort cleanup on unmount.
      const url = `${window.location.origin}/api/v1/engagements/${encodeURIComponent(engagementId)}/presence?presenceId=${encodeURIComponent(presenceIdRef.current)}`
      try {
        navigator.sendBeacon(url, '')
      } catch {
        // sendBeacon is best-effort.
      }
    }
  }, [engagementId, enabled, heartbeat, fetchSnapshot])

  return { users, connected, setFocus }
}
