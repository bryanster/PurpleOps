# M6 — Reporting (epic)

**State:** needs refinement · **Depends on:** M5

## Goal

The stated purpose of the product, unmet since the DOCX exporter was deleted in v1's PR #33
(`PLAN.md` §Context). A section-picker report builder over a **block registry**, one rendering path
to HTML, and that same HTML as both the shareable live report and the PDF input.

**M6 is the usability bar** — the branch is mergeable once M6 lands (`PLAN.md` §8).

## Candidate tickets

| ID | Title | Notes |
|---|---|---|
| M6-001 | Block registry | Each block declares id, params schema, data query, renderer |
| M6-002 | Report document model + CRUD | Ordered block instances with per-block params; saved arrangements |
| M6-003 | Report templates | Save an arrangement for reuse |
| M6-004 | Branding | Logo, colours, client name |
| M6-005–012 | Built-in blocks | Cover · executive summary · scope & RoE · coverage heatmap · per-tactic scorecard · detection-category distribution · scenario walkthrough · **engagement comparison / remediation delta** · detection gaps · findings backlog · MTTD analysis · evidence appendix · free rich text. Group into a few tickets, not one per block. Every data block reads an `M5-009` endpoint — no second aggregation path |
| M6-013 | HTML renderer | The single rendering path |
| M6-014 | PDF via headless Chromium | `chromedp`, Chromium bundled in the image (`M0B-011`), `CHROME_PATH` for bare metal, documented print-to-PDF fallback |
| M6-015 | Share links | Signed, expiring, revocable, optional password |
| M6-016 | Report versioning | Rendered reports immutable, so "the report we sent the client" stays reproducible |
| M6-017 | Builder UI | Toggle blocks, drag to reorder, configure each (scope to scenarios/rounds, verbosity) |
| M6-018 | Complete the E2E thesis spec | All seven steps of `PLAN.md` §9 **as rewritten by `M5-EPIC`** — step 6 is "create the retest engagement from the open findings' steps and score them higher", step 7's round-comparison block is the engagement-comparison block — ending with share-link revocation returning 404 |

## Settled by M5 (do not reopen here)

- **Rounds.** Dropped in M3, replaced in M5 by ad-hoc cross-engagement compare (`M5-008`). The
  comparison block reads that endpoint; there is no round vocabulary anywhere in M6.
- **Where the numbers come from.** `M5-009`'s endpoints, under `report.read`. A block that computes
  its own aggregate is a review rejection — the dashboard and the report must not be able to
  disagree in front of a client.
- **Definitions and copy.** `docs/analytics.md` is normative for every label a block prints, and for
  the heatmap colour ramp shared with the Navigator export.

## Open questions to resolve before writing tickets

1. **Rich text editing and safety.** Which editor, and what is the sanitization policy? User-authored
   HTML flows into a *publicly shareable* page — this is a stored-XSS surface aimed at the client
   who receives the link. Settle sanitization before the editor.
2. **Share-link authentication.** Anonymous with an unguessable URL, optional password, or required?
   And is a shared report a live view or a frozen snapshot? `PLAN.md` says "shareable **live** HTML"
   but also that rendered reports are immutable — reconcile these explicitly.
3. **Evidence in shared reports.** Screenshots may contain client-sensitive material. Are they
   included by default in a share link, or opt-in per report?
4. **Page-break control.** PDF quality is where reporting tools succeed or fail. Decide how much
   CSS `break-inside` work is in scope, and set expectations.
5. **Localisation / units.** Dates and number formats in client-facing output — fixed to ISO, or
   configurable per report?

## Risks

- **Scope creep is called out explicitly in `PLAN.md` §8:** v1 is section-picker + reorder + rich
  text only. Data-bound custom query blocks are **out of scope**, deferred behind a safe read-only
  query layer. Hold this line.
- Chromium in CI is slow and flaky. Keep the PDF test a smoke test — page count and no render errors
  (`PLAN.md` §9) — and put the real assertions on the HTML golden files.
- Golden-file tests rot if they include timestamps or generated IDs. Normalize them before diffing,
  or the suite becomes noise everyone ignores.
