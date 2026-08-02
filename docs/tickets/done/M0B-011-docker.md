# M0B-011 — Dockerfile (CGO + Chromium) and compose

**Milestone:** M0b · **Size:** M · **Depends on:** M0B-010

## Why

`PLAN.md` §8 names the Docker image as **the supported artifact**, precisely because CGO makes the
"static binary anywhere" story false. The image is also where headless Chromium lives for M6 PDF
rendering — bundling it now means M6 doesn't discover a packaging problem at the end.

The manual verification bar from `PLAN.md` §9: `docker compose up` on a clean machine reaches a
working login in under a minute, with **no network fetch** — a direct reaction to v1's ~1 GB
git-clone-on-first-boot seeder.

## Scope

**In**

- Multi-stage `Dockerfile`:
  1. Node stage: build `web/`.
  2. Go stage: `CGO_ENABLED=1` build with the frontend copied in, version ldflags from build args.
  3. Runtime stage: a distro with Chromium available (Debian slim), non-root user, the binary,
     Chromium, CA certificates, tzdata.
- `compose.yml`: one service, a named volume for the DuckDB file and `evidence/`, env from `.env`,
  a healthcheck hitting `/api/v1/healthz`, sensible restart policy.
- `.dockerignore` that keeps `node_modules`, `bin`, `*.duckdb`, `evidence` and `.git` out of context.
- `docs/deploy.md`: run it with Docker, run it bare-metal, required env vars, where data lives,
  how to back up (stop the process, copy the DB file and `evidence/`), how to restore.

**Out**

- Multi-arch publishing in CI (`M0B-012`).
- Kubernetes manifests, Helm. Not in v1.

## Acceptance criteria

- [x] `docker compose up` from a clean clone reaches a healthy container serving the UI, and makes
      **no** network requests to fetch content at runtime.
- [x] The container runs as a non-root user; the data volume is writable by that user.
- [x] The DuckDB file and `evidence/` persist across `docker compose down && docker compose up`.
      Verify by writing something and restarting.
- [x] `docker compose down -v` destroys the data — documented in `docs/deploy.md` with a warning.
- [x] The image healthcheck reports unhealthy when the DB is unavailable.
- [x] Chromium is present and `chromium --version` runs inside the container; `BLACKLIGHT_CHROME_PATH`
      defaults correctly in the image.
- [x] Final image size is recorded in the PR description. Under 500 MB with Chromium is the target;
      if it's larger, say why. — **over: ~640 MB unpacked / 260 MB compressed. See below.**
- [x] Build layers are ordered so a Go-source-only change does not re-run `npm install`.
- [x] The image builds for `linux/amd64` and `linux/arm64` (buildx). If arm64 is slow via emulation,
      note it — CI handles it in `M0B-012`. — **emulation does not work at all; cross-compiled
      instead. See below.**

## Tests

- A smoke script in `deploy/` (or a `make docker-smoke` target) that builds the image, runs it,
  polls `/api/v1/healthz` until healthy or times out, and exits non-zero on failure. CI reuses it.

## Notes for the implementer

- The Go stage must build with `-tags spa`, or the image ships the placeholder page instead of the
  UI and everything else here still passes. `M0B-010`'s implementation notes say why the tag exists;
  `make build` is the reference for what the flags are.
- CGO means you cannot use `scratch` or plain `alpine` without matching musl. Debian slim in both
  build and runtime stages is the boring, correct choice — take it.
- Chromium in a container needs `--no-sandbox` or a seccomp profile. Prefer keeping the default
  sandbox and adding the right `seccomp`/capabilities in compose over disabling the sandbox; if you
  must disable it, document the tradeoff in `docs/deploy.md`.

## Implementation notes

**Files.** `deploy/Dockerfile`, `deploy/entrypoint.sh`, `deploy/healthcheck.sh`, `deploy/smoke.sh`,
`.dockerignore` and `compose.yml`. `compose.yml` is at the repository root rather than in `deploy/`
so that `docker compose up` works in a clean clone with no `-f`; the Dockerfile stays in `deploy/`
where `deploy/README.md` said it would, and is built with the repository as its context. `make
docker-build` and `make docker-smoke` are the entry points. Operator documentation is
`docs/deploy.md`.

**The clean-clone requirement forced a decision about the session secret.** `BLACKLIGHT_SESSION_SECRET`
is required and correctly refuses placeholders, so `docker compose up` on a fresh clone would fail
at startup — which contradicts the acceptance criterion. `deploy/entrypoint.sh` generates 32 bytes
from `/dev/urandom` when the variable is unset, persists it on the data volume next to the database,
reuses it on later starts so a restart does not log everybody out, and says all of this in the log
in four lines. It is skipped entirely when the operator supplied a secret, and when the command is
`blacklight --version`. `docs/deploy.md` states the three ways this is worse than setting the
variable (tied to one volume, unshareable, unreadable from outside the container). The alternative —
requiring `.env` before the first `up` — was rejected because the ticket's first acceptance
criterion is explicit about a clean clone.

