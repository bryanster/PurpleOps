# M0B-007 — One error model, end to end

**Milestone:** M0b · **Size:** S · **Depends on:** M0B-005

## Why

Two things go wrong without this. Clients end up with per-endpoint error parsing, and — worse —
handlers leak internals: v1 returned raw driver errors to the browser. One shape, one translation
layer, decided now while there are two endpoints instead of eighty.

## Scope

**In**

- A single error response schema in `api/openapi.yaml`, RFC 9457 (`application/problem+json`) shaped:
  ```
  { type, title, status, detail, instance, code, errors?: [{field, message}] }
  ```
  - `code` is a stable machine-readable string (`not_found`, `validation_failed`, `forbidden`,
    `conflict`, `rate_limited`, `internal`). Clients switch on `code`, never on `detail`.
  - `errors[]` is for field-level validation failures.
- `internal/httpapi/apierr` — sentinel errors and constructors the domain layer returns
  (`apierr.NotFound("engagement", id)`, `apierr.Conflict(...)`, `apierr.Forbidden(...)`,
  `apierr.Validation(fieldErrs)`).
- One translation point that maps a Go error to a problem response, used by the generated server's
  error handler and by the validation middleware, so *every* error path produces the same shape.
- Rule, documented in the package comment: **an unrecognised error becomes 500 `internal` with a
  generic detail**, the real error goes to the log with the request ID. Never to the client.

**Out**

- Localisation. English only.
- Retry hints / `Retry-After` beyond what `rate_limited` needs in `M1-004`.

## Acceptance criteria

- [x] Every error response in the app has `Content-Type: application/problem+json` and validates
      against the spec's error schema. — `Responder.Write` is the only writer and sets the header
      (`TestWriteServesTheProblemMediaTypeAndStatus`); every test that builds a document ends in
      `validateAgainstSpec`, which checks it against the `Problem` schema in the embedded spec.
      Nothing else in the app can produce an error response yet; M0B-006 installs this as the error
      handler for the router, the strict handler and the validator, which closes "in the app".
- [x] A wrapped domain error (`fmt.Errorf("...: %w", apierr.NotFound(...))`) still maps to 404 —
      i.e. translation uses `errors.As`/`errors.Is`, not type equality.
      — `TestEachConstructorCarriesItsCodeStatusAndSentinel` wraps every constructor;
      `TestTranslateMapsAnErrorToItsProblem/a_wrapped_domain_error_keeps_its_code`.
- [x] An arbitrary error (`errors.New("boom")`) produces 500 with `code: "internal"`, and the string
      `boom` appears in the log but **not** in the response body. Assert both halves.
      — `TestWriteLogsTheCauseItDoesNotSend`.
- [x] Spec-validation failures from `M0B-006` produce `code: "validation_failed"` with populated
      `errors[]`. — `validation_test.go` drives the real `openapi3filter` validator; see note (2).
- [x] `instance` carries the request ID, so a user can quote it and an operator can find the log
      line. — asserted in the mapping table, in `TestWriteLogsTheCauseItDoesNotSend` (same ID in the
      log line) and by `TestTranslateOmitsAnAbsentRequestID` for the empty case.
- [x] A test enumerates every `code` constant and asserts each maps to exactly one HTTP status.
      — `codes_test.go` enumerates the enum **from the embedded spec**, not from a Go list, so a
      code added to one side and not the other fails.

## Tests

- [x] Table-driven mapping test: error in → (status, code, body shape) out.
      — `TestTranslateMapsAnErrorToItsProblem`.
- [x] A leak test: construct an error containing a fake connection string, assert it is absent from
      the serialized response. — `TestTranslateLeaksNothingFromAnUnrecognisedError`, plus
      `TestASpecViolationSaysNothingAboutTheValue` for the validator's messages.

## Implementation notes

### 1. A seventh code: `method_not_allowed` (agreed scope change)

The ticket lists six codes. `M0B-006` requires a wrong method to return **405** as a problem
document, and the 1:1 code/status rule — asserted by `api/conventions_test.go` and stated in
`docs/api.md` — leaves no code that can carry it. Adding it here rather than in `M0B-006` keeps the
table complete and means the next ticket does not have to reopen the spec.

That is three edits, and there is now a test for each: the `ProblemCode` enum, a
`components/responses/MethodNotAllowed` entry (unreferenced by any operation, because a 405 is
decided before a request matches one — the description says so), and the `codes` table.
`TestEveryProblemCodeHasASharedResponse` is new and is what makes "adding a code means editing both
places" a build failure rather than a convention.

There is still **no code for 401**. Until M1 nothing is authenticated, and an unsatisfied security
requirement is reported as `forbidden`/403 — see `translateSpecError`. `M1-003` should add
`unauthenticated` → 401 in the same way.

### 2. OpenAPI 3.1 changes the shape of validation errors

Worth knowing before touching `validation.go`. For a 3.1 document — which `api/openapi.yaml` is —
`openapi3filter` validates bodies and parameters with **JSON Schema 2020-12**
(`santhosh-tekuri/jsonschema`), not with kin-openapi's own validator. It is switched on
unconditionally by `IsOpenAPI31OrLater()`; there is no option to turn it off.

The consequence: `SchemaError.JSONPointer()` comes back **empty**, and the field path survives only
inside the prose of `Reason`:

```
error at "/members/0/role": at '/members/0/role': value must be one of 'lead', 'red', 'blue', 'observer'
```

`schemaFieldError` recovers the path from that with a regexp, and the per-field errors themselves
are nested one level down in `Origin` rather than being the error you are handed. Both are pinned by
tests that run the real validator instead of a fixture, so a kin-openapi upgrade that rewords this
fails loudly rather than silently serving `errors[]` entries with no `field` in them. If it does
fail, the shape of the tree is what to re-derive — the dump is four lines of `switch err.(type)`.

Two related consequences: `openapi3filter.Options.MultiError` no longer affects body validation (the
JSON Schema validator returns every cause anyway), and `missing property 'x'` is reported against
the *enclosing* object, so it is re-pointed at the missing field to make it usable in a form.

### 3. The request ID comes from chi's context key

`Responder.Write` reads it with `middleware.GetReqID` (`github.com/go-chi/chi/v5/middleware`), so
that this package, the logging middleware and any chi middleware all agree on one value.

**For `M0B-006`:** whatever `RequestID` middleware you write must store the ID under
`middleware.RequestIDKey`. chi's own `RequestID` is not usable as-is — it neither echoes the header
back on the response nor generates the UUIDv7 this project uses — but a replacement that sets the
same key works with no change here. Note also that chi's version trusts an inbound `X-Request-Id`
unconditionally; the replacement should bound its length and character set, since the value is
echoed into every problem document and every log line.

### 4. Deliberately not done

- No `Retry-After` and no rate-limit accounting: `M1-004` owns that, and `RateLimited` is here only
  so the code exists.
- `Responder` does not set `X-Request-Id` on the response. Echoing the header is the middleware's
  job in `M0B-006`, and doing it in two places would let them disagree.
- No English-language plumbing beyond what the constructors set. Localisation is out per the ticket,
  and every message is a constant, so it stays greppable when that changes.
