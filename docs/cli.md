# `popsctl`, the admin CLI

`popsctl` is the second binary in this repository. It shares the server's packages, its
`PURPLEOPS_*` environment and its database file, and it exists for the things a web interface
cannot do: the first administrator of a fresh deployment, a schema migration you want to watch
happen, and answering "what is actually in this database" without a SQL prompt.

```sh
popsctl --help                 # every command, one line each
popsctl <command> --help       # what that one does, and why it exists
```

It is in the container image, on `PATH`:

```sh
docker compose exec purpleops popsctl version
```

---

## The rules every command follows

**The result goes to stdout. Everything else goes to stderr.** Progress, log lines, warnings and
errors are diagnostics, so a pipe only ever carries the answer.

**`--json` makes the result machine-readable.** Exactly one JSON document on stdout, so
`popsctl db info --json | jq .sizeBytes` works while the log is still on your terminal.

**Exit codes mean something.**

| Code | Meaning | What to do |
|---|---|---|
| `0` | It did what it said | — |
| `1` | It ran and failed | Read the message on stderr; retrying may help |
| `2` | The command line was wrong | Retrying will not help. Fix the typo |

**One process at a time.** DuckDB gives a database file to a single process, so a command against a
deployment whose server is running fails, on purpose:

```
popsctl: store: open database "/var/lib/purpleops/purpleops.duckdb": another process has this
database open: … Conflicting lock is held in /usr/local/bin/purpleops (PID 1) …

DuckDB gives a database file to one process at a time, and something else is holding
/var/lib/purpleops/purpleops.duckdb — normally the server. Stop it and run this again;
with the container image that is:

    docker compose stop
    docker compose run --rm purpleops popsctl <command>
```

Running it inside the running container does *not* dodge the lock — `docker compose exec` is still a
second process. Commands that only read are refused too, because the store opens read-write. That is
the storage engine's rule rather than a policy here; `docs/migrations.md` covers the same ground for
deployments.

## Global flags

| Flag | Default | Notes |
|---|---|---|
| `--db <path>` | `PURPLEOPS_DB_PATH`, then `./purpleops.duckdb` | Which database to work on |
| `--log-level <level>` | `PURPLEOPS_LOG_LEVEL`, then `info` | `debug`, `info`, `warn`, `error` — on stderr |
| `--json` | off | Machine-readable result on stdout |
| `--version` | — | Same output as `popsctl version` |

Only three variables are read: `PURPLEOPS_DB_PATH`, `PURPLEOPS_LOG_LEVEL` and
`PURPLEOPS_LOG_FORMAT`. The rest of `.env.example` is the server's — `popsctl` serves no HTTP and
holds no sessions, so it does not ask you for a base URL or a session secret.

## Commands

### `popsctl version`

The build identity: version, commit, build date. The same three fields the server serves at
`GET /api/v1/version`, which is how you check the CLI you are holding matches the deployment you are
pointing it at.

### `popsctl migrate status`

Every migration this binary carries, and whether this database has applied it.

```
VERSION  NAME  STATUS   APPLIED AT
0001     init  applied  2026-08-01T20:10:26Z

1 of 1 applied, 0 pending.
```

It fails, rather than printing a reassuring table, if the database and the binary disagree about a
migration that has already run — see `docs/migrations.md`.

### `popsctl migrate up`

Applies the pending migrations, in order, each in its own transaction, and prints the status
afterwards. The server does this at startup, so running it by hand is for the deployment where you
want the schema change to happen — and to be seen to happen — before the new binary serves anything.

### `popsctl db info`

Where the database is, how large it and its write-ahead log are, its schema version, and the row
count of every table.

```
path             /var/lib/purpleops/purpleops.duckdb
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

### `popsctl user create`

How the first administrator of a deployment exists at all: there is no sign-up, and the web
interface has nothing to offer somebody who cannot sign in. It is also what the end-to-end suite
seeds its accounts with.

```
popsctl user create --email alice@example.com --name Alice --admin
Password:
Repeat the password:
Created alice@example.com (admin).
```

The password is asked for on the terminal, twice, and not echoed. When stdin is not a terminal it is
read from there instead, once:

```
printf '%s' "$PASSWORD" | popsctl user create --email alice@example.com --name Alice --admin
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

## Commands that are not built yet

They are registered so the shape of the tool is visible from `--help` rather than discovered one
milestone at a time. Each exits 1 and names the milestone that will implement it.

| Command | Arrives in |
|---|---|
| `popsctl content sync` | M2 — content sources |
| `popsctl report render` | M6 — reporting |
| `popsctl backup` | M7 — replaces the manual procedure in `docs/deploy.md` |

## Building it

`make build` produces `bin/popsctl` beside `bin/purpleops`. It carries no frontend, so it needs no
`spa` build tag and no `npm run build` — but it does need cgo, like anything that links DuckDB.

The command tree lives in [`internal/cli`](../internal/cli); `cmd/popsctl` is a `main` over it and
nothing else, so every command is reachable from a test without spawning a process. A new command is
a file in that package and one line in `newRoot`.
