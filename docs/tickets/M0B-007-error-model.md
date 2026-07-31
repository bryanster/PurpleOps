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

- [ ] Every error response in the app has `Content-Type: application/problem+json` and validates
      against the spec's error schema.
- [ ] A wrapped domain error (`fmt.Errorf("...: %w", apierr.NotFound(...))`) still maps to 404 —
      i.e. translation uses `errors.As`/`errors.Is`, not type equality.
- [ ] An arbitrary error (`errors.New("boom")`) produces 500 with `code: "internal"`, and the string
      `boom` appears in the log but **not** in the response body. Assert both halves.
- [ ] Spec-validation failures from `M0B-006` produce `code: "validation_failed"` with populated
      `errors[]`.
- [ ] `instance` carries the request ID, so a user can quote it and an operator can find the log line.
- [ ] A test enumerates every `code` constant and asserts each maps to exactly one HTTP status.

## Tests

- Table-driven mapping test: error in → (status, code, body shape) out.
- A leak test: construct an error containing a fake connection string, assert it is absent from the
  serialized response.
