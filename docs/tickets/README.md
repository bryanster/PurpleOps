# Blacklight v2 — Ticket Backlog

Every ticket here derives from [`PLAN.md`](../../PLAN.md). If a ticket and `PLAN.md` disagree,
`PLAN.md` wins — raise it rather than guessing.

## How to read a ticket

Each ticket is one file, named `<ID>-<slug>.md`, and contains:

| Section | Meaning |
|---|---|
| **Why** | The user-visible or structural reason this exists. Read this before the scope. |
| **Scope** | Explicitly in, and explicitly out. Do not widen it — open a follow-up ticket instead. |
| **Files** | Where the code goes. Paths are suggestions with a strong prior; deviating needs a reason. |
| **Acceptance criteria** | Checkable statements. A reviewer runs down this list. |
| **Tests** | What must exist before this is reviewable. |
| **Depends on** | Tickets that must be merged first. |
| **Size** | S ≈ half a day, M ≈ 1–2 days, L ≈ 3–5 days, for someone new to the codebase. |

A finished ticket moves to [`done/`](done/) with its acceptance criteria ticked, and is marked ✅ in
the tables below. Where the implementation had to deviate from the ticket, the reason is appended to
the moved file under **Implementation notes** — read those before starting a ticket that depends
on it.

## Definition of done (applies to every ticket)

A ticket is done when **all** of the following are true. Tickets do not restate these.

1. `make lint test build` is green locally and in CI.
2. `make generate && git diff --exit-code` is clean — no stale generated code.
3. New behaviour has tests at the layer described in the ticket's **Tests** section.
4. No new `TODO`, commented-out code, or `panic()` on a reachable path.
5. No package-level database global. Repositories and services take dependencies via constructor
   arguments (`PLAN.md` §6).
6. Anything a future reader would find surprising is a comment; anything an operator needs is in
   `docs/`.
7. The PR description says what was tested and how, and links this ticket file.

## Conventions the whole backlog assumes

- **Module path** — `github.com/bryanster/blacklight` (unchanged from v1).
- **Spec-first.** `api/openapi.yaml` is edited *before* the Go or TS side. Hand-writing a handler
  signature or a fetch call that isn't in the spec is a review rejection.
- **Errors.** All API errors use the single problem shape defined in `M0B-007`. No ad-hoc JSON.
- **Time.** Store UTC, `TIMESTAMP` columns, serialize as RFC 3339. Never format a time for display
  on the server.
- **IDs.** UUIDv7 (`github.com/google/uuid` v1.6+ `uuid.NewV7`), stored as `TEXT`. Sortable by
  creation, so `ORDER BY id` is a stable tiebreaker.
- **Migrations are append-only.** Never edit a migration that has been merged; add a new one.
- **SQL stays portable ANSI.** No DuckDB-only syntax outside `internal/store/duckdb/`, so the
  escape hatch in `PLAN.md` §1 stays real.

## Milestones

