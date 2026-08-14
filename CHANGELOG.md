# Changelog

All notable changes to Blacklight are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] — 2026-08-14

> **Greenfield only.** Blacklight v1.0.0 is a ground-up rebuild of the prior
> Python/Mongo application. There is **no migration path** from the old stack —
> a new installation starts with an empty database. The annotated tag `v1-final`
> exists on the pre-rebuild tree for anyone who needs the historical source.
> Back up the v1 deployment independently if its data is still needed.

### Added

- **Container image** as the supported deployment artifact — multi-arch
  (`linux/amd64`, `linux/arm64`), Debian-based, Chromium included for PDF
  rendering. Pull from `ghcr.io/bryanster/blacklight`.
- **Identity & access:** local accounts with Argon2id password hashing, TOTP
  multi-factor authentication with recovery codes, admin-enforced MFA, OIDC and
  SAML 2.0 single sign-on, scoped service tokens, CSRF protection, login
  throttling, and a central authorization policy engine with full role×action
  matrix tests.
- **Content library:** ATT&CK Enterprise (multi-version), Atomic Red Team,
  Sigma rules, and CTID emulation plans — synced from upstream or uploaded as
  offline bundles, with version pinning and a custom content API.
- **Core domain:** Engagements → Scenarios → Steps → Executions, scored in
  ATT&CK Evaluations terms (detection categories, modifiers, MTTD), with
  evidence uploads, threaded comments, and findings tracking.
- **Collaboration:** real-time war room over SSE with per-topic authorization,
  presence, live workbook updates, comment threads, activity rail, and blind
  mode for red/blue separation.
- **Analytics:** coverage rollups, detection distribution, protection rate,
  MTTD percentiles, findings burndown, cross-engagement comparison — all
  computed in SQL, served through a single query layer consumed by both the
  dashboard and report blocks.
- **Reporting:** block-registry report builder with narrative blocks, analytics
  blocks, detail blocks, rich-text editing, branded HTML rendering, PDF via
  headless Chromium, immutable published versions, and share links with optional
  password gates.
- **Admin CLI** (`blctl`) for migrations, user creation, content sync, and
  database inspection.
- **End-to-end test suite** (Playwright) that drives the real binary against a
  real database.

### Upgrade notes

1. **Read the release notes** for the version you are moving to. Breaking
   changes, new required environment variables, and migration counts are listed
   there.
2. **[Back up](../docs/deploy.md#backup-and-restore) your deployment.** Migrations
   are forward-only; the only path back to the previous release is restore from
   backup.
3. **Stop the server.** `docker compose stop` — DuckDB admits one process per
   file.
4. **Pull the new image.** `docker compose pull` (or `docker compose build` if
   building from source).
5. **Start.** `docker compose up -d`. Migrations run at startup.
6. **Verify.** Open `$BLACKLIGHT_BASE_URL` and sign in.

If the new release does not start, restore the backup and use the previous
image tag. A database that has been migrated forward cannot be opened by an
older binary.

### Changed

- The entire application was rebuilt from scratch in Go + React + TypeScript +
  DuckDB. The prior Python/Flask/MongoDB codebase is retired; its final state
  is tagged `v1-final`.

### Removed

- **MongoDB.** Blacklight stores everything in a single DuckDB file. No
  separate database process, no connection strings.
- **Retest rounds.** v1's round-over-round workflow is replaced by
  cross-engagement comparison. Operators recreate assessments rather than
  re-running steps within a locked structure.
- **Static asset directory.** The binary is the whole application; nothing is
  served from disk.

[1.0.0]: https://github.com/bryanster/blacklight/releases/tag/v1.0.0
