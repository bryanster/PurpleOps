# M7-001 — Tag pre-rebuild tip as `v1-final`

**Milestone:** M7 · **Size:** S · **Depends on:** —

## Why

`PLAN.md` §7 step 1 required tagging the v1 tip before the clean-slate deletion so the Mongo/Flask
tree stays one name away, not a archaeology exercise. The clean slate already happened
(`4d8b045` — `rem: clean worktree v2 build (#54)`). **No `v1-final` tag exists** (`git tag` is
empty). Every day without the tag is unnecessary risk if history is rewritten, shallow-cloned, or
branches pruned.

This ticket is deliberately first and unblocked by M6.

## Scope

**In**

- Identify the last pre-rebuild commit on the line that became the clean slate. **Locked candidate:**
  - **`c053fb741ba953bc8f2e151c05f966db813ec8fc`**
  - Subject: `Ensure all tests pass (#45)`
  - Date: 2026-06-06
  - Verified as first parent of `4d8b045` (clean-slate merge/commit).
- Create an **annotated** tag `v1-final` pointing at that commit:
  - Message states it is the last pre-rebuild Blacklight/PurpleOps tree before the v2 clean slate.
- Push the tag to `origin`.
- Record the SHA + tag in this ticket's completion notes and in the eventual `v1.0.0` release notes
  (cross-link from M7-009; do not wait for M7-009 to push the tag).

**Out**

- Product release tagging (`v1.0.0` is M7-009).
- Rewriting history, deleting `v2` / old branches (M7-006 may drop CI listeners only).
- Recovering untracked `files/` or `.env` — already accepted lost (`M7-EPIC` decisions).

## Files

- Git only (`git tag` / `git push origin v1-final`). No code change required.
- Optional one-line note in `docs/tickets/README.md` M0a callout once tagged.

## Acceptance criteria

- [ ] `git rev-parse v1-final` equals `c053fb741ba953bc8f2e151c05f966db813ec8fc` (or a
      documented successor if history was rewritten before this ticket — then stop and update the
      epic before tagging).
- [ ] Tag is annotated, not lightweight.
- [ ] `git ls-remote --tags origin` shows `refs/tags/v1-final`.
- [ ] Completion notes quote `git show v1-final --no-patch`.
- [ ] If the SHA is **not** reachable from `origin`, do **not** invent a tag on current `main`;
      document the gap in the epic and in M7-009 release notes instead.

## Tests

- None (git metadata). Verify with the commands above.

## Notes for the implementer

```sh
git fetch origin --tags
git merge-base --is-ancestor c053fb741ba953bc8f2e151c05f966db813ec8fc 4d8b045
git tag -a v1-final c053fb741ba953bc8f2e151c05f966db813ec8fc -m "v1-final: last pre-rebuild tree before clean slate (PLAN.md §7)"
git push origin v1-final
```

Do **not** move the tag once pushed. If the wrong commit is tagged, write a new name — do not
force-move `v1-final` without an explicit human decision recorded in the epic.

## Implementation notes

- [x] `git rev-parse v1-final^{commit}` = `c053fb741ba953bc8f2e151c05f966db813ec8fc`
- [x] Tag is annotated (`git cat-file -t v1-final` → `tag`)
- [x] `git ls-remote --tags origin` shows `refs/tags/v1-final`
- [x] Tag pushed to origin

### `git show v1-final --no-patch`

```
tag v1-final
Tagger: bryanster <45668775+bryanster@users.noreply.github.com>
Date:   Mon Aug 10 18:45:16 2026 +0000

v1-final: last pre-rebuild tree before clean slate (PLAN.md §7)

commit c053fb741ba953bc8f2e151c05f966db813ec8fc
Author: bryanster <45668775+bryanster@users.noreply.github.com>
Date:   Sat Jun 6 22:30:55 2026 +0200

    Ensure all tests pass (#45)
```
