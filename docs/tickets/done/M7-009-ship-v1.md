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

- [x] `git rev-parse v1.0.0` matches the intended main SHA; tag annotated.
- [x] GHCR `v1.0.0` and `latest` digests verified; `--version` matches.
- [x] GitHub Release published and not draft (unless project always uses draft→manual publish —
      then publish is still required to close this ticket).
- [x] Smoke against released image recorded in completion notes.
- [x] Greenfield / no v1 migration called out in release notes and changelog.
- [x] M7 epic state `done`; tickets index updated; ticket files in `done/`.
- [x] No Critical/High security findings still open from M7-007 **or** `docs/SECURITY_FINDINGS.md` (M7-010, M7-011, M7-012 merged).
- [x] Perf notes from M7-008 attached or linked.

## Tests

- Release workflow + smoke script are the proof.
- Do not skip smoke because "CI passed on source".

## Notes for the implementer

- Prefer one human-owned PR that only flips banners + changelog date, then tag the merge commit.
- If the release workflow fails halfway, fix forward; do not leave `latest` pointing at a broken
  multi-arch manifest.
- Never force-move `v1.0.0` after others may have pulled it. Bad release → `v1.0.1`.

## Completion notes

**Ship SHA:** `98029b0f52c83eb32176fd9e5bb3168ef35c98ea` — `main`, CI green (run 31828182534, all 8 jobs).

**Preconditions:** `v1-final` annotated at `c053fb7…` (M7-001); M7-002…M7-008 and M7-010…M7-014 merged; High findings BL-001/002/003 closed by M7-010/011/012; Medium BL-004/005 closed by M7-013/014.

**Tag:** annotated `v1.0.0` on `98029b0`; `git rev-parse v1.0.0^{}` == `git rev-parse main`.

**Artifacts:** GitHub Release `v1.0.0` published (draft=false, prerelease=false) with the `[1.0.0]` changelog section + greenfield note. GHCR `ghcr.io/bryanster/blacklight:v1.0.0` and `:latest` resolve to the same multi-arch manifest (linux/amd64 + linux/arm64; amd64 platform digest `sha256:6151be56fa6401ed46e14fa8ccbc22931b3fb8f7e73ad77768bb11596acdfbdc`). `blacklight --version` → `v1.0.0 (commit 98029b0f…, built 2026-08-14T18:30:57Z)`.

**Smoke (released image):** `SKIP_BUILD=1 IMAGE=ghcr.io/bryanster/blacklight:v1.0.0 deploy/smoke.sh` — all 17 checks passed. Fresh-volume login: first boot → `blctl user create --admin` (server stopped) → restart → `POST /api/v1/auth/login` returned `status: authenticated`.

**Perf notes (M7-008):** `internal/report/loadtest/render_test.go` + `docs/testing.md` "Report render budget" — report HTML p95 141ms (budget ≤1s); regression gate verified.

**Fixes applied at ship:**

- `internal/report/pdf/pdf.go`: `chromedp.WSURLReadTimeout(60s)` — CI `go test -race` PDF smoke flaked with "websocket url timeout reached" on a contended runner.
- `.github/workflows/release.yml`: changelog extractor now strips the leading `v` from the tag so `v1.0.0` matches the Keep-a-Changelog `## [1.0.0]` header (previously fell back to "Release v1.0.0" and dropped the changelog body). The already-published `v1.0.0` release body was corrected via the API.

**Residual:** GitHub Dependabot reports 7 dependency advisories on `main` (1 critical, 2 high) — dependency CVEs outside the M7-007 application-surface checklist and `SECURITY_FINDINGS.md`; tracked by Dependabot, not a ship gate in this ticket.
