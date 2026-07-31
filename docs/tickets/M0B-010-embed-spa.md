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

- [ ] `make build` produces one binary that serves the whole UI with no `web/dist` on disk. Verify
      by running it from an empty directory.
- [ ] `GET /` returns `index.html`.
- [ ] `GET /engagements/123` (a client-side route that doesn't exist on the server) returns
      `index.html` with 200.
- [ ] `GET /api/v1/nope` returns a 404 **problem+json** — not `index.html`. This is the regression
      this ticket exists to prevent; test it explicitly.
- [ ] `GET /assets/index-<hash>.js` returns the asset with the immutable cache header; `index.html`
      returns `no-cache`.
- [ ] A conditional request with a matching `If-None-Match` returns 304.
- [ ] A `POST` to an unknown non-API path returns 405 or 404 — not `index.html`. Only GET/HEAD fall
      back.
- [ ] Path traversal (`GET /../go.mod` and encoded variants) cannot escape the embedded FS.

## Tests

- `httptest` cases for each acceptance bullet, using a small test `fs.FS` rather than the real
  `dist` so the tests don't depend on a frontend build.
