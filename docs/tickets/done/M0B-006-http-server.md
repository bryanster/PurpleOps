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

- [x] `GET /api/v1/healthz` returns 200 with `{"status":"ok","checks":{"db":"ok"}}`; with the store
      closed it returns 503 and `db: "error"` — assert this, don't assume it.
      — `TestHealthzReportsAHealthyDatabase` and `TestHealthzReportsADeadDatabase`, the second
      against a real DuckDB file that is closed mid-test rather than a mock.
- [x] A request body violating the spec is rejected with **400** and the shared problem shape,
      including which field failed. The handler is never entered.
      — `TestABodyThatViolatesTheSpecNeverReachesTheHandler`; see note (2) for why it uses a fixture
      document. `TestAValidBodyReachesTheHandler` is the other half.
- [x] An unknown route under `/api/v1` returns a 404 problem response, not chi's HTML default.
      — `TestAnUnknownPathIsAProblemDocument`, which also covers a path outside the API prefix
      (chi's `NotFound`) and asserts chi's plain-text default is absent.
- [x] A wrong method returns 405, not 404. — `TestAWrongMethodIsMethodNotAllowed`.
- [x] A panic in a handler returns a 500 problem response, logs the stack server-side, and the
      response body contains no stack trace or Go type names.
      — `TestAPanicIsA500ThatLeaksNothing`: the panic value carries a fake database path, and the
      body is checked for it, for `goroutine`, for a package name and for the word `panic`.
- [x] Every log line includes the request ID; a client-supplied `X-Request-Id` is echoed back.
      — `TestAGeneratedRequestIDIsAUUIDv7AndReachesEverything`, `TestAClientsRequestIDIsEchoed`,
      and `TestAnUnusableRequestIDIsReplaced` for what is *not* echoed — see note (3).
- [x] `SIGTERM` stops accepting new connections, lets in-flight requests finish, closes the store,
      and exits 0. A hung request is cut off at the timeout and the process still exits.
      — `TestShutdownLetsAnInFlightRequestFinish` and `TestShutdownGivesUpOnAHungRequest` in
      `internal/httpapi`; `TestRunStartsAndStopsCleanly` covers the process, store close included.
- [x] The spec is embedded in the binary (`embed.FS`) — validation must not read `api/openapi.yaml`
      from disk at runtime. — the validator is built from `api.Load()`, which parses the `//go:embed`
      copy (M0B-005). Nothing in `internal/httpapi` opens a file.
- [x] Security headers present on every response, verified by test.
      — `TestSecurityHeadersAreOnEveryResponse` walks a handler response, a validator 404, a chi 404,
      a 405 and a recovered panic. `TestHSTSFollowsTheBaseURL` covers the conditional one.

## Tests

- [x] `httptest`-based tests for: healthz ok, healthz degraded, validation rejection, 404, 405, panic
      recovery, security headers, request-ID propagation. — `internal/httpapi/*_test.go`, all through
      the handler `NewServer` returns.
- [x] A shutdown test: start on port 0, fire a slow request, send the signal, assert the request
      completes and `ListenAndServe` returns `http.ErrServerClosed`. — `serve_test.go`. The signal is
      a cancelled context (what `signal.NotifyContext` produces) and the slow handler blocks on a
      channel rather than a sleep, so it is deterministic. `ErrServerClosed` is asserted inside
      `serve` and reported to the caller as `nil` — see note (5).

## Notes for the implementer

- Order matters. Recoverer must be outside Logger (so panics are logged) but inside RequestID (so
  the log line has an ID). If you're unsure, write the test first and let it tell you.
- Do not add CORS. The SPA is served from the same origin (`M0B-010`). Adding permissive CORS
  "temporarily" during frontend development is how it ships to production — use the Vite dev proxy
  instead (`M0B-008`).

## Implementation notes

### 1. The recoverer sits inside the logger, not outside it

The ticket asks for `Recoverer` outside `Logger`, "so panics are logged". Written that way, the
access-log line is produced while the panic is still unwinding — before the recoverer has written
anything — so every panic in the system is logged as a 200 with zero bytes. Inside, the line reports
the 500 the client actually received, and the panic still gets its own line with the stack, because
the recoverer logs it itself rather than relying on the logger above it.

The ticket says "if you're unsure, write the test first and let it tell you". It did:
`TestALoggedPanicReportsTheStatusTheClientSaw`.

### 2. Two things `newServer` takes that `NewServer` does not

`api/openapi.yaml` has two GETs and no request body until M1, so the acceptance criterion about a
body violating the spec cannot be exercised against the real document — and "the handler is never
entered" cannot be observed through a generated handler that has no way to record that it ran.

`newServer(deps, doc, extraRoutes)` therefore takes the document and an optional route registration
that runs on the API router, behind the validator. Production passes `nil` for both. The alternative
— asserting against the middleware in isolation — would have proved the middleware works and not
that it is mounted, which is the half that regresses.

`NewServer` also returns an `error` rather than the bare `http.Handler` in the ticket: it loads and
parses a specification, and a document that will not parse should be a startup message, not a panic
(definition of done, item 4).

### 3. The request-ID middleware, and what it will not repeat

Per `M0B-007`'s note, the ID is stored under chi's `middleware.RequestIDKey`, so `apierr`, the
logger and any chi middleware read one value. A client-supplied `X-Request-Id` is accepted — a
caller correlating its own traces has a good reason to choose it — but only if it is at most 64
characters of `[A-Za-z0-9._:-]`. The value is echoed to the client in a header and written into every
log line, so an unbounded one is a way to write a megabyte into the log, and a newline is a way to
forge a log entry. A rejected value is replaced silently: the request itself is fine.

Generation is `uuid.NewV7`, matching every other identifier in the system, with a clock-based
fallback if `crypto/rand` fails — a request with no ID at all loses the thread between the client's
response and the server's log line.

### 4. The request validator is written out, and the router matters

The ticket names `OapiRequestValidator` (oapi-codegen's `nethttp-middleware`). It is fifteen lines
around two kin-openapi calls, and writing it out hands `apierr` the library's own error values
rather than that package's re-wrapped strings — which is exactly what `M0B-007`'s translation is
tested against. One dependency saved, and the M1 authentication insertion point is visible in the
file.

One trap, worth knowing before switching routers: **the `legacy` router does not return
`routers.ErrPathNotFound`**. It returns a fresh `*RouteError` carrying the same text, which
`errors.Is` cannot match, so every unknown path is reported as a 500. `gorillamux` — the router
kin-openapi documents — returns the sentinels themselves. That is why `github.com/gorilla/mux` is
now an indirect dependency.

### 5. Serving and shutdown

`ListenAndServe` returns `nil` for a clean shutdown, including one that had to cut a request off
(logged as a warning). A non-zero exit there would show an orchestrator a crash where it asked for a
stop. `http.ErrServerClosed` is asserted internally — anything else from `Serve` on the way out is
returned as a real error.

`ReadHeaderTimeout` and `IdleTimeout` are set; `ReadTimeout` and `WriteTimeout` deliberately are not,
because they would cap evidence uploads (M3) and SSE streams (M4) with a truncated response. The
per-request deadline is a context deadline rather than `http.TimeoutHandler`: that handler writes a
plain-text 503 no `code` in the spec describes, and it does not stop the handler it gave up on.

### 6. Two new configuration variables (agreed scope change)

The ticket requires a per-request deadline "from config" and a `RealIP` that trusts `X-Forwarded-For`
"only when configured to". `M0B-002` has neither, so both were added, with `.env.example` and the
config tests updated:

- `PURPLEOPS_REQUEST_TIMEOUT` (default `30s`).
- `PURPLEOPS_TRUSTED_PROXIES` — comma-separated addresses or CIDR ranges, unset by default. The
  shape was chosen over a boolean deliberately (asked and confirmed): a boolean, once true, trusts
  the header from any peer, which lets anyone choose the address that gets throttled in `M1-004` and
  recorded in `M1-015`. Forwarded headers are read only when the connecting peer is in the list, and
  the chain is walked right to left to the first hop this deployment does not control.

### 7. The CSP allows inline styles (asked and confirmed)

`style-src 'self' 'unsafe-inline'`, because Radix — under shadcn/ui — writes style attributes when
positioning overlays. `script-src 'self'` with no inline scripts, which is the half that stops
injected markup from executing. If `M0B-008` needs more, the argument belongs in that ticket.

### 8. Deliberately not done

- No CORS, as the ticket instructs. The SPA is same-origin from `M0B-010`; use the Vite dev proxy.
- No authentication. The insertion point is a comment in `server.go`'s chain and an absent
  `AuthenticationFunc` in `validate.go` — the validator refuses any operation with a security
  requirement until one is supplied, which is what makes M1 impossible to forget.
- The SPA is not served. `/` is a 404 problem document until `M0B-010`.
- Operator documentation is `docs/http.md`, linked from `docs/api.md`.
