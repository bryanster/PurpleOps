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

- [ ] `docker compose up` from a clean clone reaches a healthy container serving the UI, and makes
      **no** network requests to fetch content at runtime.
- [ ] The container runs as a non-root user; the data volume is writable by that user.
- [ ] The DuckDB file and `evidence/` persist across `docker compose down && docker compose up`.
      Verify by writing something and restarting.
- [ ] `docker compose down -v` destroys the data — documented in `docs/deploy.md` with a warning.
- [ ] The image healthcheck reports unhealthy when the DB is unavailable.
- [ ] Chromium is present and `chromium --version` runs inside the container; `PURPLEOPS_CHROME_PATH`
      defaults correctly in the image.
- [ ] Final image size is recorded in the PR description. Under 500 MB with Chromium is the target;
      if it's larger, say why.
- [ ] Build layers are ordered so a Go-source-only change does not re-run `npm install`.
- [ ] The image builds for `linux/amd64` and `linux/arm64` (buildx). If arm64 is slow via emulation,
      note it — CI handles it in `M0B-012`.

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
