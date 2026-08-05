# `blctl`, the admin CLI

`blctl` is the second binary in this repository. It shares the server's packages, its
`BLACKLIGHT_*` environment and its database file, and it exists for the things a web interface
cannot do: the first administrator of a fresh deployment, a schema migration you want to watch
happen, and answering "what is actually in this database" without a SQL prompt.

```sh
blctl --help                 # every command, one line each
blctl <command> --help       # what that one does, and why it exists
```

It is in the container image, on `PATH`:

```sh
docker compose exec blacklight blctl version
```

---

## The rules every command follows

**The result goes to stdout. Everything else goes to stderr.** Progress, log lines, warnings and
errors are diagnostics, so a pipe only ever carries the answer.

**`--json` makes the result machine-readable.** Exactly one JSON document on stdout, so
`blctl db info --json | jq .sizeBytes` works while the log is still on your terminal.

**Exit codes mean something.**

| Code | Meaning | What to do |
|---|---|---|
| `0` | It did what it said | — |
| `1` | It ran and failed | Read the message on stderr; retrying may help |
| `2` | The command line was wrong | Retrying will not help. Fix the typo |

**One process at a time.** DuckDB gives a database file to a single process, so a command against a
deployment whose server is running fails, on purpose:

```
blctl: store: open database "/var/lib/blacklight/blacklight.duckdb": another process has this
database open: … Conflicting lock is held in /usr/local/bin/blacklight (PID 1) …

DuckDB gives a database file to one process at a time, and something else is holding
/var/lib/blacklight/blacklight.duckdb — normally the server. Stop it and run this again;
with the container image that is:

    docker compose stop
    docker compose run --rm blacklight blctl <command>
```

Running it inside the running container does *not* dodge the lock — `docker compose exec` is still a
second process. Commands that only read are refused too, because the store opens read-write. That is
the storage engine's rule rather than a policy here; `docs/migrations.md` covers the same ground for
deployments.

## Global flags

| Flag | Default | Notes |
|---|---|---|
| `--db <path>` | `BLACKLIGHT_DB_PATH`, then `./blacklight.duckdb` | Which database to work on |
| `--log-level <level>` | `BLACKLIGHT_LOG_LEVEL`, then `info` | `debug`, `info`, `warn`, `error` — on stderr |
| `--json` | off | Machine-readable result on stdout |
| `--version` | — | Same output as `blctl version` |

Only three variables are read: `BLACKLIGHT_DB_PATH`, `BLACKLIGHT_LOG_LEVEL` and
`BLACKLIGHT_LOG_FORMAT`. The rest of `.env.example` is the server's — `blctl` serves no HTTP and
holds no sessions, so it does not ask you for a base URL or a session secret.

## Commands

### `blctl version`

The build identity: version, commit, build date. The same three fields the server serves at
`GET /api/v1/version`, which is how you check the CLI you are holding matches the deployment you are
pointing it at.

### `blctl migrate status`

Every migration this binary carries, and whether this database has applied it.

```
VERSION  NAME  STATUS   APPLIED AT
0001     init  applied  2026-08-01T20:10:26Z

1 of 1 applied, 0 pending.
```

It fails, rather than printing a reassuring table, if the database and the binary disagree about a
migration that has already run — see `docs/migrations.md`.

### `blctl migrate up`

Applies the pending migrations, in order, each in its own transaction, and prints the status
afterwards. The server does this at startup, so running it by hand is for the deployment where you
want the schema change to happen — and to be seen to happen — before the new binary serves anything.

### `blctl db info`

Where the database is, how large it and its write-ahead log are, its schema version, and the row
count of every table.

```
path             /var/lib/blacklight/blacklight.duckdb
size             780.0 KiB  (798720 bytes)
write-ahead log  0 B        (0 bytes)
schema version   0001 of 0001

TABLE                   ROWS
main.schema_migrations  1
```

This is the command to run before and after anything destructive, and the one to paste into a bug
report. Counting rows reads every table.

> Both `migrate` and `db info` open the database **read-write**, and create it if it is not there.
> A typo in `--db` makes an empty database rather than an error.

### `blctl user create`

How the first administrator of a deployment exists at all: there is no sign-up, and the web
interface has nothing to offer somebody who cannot sign in. It is also what the end-to-end suite
seeds its accounts with.

```
blctl user create --email alice@example.com --name Alice --admin
Password:
Repeat the password:
Created alice@example.com (admin).
```

The password is asked for on the terminal, twice, and not echoed. When stdin is not a terminal it is
read from there instead, once:

```
printf '%s' "$PASSWORD" | blctl user create --email alice@example.com --name Alice --admin
```

One trailing newline is stripped, so `echo` works too. **There is no `--password` flag and no
environment variable**: both end up in shell history, in `ps`, and in whatever collects the logs.

