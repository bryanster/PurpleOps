# M0B-009 — Generated TypeScript client and TanStack Query wiring

**Milestone:** M0b · **Size:** M · **Depends on:** M0B-005, M0B-008

## Why

The other half of `PLAN.md` §4: "Contract drift becomes a compile error." The frontend never hand-
writes a URL, a request body type, or a response type. If the spec changes, `tsc` tells you which
components broke — before CI, before review, before a user finds it.

## Scope

**In**

- `openapi-typescript` generating `web/src/api/schema.d.ts` from `api/openapi.yaml`, wired into
  `make generate`.
- `openapi-fetch` client instance in `web/src/api/client.ts`, configured with:
  - base URL `/api/v1`, `credentials: "include"` (sessions arrive in M1),
  - a response middleware that turns a problem+json error into a typed `ApiError` carrying
    `status`, `code`, `detail`, `errors[]` from `M0B-007`,
  - the request-ID header echoed into `ApiError` so a user-facing error can quote it.
- TanStack Query provider in the app root, with deliberate defaults (staleTime, retry policy —
  **do not retry 4xx**, refetch on window focus on/off) and a comment justifying each.
- Query-key convention documented in `web/src/api/README.md`, e.g.
  `["engagements"]`, `["engagements", id]`, `["engagements", id, "executions", {round}]`.
- A thin hook layer per resource (`useVersion`, `useHealth` to start), so components import hooks,
  never the raw client.
- Global handling: a 401 anywhere redirects to login (stub until M1); a 5xx raises a toast.

**Out**

- Optimistic updates and SSE cache invalidation — M4 owns those.
- Auth flows (M1).

## Acceptance criteria

- [x] `make generate` regenerates `schema.d.ts`; running twice is a no-op; the file is committed.
- [x] Changing a response field in `openapi.yaml` and regenerating makes `npm run build` fail in the
      component that used it. **Demonstrate this in the PR description** — it is the whole point of
      the ticket.
- [x] No string literal URL for an API route exists anywhere in `web/src` outside `api/client.ts`
      (add an ESLint rule or a test that greps; automated either way).
- [x] A 404 problem response surfaces as `ApiError` with `code: "not_found"`, not as a thrown
      `SyntaxError` from parsing.
- [x] Queries do not retry on 4xx; they do retry (bounded) on network failure and 5xx.
- [x] The demo screens from `M0B-008` are rewritten to use hooks, with loading and error states that
      are real components, not `if (isLoading) return "Loading..."`.

## Tests

- MSW (Mock Service Worker) handlers built from the generated types; a test asserting the error
  middleware produces a typed `ApiError` for a problem+json body.
- A hook test covering loading → success and loading → error.

## Notes for the implementer

- Keep MSW handlers in one place and reuse them across tests; they become the frontend's fixture
  library for the rest of the project.
- Prefer `queryOptions()` factories over ad-hoc `useQuery` calls scattered in components — it keeps
  keys and fetchers together and makes prefetching possible later.

---

## Implementation notes

### The drift demonstration

Renaming `Version.commit` to `Version.commitSha` in `api/openapi.yaml`, then `make generate`:

```
$ npm --prefix web run build
src/features/system/version-page.tsx(46,52): error TS2339: Property 'commit' does not exist on
    type '{ version: string; commitSha: string; buildDate: string; }'.
src/test/msw/handlers.ts(22,3): error TS2353: Object literal may only specify known properties,
    and 'commit' does not exist in type '{ … }'.
src/features/system/version-page.test.tsx(24,44): error TS2339: Property 'commit' does not exist …
```

The component, the fixture and the test all fail, by name and line. The spec was restored and
regenerated afterwards; `git diff api/openapi.yaml` is empty.

### Decisions worth knowing

- **The base URL is absolute** — `` `${window.location.origin}/api/v1` `` — not the relative
  `/api/v1` the ticket suggests. Same thing in a browser, since the SPA is served by the API server
  (M0B-010) and proxied to it in dev, but Node's `fetch` under Vitest has no document to resolve a
  relative URL against and rejects one outright. `API_BASE_PATH` is exported separately for anything
  that wants the path.
- **`fetch` is bound per request**, not captured at client creation. `openapi-fetch` reads
  `globalThis.fetch` once when `createClient` runs, which is before MSW patches it — every test
  went to the real network and failed with `ECONNREFUSED` until the client called it through a
  closure instead. Costs nothing, and means anything that wraps `fetch` later is actually used.
- **The error middleware distinguishes on the media type, not the status.** A non-2xx carrying
  `application/problem+json` is thrown as an `ApiError`; a non-2xx carrying a body the spec
  documents is returned. That is what keeps `GET /healthz`'s 503 — which reports *which* dependency
  is down — from being flattened into "request failed". `features/system/queries.ts` is where the
  503 is turned back into data.
- **A 401 navigates with `window.location.assign`, not the router.** Losing a session should drop
  every piece of in-memory state with it, and it removes the provider's dependency on being mounted
  inside the router. `/login` is a placeholder route (`app/routes/login-placeholder.tsx`) so the
  redirect lands somewhere honest rather than on the 404 page; M1-017 replaces it.
- **`ProblemCode` is validated at runtime** against a `Record<ProblemCode, true>` in `api/errors.ts`,
  so `ApiError.code` is either a code this build knows or `undefined` — never a string typed as one.
  The `Record` is exhaustive in both directions, so a code added to the spec fails to compile here
  until it is listed.

### Deviations from scope

- **The ticket lists `useVersion` and `useHealth` as "hooks to start"; there is also a
  `queryOptions()` factory behind each**, per the implementer's note, and `systemKeys` alongside
  them. Components import only the hooks.
- **A `/login` placeholder screen was added.** The ticket scopes auth out, and this is not auth: it
  is one paragraph of text so that the global 401 handler has a destination. Ten lines, deleted by
  M1-017.

### Toolchain

`openapi-typescript@7.13.0` peers on `typescript@^5.x` and this repo pins `~6.0` (because
`typescript-eslint` supports `<6.1.0` — M0B-008). `npm install` fails outright on that conflict, so
`web/package.json` carries an `overrides` entry pointing the generator's peer at the repo's own
TypeScript. It uses the compiler API and runs on 6 without complaint; the override should go when
upstream widens the range. This is the second package now pinned around the same TypeScript
constraint — the next one is a reason to revisit the whole pin rather than add a third override.

### What was verified, and what was not

- `make lint test build` green; `make generate` twice in a row is byte-identical output.
- The dev proxy again, against a real `go run ./cmd/purpleops`: `GET /api/v1/version` and
  `/api/v1/healthz` through `http://localhost:5173` return the server's own JSON, and an unknown
  path returns `application/problem+json` — the exact shape the middleware turns into an `ApiError`.
- **Not seen in a browser.** As with M0B-008, no browser tooling was available: the toast on a 5xx
  and the 401 redirect are covered by tests of the handlers, not by watching them happen. M0B-013
  (Playwright) is where that becomes checkable.
