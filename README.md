# PurpleOps

[![CI](https://github.com/bryanster/PurpleOps/actions/workflows/ci.yml/badge.svg)](https://github.com/bryanster/PurpleOps/actions/workflows/ci.yml)

PurpleOps is a self-hosted purple-team assessment tool. Red and blue work through the same
engagement — an ordered chain of ATT&CK-mapped scenarios and steps — recording what was executed,
what was prevented, what was detected and how quickly, and what changed on retest after
remediation. The output is a report.

This is the v2 rebuild: a single Go binary that serves an OpenAPI-described API and an embedded
React SPA, backed by an embedded DuckDB database. See [`PLAN.md`](PLAN.md) for the design and
[`docs/tickets/`](docs/tickets/) for the backlog.

## Running it

```sh
docker compose up --build
```

Then <http://localhost:8080>. Nothing is fetched at first boot, and the image carries the database,
the UI and the headless Chromium that renders reports. [`docs/deploy.md`](docs/deploy.md) covers
configuration, where the data lives, and how to back it up.

From source instead:

```sh
make tools     # once: install pinned generators into ./bin
make build     # build the SPA and the binaries
make run       # build, then start the server
```

`make help` lists every target. There is a dev container with the toolchain already pinned and
installed — see [`.devcontainer/`](.devcontainer/).
[`docs/contributing.md`](docs/contributing.md) has the development loop, what CI checks, and the
branch-protection rules.

> **Status:** v2 is under construction on the `v2` branch and is not yet usable. The full
> installation and operation guide is rewritten in M7; until then, the ticket backlog is the
> accurate description of what exists.

## Licence

Apache 2.0 — see [`LICENSE`](LICENSE).
