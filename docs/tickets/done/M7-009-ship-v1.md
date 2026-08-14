# M7-009 — Ship gate: `v1.0.0` release

**Milestone:** M7 · **Size:** M · **Depends on:** M7-002, M7-003, M7-004, M7-005, M7-006, M7-007, M7-008, M7-010, M7-011, M7-012

## Why

Individual cutover tickets can be green while the release itself never happens. This ticket **is**
the ship: annotated `v1.0.0` on `main`, published artifacts, operator-facing truth, and a written
greenfield statement. M6-015 already proved the product thesis; this proves distributability.

## Scope

**In**

- Preconditions (all must be true before tagging):
  - M7-001 `v1-final` pushed.
  - M7-002…M7-008 acceptance criteria checked (or explicitly waived in writing on this ticket with
    reason — default: **no waivers** for 007 High+ or 004 publish path).
  - M7-010, M7-011, M7-012 High findings from [`docs/SECURITY_FINDINGS.md`](../SECURITY_FINDINGS.md) closed (no waiver).
  - `main` CI green on the intended ship SHA.
  - `CHANGELOG.md` `[1.0.0]` section final (date, highlights, upgrade/greenfield notes).
- Create **annotated** tag `v1.0.0` on the ship commit; push to `origin`.
- Confirm M7-004 workflow published:
  - GitHub Release `v1.0.0` with changelog body.
  - `ghcr.io/bryanster/blacklight:v1.0.0` multi-arch manifest.
  - `ghcr.io/bryanster/blacklight:latest` → same digest as `v1.0.0`.
- Smoke the published artifact:
  - `docker pull` + `blacklight --version` shows `v1.0.0`.
  - `deploy/smoke.sh` (or compose against the released image) healthy.
  - Login path works on a fresh volume (create admin per docs if required).
- Release notes **must** state in plain language:
  - This is a ground-up rebuild; **no migration** from the prior Mongo-based app.
  - Operators start with an empty database.
  - Historical tree: git tag `v1-final` (`c053fb7…`).
  - Backup before every future upgrade (link docs).
- Move completed M7 tickets to `docs/tickets/done/` and mark the epic/README ✅ when done.
- Remove any remaining pre-release banners that M7-002 left conditional on ship.

**Out**

- Marketing launch, blog post, social.
- Re-opening epic decisions.
- Tagging from a dirty or off-main commit.

## Files

- Git tag + GitHub Release (via workflow)
- `CHANGELOG.md` finalization
- `docs/tickets/README.md`, `docs/tickets/M7-EPIC.md` state → done
- Move `M7-*.md` to `done/` as part of closeout

## Acceptance criteria

- [ ] `git rev-parse v1.0.0` matches the intended main SHA; tag annotated.
- [ ] GHCR `v1.0.0` and `latest` digests verified; `--version` matches.
- [ ] GitHub Release published and not draft (unless project always uses draft→manual publish —
      then publish is still required to close this ticket).
- [ ] Smoke against released image recorded in completion notes.
- [ ] Greenfield / no v1 migration called out in release notes and changelog.
- [ ] M7 epic state `done`; tickets index updated; ticket files in `done/`.
- [ ] No Critical/High security findings still open from M7-007 **or** `docs/SECURITY_FINDINGS.md` (M7-010, M7-011, M7-012 merged).
- [ ] Perf notes from M7-008 attached or linked.

## Tests

- Release workflow + smoke script are the proof.
- Do not skip smoke because "CI passed on source".

## Notes for the implementer

- Prefer one human-owned PR that only flips banners + changelog date, then tag the merge commit.
- If the release workflow fails halfway, fix forward; do not leave `latest` pointing at a broken
  multi-arch manifest.
- Never force-move `v1.0.0` after others may have pulled it. Bad release → `v1.0.1`.
