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
| 8 | `throttleCredentials` | Rations failed sign-in attempts, per account and per client address — see below |
| 9 | `authenticate` | Resolves the session cookie into an `authn.Subject` on the context. Refuses nothing — see below |
| 10 | `requireCSRF` | Refuses a state-changing request that authenticated by cookie and carries no valid CSRF token — see [`docs/security.md`](security.md) |

Only 7 to 10 are mounted on the API router (under `/api/v1`); the rest apply to everything, so a 404
for an unknown path is still logged, still carries a request ID and still has the security headers.

**The recoverer is inside the logger**, which is the reverse of what `M0B-006` proposed. The logger
records the status when the handler beneath it returns; with the recoverer outside, the line would
be written while the panic is still unwinding and every panic would appear in the access log as a
success. `TestALoggedPanicReportsTheStatusTheClientSaw` is the test that says so.

**Authentication decides who, not whether.** A request with no cookie, an expired session or a
revoked one goes through step 8 exactly as it arrived, with no subject on its context; refusing is
authorization's job and happens in one place (`M1-013`). What step 8 *does* answer for itself is a
database failure: "the store did not answer" is not "you are not signed in", and reporting it as one
would sign everybody out whenever the database hiccupped.

M1-013 inserts authorization between 10 and the handlers, on the same router.

## Sign-in throttling

Two limiters guard every endpoint where a credential is presented — today `POST /auth/login`, and
the list is `credentialRoutes` in `internal/httpapi/throttle.go`. Both have to allow an attempt
through, and a refusal is a `429` with `code: rate_limited` and a `Retry-After` in whole seconds.

| Limit | Keyed on | Default | Cleared by a successful sign-in |
|---|---|---|---|
| `PURPLEOPS_LOGIN_ACCOUNT_FAILURES` / `_LOCKOUT` | the normalized email address | 5 failures → 15 min | yes |
| `PURPLEOPS_LOGIN_SOURCE_FAILURES` / `_LOCKOUT` | the client address | 50 failures → 15 min | **no** |

Each further lockout of the same key doubles the wait, three times — 15m, 30m, 1h, 2h. Two things
put a key back to the bottom of that ladder: a successful sign-in, and going quiet for the length of
the longest lockout, which is also how the table is kept bounded. A success does not clear the
*source* limit on purpose: an attacker who holds one valid account would otherwise top their
spraying budget up with it.

Four things worth knowing before changing any of it:

- **The right password during a lockout is refused too**, and no session is issued. A lockout that
  the right password ends is not a lockout — it is a delay on an attacker who has already won.
- **A locked-out attempt costs no password hash.** The middleware runs before the handler, which is
  half of what throttling is for: Argon2id is expensive on purpose, so an attacker must not be able
  to spend the server's CPU by sending guesses faster.
- **The 429 is identical for an account that exists and one that does not**, which is the same
  defence the 401 makes (see *Sessions*). Only the log records which key it was.
- **The state is in memory and per-process**, and it is lost on restart. That is correct for the
  single-node deployment in `PLAN.md` §1 and for nothing else; a second node would need it shared,
  and nothing here would notice on its own.

Behind a reverse proxy the source limiter counts the proxy unless `PURPLEOPS_TRUSTED_PROXIES` names
it — the same resolution the request log uses, described under *Behind a reverse proxy* below.

## Sessions

The cookie is `pops_session`: `HttpOnly`, `Secure` (except `PURPLEOPS_ENV=development`),
`SameSite=Strict`, `Path=/`, no `Domain`. Its value is 32 bytes from `crypto/rand`, base64url.

**Only a keyed hash of the token is stored** — HMAC-SHA256 under `PURPLEOPS_SESSION_SECRET` — so a
copy of the database is not a set of live sessions, and rotating that secret signs everybody out.
`internal/authn/session` is the only package that decides whether a session is usable:

