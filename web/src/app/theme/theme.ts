/**
 * Theme resolution, with no React in it so it can be unit-tested and can run
 * before the first render.
 *
 * The model has two levels. The *preference* is what the user chose — light,
 * dark, or "whatever the OS says" — and is persisted. The *resolved* theme is
 * the one of the two actual themes that follows from it, and is what gets
 * stamped onto <html>.
 *
 * The resolved theme is expressed as a `dark` class on <html>, which is the
 * convention src/index.css inherits from shadcn/ui: the light palette is the
 * `:root` default and `.dark` overrides it. Nothing reads
 * prefers-color-scheme in CSS, so the class is the single switch — which is
 * why public/theme-bootstrap.js has to set it before the first paint.
 */

export type ThemePreference = 'light' | 'dark' | 'system'
export type ResolvedTheme = 'light' | 'dark'

/** localStorage key. Namespaced: a deployment may share an origin with nothing else today, but that is not a guarantee worth relying on. */
export const THEME_STORAGE_KEY = 'purpleops.theme'

/** The class shadcn/ui's palette switches on. Also hard-coded in public/theme-bootstrap.js. */
export const DARK_CLASS = 'dark'

/** Also hard-coded in public/theme-bootstrap.js. */
export const DARK_QUERY = '(prefers-color-scheme: dark)'

export function isThemePreference(value: unknown): value is ThemePreference {
  return value === 'light' || value === 'dark' || value === 'system'
}

/**
 * The user's stored preference, or `system` when there is none.
 *
 * Storage access is guarded rather than assumed: reading localStorage throws in
 * a browser configured to block site data, and a theme is not worth a blank
 * page over.
 */
export function readStoredPreference(
  storage: Storage | undefined = safeStorage(),
): ThemePreference {
  try {
    const stored = storage?.getItem(THEME_STORAGE_KEY)
    return isThemePreference(stored) ? stored : 'system'
  } catch {
    return 'system'
  }
}

export function storePreference(
  preference: ThemePreference,
  storage: Storage | undefined = safeStorage(),
): void {
  try {
    storage?.setItem(THEME_STORAGE_KEY, preference)
  } catch {
    // Preference lost on reload. Everything else still works.
  }
}

/** What the operating system is asking for right now. */
export function systemTheme(): ResolvedTheme {
  return globalThis.matchMedia(DARK_QUERY).matches ? 'dark' : 'light'
}

export function resolveTheme(preference: ThemePreference): ResolvedTheme {
  return preference === 'system' ? systemTheme() : preference
}

/**
 * Put the resolved theme on <html>.
 *
 * `color-scheme` is set as well as the class: it is what makes the browser's
 * own chrome — scrollbars, form controls, the canvas behind the page — follow
 * the theme. Without it a dark page keeps white scrollbars.
 */
export function applyTheme(
  theme: ResolvedTheme,
  root: HTMLElement = document.documentElement,
): void {
  root.classList.toggle(DARK_CLASS, theme === 'dark')
  root.style.colorScheme = theme
}

/** Subscribe to OS theme changes. Returns an unsubscribe function. */
export function watchSystemTheme(onChange: (theme: ResolvedTheme) => void): () => void {
  const query = globalThis.matchMedia(DARK_QUERY)
  const listener = (event: MediaQueryListEvent): void => {
    onChange(event.matches ? 'dark' : 'light')
  }
  query.addEventListener('change', listener)
  return () => {
    query.removeEventListener('change', listener)
  }
}

function safeStorage(): Storage | undefined {
  try {
    return globalThis.localStorage
  } catch {
    return undefined
  }
}
