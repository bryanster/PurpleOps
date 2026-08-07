# M4-005 — Live workbook updates + 409 conflict toast

**Milestone:** M4 · **Size:** M · **Depends on:** M4-003, M4-004, M3-014, M3-015

## Why

`PLAN.md` §9 step 4: blue’s browser reflects red’s progress without reload.
M3 UI is mutation-invalidation only; this ticket makes the shared board and
execution drawer track remote scorers and surfaces optimistic-lock collisions
honestly.

## Scope

**In**

- Workbook board + execution drawer react to engagement SSE via the M4-003 map:
  - Remote red status/notes → status chips and red fields update after refetch.
  - Remote blue detection/score → category/modifiers/outcome update.
  - Structure: scenario/step create, reorder, delete, reveal → board reshapes.
  - Evidence/comment/finding badges or counts refresh when those queries exist.
- **409 conflict UX** (local writer loses the race):
  - Toast explaining stale version.
  - Reload the execution (and related) row from server.
  - Leave the user to re-apply edits — **no** automatic field merge (`M4-EPIC`).
- Visual polish (light): brief highlight on rows that changed from SSE (optional
  but recommended); must not flash the entire board on every event.
- Ensure closed engagement still receives read-side live updates (comments
  allowed after close per M3-010) but write controls stay disabled.
- Playwright or component-level demo path: two users, red runs step, blue sees
  status without manual refresh (may share setup with `M4-009`).

**Out**

- Presence avatars (`M4-006`).
- Comment unread (`M4-007`).
- Activity rail (`M4-008`).
- Changing optimistic lock server semantics.

## Acceptance criteria

- [x] Two sessions on the same engagement: red → `running`/`complete` appears on
      blue's board without reload within a small bound (SSE + refetch).
- [x] Blue scores detection; red's open drawer shows new category after event.
- [x] Stale PATCH shows toast and refreshed version; successful retry works.
- [x] Reveal makes a new row appear for blue (with blind rules) without reload.
- [x] No full-app spinner on each event.

## Tests

- Component tests with injected events for chip/drawer updates: ✅ 3 tests added
  covering status change, detection change, and row flash animation.
- Handler-level 409 path already exists — FE test that mock 409 triggers toast +
  invalidate: ✅ 3 tests added covering red 409 reset, blue 409 reset, and
  retry-after-conflict flow.
- Optional Playwright smoke tagged for war-room: deferred to M4-009 (shared setup).

- Prefer invalidating the minimum keys; debounce burst events (e.g. rapid
  typing saves if any) if the UI janks — only if measured.
- Do not open a second EventSource beside the layout subscription.

## Implementation notes

### Changes

1. **Live drawer data (M4-005 core):** `WorkbookPage` now stores only `selectedStepId` and derives `selectedStep` / `selectedExecution` from live query data via `useMemo`. This means the open drawer automatically reflects remote SSE-triggered refetches without reload.

2. **409 conflict recovery:** `RedExecutionEditor` and `BlueDetectionEditor` now reset their local `useState` fields when `execution.version` changes (detected via `useEffect`). After a 409 + cache invalidation + refetch, the editor shows the server's version so the user can see what changed and re-apply edits.

3. **Row highlight animation:** Added `useFlashOnChange` hook (`web/src/lib/use-flash-on-change.ts`) and CSS `animate-flash-update` keyframe. `StepRow` applies the flash class briefly (1.2s) when execution version or step `updatedAt` changes referentially.

4. **Test infrastructure:** `renderWithProviders` now exposes `queryClient` so tests can trigger cache invalidations. Added 6 new tests covering SSE-like live updates, 409 editor reset, and retry-after-conflict flows.

### What was already in place

- SSE event → TanStack Query invalidation pipeline (M4-003, M4-004)
- 409 toast on both red/blue PATCH mutations (pre-existing)
- `useEngagementEvents` already wired in `EngagementLayout`

