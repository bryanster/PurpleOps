# M5-013 — Dashboard UI: heatmap and scorecards

**Milestone:** M5 · **Size:** L · **Depends on:** M5-009, M3-014

## Why

Where the numbers become a thing somebody looks at. v1's hexagon coverage graphic was decorative and
never named its denominator; this replaces it with an ATT&CK heatmap and a row of scorecards that say
what they are counting.

It is also where the epic's seat-scoping decision either works or starts an argument: in a blind
engagement red and blue see different totals, correctly, and if the page does not say so the first
war room to notice will report it as a data bug.

## Scope

**In**

- New **Analytics** tab in `engagement-layout.tsx`, between Workbook and Findings, with
  `engagementAnalyticsPath` added to `web/src/features/engagements/paths.ts` alongside the others.
- `analytics-page.tsx` under `web/src/features/engagements/`, with queries in an
  `analytics-queries.ts` following the `activity-queries.ts` pattern (generated client, TanStack
  Query keys consistent with `event-invalidation.ts`).
- **Scorecards** across the top, each stating its denominator in the card, not in a tooltip:
  - Coverage — attempted techniques, with matrix coverage as the secondary line.
  - Detection — category distribution as a stacked bar, `unscored` visibly distinct from `none`.
  - Protection rate.
  - MTTD — p50 with p90 and max beneath, and **the undetected count adjacent**, never in a hover.
  - Findings — open vs closed, linking to the Findings tab.
- **ATT&CK heatmap**: tactics as columns, techniques as cells, coloured by detection-category
  ordinal using the **same ramp as the Navigator layer** (`docs/analytics.md`, `M5-010`). Sub-techniques
  nest under their parent and colour independently. Not-attempted and unscored are visually distinct
  from `none` — three different facts, three different treatments.
- **Burndown** line chart with the interval the API reported, labelled.
- **Blind label.** When `blindFiltered` is true, a persistent banner — not a tooltip, not an icon —
  saying the view covers revealed steps only. Copy comes from `docs/analytics.md` so it matches what
  a report says.
- Loading, empty and error states for each panel independently. An engagement with no steps shows
  "nothing scored yet", not a zeroed chart implying failure.
- Live-ish refresh: invalidate analytics query keys on the relevant engagement SSE verbs
  (`event-invalidation.ts`, `M4-003`) — scoring and execution changes, not every comment.

**Out**

- The compare view (`M5-014`).
- Export buttons — wire them in `M5-011`/`M5-012`'s own UI pass or a follow-up; this page is reads.
- Report building (M6).
- Cross-engagement or programme dashboards.
- A printable version of this page. M6 owns client-facing rendering, and a second print path is
  exactly the duplication `PLAN.md` §5 warns about.

## Acceptance criteria

- [ ] Every percentage on the page states its denominator in visible text.
- [ ] `unscored`, `none` and `not attempted` are distinguishable **without colour** — pattern, label
      or position — and the heatmap legend names all three.
- [ ] Colour ramp values come from one shared constant, and a test asserts they match the values
      `docs/analytics.md` documents for `M5-010`. Two ramps drifting is a client-visible defect.
- [ ] MTTD cannot render without its undetected count. Enforced in the component, not by convention —
      the props make the count required, mirroring the API shape.
- [ ] `blindFiltered: true` renders the banner; a component test asserts it, and asserts nothing on
      the page displays a count of what was withheld.
- [ ] Keyboard-navigable heatmap with accessible cell labels; `axe` clean at the level the rest of
      the app is held to (`M3-014` precedent).
- [ ] Panels fail independently: a 500 from burndown does not blank the coverage card.
- [ ] Observer seat sees the whole page — `report.read` is all-members, and an observer who cannot
      see the numbers has no reason to be in the engagement.
- [ ] Playwright: a red and a blue browser on one blind engagement, side by side, showing different
      totals and blue showing the banner. This is the epic decision made visible, and it belongs in
      the E2E suite rather than only in unit tests.

## Tests

- Component tests per panel: loaded, empty, error, blind-filtered.
- Ramp-agreement test against the documented values.
- MTTD required-props test.
- Playwright blind comparison per the last criterion, extending `M4-009`'s blind-mode spec fixtures
  rather than seeding a new engagement.

## Notes for the implementer

- Read `docs/analytics.md` before writing any copy. Every label on this page has a normative
  definition there, and M6 will print the same words next to the same numbers.
- Charts: keep them boring and legible. This screenshot ends up in `M7-002`'s README.
- The heatmap can be wide. It scrolls inside its own container; the page body does not scroll
  horizontally.
