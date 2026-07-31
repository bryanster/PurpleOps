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

- [ ] `make generate` regenerates `schema.d.ts`; running twice is a no-op; the file is committed.
- [ ] Changing a response field in `openapi.yaml` and regenerating makes `npm run build` fail in the
      component that used it. **Demonstrate this in the PR description** — it is the whole point of
      the ticket.
- [ ] No string literal URL for an API route exists anywhere in `web/src` outside `api/client.ts`
      (add an ESLint rule or a test that greps; automated either way).
- [ ] A 404 problem response surfaces as `ApiError` with `code: "not_found"`, not as a thrown
      `SyntaxError` from parsing.
- [ ] Queries do not retry on 4xx; they do retry (bounded) on network failure and 5xx.
- [ ] The demo screens from `M0B-008` are rewritten to use hooks, with loading and error states that
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
