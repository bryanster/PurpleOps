# PurpleOps v2 — Ground-Up Rebuild

## Context

PurpleOps is a self-hosted purple-team assessment tool: red and blue fill in a shared workbook
during adversary emulation, and the output is a report. Today's implementation (analysed in
`CURRENT.md`) works, but its foundations fight the goal:

- **The domain model is too flat.** An engagement is an unordered bag of test cases. There is no
  attack chain, and no concept of re-testing after remediation — which is the single metric most
  purple-team programmes are actually judged on.
- **Scoring is lossy.** `Prevented/Alerted/Logged` collapses into one derived outcome, so "we got
  raw telemetry" and "the SOC correlated it and paged someone" are indistinguishable.
- **Reporting was removed.** The DOCX exporter was deleted in PR #33; the tool now emits CSV/JSON
  and a Navigator layer. The stated goal — generate reports — is unmet.
- **Authorization is broken in ways that matter.** `/manage/access` is not admin-gated (any user can
  grant themselves Admin), Spectators fall through to write access, API keys authenticate nothing,
  and CSRF protection was added then removed. Role checks are duplicated per handler with two
  contradictory definitions of "blue".
- **The stack blocks the ambition.** Mongo with no indexes and an N×M read pattern, server-rendered
  pongo2 + jQuery, and a ~1 GB git-clone-on-first-boot seeder.

This plan rebuilds it on **Go + React + OpenAPI + DuckDB**, keeping MITRE ATT&CK as the spine.

## Decisions (locked with the user)

| Area | Decision |
|---|---|
| Deployment | Single-tenant, single node. One binary, embedded DuckDB, evidence on local disk |
| Domain model | Engagement → Scenario → Step, with **retest rounds** |
| Scoring | **MITRE ATT&CK Evaluations** detection categories + modifiers |
| Reporting | Section-picker **report builder** → server-rendered PDF + shareable live HTML + data exports |
| Content | ATT&CK Enterprise, CTID emulation plans, Atomic Red Team, Sigma — each **opt-in/installable from the UI**, live-synced from upstream, plus user-authored content |
| Integrations | Public REST API + service tokens (the API is the only integration surface in v1) |
| Migration | **Greenfield** — no Mongo importer |
| RBAC | Platform role + **per-engagement membership role** |
| Collaboration | SSE live updates, comments + activity log, blind mode, presence |
| Frontend | React + Vite + TS, shadcn/ui + Tailwind, **embedded in the Go binary** |
| Auth | Local + enforceable TOTP, OIDC, SAML 2.0, service tokens |
| Repo | Clean rebuild on a new branch, old tree deleted at cutover |
| First release | Full loop end-to-end **including retest rounds** |

---

## 1. Architecture

```
React SPA (Vite, TS, shadcn/ui, TanStack Query, typed client generated from OpenAPI)
   │  fetch → /api/v1/*        ·        EventSource → /api/v1/events
   ▼
Go binary  (net/http + chi)
   ├── generated server iface (oapi-codegen strict mode) + request validation (kin-openapi)
   ├── authn (session | OIDC | SAML | service token)  →  authz (central policy)
   ├── domain services  →  store (DuckDB repositories)
   ├── content sync workers (ATT&CK / Atomic / Sigma / CTID adapters)
   ├── SSE hub + append-only activity log
   ├── analytics (DuckDB SQL: rollups, heatmaps, MTTD, round deltas)
   └── report renderer (block registry → HTML → PDF)
   ▼
purpleops.duckdb   +   evidence/  (content-addressed blobs)   +   web/dist (embed.FS)
```

**Single binary, single file, no external services.** Postgres/Mongo/Redis all disappear.

### Why DuckDB works here, and the one constraint

Workload is tiny by OLTP standards (thousands of rows, dozens of users, low write rate) and heavily
analytical on read (per-tactic rollups, round-over-round deltas, coverage matrices) — exactly
DuckDB's shape. Reporting queries that were N×M application loops become single SQL statements.

Constraints to design around, not discover later:

- **One process may hold the database read-write.** Fine for single-node; enforced by design.
- **Concurrent write transactions can conflict.** All writes go through a single serialized writer
  (one `*sql.Conn` behind a mutex, or a write-queue goroutine); reads use the normal pool. At this
  write volume the serialization is invisible.
