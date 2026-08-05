/**
 * A timestamp, in the reader's own locale and time zone.
 *
 * The server sends RFC 3339 in UTC and never formats a time for display — that
 * is a convention of this codebase (`docs/api.md`), and this is the other end
 * of it. A fixed format here would show a London afternoon to somebody in
 * Sydney.
 *
 * A value that is not a date is returned unchanged rather than rendered as
 * "Invalid Date": the raw string is at least something a reader can quote.
 */
export function formatMoment(iso: string): string {
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) {
    return iso
  }
  return at.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' })
}
