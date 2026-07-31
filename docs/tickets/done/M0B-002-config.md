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

- [x] `config.Load()` returns a descriptive error naming the offending variable — e.g.
      `PURPLEOPS_BASE_URL: must be an absolute URL, got "localhost:8080"`. Never a bare
      `invalid config`.
- [x] All required variables missing → the error lists **all** of them, not just the first.
- [x] `PURPLEOPS_ENV=production` with `PURPLEOPS_BASE_URL` on plain `http://` is a startup error
      (except for `localhost`), because secure cookies will not work.
- [x] A secret's value never appears in `fmt.Sprintf("%v", cfg)` or in any log line.
- [x] `.env.example` lists every variable in the table, and a test fails if a `Config` field has no
      corresponding line in `.env.example`.
- [x] Zero use of `os.Getenv` outside `internal/config` (enforced by a lint rule or a test that
      greps the tree — either is acceptable, but it must be automated).

## Tests

- Table-driven: for each field, one valid case and one invalid case, asserting on the error text.
- A test that walks `Config` by reflection and asserts every field is documented in `.env.example`.

---

## Implementation notes (added on completion)

### Shape

`Config` is nested by subject (`Server`, `Database`, `Evidence`, `Session`, `Log`, `Report`) plus a
top-level `Env`, so a constructor can take `cfg.Database` rather than the world.

**There are no struct tags.** `Config.bindings()` is an explicit list of
`{name, target, default, required, sensitive}`, and it is the single source of truth for what the
process reads. Reflection lives only in the tests: `TestEveryConfigFieldIsBound` walks `Config`,
matches each leaf field against the binding list by pointer identity, and fails on a field nothing
fills. A tag-driven loader would have put the same information in two places and needed the same
test anyway.

Loading is three phases, and the order is load-bearing:

1. `parse` — bind and parse from the environment map. Pure.
2. `validate` — cross-field and read-only filesystem checks. Pure.
3. `ensurePaths` — the only step that changes the machine (creates the evidence directory, probes
   writability by actually writing).

Phases 1 and 2 report together, so an operator sees every parse problem at once. Phase 3 runs only
after those pass, so a **rejected configuration never leaves a directory behind**
(`TestLoadCreatesNothingWhenTheConfigIsInvalid`). The cost is that a bad URL and an unwritable data
directory take two restarts to find; leaving litter on a failed boot was judged worse.

### Decisions the ticket did not fix

1. **A trailing slash on `PURPLEOPS_BASE_URL` is stripped, not rejected.** The ticket says "no
   trailing slash"; the stored value never has one either way. Browsers add the slash when you copy
   an address bar, and every consumer joins paths onto this value, so `…/x` and `…/x/` must not be
   able to produce two different redirect URIs. Credentials, a query string and a fragment *are*
   rejected — those cannot be normalised away safely.
2. **"≥32 bytes after decoding" is implemented as the smallest plausible reading of the value.**
   `secretBytes` takes the minimum of the raw length and every base64/hex decoding that succeeds,
   because an encoding always expands: `openssl rand -base64 32` is 44 characters carrying 32 bytes,
   and a 40-character base64 string carries 30 however long it looks. Consumers get the raw bytes
   via `Secret.Reveal()` — the value is used as configured, only *measured* through the decoders.
3. **"Reject known-weak values" is a substring denylist plus a distinct-character floor** (8). The
   denylist catches shipped placeholders (`changeme`, `replace-me`, `example`, …); the floor catches
   `aaaa…`, which passes any length check. The placeholder in `.env.example` is deliberately one of
   the rejected values, and `TestEnvExampleIsRejectedOnlyForItsPlaceholderSecret` asserts that an
   untouched copy of the template fails **only** on the secret — which also proves the rest of the
   template is valid.
4. **`PURPLEOPS_ADDR` accepts port 0** (ask the kernel for a free port). `M0B-013`'s harness and any
   test that needs a real listener will want it.
5. **Set-but-empty is treated as unset**, and every value is trimmed of surrounding whitespace: an
   empty variable is what a compose file produces for something nobody filled in, and a trailing
   space in a `.env` file is a typo, not key material. A required variable that is set-but-empty is
   still `must be set`.
6. **The `os.Getenv` ban is a test, not a lint rule** (the ticket allows either).
   `TestOnlyConfigReadsTheEnvironment` parses every `.go` file in the tree with `go/ast` and rejects
   `os.Getenv`, `os.LookupEnv`, `os.Environ` and `os.ExpandEnv` outside `internal/config`. AST, not
   grep, so a mention in a comment or a string is not a false positive. It was verified to fail by
   adding a violation to `cmd/popsctl`.

### For the tickets that consume this

- `Secret` redacts through `String`, `GoString`, `MarshalJSON` **and** `slog.LogValuer`, so `%v`,
  `%#v`, a JSON dump and a `slog` attribute are all safe. Reaching the bytes needs
  `Secret.Reveal()`, which is greppable in review — treat a new call site as a security change.
- `cfg.Env.IsDevelopment()` is the *only* switch that may relax a security control (`PLAN.md` §4).
  `M1-003`'s cookie `Secure` flag is its first consumer.
- `LogLevel.Slog()` maps onto `log/slog` and lives with the enum, so `M0B-006` does not re-derive
  it. This is the one piece of "reading the config" kept in this ticket, on the grounds that a
  validated enum nothing can convert is not usable.
- `Server.BaseURL` is a `config.URL` wrapping `*url.URL`; the embedded pointer is exposed, so
  anything needing the stdlib type gets it, and `JoinPath` works directly on it.
- Nothing calls `config.Load()` yet — wiring it into `cmd/purpleops` belongs to `M0B-006`, which
  owns process startup.
