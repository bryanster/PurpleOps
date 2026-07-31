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

- [ ] `popsctl --help` lists every command with a one-line description.
- [ ] `popsctl migrate status` on a fresh DB lists all migrations as pending; after `migrate up`,
      all as applied, with timestamps.
- [ ] Commands respect `--json` and emit parseable output on stdout, with human/log output on stderr,
      so `popsctl db info --json | jq` works.
- [ ] Exit codes are meaningful: 0 success, 1 runtime failure, 2 usage error.
- [ ] The CLI refuses to open a database the server currently holds, with a clear message — DuckDB's
      single-writer rule (`M0B-003`) makes this a guaranteed confusion otherwise. Test it.
- [ ] Stub commands exit 1 with a message naming the milestone that will implement them.
- [ ] `popsctl` is in the Docker image and `docker compose exec app popsctl version` works.

## Tests

- Integration tests invoking the command functions directly (not by spawning a process) against a
  temp database: `migrate status` before/after, `db info` output shape.
- A test asserting `--json` output is valid JSON for every command that supports it.
