# M0B-014 — `popsctl` admin CLI skeleton

**Milestone:** M0b · **Size:** S · **Depends on:** M0B-004

## Why

`PLAN.md` §6 specifies an admin CLI for user creation, content sync, backup and report render. Two
of those are needed almost immediately: the **first admin user** has to be creatable without a
running UI (M1), and E2E seeding (`M0B-013`) should go through supported commands rather than
poking the database.

Building the skeleton now means later tickets add a subcommand instead of inventing a CLI.

## Scope

**In**

- `cmd/popsctl` using the same config loading as the server, sharing `internal/*` packages — one
  codebase, two entrypoints.
- Command framework (`cobra` or `urfave/cli` — pick one, note why) with `--help` on every command.
- Commands implemented now:
  - `popsctl version`
  - `popsctl migrate status` — list applied/pending migrations
  - `popsctl migrate up` — apply pending
  - `popsctl db info` — path, size, schema version, row counts per table
- Registered-but-stubbed commands that exit non-zero with "not implemented in this milestone", so
  the surface is visible: `user create`, `content sync`, `backup`, `report render`.
- Global flags: `--db`, `--log-level`, `--json` (machine-readable output).
- `make build` produces `bin/popsctl` alongside the server; the Docker image includes it.

**Out**

- Any command whose feature doesn't exist yet.

## Acceptance criteria

- [x] `popsctl --help` lists every command with a one-line description.
- [x] `popsctl migrate status` on a fresh DB lists all migrations as pending; after `migrate up`,
      all as applied, with timestamps.
- [x] Commands respect `--json` and emit parseable output on stdout, with human/log output on stderr,
      so `popsctl db info --json | jq` works.
- [x] Exit codes are meaningful: 0 success, 1 runtime failure, 2 usage error.
- [x] The CLI refuses to open a database the server currently holds, with a clear message — DuckDB's
      single-writer rule (`M0B-003`) makes this a guaranteed confusion otherwise. Test it.
- [x] Stub commands exit 1 with a message naming the milestone that will implement them.
- [x] `popsctl` is in the Docker image and `docker compose exec app popsctl version` works.

## Tests

- Integration tests invoking the command functions directly (not by spawning a process) against a
  temp database: `migrate status` before/after, `db info` output shape.
- A test asserting `--json` output is valid JSON for every command that supports it.

---

## Implementation notes

### cobra, and where the tree lives

The ticket offered cobra or urfave/cli. It is **cobra** (v1.10, plus pflag): the subcommand tree,
per-command `--help` and shell completion are all things this would otherwise grow by hand as M1
through M7 add commands, and cobra's conventions are the ones an operator's fingers already know.
Two dependencies, both of which only `internal/cli` imports — the server does not.

The command tree is `internal/cli`, not `package main`. `cmd/popsctl` is thirty lines: a signal
handler and `os.Exit(cli.Main(...))`. That is what makes the tests in the ticket possible — they
call `cli.Main` with buffers and read the exit code back as a value, so nothing is spawned and the
coverage profile means something.

### The CLI does not load the server's configuration

The scope says "the same config loading as the server", and it very nearly is: the same
`internal/config`, the same variables, the same parsers. But `config.Load` requires
`PURPLEOPS_BASE_URL` and `PURPLEOPS_SESSION_SECRET`, and demanding a session secret before
`popsctl db info` will open a file is a checklist with nothing behind it — an operator who hits it
once learns to export junk values, which is worse than not asking.

So `config.LoadTool` was added: the bindings marked `tool` (`PURPLEOPS_DB_PATH`,
`PURPLEOPS_LOG_LEVEL`, `PURPLEOPS_LOG_FORMAT`), parsed by the same code, returned as a `config.Tool`
rather than a half-filled `Config`. The distinct type keeps that package's promise intact: every
field of what you are handed was read and validated. `TestEveryToolFieldIsBoundToAToolVariable` ties
the two together in both directions, the way `TestEveryConfigFieldIsBound` already does for the
server.

`LoadTool` also creates nothing — no evidence directory — because an admin command that made a
directory as a side effect of starting up is a surprising way to typo a path.

### The locked-database message, and where the detection lives

`store.ErrLocked` is new. `store.Open` tags a DuckDB lock conflict with it, keeping the driver's own
message (which names the holding process and its PID) as the wrapped cause. Detection is
`*duckdb.Error` with `Type == ErrorTypeIO` **and** "Conflicting lock is held" in the message: the
driver exposes a category but no code, and IO is shared with a missing directory and a full disk.
Matching on text is fragile by nature, which is why getting it wrong is survivable — the error is
still returned and still explains itself; only the extra paragraph of advice is lost.

That paragraph deliberately does **not** say "run it inside the container". Being in the same
container is still being a second process, so an operator who reached the message from
`docker compose exec` would try it and fail again. It says to stop the holder, and gives the
`docker compose stop` / `docker compose run --rm` pair that actually works.

Testing it needs two processes: inside one, DuckDB's instance cache hands both opens the same
instance and the second one *succeeds*, so an in-process version of this test would assert the
opposite of the truth. `internal/cli/testdata/dbholder` holds the file until its stdin closes,
following the pattern `internal/store/migrate/testdata/lockprobe` already set. The commands under
test still run in the test process. `deploy/smoke.sh` makes the same assertion against the real
image with the real server running.

### Exit code 2 without matching on cobra's error strings

Cobra reports an unknown command, an unknown flag and a bad argument count as plain errors, which
`Main` would have to recognise by their text. Instead every command sets `Args` (`noArgs` for a
leaf, `subcommandArgs` for a group) and the root sets a `FlagErrorFunc`, so all of those become a
typed `*usageError` carrying the command whose usage block to print. `--log-level` is a
`pflag.Value` over `config.LogLevel` for the same reason: a bad level is rejected during parsing, so
it exits 2 as a bad command line rather than 1 as a failure to run, and the message is config's own.

A command group invoked without a subcommand — `popsctl migrate` — is a usage error and exit 2,
rather than help and exit 0. The invocation that reaches there is a script with a typo far more
often than it is a person looking around, and a person looking around types `--help`, which still
prints to stdout and exits 0. The root behaves the same way: bare `popsctl` exits 2.

### Deviations from the ticket, small

- **`docker compose exec app popsctl version`** — the compose service is named `purpleops`, not
  `app` (`compose.yml` predates this ticket). Verified as
  `docker compose exec purpleops popsctl version`, and as three checks in `deploy/smoke.sh`.
- **`db info` lists every schema, not the current one.** Migration `0001` creates `app` and
  `content`, so a listing filtered to `current_schema()` would show the migrator's bookkeeping table
  and nothing else from M3 onwards. Table names are reported qualified (`app.executions`).
- **`db info` opens the database read-write and creates it if absent**, because the store has one
  open mode. A typo in `--db` makes an empty database rather than an error; `docs/cli.md` says so.
- **`migrate up` prints the status table afterwards**, so its output and `migrate status`'s are the
  same shape, with `appliedNow` naming what this run did.

### Everything else that moved

- `deploy/Dockerfile` builds and ships `popsctl` (no `spa` tag — it imports no frontend); three
  checks in `deploy/smoke.sh` cover it, including the refusal against the live server.
- `docs/cli.md` is new, and is the operator-facing reference. `docs/deploy.md`,
  `docs/migrations.md`, `docs/testing.md`, `README.md` and `.env.example` point at it.
- The end-to-end suite seeds with `['version']` instead of `['--version']`, and its examples in
  `docs/testing.md` and `e2e/harness/test.ts` use the real (still unimplemented) command names.
