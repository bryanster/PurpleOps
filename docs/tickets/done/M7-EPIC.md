# M7 — Cutover (epic)

**State:** done · **Depends on:** M6 complete (including the **M6-015** product thesis gate)

## Goal

Ship it. Documentation, release automation, operator upgrade/backup story, and closing gates so
`v1.0.0` is a real release — not a developer branch with good tests (`PLAN.md` §7).

M6 remains the **usability bar** (`PLAN.md` §8): the product loop is mergeable once M6 lands. M7 is
the **shippability bar**: an operator who has never seen the repo can install, back up, upgrade, and
trust the security/perf posture of the published artifact.

## Decisions (locked)

| Topic | Decision |
|---|---|
| **Product version** | **`v1.0.0`.** This is the first shippable product. The rebuild was called "v2" in planning prose and branch names; that is historical. Tags, GHCR tags, `IMAGE_TAG`, release titles, and `internal/version` stamps use **`v1.0.0`** (semver with a leading `v` on git tags). Prereleases if needed: `v1.0.0-rc.1`. |
| **Default branch** | **Keep shipping on `main`.** Local/remote rebuild work already lands on `main`. No force-push ceremony, no "merge `v2` → `main`" PR required for cutover. `origin/v2` is historical; CI may keep listening to it until M7-006 drops it. |
| **Historical v1 tree tag** | **Tag the pre-rebuild tip `v1-final`.** SHA **`c053fb741ba953bc8f2e151c05f966db813ec8fc`** (`Ensure all tests pass (#45)`, 2026-06-06) — parent of clean-slate commit `4d8b045` (`rem: clean worktree v2 build (#54)`). Do this before any history rewrite or branch prune. Annotated tag, pushed to `origin`. |
| **Data / migration** | **Greenfield only.** No Mongo importer, no engagement restore from v1. Any operator moving from the old stack **starts empty**. State this in release notes. Untracked `files/` (prior evidence) and live `.env` secrets were never in git; working tree has neither — **loss accepted on the record**. `CURRENT.md` substance lives in `PLAN.md` Context; no separate revive. |
| **Release artifacts** | **GitHub Release + GHCR multi-arch image.** On tag `v*`: build/push `ghcr.io/bryanster/blacklight:<tag>` and `:latest` for `linux/amd64` + `linux/arm64` (buildx, same cross-compile approach as `M0B-011`); attach checksums; publish `CHANGELOG` section for the tag. Host binaries optional (CGO/Chromium make the image the supported artifact — `PLAN.md` §8). |
| **Docs** | **One consolidation ticket** over existing `docs/*`. Feature tickets already wrote deploy, security, SSO, API, tokens, migrations, testing. M7 reviews for drift, strips "under construction" banners, adds the missing **upgrade** narrative, and makes README the front door. Not a from-scratch rewrite. |
| **Security gate** | **Structured checklist pass** against the live surface (headers, cookies, uploads, share links, SSO, tokens, CSRF, authz regressions). Findings either fixed in-tree or filed with severity; no silent defer of High. Not an external pentest requirement for v1.0.0. **2026-08-12 follow-up:** [`docs/SECURITY_FINDINGS.md`](../SECURITY_FINDINGS.md) reopened Highs M7-007 marked PASS — tickets **M7-010…M7-012** are the fix PRs and are ship gates for M7-009. |
| **Performance gate** | **Re-run existing gates** (`M3-016`, `M4-010`, `M5-015`) under a fixture that includes **M6 report render/PDF load**. No new perf framework. Budgets hold or code fixes — do not hollow budgets. |
| **Extra scope** | **None.** No PLAN.md rounds rewrite, no archive import, no README demo GIF requirement. Keep M7 tight to cutover/release. |
| **Exit gate** | **`M7-009`:** annotated tag `v1.0.0` on `main`, GHCR images live for that tag, README + docs operator-ready (no "under construction" on the install path), `CHANGELOG` entry, security + perf checklists green, release notes state greenfield cutover. **M6-015 remains the product thesis gate** and must already be green before M7 ship. |

## Tickets

