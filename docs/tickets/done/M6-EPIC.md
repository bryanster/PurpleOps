# M6 — Reporting (epic)

**State:** refined · **Depends on:** M5 complete (including the **M5-015** gate)

## Goal

The stated purpose of the product, unmet since the DOCX exporter was deleted in v1's PR #33
(`PLAN.md` §Context). A section-picker report builder over a **block registry**, one rendering path
to HTML, and that same HTML as both the authenticated shareable report and the PDF input.

**M6 is the usability bar** — the branch is mergeable once M6 lands (`PLAN.md` §8).

## Decisions (locked)

| Topic | Decision |
|---|---|
| **Draft vs published** | **Live draft + frozen published versions.** Each report has one mutable **draft** (ordered block instances + params + branding overrides). **Publish** freezes an immutable **version**: copied block config, rendered HTML (and asset manifest), content hash, publisher, timestamp, and flags used at publish. Authenticated members edit/preview the draft (re-queries analytics). Clients and share grants always see a **published version**. Reconciles PLAN.md "shareable live HTML" (HTML page, not SPA chrome) with "rendered reports are immutable". |
| **Share authentication** | **Login required — no anonymous report HTML.** Opening a share URL demands a signed-in session. Authorization: engagement member with `report.read`, **or** a **share grant** on that published version. Optional **share password** remains a second gate before the grant/member check (defense in depth when a URL leaks inside a logged-in browser). PLAN.md's "unguesable URL alone" is rejected for client-facing delivery. |
| **Share grants / guests** | Lead (via `report.publish`) creates grants on a published version: (a) bind an **existing user**, or (b) mint a **redeemable invite** that, after login or local guest registration, attaches the grant to that user. Guest accounts are ordinary local users with **no engagement membership** and access limited to granted version(s) (view HTML + PDF of that version only). Revoke grant or revoke share → subsequent access **404** (not 403) so existence is not confirmed. No email transport required — lead copies the claim URL out of band. |
| **Document model** | `report` (engagement-owned, has draft state) → `report_block` rows (ordinal, block_id, params JSON) for the draft; `report_version` (immutable snapshot) → frozen blocks + `rendered_html` + `content_sha256` + publish options. Share links and grants point at **`report_version`**, never the draft. |
| **Authz** | **New `report.write`:** draft CRUD, reorder, templates apply, branding overrides — **all engagement members**, token scope `reports:write`. **`report.publish`** unchanged: lead only — publish, supersede, create/revoke shares & grants, set evidence-inclusion. **`report.read`:** view drafts (members), published versions the subject may access, render/export they are allowed. Share-grant viewers use a dedicated public-ish route authorized by grant, not by engagement membership. Update `M1-014` matrix + `docs/authz.md`. |
| **Blind scope at publish** | **Published versions always use the lead (full) seat scope.** Blue-scoped numbers never become the client deliverable. Draft **preview** for blue remains seat-scoped and must be labeled (same rule as `M5-013`). |
| **Evidence in deliverables** | **Opt-in per publish**, default **off.** Publish body includes `includeEvidence: bool`. When false, evidence appendix/walkthrough omit binaries and share/PDF asset routes do not serve them. Authenticated draft preview may still show evidence to members with `evidence.read`. |
| **Rich text** | **TipTap (ProseMirror) in the SPA**; storage is HTML. **Server is source of truth for safety:** `microcosm-cc/bluemonday` (or equivalent) with a **strict allowlist** — `p`, `h1`–`h3`, `ul`/`ol`/`li`, `a[href]` (http/https/mailto only), `strong`/`em`/`code`/`pre`/`blockquote`/`br`, no `script`, no event handlers, no `style`, no `iframe`, no `img` (images are evidence blocks, not inline paste). Sanitize on write **and** on render. Client-only DOMPurify is not sufficient. |
| **Page breaks** | **Pragmatic CSS** (`break-inside: avoid` on cards/tables/figures; keep-with-next on headings; cover alone) **plus** an explicit **`page_break` block** operators insert. No WYSIWYG page preview. PDF tests stay smoke-level (page count, no render error); HTML golden files carry real assertions (`PLAN.md` §9). |
| **Locale / units** | **Fixed for v1:** ISO dates (`YYYY-MM-DD`), en-US grouping for numbers, durations as defined in `docs/analytics.md`, timestamps RFC 3339 with explicit `UTC` label. No per-report locale or timezone. Server still stores UTC only (README § Conventions); formatting for report HTML is the one allowed server-side display format, confined to `internal/report`. |
| **Templates** | **Engagement-scoped.** Save draft arrangement (blocks + default params, not frozen HTML) as a template on the engagement; apply template to a report draft (replaces draft blocks). No install-wide template gallery in v1. |
| **Branding** | **Install-wide defaults** (logo blob ref, primary/secondary colours, firm name) editable by platform admin; **per-report overrides** (client name defaults from `engagement.client`, logo/colours optional override). Published version snapshots branding actually used. |
| **Numbers** | Every data block reads **`M5-009` analytics endpoints / `internal/analytics`** only — **no second aggregation path.** Labels and heatmap ramp from `docs/analytics.md`. Review rejection if a block recomputes rollups. |
| **Rounds** | None. Comparison block is **engagement comparison** via `M5-008` (`baseline` engagement id in block params). No round vocabulary in schema, UI, or copy (`M5-EPIC`). |
| **Renderer** | **One path:** blocks → HTML string(s). Draft preview, published version body, share view, and PDF input all use it. Builder preview is that HTML in `iframe`/`srcdoc`, not a parallel React approximation. |
| **PDF** | `chromedp` + bundled Chromium (`M0B-011`, `BLACKLIGHT_CHROME_PATH`). Compose sandbox trade-off documented in `docs/deploy.md` is owned here if still open. Documented operator fallback: browser print-to-PDF of the HTML. CI: smoke only. |
| **Block catalogue (v1)** | cover · executive summary · scope & RoE · free rich text · page break · coverage heatmap · per-tactic scorecard · detection-category distribution · detection gaps · MTTD analysis · engagement comparison / remediation delta · scenario walkthrough · findings backlog · evidence appendix. **No** data-bound custom query blocks (`PLAN.md` §8). |
| **Exit gate** | **M6-015** — full `PLAN.md` §9 E2E thesis as rewritten by `M5-EPIC` (retest = second engagement; comparison block; share revoke → 404). |