| Milestone | State | Tickets |
|---|---|---|
| M0a — Clean slate | ✅ done on branch `v2` (see note below) | — |
| M0b — Foundations | ✅ done — 14/14 | [14 tickets](#m0b--foundations) |
| M1 — Identity & access | ✅ done — 18/18 | [18 tickets](#m1--identity--access) |
| M2 — Content | ✅ done — 16/16 | [16 tickets](#m2--content) · [`M2-EPIC.md`](done/M2-EPIC.md) |
| M3 — Core domain | epic, needs refinement | [`M3-EPIC.md`](M3-EPIC.md) |
| M4 — Collaboration | epic, needs refinement | [`M4-EPIC.md`](M4-EPIC.md) |
| M5 — Analytics | epic, needs refinement | [`M5-EPIC.md`](M5-EPIC.md) |
| M6 — Reporting | epic, needs refinement | [`M6-EPIC.md`](M6-EPIC.md) |
| M7 — Cutover | epic, needs refinement | [`M7-EPIC.md`](M7-EPIC.md) |

> **M0a note:** the working tree is clean (only `PLAN.md` and `.devcontainer/` remain), but
> **no `v1-final` tag exists yet** — `git tag` returns nothing. `PLAN.md` §7 step 1 requires it.
> Tag the pre-deletion commit on `main` before this branch merges, or the v1 tree is only findable
> by SHA. Tracked as `M7-001`.

---

## M0b — Foundations

Goal: an empty but *correct* skeleton. At the end of M0b, `docker compose up` serves a React page
that calls a generated, spec-validated API endpoint backed by DuckDB, and CI proves it.

Build in this order — the dependency chain is real.

| ID | Title | Size |
|---|---|---|
| [M0B-001](done/M0B-001-repo-skeleton-and-tooling.md) ✅ | Repo skeleton, Go module, Makefile, pinned tooling | M |
| [M0B-002](done/M0B-002-config.md) ✅ | Typed configuration from environment | S |
| [M0B-003](done/M0B-003-duckdb-store-and-serialized-writer.md) ✅ | DuckDB connection, pools, serialized writer | L |
| [M0B-004](done/M0B-004-migrator.md) ✅ | Embedded SQL migrator + `schema_migrations` | M |
| [M0B-005](done/M0B-005-openapi-and-server-codegen.md) ✅ | `api/openapi.yaml` + oapi-codegen strict server | M |
| [M0B-007](done/M0B-007-error-model.md) ✅ | One error/problem model, end to end | S |
| [M0B-006](done/M0B-006-http-server.md) ✅ | chi server, middleware chain, request validation, shutdown | M |
| [M0B-008](done/M0B-008-spa-scaffold.md) ✅ | React + Vite + TS + Tailwind + shadcn/ui scaffold | M |
| [M0B-009](done/M0B-009-typed-api-client.md) ✅ | Generated TS client + TanStack Query wiring | M |
| [M0B-010](done/M0B-010-embed-spa.md) ✅ | Embed `web/dist` in the binary, SPA fallback routing | S |
| [M0B-011](done/M0B-011-docker.md) ✅ | Dockerfile (CGO + Chromium) and compose | M |
| [M0B-012](done/M0B-012-ci.md) ✅ | CI: lint, test, build matrix, codegen-drift gate | M |
| [M0B-013](done/M0B-013-e2e-harness.md) ✅ | Playwright harness that fails loudly | M |
| [M0B-014](done/M0B-014-blctl-skeleton.md) ✅ | `blctl` admin CLI skeleton | S |

## M1 — Identity & access

Goal: nobody can do anything they shouldn't, and there is one place that decides. Every ticket
here traces to a named defect in `PLAN.md` §4 — the regression cases are the point.

| ID | Title | Size |
|---|---|---|
| [M1-001](done/M1-001-identity-schema.md) ✅ | Users, sessions, memberships schema | M |
| [M1-002](done/M1-002-password-hashing.md) ✅ | Argon2id hashing + password policy | S |
| [M1-003](done/M1-003-local-login-sessions.md) ✅ | Local login/logout, session cookies, rotation | L |
| [M1-004](done/M1-004-login-throttling.md) ✅ | Login throttling and lockout | M |
| [M1-005](done/M1-005-csrf.md) ✅ | CSRF double-submit for cookie sessions | M |
| [M1-006](done/M1-006-totp.md) ✅ | TOTP enrolment and verification | M |
| [M1-007](done/M1-007-recovery-codes.md) ✅ | MFA recovery codes | S |
| [M1-008](done/M1-008-mfa-enforcement.md) ✅ | Admin-enforced MFA (closes the skip-enrolment hole) | M |
| [M1-009](done/M1-009-oidc.md) ✅ | OIDC discovery login + group→role mapping | L |
| [M1-010](done/M1-010-saml.md) ✅ | SAML 2.0 service provider | L |
| [M1-011](done/M1-011-service-tokens.md) ✅ | Scoped service tokens, actually enforced | L |
| [M1-012](done/M1-012-authz-policy.md) ✅ | Central `authz.Can` policy engine | L |
| [M1-013](done/M1-013-authz-middleware.md) ✅ | One authorization middleware, zero handler checks | M |
| [M1-014](done/M1-014-permission-matrix-tests.md) ✅ | Full role × action × resource matrix tests | M |
| [M1-015](done/M1-015-activity-log.md) ✅ | Append-only activity log | M |
| [M1-016](done/M1-016-user-management-api.md) ✅ | Admin user management API | M |
| [M1-017](done/M1-017-auth-ui.md) ✅ | Login, MFA, account and admin UI | L |
| [M1-018](done/M1-018-admin-token-management.md) ✅ | Administrative service token management | S |

## M2 — Content

Goal: reference content is installable from the UI, not seeded as a 1 GB git clone. Sources are
enabled, synced, versioned and disabled; ATT&CK versions coexist; Atomic structure is preserved.
Decisions are locked in [`M2-EPIC.md`](done/M2-EPIC.md).

Build roughly in this order — the dependency chain is real.

| ID | Title | Size |
|---|---|---|
| [M2-001](done/M2-001-content-schema.md) ✅ | `content` schema: sources, versions, jobs, raw snapshots, seed rows | L |
| [M2-002](done/M2-002-source-registry-api.md) ✅ | Source registry API, enable/disable/delete, authz actions | M |
| [M2-003](done/M2-003-adapter-and-job-runner.md) ✅ | Adapter interface + global DB-backed job runner | L |
| [M2-004](done/M2-004-sse-hub.md) ✅ | Minimal shared SSE hub + sync progress | L |
| [M2-005](done/M2-005-bundle-upload-and-reprocess.md) ✅ | Offline bundle upload + reprocess-from-raw | M |
| [M2-006](done/M2-006-attack-adapter.md) ✅ | ATT&CK Enterprise adapter (multi-version) | L |
| [M2-007](done/M2-007-attack-version-pin-surface.md) ✅ | ATT&CK version catalog & pin surface | M |
| [M2-008](done/M2-008-atomic-adapter.md) ✅ | Atomic Red Team adapter | M |
| [M2-009](done/M2-009-sigma-adapter.md) ✅ | Sigma adapter (technique-mapped rules) | M |
| [M2-010](done/M2-010-ctid-adapter.md) ✅ | CTID emulation-plan catalog adapter | M |
| [M2-011](done/M2-011-custom-content-api.md) ✅ | Custom content API: templates, rules, notes | M |
| [M2-012](done/M2-012-v1-format-import.md) ✅ | Import v1 `testcases.json` + knowledgebase YAML | M |
| [M2-013](done/M2-013-library-browser-ui.md) ✅ | Content library browser UI | L |
| [M2-014](done/M2-014-sources-admin-ui.md) ✅ | Sources admin UI: sync, bundle, status, reprocess | L |
| [M2-015](done/M2-015-custom-and-import-ui.md) ✅ | Custom editor + v1 import UI | M |
| [M2-016](done/M2-016-sync-write-load-test.md) ✅ | Sync write load test (serialized writer fairness) | M |