Build roughly in this order — dependency chain is real. **M6-015 is a gate before any M7 ship
ticket merges.** M7-001 is cheap and should land immediately (even before other M7 work).

| ID | Title | Size | Depends on |
|---|---|---|---|
| [M7-001](M7-001-tag-v1-final.md) | Tag pre-rebuild tip as `v1-final` | S | — (do now) |
| [M7-002](M7-002-readme-rewrite.md) | README rewrite (product front door) | M | M6-015 |
| [M7-003](M7-003-docs-consolidation.md) | Docs consolidation & operator readiness | L | M6-015 |
| [M7-004](M7-004-release-workflow.md) | Release workflow: GHCR + GitHub Release + changelog | L | M0B-011, M0B-012 |
| [M7-005](M7-005-upgrade-and-backup.md) | Upgrade path, backup procedure, optional `blctl backup` | M | M7-003 (docs contract), M0B-014 |
| [M7-006](M7-006-cutover-hygiene.md) | Cutover hygiene: v1 refs, CI branches, status banners | S | M7-001 |
| [M7-007](M7-007-security-review.md) | Security review pass (checklist) | L | M6-015, M1-014 |
| [M7-008](M7-008-performance-sanity.md) | Performance sanity pass (re-run gates + report load) | M | M6-015, M3-016, M4-010, M5-015 |
| [M7-009](M7-009-ship-v1.md) | Ship gate: `v1.0.0` release | M | M7-002…M7-008, **M7-010, M7-011, M7-012** |
| [M7-010](M7-010-engagement-list-authz.md) | Engagement list leaks every engagement (BL-001) | S | M3-002, M1-013 |
| [M7-011](M7-011-ownership-facts-loader.md) | Replace production Ownership.Facts stub (BL-003) | L | M3-002, M1-013 |
| [M7-012](M7-012-cross-engagement-idor.md) | Bind nested IDs to authorized engagement (BL-002) | L | M7-011 |
| [M7-013](M7-013-share-token-logging.md) | Share tokens in logs; unthrottled claim/password (BL-005) | M | M6-012, M1-004 |
| [M7-014](M7-014-content-sync-ssrf.md) | Content sync SSRF allowlist (BL-004) | M | M2-002, M2-003 |

## Risks

- **`v1-final` still missing.** Until M7-001 pushes the tag, the Mongo/Flask tree is only one
  `git gc` / shallow-clone away from being annoying to find. SHA is known; the work is minutes.
- **Version vocabulary collision.** Planning docs say "v2 rebuild" and Makefile defaults still
  mention `v2-dev`. M7-004/M7-006 must make **product** tags `v1.x` while leaving historical prose
  honest ("ground-up rebuild of the prior codebase"). Do not retcon git history.
- **Docs written at the end are docs written badly.** M7-003 is review and consolidation. If large
  sections are still missing, that is a signal earlier tickets skipped documentation criteria —
  fix the gaps, but do not invent a second docs tree.
- **Release workflow is the first secret-bearing pipeline.** GHCR push needs `packages: write` and
  a clean tag trigger. Fork PRs must not inherit publish credentials (`M0B-012` invariant: PR CI
  stays secretless).
- **CGO + multi-arch.** `M0B-011` already cross-compiles; release must reuse that path, not QEMU
  Go builds. Smoke Chromium only on native arch runners.
- **Greenfield surprise.** Someone will ask where their old assessments went. Release notes and
  README must say "no migration" in plain language once, not buried in PLAN.md.
- **Security/perf as rubber stamps.** Checklists without a failed-finding example or a recorded
  measurement are theatre. Completion notes need evidence the way M3-016/M5-015 did.

## Out of milestone (do not pull in)

- Mongo / v1 data importer; engagement archive **import** (export stays M5-012).
- Rewriting `PLAN.md` to remove rounds (standing deviation stays in tickets README until a docs
  hygiene pass outside ship critical path — not required to tag `v1.0.0`).
- Kubernetes/Helm, HA, external Postgres, multi-node SSE.
- External pentest report as a merge gate.
- Demo marketing site, GIF recording pipeline, install-wide template marketplace.
- Retest **rounds** (still dropped — cross-engagement compare).
- Changing the supported artifact away from the container image.
