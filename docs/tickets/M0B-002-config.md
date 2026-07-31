# M0B-002 — Typed configuration from environment

**Milestone:** M0b · **Size:** S · **Depends on:** M0B-001

## Why

v1 read `os.Getenv` at call sites, so a missing secret surfaced as a runtime nil or a silently
insecure default. Configuration should be parsed once, validated once, and passed down as a value.
A misconfigured server must refuse to start rather than start wrong.

## Scope

**In**

- `internal/config.Config` — a nested struct, loaded by `config.Load() (Config, error)`.
- Environment variables only (no config file). Prefix `PURPLEOPS_`.
- Validation at load: required values present, ports in range, paths writable, durations positive.
- `.env.example` documenting every variable with a comment and a safe default.
- Redacted `String()` on any secret-bearing type so a config dump can't leak into logs.

**Out**

- Anything that reads the config. Consumers arrive with their own tickets.
- Hot reload. Restart is the reload.

## Config surface

Model it on this; extend as later tickets need, always with validation.

| Variable | Type | Default | Required | Notes |
|---|---|---|---|---|
| `PURPLEOPS_ADDR` | string | `:8080` | no | Listen address |
| `PURPLEOPS_BASE_URL` | URL | — | **yes** | Public URL. OIDC/SAML redirects and share links need it. Must be absolute, no trailing slash |
| `PURPLEOPS_DB_PATH` | path | `./purpleops.duckdb` | no | Parent dir must exist and be writable |
| `PURPLEOPS_EVIDENCE_DIR` | path | `./evidence` | no | Created at startup if absent |
| `PURPLEOPS_SESSION_SECRET` | secret | — | **yes** | ≥32 bytes after decoding. Reject known-weak values |
| `PURPLEOPS_LOG_LEVEL` | enum | `info` | no | `debug\|info\|warn\|error` |
| `PURPLEOPS_LOG_FORMAT` | enum | `json` | no | `json\|text` |
| `PURPLEOPS_ENV` | enum | `production` | no | `development\|production`. Only this may relax cookie `Secure` |
| `PURPLEOPS_CHROME_PATH` | path | — | no | For M6 PDF rendering |
| `PURPLEOPS_SHUTDOWN_TIMEOUT` | duration | `15s` | no | |

## Acceptance criteria

- [ ] `config.Load()` returns a descriptive error naming the offending variable — e.g.
      `PURPLEOPS_BASE_URL: must be an absolute URL, got "localhost:8080"`. Never a bare
      `invalid config`.
- [ ] All required variables missing → the error lists **all** of them, not just the first.
- [ ] `PURPLEOPS_ENV=production` with `PURPLEOPS_BASE_URL` on plain `http://` is a startup error
      (except for `localhost`), because secure cookies will not work.
- [ ] A secret's value never appears in `fmt.Sprintf("%v", cfg)` or in any log line.
- [ ] `.env.example` lists every variable in the table, and a test fails if a `Config` field has no
      corresponding line in `.env.example`.
- [ ] Zero use of `os.Getenv` outside `internal/config` (enforced by a lint rule or a test that
      greps the tree — either is acceptable, but it must be automated).

## Tests

- Table-driven: for each field, one valid case and one invalid case, asserting on the error text.
- A test that walks `Config` by reflection and asserts every field is documented in `.env.example`.
