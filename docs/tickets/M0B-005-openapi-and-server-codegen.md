# M0B-005 — `api/openapi.yaml` and the strict-mode generated server

**Milestone:** M0b · **Size:** M · **Depends on:** M0B-001

## Why

`PLAN.md` §4 makes the spec the source of truth: "Nothing is hand-written on both sides." In v1 the
frontend and backend drifted silently because each side described the API in its own language. Here,
drift is a compile error.

This ticket establishes the pattern with two trivial endpoints. Every later endpoint follows it.

## Scope

**In**

- `api/openapi.yaml` — OpenAPI 3.1, with:
  - `info`, `servers` (`/api/v1`), and a `tags` list matching the milestone areas.
  - `GET /healthz` → `{status, checks: {db: "ok"|"error"}}`.
  - `GET /version` → `{version, commit, buildDate}`.
  - The shared error schema from `M0B-007` referenced by every operation's 4xx/5xx.
  - Security schemes declared (`cookieSession`, `bearerServiceToken`) even though nothing uses them
    until M1 — so the shape is set before there are twenty endpoints to retrofit.
- `oapi-codegen` config (`api/codegen-server.yaml`) generating, in **strict mode**:
  types, server interface, and chi route registration into `internal/httpapi/gen`.
- A `spec_test.go` that loads `openapi.yaml` with `kin-openapi` and fails if it is invalid.
- `docs/api.md`: how to add an endpoint (edit spec → `make generate` → implement the interface
  method). Short — half a page.

**Out**

- Serving the routes (`M0B-006`), the TS client (`M0B-009`), auth (M1).

## Spec conventions (these are load-bearing — later tickets assume them)

- `operationId` is `camelCase`, verb-first: `listEngagements`, `getEngagement`, `patchExecutionDetection`.
  It becomes the Go method name and the TS hook name.
- Every operation has: `summary`, `tags`, explicit response codes, and a `requestBody` schema where
  applicable. No `additionalProperties: true` on a request body, ever — that is how a blue user ends
  up submitting a red field (`PLAN.md` §4).
- Every schema property has a `type`. Nullable is `type: [string, "null"]` (3.1 style), never
  `nullable: true`.
- Pagination is uniform: query params `limit` (default 50, max 200) and `cursor`; responses wrap as
  `{items: [...], nextCursor: string|null}`.
- Timestamps are `type: string, format: date-time`, UTC, RFC 3339.
- IDs are `type: string, format: uuid`.
- **Separate request bodies for separate roles.** Where red and blue write different fields of the
  same object, they get different operations and different schemas. This is the structural fix in
  `PLAN.md` §4 — do not collapse them for convenience.

## Acceptance criteria

- [ ] `make generate` produces `internal/httpapi/gen/*.go` and running it twice is a no-op.
- [ ] Generated code is committed (so `go build` works without generators installed) and the drift
      gate in `M0B-012` will catch stale checkins.
- [ ] The generated server interface uses strict mode: typed request and response structs, no
      `http.ResponseWriter` in handler signatures.
- [ ] `spec_test.go` fails when the spec is invalid — verify by temporarily breaking the spec.
- [ ] A lint step (`vacuum`, `spectral`, or a hand-written test) enforces: every operation has an
      `operationId`, a `summary`, at least one tag, and a 4xx response referencing the shared error
      schema. Wire it into `make lint`.
- [ ] `docs/api.md` exists and someone who has never seen the repo can follow it.

## Tests

- Spec validity test.
- Spec convention test (the lint step above, runnable as `go test`).

## Notes for the implementer

- Strict mode is `generate: {strict-server: true, chi-server: true, models: true}` in the codegen
  config. Read the oapi-codegen docs for the exact keys for the pinned version rather than copying
  from a blog post.
- OpenAPI 3.1 support differs between tools. If `oapi-codegen` and `kin-openapi` disagree on a
  construct, prefer the subset both accept and note it in `docs/api.md` — don't fight it.
