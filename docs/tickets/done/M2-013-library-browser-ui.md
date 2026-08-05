# M2-013 — Content library browser UI

**Milestone:** M2 · **Size:** L · **Depends on:** M2-006, M2-007, M2-008, M2-009, M2-010, M2-011, M0B-009

## Why

Synced content is useless if people cannot find a technique or procedure when building work. This is
the read-side surface for the whole milestone: search, filter, detail, empty states.

## Scope

**In**

- Routes under something like `/content` (member-accessible):
  - **Techniques** — version selector (installed ATT&CK versions), filters (tactic, subtechnique),
    substring search on id/name/description, detail drawer/page with description, tactics,
    related mitigations/software/groups as available.
  - **Procedures** — filters technique/platform/source; detail shows structured command, cleanup,
    args (not a single blob).
  - **Detection rules** — filters technique/level; detail shows rule body in a read-only viewer with
    copy; badge/text **"Reference only — not deployed by Blacklight"**.
  - **Emulation plans** — list + detail with ordered steps.
  - **Notes** — list/search KB notes.
- Empty installation state: no ATT&CK synced → clear empty state **"Ask an admin to install ATT&CK"**
  (link to sources admin only if caller is admin; otherwise plain message).
- Disabled sources' objects do not appear.
- Every list: loading, empty, error states with request id (`M0B-007`).
- Hooks only from generated client (`M0B-009`); no raw URL strings.
- Keyboard navigable; works light/dark at 1280 and 768.

**Out**

- Sources admin (`M2-014`), custom editors (`M2-015`).
- Adding a technique to an engagement (M3) — detail may show a disabled "Use in scenario" placeholder
  only if it does not imply the API exists.

## Acceptance criteria

- [x] With fixtures loaded in e2e/dev seed, user can search `T1059` and open the technique detail for
      the selected version.
- [x] Switching ATT&CK version changes the technique detail identity (no stale cache across versions).
- [x] Atomic procedure detail shows separate command and cleanup sections when both exist.
- [x] Sigma detail shows reference-only messaging.
- [x] Non-admin never sees enable/sync controls on this surface.
- [x] Empty library CTA copy matches product decision (admin vs member).

## Tests

- Component tests with MSW for search/filter/empty/error.
- E2E slice: admin has synced fixture content (harness seed); member browses technique + procedure.

## Notes for the implementer

- TanStack Query keys must include version and filters.
- Prefer shared filter chrome across tabs for consistency.
- Do not prefetch entire ATT&CK into memory — paginate/cursor as API provides.

## Implementation notes

- **UI:** `web/src/features/content/` — `LibraryPage` at `/content` with tabs (techniques, procedures,
  detection rules, emulation plans, notes). Shared filter chrome + dialog detail drawers. Nav
  "Content" is live (no longer pending M2). Empty ATT&CK: member copy "Ask an admin to install
  ATT&CK."; admin gets link to `/admin/content/sources` (M2-014 route placeholder).
- **Queries:** TanStack keys include version and filters; detail queries keyed by id. Technique list
  is gated on a browsable ATT&CK version (`sourceEnabled && ready && itemCount > 0`). Version pin is
  derived (override + latest) — no effect-driven setState. Switching version clears open detail.
- **API surface used as-is:** list/get for techniques, tactics, procedure-templates, detection-rules,
  emulation-plans, custom notes, attack versions. Disabled sources already filtered server-side.
  Technique detail exposes tactics + mitigations external ids (software/groups not on the detail
  contract). Content lists are limit-capped (no cursor); UI requests `limit=200`.
- **Tests:** `library-page.test.tsx` (MSW empty/admin/member, search, version switch, procedure
  sections, reference-only, error request id). E2E `e2e/specs/content-library.spec.ts` seeds via
  `blctl content enable` + `import-bundle` of adapter mini fixtures, then member browses technique +
  procedure. Harness now sets `BLACKLIGHT_CONTENT_DIR` per server.
- **Incidental fixes (required for seed path):**
  1. `config.parseTool` dropped `Content` when building `Tool` — blctl content never saw
     `CONTENT_DIR` / max-bytes (always zero). Now copies `cfg.Content`.
  2. `store/content.NewPaths` absolutizes the root so relative `./content` spool paths satisfy
     `requirePathUnderRoot`.