## Tickets

Build roughly in this order — the dependency chain is real. **M5-015 is a gate before any M6
ticket merges.**

| ID | Title | Size | Depends on |
|---|---|---|---|
| [M6-001](M6-001-block-registry.md) | Block registry (id, params schema, data deps, renderer hook) | M | M5-015 |
| [M6-002](M6-002-report-document-model.md) | Report document model, draft CRUD, `report.write` | L | M6-001, M1-012 |
| [M6-003](M6-003-report-templates.md) | Engagement-scoped report templates | M | M6-002 |
| [M6-004](M6-004-branding.md) | Install branding defaults + per-report overrides | M | M6-002 |
| [M6-005](M6-005-rich-text-sanitization.md) | TipTap + server HTML allowlist (bluemonday) | M | M6-002 |
| [M6-006](M6-006-narrative-blocks.md) | Narrative blocks: cover, exec summary, scope/RoE, rich text, page break | M | M6-001, M6-005, M6-004 |
| [M6-007](M6-007-analytics-blocks.md) | Analytics blocks: heatmap, scorecard, distribution, gaps, MTTD, compare | L | M6-001, M5-009 |
| [M6-008](M6-008-detail-blocks.md) | Detail blocks: scenario walkthrough, findings backlog, evidence appendix | L | M6-001, M3-009, M5-009 |
| [M6-009](M6-009-html-renderer.md) | Single HTML rendering path + golden files | L | M6-006, M6-007, M6-008, M6-004 |
| [M6-010](M6-010-pdf-chromedp.md) | PDF via headless Chromium (`chromedp`) | M | M6-009, M0B-011 |
| [M6-011](M6-011-publish-and-versioning.md) | Publish, immutable versions, evidence opt-in, lead scope | L | M6-009, M6-002 |
| [M6-012](M6-012-share-links.md) | Share links, grants/guests, password gate, revoke → 404 | L | M6-011, M1-003 |
| [M6-013](M6-013-builder-ui.md) | Builder UI: blocks, reorder, params, HTML preview | L | M6-002…M6-009, M0B-009 |
| [M6-014](M6-014-publish-share-ui.md) | Publish / versions / share & guest-grant UI | M | M6-011, M6-012, M6-013 |
| [M6-015](M6-015-e2e-thesis.md) | Complete PLAN.md §9 E2E thesis (M5 rewrite) | L | M6-010, M6-014, M0B-013 |

## Risks

- **Scope creep is called out in `PLAN.md` §8:** section-picker + reorder + rich text + built-in
  data blocks only. Data-bound custom query blocks stay out.
- **Stored XSS on the client deliverable.** Sanitization policy is locked before the editor ships;
  golden tests must include a rejected payload suite, not only happy-path HTML.
- **Login-required share changes the PLAN.md story.** Guests and grants are new surface area
  (accounts, authz, revoke semantics). Keep guest power minimal — one version, read-only.
- **Chromium in CI is slow/flaky.** PDF = smoke (page count, no error). Real assertions live on HTML
  goldens; normalize timestamps/ids before diff (`PLAN.md` §9).
- **Compose Chromium sandbox** was deferred in `M0B-011`. M6-010 must pick a documented default that
  actually prints PDFs in `docker compose up`.
- **Draft preview vs published drift.** One renderer prevents layout drift; publish options
  (`includeEvidence`, seat scope) can still surprise — UI must echo them at publish time.
- **Analytics cost at render.** M5-015 budgeted queries under write load; a full report may run many.
  Render should parallelize reads within budget and fail a block without failing the whole report
  where possible (published snapshot must still be complete — fail the publish, not serve partial).

## Out of milestone (do not pull in)

- Data-bound **custom query** blocks / operator SQL.
- Anonymous (no-login) share pages.
- Install-wide template marketplace; scheduled/emailed reports.
- Per-report locale/timezone; WYSIWYG page-preview canvas.
- Retest **rounds** (still dropped — use second engagement + compare block).
- Archive import / engagement restore (M5/M7).
- Pixel-perfect print designers or DOCX/PPTX exporters.
- Real-time collaborative cursors on the draft.
