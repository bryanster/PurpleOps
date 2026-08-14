# Changelog

All notable changes to Blacklight are documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **Importing a step from a template works again.** From Template in an
  engagement workbook hung for thirty seconds and then answered 500, every time
  — the step was never created. The activity entry recorded alongside the new
  step opened a second write transaction from inside the one creating it, and
  the store serializes writers: the recording queued behind the transaction that
  was waiting on it until the request deadline fired, and the commit then failed
  on a transaction the cancelled context had already rolled back. Recording now
  happens on the caller's transaction, as the hook was designed to, so the step
  and its activity row share one commit. Patching a step and patching a scenario
  recorded the same broken way and were failing identically; both are fixed with
  it.
- **The CTID content pack syncs again.** Its source is a GitHub archive of the
  whole `adversary_emulation_library` repository, which passed 512 MiB and so
  failed every sync with `download exceeds content max bytes limit`. The
  adapter no longer downloads it: it reads the repository listing, fetches only
  the plan files it actually parses, and reassembles them into a small zip with
  the archive's layout — about 75 KB moved instead of 640 MB, in a few seconds.
  Mirrors and offline bundles are unaffected, and the raw snapshot's digest now
  changes when a plan changes rather than on every upstream commit. See
  `docs/content-ctid.md`.
- **Administrators can edit engagements again.** An administrator opening an
  engagement got no workbook toolbar — no Add Scenario, Add Step, Import CTID or
  From Template — read-only red and blue execution editors, and no way to raise
  a finding. The server had always allowed all of it (every engagement-scoped
  rule grants `Platform: admins`); only the interface disagreed, so the buttons
  were absent rather than refused. The interface now reads the caller's actual
  seat first, falling back to the platform seat only for an administrator who
  holds no membership, and its permission predicates match the server's table
  for every role. This bit hardest on a fresh install, where the bootstrapped
  first account is an administrator and there is nobody else to be a lead yet.

## [1.0.1] — 2026-08-14

### Added

- **First administrator from the environment.** `BLACKLIGHT_BOOTSTRAP_ADMIN_EMAIL`,
  `BLACKLIGHT_BOOTSTRAP_ADMIN_NAME` and one of
  `BLACKLIGHT_BOOTSTRAP_ADMIN_PASSWORD_FILE` / `BLACKLIGHT_BOOTSTRAP_ADMIN_PASSWORD`
  create that account at startup — but only on a database with no accounts at
  all, so the configuration is inert on every start afterwards and can never
  change an account that exists. It is for deployments where `blctl user create`
  means stopping the server to get at the database (Container Apps, Cloud Run,
  ECS); keep using the CLI everywhere else.
- **Azure Terraform example creates the first administrator.** Set `admin_email`
  and the configuration generates a password, stores it in Key Vault write-only
  beside the other two secrets, and passes it to the app;
  `terraform output admin_password_command` prints the command that reads it
  back.

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

[1.0.1]: https://github.com/bryanster/blacklight/releases/tag/v1.0.1
[1.0.0]: https://github.com/bryanster/blacklight/releases/tag/v1.0.0
