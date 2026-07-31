# api/

`openapi.yaml` lives here and is the source of truth for the HTTP API: the Go server interface and
the TypeScript client are both generated from it, so hand-writing either side is a review rejection.

The spec and its codegen configuration arrive in M0B-005; the TypeScript client in M0B-009.
