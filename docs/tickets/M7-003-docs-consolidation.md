# M7-003 — Docs consolidation & operator readiness

**Milestone:** M7 · **Size:** L · **Depends on:** M6-015

## Why

Feature tickets already produced a real docs set (`deploy`, `security`, `sso-*`, `api`,
`api-tokens`, `migrations`, `testing`, content/*, etc.). Cutover fails if those pages still say
"under construction", contradict each other, or omit the **upgrade** story operators need the day
after `v1.0.0`.

This is **review + consolidation + gap fill**, not a second documentation project.

## Scope

**In**

- Inventory every file under `docs/*.md` (not tickets) and for each:
  - Strip or rewrite stale **status** banners that claim the product has no login/content/engagements
    when those exist.
  - Fix internal links and renames (blacklight vs purpleops leftovers, old round vocabulary in
    operator-facing pages — prefer engagement compare language per M5-EPIC).
  - Ensure authz/security/http cross-links still match the code.
- **Operator path must answer, with working anchors:**
  1. Install (compose + bare metal pointer) — `docs/deploy.md`
  2. Configure (required secrets, `BASE_URL`, TLS reverse proxy notes)
  3. SSO setup — `docs/sso-oidc.md`, `docs/sso-saml.md`
  4. API + service tokens — `docs/api.md`, `docs/api-tokens.md`
  5. Backup / restore — `docs/deploy.md` (keep stop-the-world file copy as source of truth until
     M7-005 adds `blctl backup`)
  6. **Upgrade** — new section (can live in `docs/deploy.md` or `docs/migrations.md`, linked from
     both): read release notes → back up DB + `evidence/` (+ keys) → stop → replace image/binary →
     migrate (startup or `blctl`) → start → smoke login. Explicit: migrations are forward-only;
     recovery is restore + previous release (`docs/migrations.md` already says this — make it the
     upgrade spine).
  7. Security model pointer — `docs/security.md`, `docs/authz.md`
- Greenfield cutover statement: **no import from the Mongo v1 app**; new install starts empty.
  One clear paragraph in deploy (and reused by M7-009 notes).
- `docs/cli.md`: replace "M7 — replaces manual procedure" placeholders only where M7-005 has not
  landed yet with either a working command or a dated pointer — do not leave forever-TODO tables
  after M7-005 merges (coordinate: if this ticket merges first, point at manual backup; M7-005
  updates the row).
- Contributing/testing docs: remove "branch is not usable" claims; keep accurate CI/e2e instructions.
- Optional: a thin `docs/README.md` index **only if** the root README docs map is insufficient —
  prefer not to create new top-level docs without need.

**Out**

- Rewriting `PLAN.md` rounds sections (out of milestone).
- Ticket backlog reformatting beyond linking from README (M7-006 may touch tickets README status).
- CHANGELOG generation (M7-004).
- New product features to "make the docs true".

## Files

- `docs/*.md` (especially `deploy.md`, `migrations.md`, `security.md`, `cli.md`, `testing.md`,
  `contributing.md`, `api.md`, SSO pages)
- Root `README.md` only if cross-links require it after M7-002 (prefer not to fight that ticket)

## Acceptance criteria

- [ ] No operator-facing doc claims the server is an empty shell without login/engagements.
- [ ] Upgrade section exists, is linked from deploy quickstart or top nav area of deploy.md, and
      matches migrator reality (forward-only, backup first).
- [ ] Backup/restore instructions work as written for compose volume layout.
- [ ] Greenfield / no v1 data migration stated once in operator docs.
- [ ] SSO, API tokens, and security pages link to each other without contradictions on cookie vs
      token auth.
- [ ] Content docs still match install-from-UI model (no 1 GB seeder instructions).
- [ ] Completion notes list every doc file touched and any **gap found that belongs to an earlier
      milestone** (file follow-ups; do not silently expand M7).

## Tests

- Link check mindset: open every path the README docs map cites.
- No new Go tests required. If authz doc is generated, regenerate per existing make target.

## Notes for the implementer

- Prefer editing in place. Do not create `docs/v1/` or parallel trees.
- Round vocabulary: operator docs should not teach `round` APIs; if PLAN.md still says rounds,
  tickets README already owns that standing deviation — operator docs follow the epics.
- Coordinate with M7-005: upgrade prose must not assume `blctl backup` until that command exists;
  write the manual procedure as canonical, then let M7-005 wrap it.
