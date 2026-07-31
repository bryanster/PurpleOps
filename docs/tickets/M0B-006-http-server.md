# M0B-006 — chi server, middleware chain, request validation, graceful shutdown

**Milestone:** M0b · **Size:** M · **Depends on:** M0B-003, M0B-004, M0B-005, M0B-007

## Why

This is where the generated interface becomes a running process. The middleware chain established
here is the one every later feature plugs into — including the single authorization middleware in
`M1-013`, which only works if there is exactly one chain and everything goes through it.

## Scope

**In**

- `internal/httpapi.NewServer(deps) http.Handler` — builds the router, mounts generated routes under
  `/api/v1`.
- Middleware chain, in this order:
  1. `RequestID` — generate or accept `X-Request-Id`, put it in the context.
  2. `RealIP` — behind a reverse proxy; only trust `X-Forwarded-For` when configured to.
  3. `Recoverer` — panic → 500 problem response, log with stack, never leak the stack to the client.
  4. `Logger` — structured (`log/slog`), one line per request: method, path, status, duration, bytes,
     request ID, user ID once M1 exists.
  5. `Timeout` — per-request deadline from config.
  6. Security headers — `X-Content-Type-Options`, `Referrer-Policy`, `X-Frame-Options: DENY`, a CSP
     compatible with the SPA, and HSTS when the base URL is HTTPS.
  7. **Request validation** — `kin-openapi` `OapiRequestValidator` against the loaded spec.
  8. (M1 inserts authn then authz here.)
- `cmd/purpleops/main.go`: load config → open store → migrate → build server → listen → shut down
  gracefully on `SIGINT`/`SIGTERM` within `PURPLEOPS_SHUTDOWN_TIMEOUT`.
- `/healthz` and `/version` implemented against the generated interface.

**Out**

- Auth of any kind (M1). Leave a clearly-marked insertion point in the chain with a comment.
- Serving the SPA (`M0B-010`).

## Acceptance criteria

- [ ] `GET /api/v1/healthz` returns 200 with `{"status":"ok","checks":{"db":"ok"}}`; with the store
      closed it returns 503 and `db: "error"` — assert this, don't assume it.
- [ ] A request body violating the spec is rejected with **400** and the shared problem shape,
      including which field failed. The handler is never entered.
- [ ] An unknown route under `/api/v1` returns a 404 problem response, not chi's HTML default.
- [ ] A wrong method returns 405, not 404.
- [ ] A panic in a handler returns a 500 problem response, logs the stack server-side, and the
      response body contains no stack trace or Go type names.
- [ ] Every log line includes the request ID; a client-supplied `X-Request-Id` is echoed back.
- [ ] `SIGTERM` stops accepting new connections, lets in-flight requests finish, closes the store,
      and exits 0. A hung request is cut off at the timeout and the process still exits.
- [ ] The spec is embedded in the binary (`embed.FS`) — validation must not read `api/openapi.yaml`
      from disk at runtime.
- [ ] Security headers present on every response, verified by test.

## Tests

- `httptest`-based tests for: healthz ok, healthz degraded, validation rejection, 404, 405, panic
  recovery, security headers, request-ID propagation.
- A shutdown test: start on port 0, fire a slow request, send the signal, assert the request
  completes and `ListenAndServe` returns `http.ErrServerClosed`.

## Notes for the implementer

- Order matters. Recoverer must be outside Logger (so panics are logged) but inside RequestID (so
  the log line has an ID). If you're unsure, write the test first and let it tell you.
- Do not add CORS. The SPA is served from the same origin (`M0B-010`). Adding permissive CORS
  "temporarily" during frontend development is how it ships to production — use the Vite dev proxy
  instead (`M0B-008`).
