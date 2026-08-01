# The HTTP API

`api/openapi.yaml` is the source of truth. The Go server interface and the TypeScript client are
generated from it, so the two sides cannot describe the API differently — which is what happened in
v1, silently, for months.

Nothing is hand-written on both sides. A handler signature or a `fetch` call that is not in the spec
is a review rejection.

## Adding an endpoint

1. **Edit `api/openapi.yaml`.** Add the path, its `operationId`, `summary`, `tags`, request body if
   any, every response you intend to return, and the problem responses it can fail with.
2. **`make generate`.** `oapi-codegen` rewrites `internal/httpapi/gen/server.gen.go` from
   `api/codegen-server.yaml`. Never edit that file; it is overwritten and CI compares it against a
   fresh run.
3. **Implement the new method** on the handler type that satisfies `gen.StrictServerInterface`. It
   takes a typed request and returns a typed response, one implementation per documented status
   code — there is no `http.ResponseWriter` to write something undocumented to.
4. **`make lint test`.** `lint` runs the spec's own linter (`go test ./api`); a convention breach
   fails there with the rule it broke.
5. The route is served automatically. Registration is generated too, so there is no router to
   update — if the operation is in the spec, it is mounted under `/api/v1`.

Removing or renaming an operation is the same loop. The generated method disappears, and the
handler that still implements it stops compiling: that is the drift check working.

## Conventions

These are enforced by `api/conventions_test.go`. Each failure message says what breaking the rule
costs; if one is wrong, argue with the message.

| Rule | Why |
|---|---|
| `operationId` is camelCase, verb-first, unique | It becomes the Go method name and the TypeScript hook name |
| Every operation has a one-line `summary` and at least one declared `tag` | The only description a client reader sees, and how endpoints are grouped |
| Explicit status codes, no `default` response | So the generated client has a type per outcome |
| Every operation documents a 500 and at least one problem response | A caller with no error type cannot handle a failure |
| Error responses `$ref` a shared response in `components/responses` | One error shape; the code/status pairing stays 1:1 with `internal/httpapi/apierr` |
| Every `ProblemCode` has one shared response whose description says "`` `code` is `x` ``" | A code with no response is a status no operation can document |
| Every schema declares a `type` | An untyped schema generates as `interface{}` in Go and `unknown` in TypeScript |
| Nullable is `type: [string, "null"]`, never `nullable: true` | The latter is OpenAPI 3.0 and a 3.1 reader ignores it |
| No `additionalProperties: true` on a request body | A request type that accepts unknown fields is how a caller writes a field it has no right to (`PLAN.md` §4) |
| `limit` and `cursor` come from `components/parameters` | One page contract everywhere: `{items, nextCursor}`, `limit` defaulting to 50 and capped at 200 |
| One server, `/api/v1`, relative | The SPA is same-origin; an absolute URL would pin every deployment to one host |

Two more the reader should know, though a test cannot check them:

- **Timestamps** are UTC RFC 3339 (`type: string, format: date-time`); **IDs** are UUIDv7
  (`type: string, format: uuid`). Never format a time for display on the server.
- **Red and blue write through different operations with different request bodies.** Where two roles
  write different fields of the same object, they get separate endpoints — a user cannot submit a
  field that does not exist in their request type. Do not collapse them for convenience;
  that is the structural fix in `PLAN.md` §4.

## Authentication

The document declares `security` at the top level, so **an endpoint is authenticated unless it says
otherwise**. An operation that is genuinely public sets `security: []` and explains itself in its
`description` — the test rejects an unexplained one.

One trap, for whoever adds the first authenticated operation: the `kin-openapi` request validator
refuses to serve any operation carrying a security requirement unless it is given an
`AuthenticationFunc` (`openapi3filter.ErrAuthenticationServiceMissing`). The endpoint and the
authentication middleware have to land in the same change. Do not paper over it with a
permissive stub.

## Errors

Every error is an RFC 9457 problem document, served as `application/problem+json`:

```json
{ "type": "about:blank", "title": "Not Found", "status": 404, "detail": "no such engagement",
  "instance": "018f3b2c-7a41-7c3e-9b0d-2f1a4c6e8d90", "code": "not_found" }
```

Clients switch on `code`, never on `detail` (prose, may be reworded) and never on the status alone.
`instance` is the request ID, so a user can quote it and an operator can find the log line.

| `code` | Status | Raised by |
|---|---|---|
| `validation_failed` | 400 | The request validator, or `apierr.Validation(...)` for a rule the spec cannot express |
| `forbidden` | 403 | `apierr.Forbidden(...)`, and the authorization middleware (M1-013) |
| `not_found` | 404 | `apierr.NotFound(...)`, and a path that is not in the spec |
| `method_not_allowed` | 405 | The request validator, for a path that exists with other methods |
| `conflict` | 409 | `apierr.Conflict(...)` |
| `rate_limited` | 429 | Login throttling (M1-004) |
| `internal` | 500 | Anything else at all — see below |

`ProblemCode` in the spec and that table (`internal/httpapi/apierr/codes.go`) are two halves of one
thing: adding a code means editing both in the same change, plus a `components/responses` entry
describing it. Three tests fail if you do less than that.

### Returning an error from a handler

Return an `apierr` value and wrap it as you normally would; translation uses `errors.As`, so
wrapping does not change the answer.

```go
engagement, err := s.engagements.Get(ctx, id)
if err != nil {
    return nil, fmt.Errorf("get engagement: %w", err) // still a 404 if Get returned apierr.NotFound
}
```

**An error this vocabulary does not recognise becomes a 500 with `code: "internal"` and a generic
detail.** The real error goes to the log with the request ID; it never reaches the client. v1
returned raw driver errors to the browser, and this is the structural answer to that — so an error
that should tell the caller something has to say so through a constructor. There is no path by which
an unclassified error surfaces in a response.

The same split applies inside a constructor: `apierr.NotFound("engagement", id)` tells the client
"no such engagement" and tells the log which id. `apierr.Forbidden(action)` tells the client nothing
about what was attempted.

## OpenAPI 3.1, and where the tools disagree

The document is 3.1. The Go toolchain — `oapi-codegen` and the runtime validator — is `kin-openapi`
in both cases, so it agrees with itself. Known constraints, worth knowing before fighting them:

- **`format` must be one the specification defines.** The loader validates formats, so an invented
  one (`format: attack-technique`) is an error, not a hint. Registering a custom format is possible
  (`openapi3.DefineStringFormat`) but means teaching every generator about it — prefer a `pattern`.
- **`examples:` (a list) is the 3.1 spelling.** `example:` singular is 3.0 and is not carried through.
- **Type arrays are how nullability is spelled**, and `nullable-type` in the codegen config turns
  them into a wrapper that distinguishes "absent" from "explicitly null".

When the TypeScript generator lands (M0B-009) and disagrees with a construct, prefer the subset both
accept and add the reason to this list.

## Files

| Path | What it is |
|---|---|
| `api/openapi.yaml` | The spec. The source of truth |
| `api/codegen-server.yaml` | What `oapi-codegen` is asked to produce, and why |
| `api/spec.go` | Embeds the spec and parses it; the only loader. Also holds the `go:generate` line |
| `api/spec_test.go`, `api/conventions_test.go` | The spec's validity and its conventions |
| `internal/httpapi/gen/server.gen.go` | Generated. Do not edit |
| `internal/httpapi/gen/strictmode_test.go` | Asserts the generator was asked for a strict chi server |
| `internal/httpapi/apierr` | The error vocabulary, the code/status table, and the one place a Go error becomes a response |
