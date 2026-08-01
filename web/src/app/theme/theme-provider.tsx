import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react'

import { ThemeContext, type ThemeContextValue } from './theme-context'
import {
  applyTheme,
  readStoredPreference,
  resolveTheme,
  storePreference,
  watchSystemTheme,
  type ResolvedTheme,
  type ThemePreference,
} from './theme'

export function ThemeProvider({ children }: { children: ReactNode }): ReactNode {
  const [preference, setPreferenceState] = useState<ThemePreference>(readStoredPreference)
  const [theme, setTheme] = useState<ResolvedTheme>(() => resolveTheme(preference))

  // main.tsx has already applied the theme once before the first render, so on
  // load this is a no-op; it exists for every change after that.
  useEffect(() => {
    applyTheme(theme)
  }, [theme])

  // Only while the preference is `system`: someone who has explicitly chosen
  // light has chosen it for the whole session, including across the sunset that
  // flips their OS to dark.
  useEffect(() => {
    if (preference !== 'system') {
      return
    }
    return watchSystemTheme(setTheme)
  }, [preference])

  const setPreference = useCallback((next: ThemePreference) => {
    setPreferenceState(next)
    storePreference(next)
    setTheme(resolveTheme(next))
  }, [])

  const value = useMemo<ThemeContextValue>(
    () => ({ preference, theme, setPreference }),
    [preference, theme, setPreference],
  )

  return <ThemeContext value={value}>{children}</ThemeContext>
}
