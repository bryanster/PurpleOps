# Blacklight

[![CI](https://github.com/bryanster/blacklight/actions/workflows/ci.yml/badge.svg)](https://github.com/bryanster/blacklight/actions/workflows/ci.yml)

Blacklight is a self-hosted purple-team assessment tool. Red and blue work through the same
engagement — an ordered chain of ATT&CK-mapped scenarios and steps — recording what was executed,
what was prevented, what was detected and how quickly, and what changed on retest after remediation.
The output is a report.

## How it works

1.  **Baseline engagement.** Map your adversary emulation plan to scenarios and ATT&CK-mapped steps.
    Record executions, attach evidence, and score detection — red and blue fill in the same workbook
    as the assessment runs.
2.  **Findings.** Open findings from missed detections and gaps. Blue remediates.
3.  **Retest engagement.** Run a second engagement targeting the same techniques, re-importing or
    re-creating the steps behind open findings.
4.  **Compare.** Cross-engagement comparison shows exactly what improved — coverage, detection
    categories, MTTD, and protection rate — and the report prints the delta.

The entire loop runs from one binary with an embedded database, an embedded React SPA, and an
OpenAPI-described API. Nothing is fetched at first boot.

## Quickstart

```sh
git clone https://github.com/bryanster/blacklight
cd blacklight
docker compose up --build
```

Then open <http://localhost:8080>. The first build takes a few minutes; the first boot takes seconds.

That works with no configuration — fine for a laptop, not fine for anything else. Before anyone else
can reach it, read [`docs/deploy.md`](docs/deploy.md): at minimum you must set
`BLACKLIGHT_BASE_URL`, `BLACKLIGHT_SESSION_SECRET` and `BLACKLIGHT_ENCRYPTION_KEY`.

## What you get

- **Single binary** (Go, CGO). The container image is the supported artefact: `linux/amd64` and
  `linux/arm64`.
- **Embedded DuckDB.** No Postgres, no Mongo, no Redis. One database file plus a WAL.
- **Evidence on disk.** Content-addressed blob store; back it up with the database.
- **Full OpenAPI API.** Spec-first, strict-mode server, typed TypeScript client. External automation
  authenticates with scoped, expiring service tokens.
- **Authentication.** Local accounts with Argon2id passwords and admin-enforceable TOTP. OIDC and
  SAML 2.0 for enterprise single sign-on, with group-to-role mapping.
- **Admin CLI.** `blctl` ships in the image for migrations, user creation, content management, and
  backups — [`docs/cli.md`](docs/cli.md).
- **SSE live updates.** Shared war room: presence, live workbook, comment threads, activity rail,
  blind mode.

## Docs

| You want to… | Read |
|---|---|
| Deploy or configure | [`docs/deploy.md`](docs/deploy.md) |
| Understand the security model | [`docs/security.md`](docs/security.md) |
| Set up SSO (OIDC / SAML) | [`docs/sso-oidc.md`](docs/sso-oidc.md) · [`docs/sso-saml.md`](docs/sso-saml.md) |
| Use the API | [`docs/api.md`](docs/api.md) |
| Manage service tokens | [`docs/api-tokens.md`](docs/api-tokens.md) |
| Use the admin CLI | [`docs/cli.md`](docs/cli.md) |
| Understand authorization | [`docs/authz.md`](docs/authz.md) |
| Run content sync / work with ATT&CK | [`docs/content-attack.md`](docs/content-attack.md) |
| Contribute | [`docs/contributing.md`](docs/contributing.md) |
| Run or write tests | [`docs/testing.md`](docs/testing.md) |

Design and backlog live in [`PLAN.md`](PLAN.md) and [`docs/tickets/`](docs/tickets/).

## Status

Blacklight is a ground-up rebuild of the prior Python/Mongo codebase, approaching its first stable
release. The product loop — baseline, retest, compare, report — is implemented and end-to-end tested.
A pre-release status banner may remain on in-depth docs pages until `v1.0.0` is tagged; the
quickstart path above is the supported install.

## Licence

Apache 2.0 — see [`LICENSE`](LICENSE).

Blacklight is an independent rewrite. It began as a fork of
[PurpleOps](https://github.com/CyberCX-STA/PurpleOps) (Copyright 2023 Willem Mouton & Harrison
Mitchell, Apache-2.0); no code from that project remains.
