/**
 * Where the first-run wizard lives.
 *
 * A constant because three things have to agree on it — the route table, the
 * guard that redirects an administrator into it, and the guard that redirects
 * back out once setup is done — and a fourth spelling would be a redirect loop
 * rather than a 404.
 */
export const SETUP_PATH = '/setup'