- **CGO is required.** The driver is [`github.com/duckdb/duckdb-go/v2`](https://github.com/duckdb/duckdb-go)
  — the official driver as of v2.5.0, moved from `marcboeker/go-duckdb`; versions encode the DuckDB
  version as `v2.MAJOR_MINOR_PATCH.x`. So builds need `CGO_ENABLED=1` and cross-compiles need a
  cross-compiler. "Single binary" still holds; "pure-Go static musl build" does not.
- **Escape hatch:** keep the schema portable ANSI SQL and all queries behind repository interfaces,
  so a future HA requirement means swapping the store, not rewriting the app.

---

## 2. Domain model

Two schemas in one database: `content` (reference data, replaceable) and `app` (engagement data,
precious). This makes "reinstall ATT&CK v17" a safe operation.

### Engagement structure

```sql
engagement        id, name, client, description, status, starts_on, ends_on,
                  attack_version,            -- PINNED per engagement
                  mode ('standard'|'blind'), created_by, created_at
engagement_member engagement_id, user_id, role ('lead'|'red'|'blue'|'observer')
round             id, engagement_id, ordinal, name, opened_at, closed_at
                  -- round 1 "Baseline", round 2 "Post-remediation", …
scenario          id, engagement_id, ordinal, name, narrative,
                  source ('manual'|'ctid'|'imported'), threat_actor, source_ref
step              id, scenario_id, ordinal, name, objective,
                  technique_id, subtechnique_id, tactic_id,   -- ATT&CK, version-pinned
                  procedure  JSON,   -- {platform, executor, command, cleanup, args}
                  template_id, target_asset, tools[], controls_in_scope[],
                  revealed_at        -- NULL = hidden from blue in blind mode
```

Pinning `attack_version` on the engagement is what stops a report re-mapping itself when ATT&CK
updates mid-engagement — a real hazard with today's live-sync seeder.

### Execution — the retest join

An **execution** is one step run in one round. `(step_id, round_id)` is the grain, so retesting is
free: re-run the step in round 2, and every before/after delta is a self-join.

```sql
execution   id, step_id, round_id,
            -- red
            status ('pending'|'running'|'complete'|'blocked'|'skipped'),
            executed_by, started_at, ended_at, command_run, source_host, target_host, red_notes,
            -- blue (ATT&CK Evaluations vocabulary)
            detection_category ('none'|'telemetry'|'general'|'tactic'|'technique'),
            detection_modifiers[]    -- alert, correlated, delayed, config_change, residual_artifact
            protection ('blocked'|'partial'|'not_blocked'|'n/a'),
            detected_at,             -- MTTD = detected_at − started_at
            detecting_source, detecting_rule_ref, alert_severity, blue_notes,
            scored_by, scored_at
```

`detection_category` is ordinal (`none`=0 … `technique`=4), which makes coverage scoring, heatmap
gradients and round-over-round improvement plain SQL. Outcome stays **derived, never entered** —
same principle as today, richer inputs.

### Supporting entities

```sql
finding      id, engagement_id, title, description, severity, recommendation,
             owner, status ('open'|'in_progress'|'resolved'|'accepted_risk'), created_from_execution
finding_step finding_id, step_id      -- what gets re-run in the next round
evidence     id, sha256, filename, mime, size, caption, side ('red'|'blue'),
             execution_id|comment_id, uploaded_by, uploaded_at   -- content-addressed, dedup for free
comment      id, execution_id, author_id, body, created_at, edited_at
activity     id, engagement_id, actor_id, verb, object_type, object_id, delta JSON, at
             -- append-only; drives the SSE feed AND the report timeline
```

Findings are what make the retest loop meaningful: open a finding from a missed detection, blue
remediates, round 2 re-runs exactly the steps behind open findings, the report shows the delta.

---

## 3. Content system

Reference content is **installable from the UI**, not baked into a seeder. Every source is a row
users can enable, sync, version and disable.

```sql
content_source  id, kind, name, url, ref, enabled, status, last_synced_at, item_count, error
```

| Adapter | Source | Produces |
|---|---|---|
| `attack` | `mitre-attack/attack-stix-data` STIX bundles | tactics, techniques, sub-techniques, data sources, mitigations, groups, software — **stored per version**, multiple versions coexist |
| `ctid` | Center for Threat-Informed Defense adversary emulation library | `emulation_plan` + ordered `emulation_plan_step` → imports as a ready-made Scenario |
| `atomic` | `redcanaryco/atomic-red-team` | `procedure_template` preserving **structure** (platform, executor, command, cleanup, input args) instead of flattening to one `actions` string as today |
| `sigma` | `SigmaHQ/sigma` (+ optional Elastic/ESCU) | `detection_rule_ref` indexed by technique — reference only, never executed or deployed |
| `custom` | User-authored in the UI | Same tables, `source='custom'`, editable and exportable as YAML/JSON |

Each adapter implements one interface (`Fetch → Parse → Normalize → Upsert`), runs as a background
job with progress streamed over SSE, and is resumable. First boot is instant — you install only what
you want. The existing `custom/testcases.json` and `custom/knowledgebase/*.yaml` shapes are supported
as import formats (and this fixes the current bug where the seeder globs `custom/testcases/*.yaml`
while the repo ships `custom/testcases.json`).

---

## 4. API, auth, authorization

### Spec-first

`api/openapi.yaml` is the source of truth. Nothing is hand-written on both sides.

- **Go server** — `oapi-codegen` in **strict mode** (typed request/response structs, no manual
  binding), served on `chi`. Runtime request validation via `kin-openapi` middleware, so the spec is
  enforced, not just documented.
- **TS client** — `openapi-typescript` + `openapi-fetch` for types and calls, wrapped in TanStack
  Query hooks. Contract drift becomes a compile error.
- CI fails if generated code is stale (`make generate && git diff --exit-code`).

### Authorization — one policy, zero per-handler checks

Every failure in §12 of `CURRENT.md` traces to permission logic scattered across handlers. So:

- Two levels: **platform role** (`admin` | `member`) and **engagement role** (`lead` | `red` |
  `blue` | `observer`).
- One function, `authz.Can(ctx, subject, action, resource)`, called from one middleware. No handler
  makes its own role decision.
- **Field safety comes from the schema, not from `if` statements.** Red and blue write through
  separate endpoints with separate request bodies — `PATCH /executions/{id}/execution` (red fields)
  and `PATCH /executions/{id}/detection` (blue fields). A blue user cannot send a red field because
  no such field exists in their request type. This is what today's `testcase.go:190` field-whitelist
  is trying to do, made structural.
- **Blind mode** is enforced in the query layer: unrevealed steps are filtered for blue members at
  the repository boundary, so no endpoint can leak them.
- Table-driven tests assert the full (role × action × resource) matrix, including the two bugs above
  as explicit regression cases.

### Authentication

| Method | Notes |
|---|---|
| Local | Argon2id, login throttling (restoring the rate limiting deleted in the working tree), secure session cookies, rotation on privilege change |
| TOTP | **Admin-enforceable** — today `MFA=True` only redirects users who already enrolled, so anyone who skips `/mfa/register` logs in with a password alone. Adds recovery codes |
| OIDC | Discovery-based (Entra, Okta, Google, Keycloak, Authentik), group→role mapping. Replaces the hand-rolled generic OAuth2 flow |
| SAML 2.0 | Retained for enterprises without OIDC |
| Service tokens | Scoped + expiring, hashed at rest, shown once, **actually enforced on every API route** — today's API keys authenticate nothing |

CSRF: not applicable to token auth; for cookie sessions, `SameSite=Strict` plus a double-submit
token on state-changing routes — properly wired this time, not left as vestigial header plumbing.

---

## 5. Analytics and reporting

### Analytics (DuckDB SQL, not application loops)

Coverage per technique/tactic, detection-category distribution, MTTD percentiles, protection rate,
round-over-round improvement, findings burndown. Exposed as read endpoints and consumed by both the
UI and the report blocks. The ATT&CK Navigator layer export is one query; the hexagon coverage
graphic becomes a proper heatmap.

### Report builder

A **block registry** — each block declares an id, a params schema, a data query, and a renderer.

Built-in blocks: cover page · executive summary · scope & rules of engagement · coverage heatmap ·
per-tactic scorecard · detection-category distribution · scenario walkthrough (narrative + steps +
evidence) · **round comparison / remediation delta** · detection gaps · findings & remediation
backlog · MTTD analysis · evidence appendix · free rich-text.

- Toggle blocks on/off, drag to reorder, configure each (scope to scenarios/rounds, verbosity).
- Save an arrangement as a reusable **report template**; brand it (logo, colours, client name).
- One rendering path: blocks → HTML. That same HTML is the **shareable live report** (signed,
  expiring, revocable share link, optional password) and the input to PDF.
- **PDF** via headless Chromium (`chromedp`) bundled in the Docker image, `CHROME_PATH` for bare
  metal; documented fallback is browser print-to-PDF. One renderer, two outputs — no second layout
  engine to keep in sync.
- Rendered reports are versioned and immutable, so "the report we sent the client" stays reproducible.

Data exports (JSON, CSV, Navigator layer, full engagement archive) carry over — with a round-trip
test, fixing today's `export/entire` writes `export.csv` / `import/entire` reads `export.json` bug.

---

## 6. Repository layout

```
api/openapi.yaml              # source of truth
cmd/purpleops/                # server
cmd/popsctl/                  # admin CLI: user create, content sync, backup, report render
internal/
  config/  store/             # DuckDB: embedded SQL migrations, repositories, serialized writer
  domain/                     # entities + rules: scoring, MTTD, rollups, retest deltas
  content/                    # source registry + attack|atomic|sigma|ctid|custom adapters
  httpapi/                    # generated server, handlers, middleware
  authn/  authz/              # local|totp|oidc|saml|token   ·   central policy
  evidence/                   # content-addressed blob store
  events/                     # SSE hub, presence, activity log
  analytics/  report/         # DuckDB queries   ·   block registry, HTML + PDF renderers
web/                          # React + Vite + TS + shadcn/ui; dist/ embedded via embed.FS
deploy/  docs/
```

**No package-level DB global.** Repositories take an explicit handle via constructor injection —
this structurally removes the `db.Col()` nil-panic class of bug noted in `CURRENT.md` §12.15 and in
project memory.

Migrations: embedded ordered `.sql` files applied by a small in-house migrator with a
`schema_migrations` table (avoids depending on third-party migrate tooling having a DuckDB driver).

---

## 7. Milestones

Each milestone ends green on CI and demoable.

| # | Milestone | Contents |
|---|---|---|
| **M0a** | Clean slate | New branch off `checks`; **delete the entire existing tree**; commit this document as `PLAN.md` as the sole file. See "Clean slate" below for exactly what is removed |
| **M0b** | Foundations | New layout, OpenAPI toolchain and codegen wired, DuckDB store + migrator + serialized writer, config, embedded SPA shell, Docker, CI (build/lint/test/codegen-drift) |
| **M1** | Identity & access | Local + TOTP + recovery codes, OIDC, SAML, service tokens, platform/engagement roles, central `authz`, activity log, full permission-matrix tests |
| **M2** | Content | Source registry + UI install/sync/disable, ATT&CK (versioned) · Atomic · Sigma · CTID adapters, custom content CRUD + import of existing `custom/` formats |
| **M3** | Core domain | Engagements, members, scenarios, steps, rounds, executions, ATT&CK Evals scoring, evidence, comments, findings. Emulation-plan → scenario import |
| **M4** | Collaboration | SSE hub, live step/score updates, presence, blind mode end-to-end |
| **M5** | Analytics | Rollups, coverage heatmap, MTTD, round-over-round deltas, Navigator layer + data exports |
| **M6** | Reporting | Block registry, builder UI, templates, branding, HTML render, share links, PDF. **← first usable release: full loop incl. retest** |
| **M7** | Cutover | Docs/README rewrite, deploy assets, tag the last v1 commit on `main` |

### Clean slate (M0a) — exactly what happens

1. Tag the current v1 tip so the old implementation stays reachable (`git tag v1-final`).
2. Branch: `git switch -c rebuild` off `checks`.
3. Remove **everything**: all tracked files (`git rm -r .`) plus the untracked/ignored ones —
   `files/`, `.env`, `vendor/`, `node_modules/`, and the built `purpleops` / `seed` binaries.
4. Write `PLAN.md` (this document) and commit it as the first commit on the branch.

**Recoverability — read before approving.** Tracked files are safe forever in git history and via
the `v1-final` tag. These are **not** in git and will be gone permanently:

| Path | What it is |
|---|---|
| `files/` | 22 assessment directories of **real evidence** from prior engagements/testing |
| `.env` | Live secrets (`SECRET_KEY`, `PASSWORD_SALT`) for the current deployment |
| `CURRENT.md` | Untracked analysis of the v1 system. Its substance is carried into the Context section above, so this is an acceptable loss — say so if you'd rather it survive as a second file |
| `vendor/`, `node_modules/`, `purpleops`, `seed` | Regenerable build artefacts — no loss |

The Mongo database itself is untouched by this and remains available independently. If any of
`files/` matters, copy it out before approving; the "greenfield, no migration" decision assumes it
does not.

---

## 8. Risks

| Risk | Handling |
|---|---|
| CGO breaks the clean cross-compile story | Accepted and documented up front; CI builds linux/amd64 + linux/arm64 with the right cross-compilers; Docker image is the supported artifact |
| DuckDB write contention under concurrent editing | Single serialized writer; load-test M3 with a simulated war-room (20 users, concurrent scoring) before building on top of it |
| ATT&CK Evaluations vocabulary feels heavy for casual users | UI presents a simple 5-button scale with hover definitions; modifiers are optional and collapsed |
| Report builder scope creep | v1 is section-picker + reorder + rich text only. Data-bound custom query blocks are explicitly out of scope, deferred behind a safe read-only query layer |
| Live upstream content sync shifting mid-engagement | `attack_version` is pinned per engagement; syncs add versions, never mutate the pinned one |
| Rebuild stalls before parity | M6 is the usability bar, not M7 — the branch is mergeable once M6 lands, with old-tree deletion as separate work |

---

## 9. Verification

**Per-layer**

- **Domain** — table-driven unit tests for scoring (all category × modifier × protection
  combinations), MTTD, round deltas. Pure functions, no I/O.
- **Store** — integration tests against a temp DuckDB **file** (milliseconds, no container — a
  significant win over the current Mongo-dependent setup). Migrations tested forward from empty.
- **Authorization** — the full (platform role × engagement role × action × resource) matrix asserted
  in one table, with named regression cases for: non-admin cannot reach user management; observer
  cannot write; blue cannot write red fields; blue cannot see unrevealed steps in blind mode;
  service token cannot exceed its grants or its owner's live permissions.
- **API contract** — server responses validated against `openapi.yaml` in test; CI fails on
  generated-code drift.
- **Content** — adapters tested against checked-in upstream fixtures, so tests don't need network.
- **Report** — golden-file tests on rendered HTML per block; PDF smoke test asserts page count and
  no render errors.

**End-to-end (Playwright)** — one spec that walks the whole product thesis:

1. Admin installs ATT&CK + Atomic content from the UI.
2. Creates an engagement, adds a red and a blue member.
3. Imports a CTID emulation plan as a scenario; adds steps from Atomic templates.
4. Red opens round 1, executes steps, uploads evidence — blue's browser receives the updates over
   SSE without reloading.
5. Blue scores detections in ATT&CK Evaluations terms; a missed detection becomes a finding.
6. Opens round 2, re-runs the finding's steps, scores them higher.
7. Builds a report with the round-comparison block, renders HTML + PDF, opens the share link in a
   fresh context, confirms the delta appears and that revoking the link 404s it.

E2E runs against a seeded DuckDB file in CI. Note that today's `global-setup.ts` **exits 0 and skips
the suite** when nothing answers on `BASE_URL` — the new setup fails loudly instead, so a green run
means the tests actually ran.

**Manual** — `docker compose up` on a clean machine reaches a working login in under a minute with
no network fetch, since content install is now an explicit in-app action.
