# Serving HTTP

One process, one router, one middleware chain. Every request the server answers goes through it —
including the ones that never reach a handler — which is what makes it possible to say "no endpoint
can skip authorization" when M1 inserts that step (`M1-013`).

`internal/httpapi.NewServer` builds the handler; `internal/httpapi.ListenAndServe` runs it.
`cmd/purpleops` does nothing else: load the configuration, open the store, migrate, build, serve.

## The chain

In order, outermost first:

| # | Middleware | What it does |
|---|---|---|
| 1 | `requestID` | Generates a UUIDv7, or accepts the client's `X-Request-Id` if it is short and printable. Echoed on the response, on every log line, and as `instance` in every problem document |
| 2 | `realIP` | Resolves the client address, honouring forwarding headers only from a configured proxy — see below |
| 3 | `requestLogger` | One `slog` line per request: method, path, status, bytes, duration, request ID, client IP |
| 4 | `recoverer` | A panic becomes a 500 problem document; the stack goes to the log and never to the client |
| 5 | `timeout` | Puts `PURPLEOPS_REQUEST_TIMEOUT` on the request context |
| 6 | `securityHeaders` | The response headers below |
| 7 | `requestValidator` | Rejects anything `api/openapi.yaml` does not describe, before any handler runs |

Only 7 is mounted on the API router (under `/api/v1`); the rest apply to everything, so a 404 for an
unknown path is still logged, still carries a request ID and still has the security headers.

**The recoverer is inside the logger**, which is the reverse of what `M0B-006` proposed. The logger
records the status when the handler beneath it returns; with the recoverer outside, the line would
be written while the panic is still unwinding and every panic would appear in the access log as a
success. `TestALoggedPanicReportsTheStatusTheClientSaw` is the test that says so.

M1 inserts authentication and then authorization between 7 and the handlers, on the API router.

## Behind a reverse proxy

`X-Forwarded-For` and `X-Real-IP` are ignored unless the peer that opened the connection is listed
in `PURPLEOPS_TRUSTED_PROXIES` (a comma-separated list of addresses or CIDR ranges). Unset — the
default — means the client address is always the address the connection came from.

This matters because the client address is what login throttling counts (`M1-004`) and what the
activity log records (`M1-015`). A server that believed the header unconditionally would let any
caller choose which address gets throttled and which one appears in the audit trail.

Set it to the proxy's address. Never to `0.0.0.0/0`: that is the same as trusting everyone.

With several hops, the chain is read right to left and the first address that is *not* a listed
proxy is taken as the client — everything further left was written by a hop this deployment does not
control.

## Response headers

Set on every response:

| Header | Value |
|---|---|
| `X-Content-Type-Options` | `nosniff` |
| `Referrer-Policy` | `no-referrer` |
| `X-Frame-Options` | `DENY` |
| `Content-Security-Policy` | `default-src 'self'` and per-directive tightening; see `internal/httpapi/headers.go` |
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` — **only** when `PURPLEOPS_BASE_URL` is `https://` |

The CSP allows inline *styles*, because Radix — the primitives shadcn/ui is built on — writes style
attributes when it positions a popover. Inline *scripts* are forbidden, which is the half that stops
an injected `<script>` or `onclick` from running. Everything loads from this origin: no CDN, no
fonts service, no analytics, because a deployment may be air-gapped.

There is no CORS, deliberately. The SPA is served from this same origin (`M0B-010`); during frontend
development, use the Vite dev proxy rather than opening the API up.

## Timeouts

| Setting | Applies to |
|---|---|
| `PURPLEOPS_REQUEST_TIMEOUT` | The deadline on each request's context. A handler that respects its context — every database call does — gives up there, and the failure travels back as a 500 |
| `PURPLEOPS_SHUTDOWN_TIMEOUT` | How long in-flight requests get after a termination signal |
| `readHeaderTimeout` (10s), `idleTimeout` (2m) | Constants in `serve.go`: properties of the protocol rather than of a deployment |

There is deliberately no `ReadTimeout` or `WriteTimeout` on the server: the first would cap how long
an evidence upload may take (`M3`) and the second how long an SSE stream may stay open (`M4`), and
both would fail as a truncated response rather than as anything a user could act on.

## Shutdown

`SIGINT` or `SIGTERM` stops the listener, lets in-flight requests finish within
`PURPLEOPS_SHUTDOWN_TIMEOUT`, closes the store, and exits 0. A request still running when the grace
period expires is cut off — logged as a warning, still exit 0, because an orchestrator that asked
for a stop should not be shown a crash.

A second signal kills the process outright: the handler is installed with `signal.NotifyContext` and
restores the default disposition after the first one.

## Errors

Every failure is an RFC 9457 problem document — see [`api.md`](api.md). That includes the ones
produced before a handler is reached: an unknown path, a wrong method, a body that does not match
the specification, and a panic. There is one writer (`apierr.Responder.Write`), installed as the
error handler for the router, the generated strict handler and the request validator, so the three
cannot disagree.
