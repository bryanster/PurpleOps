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

- [x] User A comments; user B with drawer open on that execution sees the new
      comment without manual refresh.
- [x] User B with drawer closed sees an unread badge; opening clears it for
      that browser.
- [x] Edit by author updates body for peers via SSE invalidation.
- [x] Observer can participate end-to-end (authz unchanged).

## Tests

- Component tests: event → thread refresh; localStorage unread clear on open.
- No server schema migration.

## Notes for the implementer

- Reuse M4-003 invalidation keys for comments.
- Avoid scrolling hijack on every remote comment if the user has scrolled up —
  stick-to-bottom only when already at bottom (nice-to-have, accept if easy).

## Implementation notes

- **`usePatchComment` hook** added to `queries.ts` — wraps `PATCH /engagements/{engagementId}/comments/{commentId}`. On success, invalidates `engagementKeys.executions` which cascades to comment lists via the SSE-driven invalidation pipeline.
- **`useCommentUnread` hook** (`use-comment-unread.ts`) — client-only localStorage unread tracking keyed by `engagementId + executionId`. Uses `useSyncExternalStore` with string snapshots (not objects) to avoid the infinite re-render loop from JSON.parse creating new references. Cross-tab sync via `storage` event listener.
- **`CommentsSection`** rewritten:
  - Author display names resolved via `useEngagementMembers` lookup map (`userId → displayName`).
  - Inline edit mode: author/lead/admin see an edit icon; click opens textarea with save/cancel.
  - Auto-scroll: `useRef` tracks whether user is at bottom; new comments only auto-scroll when already at bottom.
  - Composer draft preserved across refetches (local `useState` survives cache invalidation).
- **`UnreadCommentBadge`** — inline component in workbook-page that loads comments per-execution (TanStack Query deduplicates) and renders a circular count badge when `hasUnread` is true. Respects blind conceal: only renders for revealed executions.
- **`markCommentRead`** called imperatively in `openExecution` callback when drawer opens.
- **`StepRow`** now accepts `engagementId` prop; `ScenarioSection` forwards it.
- Added `formatMoment` import (was missing — pre-existing bug).
- 6 component tests: SSE invalidation → thread refresh (created + edited), unread badge shows and clears, edit button visibility, edit-save flow.
