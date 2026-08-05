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

- [ ] Admin enables ATT&CK and starts sync; progress updates without full page refresh (SSE or
      documented fallback).
- [ ] Bundle upload of a fixture succeeds offline (e2e can mock API).
- [ ] Failed job surfaces error text from the API, not a generic toast only.
- [ ] Non-admin cannot reach the route via URL.
- [ ] Custom source cannot be deleted (UI hides or explains 409).
- [ ] License fields visible for each upstream seed source.

## Tests

- Component tests: action enablement while job running, error rendering, license display.
- E2E: admin opens sources, enables custom path with MSW/job fake, sees terminal success state.

## Notes for the implementer

- Reuse admin layout patterns from `M1-017`.
- Never put the bundle file into Redux/global state; upload directly via mutation.
- When SSE reconnects mid-job, reconcile from `GET /content/jobs/{id}` once.