**Chromium's sandbox is left on, and on a stock Docker host it does not work.** Measured, not
assumed: with Docker's default seccomp profile the renderer aborts with `Failed to move to new
namespace`, because the profile blocks `clone(CLONE_NEWUSER)`. Both `seccomp=unconfined` and
`cap_add: SYS_ADMIN` fix it, and both were verified to render a PDF as the non-root user. Neither is
set in `compose.yml`: turning off syscall filtering for the whole container to protect one process
is a real trade, and M6 is the ticket that gets to make it with a working renderer in front of it.
The narrow fix — a seccomp profile that is Docker's default plus unrestricted `clone`/`unshare` —
would mean vendoring ~1,200 lines of JSON that goes stale silently, so it was not taken.
`docs/deploy.md` documents all four options in order of preference, and notes that
`no-new-privileges`, `cap_drop: [ALL]` and a read-only root filesystem each break the sandbox too.

**Emulated cross-architecture builds do not work; the Go stage cross-compiles.** Building
`linux/amd64` on arm64 through QEMU fails — not slowly, but with the Go toolchain segfaulting at a
different package each run (`compile: signal: segmentation fault`). So both build stages are pinned
to `$BUILDPLATFORM`: the node stage because JavaScript is architecture-independent, and the Go stage
because it has to be. The Go stage installs `g++-x86-64-linux-gnu` or `g++-aarch64-linux-gnu` when
the architectures differ — `g++` rather than `gcc` because the DuckDB engine is a C++ static archive
and the link needs the target's `libstdc++` — and sets `GOARCH`, `CC` and `CXX`. Only the runtime
stage's `apt-get` runs emulated, which is fine. Verified: `linux/amd64` builds in about the same
time as native, and the resulting binary boots, serves and passes 13 of the 14 smoke checks under
QEMU. The fourteenth is `chromium --version`, which fails with "lacks support for the sse3
instruction set" — `qemu-user` does not advertise SSE3. That is the emulator, not the image, and it
is documented in both `deploy/smoke.sh` and `docs/deploy.md` rather than skipped, because a silent
skip is how a real breakage gets through. This makes the `M0B-012` matrix cheaper than the ticket
assumed — one runner can *build* both architectures, though only a native runner can smoke-test the
Chromium half.

**Image size: ~640 MB unpacked, 260 MB compressed**, near-identical on both architectures. Over the
500 MB target, and the overshoot is entirely Chromium (~345 MB unpacked after trimming; the base is
~109 MB, the binary ~64 MB, mostly the statically linked DuckDB engine). The build already deletes
~230 MB in the same layer as the install — the Mesa hardware GL stack (LLVM, Gallium, Z3, the DRI
drivers), the Vulkan validation layer and the GTK icon themes — none of which a headless renderer
without a GPU can use; Chromium falls back to its bundled SwiftShader and `--print-to-pdf` was
verified to still work. There is no smaller glibc-compatible headless Chromium to swap in: Google
publishes `chrome-headless-shell` for amd64 only, which would give up arm64. Without Chromium the
image would be ~290 MB, which is the shape of a future `blacklight:slim` if size ever outranks PDF
reports working out of the box.

**Debian trixie, not bookworm.** Same release in all three stages, so the glibc the binary links
against is the glibc it finds. Chromium 151 rather than bookworm's older package.

**`deploy/smoke.sh` proves 14 things, not one.** Beyond polling health, it asserts the container
runs as a non-root user with a writable data directory, serves the real embedded SPA rather than the
`-tags spa` placeholder, has a working `BLACKLIGHT_CHROME_PATH`, survives being destroyed and
recreated on the same volume with its database, evidence and session secret intact, and that
`blacklight-healthcheck` actually *fails* when nothing is answering. First boot runs with
`--network none`, which is the cheap proof that nothing is fetched at runtime — a packet capture
would be the rigorous version. `PLATFORM=linux/amd64 deploy/smoke.sh` runs the whole thing against
the other architecture.

**Health check via `curl`, in the image rather than in compose.** `HEALTHCHECK` in the Dockerfile
means `docker run` and any third-party compose file inherit it; `compose.yml` says so in a comment
rather than repeating it. The script reads the port out of `BLACKLIGHT_ADDR` instead of hardcoding
8080, and refuses port 0 rather than reporting a healthy container nobody is probing. The 503-on-dead-database
half of the acceptance criterion is `TestHealthzReportsADeadDatabase`, which already existed;
`curl --fail` is what turns that 503 into an unhealthy container, and the smoke test covers the
`--fail` half.

**No `VOLUME` instruction.** It would leave an anonymous volume behind on every `docker run` without
`-v`, and would not actually save an operator who meant to persist their data. `compose.yml`
declares the named volume explicitly instead, and `docs/deploy.md` leads with where the data lives.
