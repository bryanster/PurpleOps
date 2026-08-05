/**
 * Where a completed sign-in lands, and the check that keeps that from being
 * anywhere on the internet.
 *
 * This is the client's half of `internal/authn/returnto`, and it applies the
 * same rule: an allowlist of one shape, a relative path within this
 * application. The server has its own copy because the single sign-on
 * endpoints take `return_to` as a query parameter and must not trust it
 * (M1-009); this copy exists because the local login never reaches that code —
 * the redirect after a password sign-in happens entirely in the browser, so a
 * check that lived only on the server would not run at all.
 *
 * The failure this prevents is specific. A login screen that redirects wherever
 * its query string says is a phishing page hosted on your own domain: the URL
 * bar says blacklight, the user signs in, and the next thing they see belongs
 * to somebody else.
 */

/** The maximum a path may be, matching `returnto.maxBytes` on the server. */
const MAX_LENGTH = 512

/** The query parameter the login route reads, and the server's spelling of it. */
export const RETURN_TO_PARAM = 'return_to'

/** Where somebody who asked for nothing in particular ends up. */
export const DEFAULT_LANDING = '/'

/**
 * The path to land on after signing in, or [DEFAULT_LANDING] when the value is
 * absent or is not a path within this application.
 *
 * Refused, in the order they are checked: anything over the length limit;
 * anything holding a backslash or a control character, because a browser
 * normalises `\` to `/` *after* a naive check has approved it; anything that
 * does not begin with exactly one `/`, which is what makes `//evil.example` a
 * URL to another origin that reads like a path; and anything that parses with a
 * scheme, a host or credentials.
 *
 * A bad value falls back rather than throwing. The server refuses one outright
 * because somebody constructed the request; here the value has usually come
 * from a stale bookmark, and the useful answer is to sign them in and send them
 * somewhere sane.
 */
export function safeReturnTo(raw: string | null | undefined): string {
  if (raw === null || raw === undefined || raw === '') {
    return DEFAULT_LANDING
  }
  if (raw.length > MAX_LENGTH) {
    return DEFAULT_LANDING
  }
  // eslint-disable-next-line no-control-regex -- the control characters are the point: they are how a redirect is smuggled past a check that only looks at the start of the string.
  if (/[\\\u0000-\u001f\u007f]/.test(raw)) {
    return DEFAULT_LANDING
  }
  if (!raw.startsWith('/') || raw.startsWith('//')) {
    return DEFAULT_LANDING
  }

  // Parsed against a base that is not this origin, so that anything which
  // resolves away from that base — which is everything with a scheme or a host,
  // however it is spelled — is visible as a changed origin rather than having to
  // be pattern-matched for.
  let parsed: URL
  try {
    parsed = new URL(raw, 'https://return-to.invalid')
  } catch {
    return DEFAULT_LANDING
  }
  if (parsed.origin !== 'https://return-to.invalid') {
    return DEFAULT_LANDING
  }

  // Re-rendered from the parsed form rather than passed through, so what the
  // router is handed is what was actually parsed and approved.
  return `${parsed.pathname}${parsed.search}${parsed.hash}`
}

/**
 * The `return_to` for a screen the user was thrown off, as a relative path.
 *
 * Built from the router's own location rather than from `window.location`, so
 * it is the in-app path and never carries the origin — a redirect target that
 * arrives already absolute is one [safeReturnTo] would refuse on the way back
 * in, which would be a bug that only shows up in deployment.
 */
export function returnToFor(location: { pathname: string; search: string; hash: string }): string {
  return `${location.pathname}${location.search}${location.hash}`
}

/** The login URL that comes back to `path` afterwards. */
export function loginUrlFor(path: string): string {
  if (path === DEFAULT_LANDING) {
    return '/login'
  }
  return `/login?${RETURN_TO_PARAM}=${encodeURIComponent(path)}`
}
