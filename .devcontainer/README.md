# .devcontainer/

A ready-made development environment: the Go toolchain, the pinned Node, and nothing that has to be
installed by hand. VS Code offers to reopen the repository in it; the
[Dev Containers CLI](https://github.com/devcontainers/cli) does the same from a terminal.

It carries the same base as `deploy/Dockerfile`'s build stage — same Go, same Debian release, same
glibc. The DuckDB driver is cgo, so "it compiles in my editor" and "it compiles in the release
image" should not be two different questions.

The container runs as the non-root `vscode` user (passwordless `sudo` when needed). Rebuild after
pulling these changes; if Go module or build caches were created as root under the old image, drop
them once:

```sh
docker volume rm blacklight-go-mod blacklight-go-build
```

## What is in it, and what is not

| | |
|---|---|
| Go 1.25, gcc | The server. cgo, so a C compiler is not optional |
| Node 24.18.1 | Exactly the version in `.prototools`, `web/.nvmrc` and `web/package.json` — copied from the official image rather than installed from a distribution repo, which would drift |
| `make`, git, jq | The Makefile is the interface to this repository |
| Docker CLI | Via the `docker-outside-of-docker` feature, pointed at the host daemon — so `make docker-build` and `make docker-smoke` work, sharing the host's layer cache |
| Claude Code | `npm install -g @anthropic-ai/claude-code` |
| Oh My Pi (`omp`) | Prebuilt binary from [omp.sh](https://omp.sh/install) into `/usr/local/bin` (shared PATH; not root's `~/.local/bin`) |
| **Not** golangci-lint or oapi-codegen | The Makefile pins both and installs them into `./bin`. A second copy in the image is a second version to drift. `make tools` runs as `postCreateCommand` |
| **Not** a database container | v2's database is a file inside the process. v1 needed compose for MongoDB; there is nothing left to orchestrate |
| **Not** any `BLACKLIGHT_*` value | The server refuses configuration it cannot use, so a container-wide default would be a value nobody chose, quietly in force. `cp .env.example .env` |

Ports 8080 (the server) and 5173 (`npm --prefix web run dev`, which proxies `/api` to 8080) are
forwarded.

Host `~/.gitconfig`, `~/.ssh`, `~/.claude`, and `~/.omp` are bind-mounted into the `vscode` home so
git identity, keys, and agent config survive rebuilds.

## Caches

The Go module and build caches are named volumes, so rebuilding the container does not re-download
every module and recompile every package. They are caches: deleting them costs time and nothing
else.

```sh
docker volume rm blacklight-go-mod blacklight-go-build
```

## Chromium is not here

The image has no Chromium, so PDF rendering (M6) cannot be exercised in the dev container itself.
Use the real image for that — `make docker-build`, then run it — which is what
[`docs/deploy.md`](../docs/deploy.md) describes and what CI tests.
