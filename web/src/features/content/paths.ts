/**
 * Routes the content library lives at (M2-013).
 *
 * Kept as constants so the route table, the nav, and the empty-state CTA to the
 * sources admin all spell the same path. Sources admin itself lands in M2-014;
 * the path is fixed here so the library CTA does not have to guess later.
 */
export const CONTENT_PATH = '/content'

/** Admin control plane for sources (M2-014). Linked from the empty library CTA. */
export const CONTENT_SOURCES_PATH = '/admin/content/sources'
