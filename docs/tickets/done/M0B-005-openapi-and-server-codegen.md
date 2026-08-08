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

- [x] `make generate` produces `internal/httpapi/gen/*.go` and running it twice is a no-op.
- [x] Generated code is committed (so `go build` works without generators installed) and the drift
      gate in `M0B-012` will catch stale checkins.
- [x] The generated server interface uses strict mode: typed request and response structs, no
      `http.ResponseWriter` in handler signatures.
- [x] `spec_test.go` fails when the spec is invalid — verify by temporarily breaking the spec.
- [x] A lint step (`vacuum`, `spectral`, or a hand-written test) enforces: every operation has an
      `operationId`, a `summary`, at least one tag, and a 4xx response referencing the shared error
      schema. Wire it into `make lint`.
- [x] `docs/api.md` exists and someone who has never seen the repo can follow it.

## Tests

- Spec validity test.
- Spec convention test (the lint step above, runnable as `go test`).

## Notes for the implementer

- Strict mode is `generate: {strict-server: true, chi-server: true, models: true}` in the codegen
  config. Read the oapi-codegen docs for the exact keys for the pinned version rather than copying
  from a blog post.
- OpenAPI 3.1 support differs between tools. If `oapi-codegen` and `kin-openapi` disagree on a
  construct, prefer the subset both accept and note it in `docs/api.md` — don't fight it.

---

## Implementation notes (added on completion)

### What exists now

`api/openapi.yaml` (OpenAPI 3.1) with `GET /healthz` and `GET /version`, the RFC 9457 problem model,
both security schemes, the ten milestone tags and the shared pagination parameters.
`make generate` turns it into `internal/httpapi/gen/server.gen.go` — models, a strict server
interface and chi route registration. `api/spec.go` embeds and parses the document; it is the only
loader in the tree, and the `go:generate` line lives there so every path around the generator is
relative to `api/`.

### `api/` is a Go package now

`//go:embed` cannot reach outside its own directory, so the spec can only be embedded from `api/`.
That decided where the embed, the loader and the spec tests live. The alternative — a test that
reads `../api/openapi.yaml` by relative path — breaks the moment it is run from anywhere else, and
would have left M0B-006 to write the embed a second time.

`api.Load()` parses on every call rather than caching: the returned `*openapi3.T` is a mutable tree
that the kin-openapi validator writes into, and a shared copy would make one caller's state another
caller's problem. It is called once, at startup.

### Decisions the ticket left open

1. **Authenticated by default.** The document declares a top-level `security` requirement, so an
   operation added without a thought about auth inherits "needs a session". The two current
   operations opt out with `security: []`, and a test rejects an opt-out that does not explain itself
   in its description. This is the shape `PLAN.md` §4 asks for, set before there are twenty
   endpoints.

   **The trap M1 will hit:** `kin-openapi`'s request validator refuses to serve any operation
   carrying a security requirement unless it is given an `AuthenticationFunc`
   (`openapi3filter.ErrAuthenticationServiceMissing`). The first authenticated operation and the
   authentication middleware must therefore land in the same change. Both current operations being
   public is what lets M0B-006 wire the validator with no authenticator and no allow-all stub —
   a stub that would certainly have outlived its usefulness.

2. **`/healthz` returns 503 with a health document, not a problem document.** A monitor reads
   `checks` to learn *what* is unhealthy; a problem document would tell it only *that* something is.
   The convention test therefore checks "every response carrying `application/problem+json`
   references the shared `Problem` schema, and every operation documents at least one of them",
   with a one-entry `nonProblemErrorResponses` map naming this exception and its reason. An
   exception you have to write a sentence for is better than a rule with a hole in it.

3. **Six problem codes, six shared responses, no more.** `ProblemCode` is exactly the enum M0B-007
   specifies (`validation_failed`, `forbidden`, `not_found`, `conflict`, `rate_limited`, `internal`)
   and `components/responses` has exactly one entry per code. No `unauthenticated` and no
   `method_not_allowed` were invented here: M0B-007 owns the code→status table and asserts it is
   1:1, so a code in the spec with no constant behind it would break that test. M0B-006 (405) and
   M1-003 (401) add theirs in both places, in the same change.

4. **`/version` is public.** The SPA shows it on the login page and support requests are
   unanswerable without it; it reveals the version and commit of software whose source is public.
   Requiring a session for it would also have meant an authenticator in M0B-006 — see (1).

5. **`buildDate` is `type: string` with no `format: date-time`.** An unstamped build reports the
   literal `unknown` (`internal/version`), and a client must not be told to expect a parseable
   timestamp that sometimes is not one.

6. **Two codegen options beyond the ones the ticket named.**
   `compatibility.always-prefix-enum-values: true` — without it, constants are only prefixed with
   their type name once two enums collide, so adding an enum with an `ok` value silently renames
   `Ok` to `HealthStateOk` and every handler using it stops compiling.
   `output-options.nullable-type: true` — makes `type: [string, "null"]` generate a wrapper that
   keeps "absent" and "explicitly null" apart, which the PATCH endpoints in M3 need.
   `generate.embedded-spec: false` — the generator can embed a gzipped copy of the spec; that would
   be a second source of truth alongside `api/spec.go`, able to disagree with it.

