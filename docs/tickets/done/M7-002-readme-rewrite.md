# M7-002 — README rewrite (product front door)

**Milestone:** M7 · **Size:** M · **Depends on:** M6-015

## Why

The README is still a rebuild status page: correct for M0–M6, wrong for a shipped product. First-time
operators and GitHub browsers need the **retest loop as the headline**, a honest quickstart, and
pointers into the real docs — not "v2 is under construction on the `v2` branch".

## Scope

**In**

- Rewrite root `README.md` as the product front door:
  - **What it is** in one short paragraph (self-hosted purple-team assessments; shared red/blue
    workbook; ATT&CK-mapped scenarios; report out).
  - **Headline workflow:** baseline engagement → findings → retest engagement → comparison in the
    report (M5/M6 standing deviation: **no rounds**).
  - **Quickstart:** `docker compose up --build` → URL → point at `docs/deploy.md` for real config
    (`BLACKLIGHT_BASE_URL`, session secret, encryption key).
  - **What you get:** single binary/image, embedded DuckDB, evidence on disk, OpenAPI API, SSO,
    service tokens — without turning the README into the deploy guide.
  - **Docs map:** deploy, security, SSO, API/tokens, CLI, contributing/testing.
  - **Status:** remove "under construction" once M7 ship criteria are met; until `v1.0.0` is tagged,
    a single clear pre-release note is OK if it does not contradict M7-003.
  - **Licence / provenance** paragraph (keep Apache-2.0 + PurpleOps independence note).
  - CI badge stays.
- Align version language with **`v1.0.0`** product numbering (`M7-EPIC`). "Ground-up rebuild" may
  remain as history; do not call the release "v2.0.0".
- Link `PLAN.md` and `docs/tickets/` as design/backlog, not as the operator path.

**Out**

- Screenshots/GIF pipeline (nice later; not required).
- Full install/SSO/API documentation (M7-003 owns consolidation).
- Changelog (M7-004).
- Rewriting every `docs/*.md` status banner (M7-003 / M7-006).

## Files

- `README.md`
- Possibly a short cross-link tweak in `docs/deploy.md` quick start if the README and deploy intro
  disagree after the rewrite.

## Acceptance criteria

- [ ] README describes the shipped product loop including baseline → retest → report compare.
- [ ] Quickstart works as written against the supported compose path; no network content fetch
      claimed at first boot.
- [ ] No instruction to use the Mongo/Flask app or a `v2`-only branch as the primary path.
- [ ] Docs map links resolve to existing files.
- [ ] Version language matches `v1.0.0` product tags (not "install v2").
- [ ] Licence/provenance block retained and accurate.

## Tests

- Manual: follow quickstart from a clean clone narrative (or point at `deploy/smoke.sh` + login
  path already covered by e2e). No new automated README test required.

## Notes for the implementer

- Read `docs/deploy.md` quick start and `.env.example` before promising env vars.
- Do not duplicate the entire security model; one sentence + link.
- Keep it short. Operators bounce; contributors have `docs/contributing.md`.
