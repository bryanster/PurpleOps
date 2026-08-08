/*
 * Applies the saved theme before the browser paints anything.
 *
 * This is a separate file rather than an inline <script> because the server
 * sends `script-src 'self'` (internal/httpapi/headers.go) — an inline script
 * would be blocked in production and every dark-mode user would get a white
 * flash on every load. It lives in public/ so Vite copies it through
 * unbundled and unhashed, which is what lets index.html reference it by a
 * fixed path and load it synchronously ahead of the module bundle.
 *
 * It duplicates a few lines of src/app/theme/theme.ts, which is the price of
 * running before that module exists. The two are kept honest by
 * src/app/theme/theme-bootstrap.test.ts, which fails if the storage key or the
 * class they agree on drifts apart.
 */
;(function applyStoredTheme() {
  var preference = 'system'
  try {
    var stored = window.localStorage.getItem('blacklight.theme')
    if (stored === 'light' || stored === 'dark' || stored === 'system') {
      preference = stored
    }
  } catch {
    // Site data blocked. Fall through to the OS preference.
  }

  var dark =
    preference === 'dark' ||
    (preference === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)

  document.documentElement.classList.toggle('dark', dark)
  document.documentElement.style.colorScheme = dark ? 'dark' : 'light'
})()
