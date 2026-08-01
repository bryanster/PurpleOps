import { useCallback, useEffect, useState } from 'react'

/**
 * Minimal load-once-and-report-state hook.
 *
 * TEMPORARY: M0B-009 brings TanStack Query, which does this properly — caching,
 * retries, deduplication, invalidation. This exists only so the two demo
 * screens in M0B-008 can show a loading and an error state without pulling that
 * dependency forward, and it is deleted by that ticket.
 */
export type AsyncState<T> =
  { status: 'loading' } | { status: 'error'; error: Error } | { status: 'ready'; data: T }

export interface UseAsyncResult<T> {
  state: AsyncState<T>
  reload: () => void
}

/**
 * `load` must be a stable reference — a module-level function, or one wrapped
 * in useCallback. It is a dependency of the effect, so an inline arrow would
 * re-fetch on every render.
 */
export function useAsync<T>(load: (signal: AbortSignal) => Promise<T>): UseAsyncResult<T> {
  const [state, setState] = useState<AsyncState<T>>({ status: 'loading' })
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    const controller = new AbortController()

    load(controller.signal).then(
      (data) => {
        if (!controller.signal.aborted) {
          setState({ status: 'ready', data })
        }
      },
      (cause: unknown) => {
        // An abort is this effect being cleaned up — a navigation, or React 19
        // StrictMode running it twice in development. Not something to report.
        if (controller.signal.aborted) {
          return
        }
        setState({
          status: 'error',
          error: cause instanceof Error ? cause : new Error(String(cause)),
        })
      },
    )

    return () => {
      controller.abort()
    }
  }, [load, attempt])

  // The loading state is set here rather than at the top of the effect: a
  // synchronous setState inside an effect body is a cascading render, and
  // react-hooks/set-state-in-effect rejects it. An event handler is the right
  // place for it anyway, and the initial state is already `loading`.
  const reload = useCallback(() => {
    setState({ status: 'loading' })
    setAttempt((n) => n + 1)
  }, [])

  return { state, reload }
}