7. **Format validation is on** in the loader (`openapi3.EnableSchemaFormatValidation`). A `format`
   the specification does not define — a typo, or an invented `attack-technique` — is an error
   rather than a silently-ignored hint. This was measured: unknown formats and mismatched ones
   (`format: uuid` on an integer) are both rejected at load. It is why there is no separate
   convention rule for formats; see below.

### Mutation testing: what the tests actually catch

Every convention was broken on purpose and the owning test re-run. All twenty were caught, most with
a message naming the fix:

| Broken | Caught by |
|---|---|
| `operationId` removed / duplicated / not camelCase | `TestEveryOperationHasAUniqueOperationID` (the duplicate is caught by kin-openapi first) |
| `summary` removed | `TestEveryOperationHasAOneLineSummary` |
| Operation tagged with an undeclared tag; a declared tag with no description | `TestEveryOperationIsTaggedWithADeclaredTag` |
| Operation with no 500; problem response declared inline; a 4xx that is not a problem document; a `default` response | `TestEveryOperationDocumentsItsErrors` |
| A property with no `type` | `TestEverySchemaDeclaresItsType` |
| `nullable: true` (3.0 spelling) | `TestNoSchemaUsesTheOpenAPI30NullableFlag` |
| A request body with `additionalProperties: true` | `TestNoRequestBodyAcceptsUnknownFields` |
| Document-level `security` removed; an undeclared scheme referenced | `TestTheAPIIsAuthenticatedByDefault` |
| A public operation with no explanation | `TestEveryPublicOperationSaysWhyItIsPublic` |
| The page limit's maximum changed; an inline `limit` parameter | `TestPaginationIsDeclaredOnce` |
| An absolute server URL | `TestTheOnlyServerIsTheVersionedRelativePath` |
| `strict-server` turned off in the codegen config | `internal/httpapi/gen/strictmode_test.go` — stops compiling |
| A dangling `$ref`, a response with no description, a mistyped `type`, malformed YAML | `TestLoadRejectsABrokenSpec`, which mutates the real document rather than a fixture |

A convention rule for "string formats belong on strings" was written and then **deleted**: the
loader's own format validation rejects every case first, so the rule could never be the thing that
failed. A rule that cannot fail is noise in a file whose whole purpose is to fail usefully.

`TestLoadRejectsABrokenSpec` fails if a mutation stops applying — a rule that quietly stops being
exercised is worse than one that was never written.

### Deviations from the ticket

- **The lint rule is "at least one problem response, and a mandatory 500", not "a 4xx response".**
  Neither current operation has a reachable 4xx: they take no parameters and no body, and both are
  public. Declaring a 400 to satisfy a linter would be documentation that lies, and the scope
  section of this ticket says "4xx/5xx" where the criterion says "4xx". Every problem response must
  additionally be one of the shared `components/responses`, which is stricter than what was asked
  and is the part that actually keeps the error model singular.
- **No `vacuum` or `spectral`.** The hand-written Go test the ticket permits needs no second
  toolchain, needs no network, and can assert things a generic linter cannot (that a public endpoint
  explains itself; that pagination is declared once). `make lint` gains a `lint-spec` target that
  runs `go test ./api`.
- **The pagination parameters and eight of the ten tags are declared but unused.** Same reasoning
  the ticket gives for the security schemes: set the shape before there are twenty endpoints to
  retrofit. `oapi-codegen` prunes unreferenced components, so they cost nothing in generated code.

### For the tickets that consume this

- **M0B-006**: `api.Load()` returns the validated document for the `OapiRequestValidator`; there is
  no need to read the file from disk and no need for an `AuthenticationFunc` yet — see decision (1).
  Mount with `gen.HandlerFromMux(gen.NewStrictHandler(handlers, middlewares), router)` under
  `/api/v1`. `/healthz` returns `GetHealth200JSONResponse` or `GetHealth503JSONResponse`, both
  `Health`; `/version` returns `GetVersion200JSONResponse`, whose fields match
  `internal/version.Info` exactly.
- **M0B-007**: the `Problem`, `ProblemCode` and `FieldError` schemas are in the spec and generated as
  `gen.Problem` etc. The generated response types already set `Content-Type:
  application/problem+json`. Six codes; adding one means editing the enum and the `apierr` table
  together.
- **M0B-009**: generate the TypeScript client from the same `api/openapi.yaml`. If
  `openapi-typescript` disagrees with a 3.1 construct, prefer the subset both accept and add it to
  the list in `docs/api.md`.
- **Adding an endpoint**: `docs/api.md` is the procedure. The short version is edit the spec, run
  `make generate`, implement the method — and the conventions above are enforced, so a breach fails
  `make lint` rather than review.
