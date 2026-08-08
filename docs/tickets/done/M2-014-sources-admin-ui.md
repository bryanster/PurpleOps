# M2-014 — Sources admin UI: sync, bundle, status, reprocess

**Milestone:** M2 · **Size:** L · **Depends on:** M2-002, M2-003, M2-004, M2-005, M0B-009

## Why

Installable content fails as a product if admins cannot see status, errors, and progress. This is
the control plane for M2: enable, sync, upload bundle, reprocess, disable, read licenses.

## Scope

**In**

- Admin route `/admin/content/sources` (or `/content/sources` admin-gated):
  - Table/cards of sources: kind, enabled, status, last synced, item count, error summary.
  - Actions per source: Enable/Disable, Sync, Upload bundle, Reprocess, Edit URL/ref (advanced),
    Delete (confirm + consequence text).
  - ATT&CK: version list with per-version sync/reprocess/delete where API allows.
  - Job progress panel driven by SSE (`M2-004`) with polling fallback if EventSource fails.
  - Terminal failure shows full error message + request/job id.
- License block on source detail: SPDX, name, link, attribution text.
- Global "a job is running" indicator; Sync buttons disabled with explanation while slot held.
- Confirm dialogs for disable/delete/reprocess.
- Route guard: non-admin → 403 page; nav entry admin-only.
- Real loading/empty/error states; dark/light; 1280/768.

**Out**

- Library browser (`M2-013`), custom editor (`M2-015`).
- Scheduled sync UI.

## Acceptance criteria

- [x] Admin enables ATT&CK and starts sync; progress updates without full page refresh (SSE or
      documented fallback).
- [x] Bundle upload of a fixture succeeds offline (e2e can mock API).
- [x] Failed job surfaces error text from the API, not a generic toast only.
- [x] Non-admin cannot reach the route via URL.
- [x] Custom source cannot be deleted (UI hides or explains 409).
- [x] License fields visible for each upstream seed source.

## Tests

- Component tests: action enablement while job running, error rendering, license display.
- E2E: admin opens sources, enables custom path with MSW/job fake, sees terminal success state.

## Notes for the implementer

- Reuse admin layout patterns from `M1-017`.
- Never put the bundle file into Redux/global state; upload directly via mutation.
- When SSE reconnects mid-job, reconcile from `GET /content/jobs/{id}` once.

## Implementation notes

- **UI:** `web/src/features/content/sources-page.tsx` at `/admin/content/sources` behind
  `RequireAdmin`. Nav "Content sources" under Administration (`adminOnly`). Table lists every seed
  source with enable/disable, sync, upload bundle, reprocess, delete. Custom hides Delete entirely.
  Detail dialog: license block (SPDX/name/link/attribution), last job, advanced URL/ref edit,
  installed versions with per-version sync/reprocess and ATT&CK version delete.
- **Jobs:** `job-progress.tsx` subscribes to SSE `content.jobs` via `useEventSource`. REST
  `GET /content/jobs` is the durable source of truth for the global slot; SSE refines phase/counters.
  On reconnect after a drop, invalidates the watched job once (no Last-Event-ID replay in M2).
  When EventSource is missing or in error, jobs query `refetchInterval` (2s while active) is the
  fallback and the panel says so. Slot held → Sync/Upload/Reprocess disabled with reason text.
- **Queries:** `sources-queries.ts` — list/detail/versions/jobs + mutations. Bundle upload builds
  `FormData` at submit time only (never cached). 204 DELETEs match the auth/token pattern (no
  `unwrap`).
- **Tests:** `sources-page.test.tsx` (license detail, custom no-delete, slot-held disablement, API
  error text, enable mutation, list error request id, multipart bundle upload). E2E
  `e2e/specs/content-sources.spec.ts` seeds ATT&CK/Atomic via `blctl content import-bundle`, then
  admin verifies licenses/versions/custom delete hide + live disable/enable; member hits Forbidden
  page. Shell keyboard-tab count bumped for the new nav link.
- **Verified:** `make lint test build`; web vitest 107+; e2e content-sources 2 passed.
