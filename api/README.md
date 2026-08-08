# api/

`openapi.yaml` is the source of truth for the HTTP API: the Go server interface and the TypeScript
client are both generated from it, so hand-writing either side is a review rejection.

| File | What it is |
|---|---|
| `openapi.yaml` | The spec |
| `codegen-server.yaml` | `oapi-codegen` configuration for the Go server (`internal/httpapi/gen`) |
| `spec.go` | Embeds and parses the spec, and holds the `go:generate` line that runs the generator |
| `spec_test.go`, `conventions_test.go` | The spec's validity and its conventions — run by `make lint` |

Read [`docs/api.md`](../docs/api.md) before adding an endpoint. The TypeScript client arrives in
M0B-009.
