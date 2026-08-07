import { useEffect, useRef, useState } from 'react'

/**
 * Returns `true` for a brief period (default 1200 ms) whenever `value`
 * changes referentially.  Useful for triggering a CSS transition that
 * highlights a row after a remote data update (M4-005).
 *
 * The flash is suppressed on the initial render — only subsequent
 * changes trigger it.
 */
export function useFlashOnChange(value: unknown, durationMs = 1200): boolean {
  const [flashing, setFlashing] = useState(false)
  const mounted = useRef(false)
  const timer = useRef<ReturnType<typeof setTimeout>>()

  useEffect(() => {
    if (!mounted.current) {
      mounted.current = true
      return
    }
    setFlashing(true)
    timer.current = setTimeout(() => {
      setFlashing(false)
    }, durationMs)
    return () => {
      clearTimeout(timer.current)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value])

  return flashing
}
