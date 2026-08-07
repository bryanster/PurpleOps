# M4-007 — Live comment threads + lightweight unread

**Milestone:** M4 · **Size:** M · **Depends on:** M4-005, M3-010

## Why

Observers’ main write is comment. Threads already exist in the execution drawer
(M3); M4 makes them feel live and marks what you have not opened yet — without
server-side read receipts (`M4-EPIC`).

## Scope

**In**

- Execution drawer comment thread:
  - Remote `comment.created` / `comment.edited` → invalidate/refetch thread;
    append/replace in view without closing the drawer.
  - Preserve composer draft text across refetch.
  - Show edited affordance when `edited_at` set; link/expand revisions if UI
    already has it (or minimal “edited” badge).
- **Lightweight unread** (client-only):
  - Keyed by `engagementId` + `executionId` (and user id if available) in
    `localStorage`.
  - Track `lastViewedAt` or last seen comment id when the user opens the thread.
  - Board/drawer entry: badge when newest comment is newer than last view.
  - Clear badge on open. No cross-device sync — document in UI copy if needed
    (“this browser”).
- Live updates respect blind conceal (no badges for unrevealed executions blue
  cannot see).
- Author attribution uses existing user display fields from API.

**Out**

- `@mentions`, reactions, delete, markdown preview overhaul.
- Server `last_read_at`.
- Engagement-wide unread inbox.

## Acceptance criteria

- [ ] User A comments; user B with drawer open on that execution sees the new
      comment without manual refresh.
- [ ] User B with drawer closed sees an unread badge; opening clears it for
      that browser.
- [ ] Edit by author updates body for peers via SSE invalidation.
- [ ] Observer can participate end-to-end (authz unchanged).

## Tests

- Component tests: event → thread refresh; localStorage unread clear on open.
- No server schema migration.

## Notes for the implementer

- Reuse M4-003 invalidation keys for comments.
- Avoid scrolling hijack on every remote comment if the user has scrolled up —
  stick-to-bottom only when already at bottom (nice-to-have, accept if easy).
