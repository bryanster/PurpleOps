# M7 — Cutover (epic)

**State:** needs refinement · **Depends on:** M6

## Goal

Ship it: documentation, deploy assets, and closing out the v1 tree (`PLAN.md` §7).

## Candidate tickets

| ID | Title | Notes |
|---|---|---|
| **M7-001** | **Tag the v1 tip as `v1-final`** | **Do this now, not at M7.** `PLAN.md` §7 step 1 requires it and `git tag` currently returns nothing — the pre-rebuild tree is reachable only by SHA until someone tags it. Cheap, and irreversible-ish if a branch is ever pruned |
| M7-002 | README rewrite | What it is, screenshots, quickstart, the retest loop as the headline |
| M7-003 | Docs set | Install, configure, SSO setup, API + tokens, backup/restore, upgrade, security model. Several already exist from earlier tickets — this consolidates and reviews them |
| M7-004 | Release workflow | Tagged releases, multi-arch image publish, checksums, changelog |
| M7-005 | Upgrade path | Migration behaviour across releases, and what an operator does before upgrading (back up the DuckDB file and `evidence/`) |
| M7-006 | Old-tree deletion follow-through | Confirm nothing v1-only is still referenced; `PLAN.md` §8 treats this as work separable from the merge |
| M7-007 | Security review pass | Fresh review over the whole surface: headers, cookies, uploads, share links, SSO, tokens |
| M7-008 | Performance sanity pass | Realistic engagement size; confirm the M3-016 load-test conclusions still hold with M4–M6 on top |

## Open questions

1. **Does v1 keep running anywhere?** If an existing deployment stays live, "greenfield, no
   migration" (`PLAN.md`) means those users start empty. Confirm nobody expects their data to
   appear, and say so in the release notes.
2. **`files/` and `.env`.** `PLAN.md` §7 flags that `files/` (22 assessment directories of real
   evidence) and `.env` (live `SECRET_KEY`, `PASSWORD_SALT`) are **not in git** and are permanently
   lost at clean slate. The working tree is already clean — confirm these were copied out, or accept
   the loss on the record.
3. **`CURRENT.md`.** Same situation: untracked, and its substance was folded into `PLAN.md`'s
   Context section. Confirm it isn't wanted as a separate document.
4. **Default branch strategy.** Does `v2` become `main`, or does it merge into `main`? Affects
   tagging, release notes, and every existing PR and branch on the remote.
5. **Version numbering.** Is this `2.0.0`, and does the tag scheme change?

## Risks

- M7-001 is the only item here with a real deadline attached: the longer the v1 tip goes untagged,
  the more chance a branch cleanup makes it hard to find. Do it in the next working session.
- Docs written at the end are docs written badly. Most doc tickets are already attached to the
  feature that needs them (`docs/deploy.md`, `docs/authz.md`, `docs/sso-*.md`, `docs/security.md`) —
  M7-003 should be consolidation and review, not first drafting. If it turns into first drafting,
  that's a signal earlier tickets skipped their documentation criteria.
