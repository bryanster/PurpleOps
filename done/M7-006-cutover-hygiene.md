# M7-006 — Cutover hygiene (v1 refs, CI branches, status banners)

**Milestone:** M7 · **Size:** S · **Depends on:** M7-001

## Why

`PLAN.md` §8 treats old-tree deletion follow-through as separable from merge. The tree is already
the rebuild, but leftovers remain: CI listening to `v2`, README/docs status lines, tickets index
claiming M5/M6 are unstarted, Makefile `v2-dev` default, and possible PurpleOps-era string refs in
operator paths. Hygiene prevents shipping a product that still presents as a side branch.

## Scope

**In**

- **Tickets index** (`docs/tickets/README.md`):
  - Mark M5/M6/M7 states accurately (done vs refined).
  - Update M0a note once `v1-final` exists (M7-001).
  - Add M7 ticket table (mirror epic).
- **CI** (`.github/workflows/ci.yml`): decide whether to keep `v2` in `on.push.branches` and
  concurrency exceptions. **Default:** drop `v2` if the branch is no longer used; if kept, document
  why in contributing. Do not break `main`.
- Grep-driven cleanup of **operator-facing** "under construction" / "not yet usable" strings left
  after M7-002/M7-003 (fix residuals only; do not fight in-flight doc PRs — complete the job).
- Makefile / compose default `VERSION` fallback: prefer `v1-dev` or `dev` over `v2-dev` so local
  builds do not advertise the wrong product generation (`M7-EPIC` version decision). Update tests
  that assert the old default string.
- Confirm no runtime code path still expects Mongo, `purpleops` module paths, or deleted v1
  commands. Fix any **broken** reference; leave historical PLAN.md/context alone.
- Optional: archive or note `origin/v2` in contributing — do not delete remote branches without
  explicit owner approval in the PR description.

**Out**

- Rewriting PLAN.md domain rounds.
- Mass renames of every historical "v2 rebuild" phrase in completed tickets.
- GHCR publish (M7-004).

## Files

- `docs/tickets/README.md`
- `.github/workflows/ci.yml`
- `Makefile`, `compose.yml` (VERSION default)
- Stray status banners in `README.md` / `docs/*` if still present
- Any one-off broken refs discovered by search

## Acceptance criteria
- [x] `docs/tickets/README.md` shows M7 refined with linked tickets; M5/M6 completion state matches
      reality.
- [x] CI primary path is `main`; `v2` either removed or justified in docs.
- [x] `make build` stamped version default does not claim `v2-dev` as the product line.
- [x] No remaining top-level "product is unusable" banner on README after M7-002 intent.
- [x] Completion notes list refs fixed and any intentionally kept historical wording.
- [ ] No remaining top-level "product is unusable" banner on README after M7-002 intent.
- [ ] Completion notes list refs fixed and any intentionally kept historical wording.

## Tests

- `make test` for any default-version string assertions updated.
- CI config must still be valid YAML (workflow runs on the PR).

## Notes for the implementer

- Be conservative with `git` branch deletion.
- Search terms worth running: `under construction`, `v2-dev`, `not yet usable`, `purpleops`,
  `on branch v2`, `Mongo`.

## Implementation notes

- `v2-dev` default → `v1-dev` in: Makefile:33, compose.yml:23, deploy/Dockerfile:94,248, deploy/smoke.sh:150
- CI `.github/workflows/ci.yml`: dropped `v2` from `on.push.branches` (now `[main]`) and concurrency exception (now `refs/heads/main` only)
- `docs/tickets/README.md`: M7 state updated `refined — 0/9` → `in progress — 5/9`; M7-002 through M7-005 marked ✅ with `done/` links
- Mongo/purpleops grep: all occurrences are appropriate (CHANGELOG greenfield notice, PLAN.md history, docs deploy notes, ticket prose) — no broken runtime references
- "under construction"/"not yet usable"/"on branch v2": only in ticket docs and M7-EPIC risks — no operator-facing residuals after M7-002/M7-003 cleanup
- `internal/version/version_test.go` uses sentinel values, not hard-coded defaults; no test changes needed
- Version ldflags test passes; CI YAML is valid
- No remote branch deletion performed (`origin/v2` left as-is per ticket's conservative guidance)
