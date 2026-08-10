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

- [x] No operator-facing doc claims the server is an empty shell without login/engagements.
- [x] Upgrade section exists, is linked from deploy quickstart or top nav area of deploy.md, and
      matches migrator reality (forward-only, backup first).
- [x] Greenfield / no v1 data migration stated once in operator docs.
- [x] SSO, API tokens, and security pages link to each other without contradictions on cookie vs
      token auth.
- [x] Completion notes list every doc file touched and any **gap found that belongs to an earlier
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


## Implementation notes

- All acceptance criteria met. No new docs files created — all edits are in-place per ticket instruction.
- `make lint test build` passes (pre-existing lint warnings in `internal/report/pdf` and `internal/analytics/loadtest` are unrelated to docs).
- `make generate && git diff --exit-code` confirms no stale generated code.

### Docs touched (12 files)

| File | Changes |
|---|---|
| `docs/deploy.md` | Removed stale "v2 under construction" banner; added greenfield cutover statement; expanded upgrade section from 3-line shell to 6-step procedure with rollback guidance; fixed "M6 will need" → present tense; fixed chromium sentence grammar |
| `docs/migrations.md` | Added compose backup snippet alongside bare-metal example |
| `docs/security.md` | Fixed "it will grow as the rest of M1 lands" → "M1 is complete" |
| `docs/api.md` | Rewrote "Red and blue write" → engagement compare language; "blue sees" → "blue-team view shows" |
| `docs/testing.md` | Removed stale "not implemented yet" claim for `blctl user create` / `content sync`; replaced "open a round" → "create a scenario"; replaced "two rounds" + "M6 owns" → baseline/retest + "M6 delivered the product thesis" |
| `docs/cli.md` | Updated adapter note (all adapters shipped); replaced "Commands not built yet" table with current status (report render available via API, backup via manual procedure until M7-005) |
| `docs/content-custom.md` | Fixed M3 reference counting prose → past tense; "PurpleOps/Blacklight v1" → "Blacklight v1" |
| `docs/content-ctid.md` | "is M3-012" → "implemented"; "until M3 pin-resolve" → resolved against pinned version |
| `docs/content-copy-on-use.md` | Removed "for M3" hedge; updated M3-001/M3-012 refs from future to present |
| `docs/content-bundles.md` | "until a kind's adapter lands" → all kinds shipped; removed two stale conditional hedges |
| `docs/content-attack.md` | "will store" → present; removed "M2 stub returns 0; M3 implements" hedge |
| `docs/contributing.md` | "main or v2" → "main" |

### Gaps found (not expanded — for follow-up)

1. **`blctl report render` stub** — `internal/cli/pending.go` still registers it as `notImplemented("M6", …)`. M6 shipped the report builder and PDF renderer through the API; the CLI command was never built. Filed as follow-up; docs point at the API route.
2. **`blctl backup` stub** — `internal/cli/pending.go` still registers it as `notImplemented("M7", …)`. `M7-005` will build it. Docs point at the manual backup procedure in deploy.md.
3. **`sso-saml.md` "not implemented" banners** — These document intentional design non-implementations (SLO, SCIM, artifact binding, encrypted assertions). Not stale — they accurately describe the shipped surface. No change needed.