import { useCallback, useSyncExternalStore } from 'react'

const PREFIX = 'bl_comment_unread:'

function storageKey(engagementId: string, executionId: string): string {
  return PREFIX + engagementId + ':' + executionId
}

/**
 * Client-only unread tracking via localStorage (M4-007).
 *
 * The `lastViewedAt` timestamp is written when the user opens an execution
 * thread. A step row shows an unread badge when the newest comment on that
 * execution is newer than `lastViewedAt`.
 *
 * No cross-device sync — "this browser" by design per M4-EPIC.
 */

function readTimestamp(key: string): string | null {
  try {
    const raw = localStorage.getItem(key)
    if (!raw) return null
    const parsed = JSON.parse(raw) as { lastViewedAt: string }
    return parsed.lastViewedAt
  } catch {
    return null
  }
}

function writeTimestamp(key: string, at: string): void {
  try {
    localStorage.setItem(key, JSON.stringify({ lastViewedAt: at }))
  } catch {
    // localStorage full or unavailable — best effort.
  }
}

const subscribers = new Set<() => void>()

function notify(): void {
  for (const cb of subscribers) cb()
}

// Listen for cross-tab localStorage changes so all tabs stay in sync.
if (typeof window !== 'undefined') {
  window.addEventListener('storage', (e) => {
    if (e.key?.startsWith(PREFIX)) notify()
  })
}

function subscribe(cb: () => void): () => void {
  subscribers.add(cb)
  return () => {
    subscribers.delete(cb)
  }
}

/**
 * Returns whether an execution has unread comments, and a function to mark
 * it as read.
 *
 * @param engagementId  The engagement id.
 * @param executionId   The execution id (null = no execution yet).
 * @param newestCommentAt ISO timestamp of the newest comment on this execution (null = no comments).
 */
export function useCommentUnread(
  engagementId: string,
  executionId: string | null,
  newestCommentAt: string | null,
): { hasUnread: boolean; markRead: () => void } {
  const key = executionId ? storageKey(engagementId, executionId) : null

  // useSyncExternalStore snapshot returns a string (stable by value) to
  // avoid the infinite re-render loop that object references would cause.
  const lastViewedAt = useSyncExternalStore(subscribe, () => (key ? readTimestamp(key) : null))

  const markRead = useCallback(() => {
    if (!key) return
    writeTimestamp(key, new Date().toISOString())
    notify()
  }, [key])

  if (!executionId || !newestCommentAt) {
    return { hasUnread: false, markRead }
  }

  const hasUnread = !lastViewedAt || newestCommentAt > lastViewedAt

  return { hasUnread, markRead }
}

/**
 * Imperative mark-read for use in callbacks that don't have access to
 * the hook (e.g. event handlers passed from a parent).
 */
export function markCommentRead(engagementId: string, executionId: string): void {
  const key = storageKey(engagementId, executionId)
  writeTimestamp(key, new Date().toISOString())
  notify()
}
