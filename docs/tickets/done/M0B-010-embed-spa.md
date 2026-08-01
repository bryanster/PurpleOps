# M0B-010 — Embed the SPA in the binary with correct fallback routing

**Milestone:** M0b · **Size:** S · **Depends on:** M0B-006, M0B-008

## Why

"One binary, no external services" (`PLAN.md` §1) requires the frontend to live inside it. The
subtlety is routing: a SPA needs unknown paths to return `index.html`, but an unknown *API* path
must still return a JSON 404 — otherwise a typo'd API call gets HTML and the client's error handling
explodes somewhere unhelpful.

## Scope

**In**

- `web/embed.go` exporting `web/dist` via `embed.FS`, behind a build tag or with a committed
  placeholder so `go build ./...` works before the frontend has been built.
- Static file serving with:
  - hashed asset paths served with `Cache-Control: public, max-age=31536000, immutable`,
  - `index.html` served with `no-cache` so deploys take effect immediately,
  - ETag / `If-None-Match` support,
  - correct MIME types (including `.svg`, `.woff2`, `.webmanifest`).
- SPA fallback: any GET that doesn't match a file and doesn't start with `/api/` serves
  `index.html` with 200.
- `/api/*` misses continue to return the JSON 404 problem from `M0B-006`.
- A development mode (`PURPLEOPS_ENV=development`) that serves from disk rather than the embedded
  FS, so the frontend can be rebuilt without recompiling Go — optional, but note the choice either
  way.

**Out**

- Anything about what the SPA contains.

## Acceptance criteria

- [x] `make build` produces one binary that serves the whole UI with no `web/dist` on disk. Verify
      by running it from an empty directory.
- [x] `GET /` returns `index.html`.
- [x] `GET /engagements/123` (a client-side route that doesn't exist on the server) returns
      `index.html` with 200.
- [x] `GET /api/v1/nope` returns a 404 **problem+json** — not `index.html`. This is the regression
      this ticket exists to prevent; test it explicitly.
- [x] `GET /assets/index-<hash>.js` returns the asset with the immutable cache header; `index.html`
      returns `no-cache`.
- [x] A conditional request with a matching `If-None-Match` returns 304.
- [x] A `POST` to an unknown non-API path returns 405 or 404 — not `index.html`. Only GET/HEAD fall
      back.
- [x] Path traversal (`GET /../go.mod` and encoded variants) cannot escape the embedded FS.

## Tests

- `httptest` cases for each acceptance bullet, using a small test `fs.FS` rather than the real
  `dist` so the tests don't depend on a frontend build.

## Implementation notes

**Files.** `web/dist.go` (+ `dist_spa.go`, `dist_placeholder.go`, `placeholder/index.html`) is the
embed; `internal/httpapi/spa.go` is the serving. The handler takes an `fs.FS` through `Deps.UI`
rather than importing `web`, so the tests build the server over a `fstest.MapFS` and never need a
frontend build. `Deps.UI == nil` serves no UI at all, which is what the existing API tests do.

**Build tag, not a committed placeholder.** The ticket allowed either. A committed
`web/dist/index.html` does not survive `npm run build`: Vite empties `outDir` before writing, so the
placeholder would be deleted by every build and leave the tree dirty. So the embed is behind
`-tags spa`, which `make build` passes after building the app, and the default build carries
`web/placeholder/index.html` — a page that says the binary was built without the tag. The server
logs a warning at startup when it is serving that. `make test-spa` (`go test -tags spa ./web`)
exercises the tagged path and asserts the embed captured a real Vite build; **M0B-012 should run it
after `make build`** in CI, since `make test` cannot — it does not build the frontend.

**No development disk mode.** The optional `PURPLEOPS_ENV=development` serve-from-disk mode was not
implemented, deliberately: `npm run dev` already serves the frontend with hot reload and proxies
`/api` to the Go server (`web/vite.config.ts`), which is strictly better for the loop it would
serve. A disk mode would add a second code path through the thing that decides what `/api/…` means,
which is the one decision this ticket exists to get right.

**Routing.** The SPA is a `GET`/`HEAD` catch-all registered on the root router *after* the API
mount, and it refuses `/api/` itself before it looks anything up. The API sub-router now sets its
own `NotFound`/`MethodNotAllowed` instead of inheriting the parent's, so "an unknown API path is
JSON" no longer depends on the order of two calls in `newServer`.

**Caching.** Immutable requires both `assets/` *and* a hash-shaped name — a file dropped into
`web/public/assets/` is copied there unhashed, and a year of caching on a name that never changes is
a deploy that cannot take effect. Everything else, `index.html` included, is `no-cache` plus a
SHA-256 ETag, so a repeat load is a 304 rather than a re-download.

**Verified by hand**, from an empty directory with no `web/dist` present: `GET /` and
`/engagements/123` served the app, `/api/v1/nope` and `/api/v2/engagements` were `problem+json`
404s, the hashed asset came back `immutable` and 304'd against its ETag, `POST /nope` was a 405
problem, and `/../go.mod` and `/%2e%2e/go.mod` returned `index.html` rather than the file.
