# The HTTP API

`api/openapi.yaml` is the source of truth. The Go server interface and the TypeScript client are
generated from it, so the two sides cannot describe the API differently — which is what happened in
v1, silently, for months.

Nothing is hand-written on both sides. A handler signature or a `fetch` call that is not in the spec
is a review rejection.

## Adding an endpoint

1. **Edit `api/openapi.yaml`.** Add the path, its `operationId`, `summary`, `tags`, request body if
   any, every response you intend to return, the problem responses it can fail with, and **what it
   requires of its caller** — see [Authorization](#authorization). Leaving the last one out fails
   `go test ./api` and, if you get that far, stops the server from starting.
2. **`make generate`.** `oapi-codegen` rewrites `internal/httpapi/gen/server.gen.go` from
   `api/codegen-server.yaml`, and `openapi-typescript` rewrites `web/src/api/schema.d.ts`. Never
   edit either; both are overwritten and CI compares them against a fresh run.
3. **Implement the new method** on the handler type that satisfies `gen.StrictServerInterface`. It
   takes a typed request and returns a typed response, one implementation per documented status
   code — there is no `http.ResponseWriter` to write something undocumented to.
4. **`make lint test`.** `lint` runs the spec's own linter (`go test ./api`); a convention breach
   fails there with the rule it broke.
5. The route is served automatically. Registration is generated too, so there is no router to
   update — if the operation is in the spec, it is mounted under `/api/v1`.
6. **On the frontend**, add a `queryOptions()` factory and a hook to the feature's `queries.ts` and
   call `api.GET("/your-path")`. The path and both bodies are checked against the regenerated
   schema. See [`web/src/api/README.md`](../web/src/api/README.md).

Removing or renaming an operation is the same loop. The generated method disappears, and the
handler that still implements it stops compiling — as does the TypeScript that read the field that
went away. That is the drift check working, on both sides.

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

Requiring a session in the document is not what enforces it. Three things are separate, and they
stay separate:

| Where | What it decides |
|---|---|
| `security` in this document | which credential an operation is *for*, and what a generated client sends |
| `authenticate` (`internal/httpapi/authn.go`) | who the caller is, if anybody. It refuses nothing |
| `x-authz-*` in this document, enforced by `internal/httpapi/authorize.go` | whether this caller may do this |

The `kin-openapi` request validator refuses to serve any operation carrying a security requirement
unless it is given an `AuthenticationFunc` (`openapi3filter.ErrAuthenticationServiceMissing`).
M1-003 supplies one that allows everything, and the comment on it in `internal/httpapi/validate.go`
says why that is not a stub: the validator runs before the cookie has been resolved, so the only
question it could answer is "is there a cookie header", and a 401 from it would not be this API's
401. **Do not put an access check there.**

## Authorization

`security` says which credential an operation takes. `x-authz-*` says what holding it entitles you
to, and every operation declares exactly one of three things. There is no default: an operation that
says nothing fails `go test ./api`, and `NewServer` refuses to build a chain over a document with a
gap in it.

```yaml
    get:
      operationId: getEngagement
      x-authz-action: engagement.read                        # the action, by its wire name
      x-authz-resource: {type: engagement, engagement: engagementId}
```

```yaml
    get:
      operationId: getCurrentUser
      x-authz-self: true                                     # signed in, acting on your own account
      x-authz-because: your own profile, and the only account this operation can name is the one that asked.
```

```yaml
    post:
      operationId: login
      x-authz-public: true                                   # no credential, no permission
      x-authz-because: this endpoint issues the credential; requiring one to reach it would be a closed door with the key inside.
```

**`x-authz-action`** names an action from the rule table in `internal/authz` — the wire names are
the left-hand column of [`docs/authz.md`](authz.md). A name that package does not define is
refused, because an operation mapped to an action nobody wrote a rule for is an unprotected
operation.

**`x-authz-resource`** says where the thing being acted on is named in the request:

| Key | Meaning |
|---|---|
| `type` | the resource kind. Must be the one the action acts on — the check exists so the endpoint and the rule table cannot read differently |
| `engagement` | the path parameter carrying the owning engagement's id. **Required** for anything an engagement owns; without it, nobody needs a membership to reach it |
| `param` | the path parameter carrying the resource's own id. Omit it where the operation names none, and for `type: engagement`, where `engagement` already does |

Both parameters must be ones the operation actually declares, or the mapping is refused.

**The two exemptions** each require a one-line `x-authz-because`, for the reason `csrfExemptRoutes`
and `enrolmentOnlyRoutes` in `internal/httpapi` require one: an exemption nobody had to justify is
an exemption nobody reviewed.

- `x-authz-public` — no credential and no permission: health, version, and the sign-in exchanges
  that issue the credential everything else needs. `security: []` implies this, and an operation
  that declares no credential *and* requires a permission is refused: nobody could satisfy it.
- `x-authz-self` — a signed-in caller acting on their own account. It is only honest while the
  operation has no way to name anybody else's, so an operation with **any path parameter** may not
  claim it. `/users/{userId}` needs an action, however the description is worded.

What happens to a refused request — 403, or 404 where confirming existence is the leak — is in
[`docs/http.md`](http.md#authorization).

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
| `unauthenticated` | 401 | `apierr.Unauthenticated(...)` for no usable session, `apierr.BadCredentials(...)` for a failed sign-in |
| `forbidden` | 403 | `apierr.Forbidden(...)`, and the authorization middleware (M1-013) |
| `mfa_enrolment_required` | 403 | The MFA enrolment gate (M1-008), for a session that must enrol a second factor before it may do anything else. **Refines `forbidden`** — see below |
| `not_found` | 404 | `apierr.NotFound(...)`, and a path that is not in the spec |
| `method_not_allowed` | 405 | The request validator, for a path that exists with other methods |
| `conflict` | 409 | `apierr.Conflict(...)` |
| `rate_limited` | 429 | `apierr.RateLimited(...)`, and the sign-in throttle (M1-004). Always carries `Retry-After`, in whole seconds |
| `internal` | 500 | Anything else at all — see below |

`ProblemCode` in the spec and that table (`internal/httpapi/apierr/codes.go`) are two halves of one
thing: adding a code means editing both in the same change, plus a `components/responses` entry
describing it. Three tests fail if you do less than that.

**Refinements.** Every code has one status, and — with one exception — every status has one code. A
code may share a status with another only by declaring itself a *refinement* of it in
`apierr.refinements`, and a refinement must be strictly more specific, so that a client which has
never heard of it can treat it as the code it refines and still be right. `mfa_enrolment_required`
refines `forbidden`: both mean "not this, not now", and the refinement additionally says the caller
is one enrolment away from being allowed. Two unrelated codes on one status stay a test failure.

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

The TypeScript side is `openapi-typescript`, which reads 3.1 natively and has so far agreed with
`kin-openapi` on every construct in this document. Where the two ever disagree, prefer the subset
both accept and add the reason to this list.

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
| `web/src/api/schema.d.ts` | Generated. Do not edit |
| `web/src/api/` | The typed client, the `ApiError` model and the query defaults — see its `README.md` |
| [`docs/http.md`](http.md) | How the server runs it: the middleware chain, headers, proxies, timeouts and shutdown |
| [`docs/security.md`](security.md) | The CSRF model: the two cookies, what is exempt and why |