| Ends a session | Setting |
|---|---|
| Absolute expiry, from when it was issued. Nothing extends it | `PURPLEOPS_SESSION_LIFETIME` (12h) |
| Idle timeout, from when it was last used | `PURPLEOPS_SESSION_IDLE_TIMEOUT` (2h) |
| Revocation — logout, a password change elsewhere, an administrator | `revoked_at` on the row |
| The account being disabled | `status` on the user, checked on every request |

`last_seen_at` is written at most once a minute per session (`touchInterval`). Writes are serialized
(`PLAN.md` §1), and a column whose only consumer is a timeout measured in hours does not deserve the
write lock on every read.

The browser holds a second cookie beside it, `pops_csrf`, which is derived from the session token
and is deliberately readable by script. [`docs/security.md`](security.md) is the whole of that
model; the one thing to know here is that the pair is issued together, rotated together and cleared
together, by the CSRF middleware rather than by any handler.

**Rotation replaces the token and keeps the session**: same row, same identifier, same absolute
expiry. It happens on sign-in (a new session), on a password change, and — when they land — on MFA
completion and a platform-role change. The token it replaces stops resolving to anything, which is
what makes a session an attacker fixed before login worthless afterwards.

## Serving the app

The frontend is in the binary: `web/dist` is embedded as an `fs.FS` (`web/dist.go`) and handed to
`NewServer` as `Deps.UI`. A release is one file, and there is no static file server to deploy beside
it.

| Request | Answer |
|---|---|
| `GET /` | `index.html`, 200 |
| `GET /engagements/123` — a route that only exists in the browser | `index.html`, 200. Otherwise every deep link and every page refresh is a 404 |
| `GET /assets/index-<hash>.js` | The asset, `Cache-Control: public, max-age=31536000, immutable` |
| `GET /theme-bootstrap.js` — a name with no hash in it | The file, `Cache-Control: no-cache` and an ETag |
| A conditional request whose `If-None-Match` matches | 304, keeping the caching headers |
| `GET /api/…` matching no endpoint | **A `problem+json` 404**, never `index.html` |
| `POST /engagements/123` | 405. Only GET and HEAD fall back to the app |

The `/api/` rule is the point of the whole arrangement. A client that asks for an endpoint this
server does not have — a typo, or a build against a later version — must get a document it can
read; 200 and a page of HTML makes its error handling fail somewhere unrelated to the mistake.
The prefix is wider than the `/api/v1` the API is mounted at, so `/api/v2/engagements` is JSON too.

Content types come from a table in `spa.go` rather than from `mime.TypeByExtension`, which reads
`/etc/mime.types` and so answers differently depending on the machine. ETags are the SHA-256 of the
contents, computed once at startup. Path traversal is not a special case: the lookup is a map of the
files that exist, so `/../go.mod` matches nothing and gets `index.html` like any other unknown path.

### The `spa` build tag

`web/dist` is build output — gitignored, and absent from a fresh checkout — but `//go:embed` does
not compile when its pattern matches nothing. So the real embed is behind the `spa` build tag:

- `make build` runs `npm run build` first and passes `-tags spa`. This is the release build.
- `go build ./...` without it compiles a placeholder page that says so, so a Go-only checkout, a
  container stage that never copies `web/`, and `go test ./...` all still work. The server logs a
  warning at startup when it is carrying the placeholder.
- `make test-spa` runs the embed's own tests against a real `web/dist` — run it after `make build`.

There is deliberately **no** development mode that serves `web/dist` from disk. Frontend work runs
`npm run dev`, which has hot reload and proxies `/api` to this server (`web/vite.config.ts`); a
disk-serving mode would be a second, worse version of that, and a second code path in the thing that
decides what `/api/…` means.

## Behind a reverse proxy

`X-Forwarded-For` and `X-Real-IP` are ignored unless the peer that opened the connection is listed
in `PURPLEOPS_TRUSTED_PROXIES` (a comma-separated list of addresses or CIDR ranges). Unset — the
default — means the client address is always the address the connection came from.

This matters because the client address is what login throttling counts (`M1-004`, above) and what the
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

A `429` also carries `Retry-After`, in whole seconds and always rounded up. It comes off the error
itself rather than from whoever did the limiting, so there is no way to send one of these without
telling the caller when to come back.
