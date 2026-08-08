import { createContext, use } from 'react'

import type { ResolvedTheme, ThemePreference } from './theme'

export interface ThemeContextValue {
  /** What the user chose. `system` means "follow the OS". */
  preference: ThemePreference
  /** Which of the two themes that currently works out to. */
  theme: ResolvedTheme
  setPreference: (preference: ThemePreference) => void
}

/**
 * Separate from the provider component on purpose: a module that exports both a
 * component and a plain value defeats react-refresh, which then reloads the
 * whole page instead of the component (`react-refresh/only-export-components`).
 */
export const ThemeContext = createContext<ThemeContextValue | null>(null)

export function useTheme(): ThemeContextValue {
  const value = use(ThemeContext)
  if (value === null) {
    throw new Error('useTheme must be used inside a <ThemeProvider>')
  }
  return value
}
