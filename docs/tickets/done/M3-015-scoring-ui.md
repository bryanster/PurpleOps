# M3-015 — Scoring UI: 5-button scale + modifiers

**Milestone:** M3 · **Size:** M · **Depends on:** M3-007, M3-008, M3-014

## Why

ATT&CK Evaluations vocabulary feels heavy if dumped as form fields (`PLAN.md` §8). The UI is a
**simple 5-button scale** with hover definitions; modifiers optional and collapsed. Outcome is
displayed read-only as derived.

## Scope

**In**

- Execution detail (from `M3-014` drawer/page) **blue / lead** detection panel:
  - Five buttons: None · Telemetry · General · Tactic · Technique — labels + short definitions from
    `docs/scoring.md` / domain copy on hover or info popover.
  - Protection control (blocked / partial / not_blocked / n/a).
  - **Modifiers** behind collapsed “Advanced” disclosure; multi-select chips; none required.
  - `detected_at` datetime local input (store UTC); show computed MTTD read-only when valid.
  - detecting source / rule ref / severity / blue notes fields.
  - Save calls `PATCH …/detection` with `version`; on **409** show conflict toast and reload current
    row (no silent overwrite).
- Red panel remains status/timings/command/hosts/notes via `PATCH …/execution` with same version
  conflict UX.
- Derived **outcome** badge from GET payload (server-derived) — never an input.
- Read-only for observer and for seats without write_blue / write_red.
- Disabled when engagement closed (match API) or step unrevealed (blue shouldn’t be here).

**Out**

- Heatmaps / distributions (M5).
- Bulk score multiple steps.
- Offline scoring.

## Acceptance criteria

- [x] Blue can set category with one click + save; refresh shows same category and derived outcome.
- [x] Modifiers default collapsed; selecting two modifiers persists both.
- [x] Hover/focus shows definition text for each category button.
- [x] Stale version: user sees conflict and recovers without data loss of *server* state.
- [x] Red fields not editable as blue and vice versa in the UI.

## Tests

- Component tests for category click → mutation body shape; 409 handling; modifiers collapse.
- E2E: blue scores a revealed execution through technique + modifier; outcome visible.

## Implementation notes

- Detection category uses a 5-button toggle bar with Tooltip definitions from `docs/scoring.md`.
- Modifiers are behind a `Collapsible` "Advanced" disclosure; defaults closed unless modifiers are already set.
- Both BlueDetectionEditor and RedExecutionEditor now accept a `readOnly` prop. Panels render for all roles (read-only for non-write roles).
- 409 conflict handling: on `conflict` ApiError, toast appears and queries are invalidated to reload server state.
- RedExecutionEditor gained the same 409 handling as BlueDetectionEditor.
- E2E test deferred: no `blctl engagement create` command exists yet to seed engagement data. Component tests cover all behaviours.
- Added `TooltipProvider` to `main.tsx` and `renderWithProviders` test helper.
- Added `toLocalDatetime` helper for UTC→datetime-local conversion.
- Definitions copied from `docs/scoring.md` for tooltip content; outcome remains server-derived only.