| Flag | |
|---|---|
| `--email` | required. Matched without regard to case; stored as typed, for display |
| `--name` | required. The name shown for this person |
| `--admin` | make them a platform administrator. Without it they are a member |

The password is held to the same policy the API applies — at least 12 characters, at most 128, and
not one attackers try first — and the account is created active, with a local password login. The
database must already be migrated; the command says so rather than failing with the driver's
complaint about a missing table.

### `blctl user reset-mfa`

The break-glass path (`M1-007`). It removes an account's authenticator enrolment, every recovery
code it holds and any half-finished sign-in, so that whoever lost their phone can get back in.

```
blctl user reset-mfa --email alice@example.com
Reset the second factor of alice@example.com.

id                     0192f3c4-5d6e-7f80-9123-456789abcdef
authenticator          removed
unused recovery codes  7 codes removed

WARNING: alice@example.com now signs in with a password and nothing else.
Anyone holding that password holds the account. Have them enrol an
authenticator again as soon as they are back in, and take a new set of
recovery codes when they do — the old ones no longer work.
```

**What it leaves behind depends on the policy** (`M1-008`). The warning above is what an account
nothing requires a factor of gets: a password is now sufficient, which is the lock being broken. An
account the platform policy or its own `mfa_enforced` flag still covers gets the opposite note —
its next sign-in can do exactly one thing, which is enrol a new authenticator. This command touches
the factor and nothing else: not the password, not the flag, not the platform policy, not any live
session. So it turns "locked out" into "enrol again", and it never quietly relaxes enforcement.

**Reach for the recovery codes first.** They were issued when the authenticator was enrolled,
they need nobody's help, and using one produces a fully signed-in session from which the person can
enrol a new device and take a fresh set. This command is for when those are gone too.

It is genuinely a lock being broken: the point of a second factor is that a password is not enough,
and this makes it enough. It exists because the alternative in a self-hosted, single-tenant tool is
worse — a lost phone belonging to the only administrator would otherwise mean editing the database
by hand or reinstalling. There is deliberately **no API for it**: needing the database file means
needing the host, and that is the access control. An endpoint that strips somebody's second factor
is an endpoint worth attacking.

| Flag | |
|---|---|
| `--email` | required. The account to reset, matched without regard to case |

It does not touch the password, the role or `mfa_enforced`, and it does not sign anybody out — an
account an administrator requires MFA of is still required to have it, and will be walked through
enrolling again (`M1-008`). An account that had no second factor is not an error: it reports that
nothing was removed. The reset is also written to the log at warn level, which is the audit record
until `M1-015` gives it a durable home.

### `blctl content sources`

Lists every content source (kind, enabled, status, item count), or shows one
with `--id`. Filter with `--kind` and/or `--enabled=true|false`.

```sh
blctl content sources
blctl content sources --kind attack
blctl content sources --id 01900000-0000-7000-8000-000000000001 --json
```

### `blctl content enable` / `blctl content disable`

Flip the soft switch on a source. Idempotent. Prefer disable over delete for
builtin upstream seeds — delete is available over the API and is permanent.

```sh
blctl content enable --id 01900000-0000-7000-8000-000000000001
blctl content disable --id 01900000-0000-7000-8000-000000000001
```

### `blctl content sync`

Enqueues an online sync job for a source (`--source` id or kind). Optional
`--version` pins an ATT&CK release. `--wait` blocks until the job finishes.

```sh
blctl content sync --source atomic --wait
blctl content sync --source attack --version 15.1 --wait
```

Production adapters land with `M2-006`…; until then a sync of a seeded kind
fails with "no adapter registered".

### `blctl content import-bundle`

Offline install from a release archive on disk. Same parse path as online sync;
no network. See [`docs/content-bundles.md`](content-bundles.md).

```sh
blctl content import-bundle --source atomic --file ./atomics.zip --wait
```

### `blctl content reprocess`

Re-parse the last successful raw snapshot for a source/version (no download).
ATT&CK requires `--version`. Fails if no raw snapshot exists.

```sh
blctl content reprocess --source atomic --wait
blctl content reprocess --source attack --version 15.1 --wait
```

## Commands that are not built yet

They are registered so the shape of the tool is visible from `--help` rather than discovered one
milestone at a time. Each exits 1 and names the milestone that will implement it.

| Command | Arrives in |
|---|---|
| `blctl report render` | M6 — reporting |
| `blctl backup` | M7 — replaces the manual procedure in `docs/deploy.md` |

## Building it

`make build` produces `bin/blctl` beside `bin/blacklight`. It carries no frontend, so it needs no
`spa` build tag and no `npm run build` — but it does need cgo, like anything that links DuckDB.

The command tree lives in [`internal/cli`](../internal/cli); `cmd/blctl` is a `main` over it and
nothing else, so every command is reachable from a test without spawning a process. A new command is
a file in that package and one line in `newRoot`.
