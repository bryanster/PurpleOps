# SDKs

Four clients for the Blacklight API, generated from [`api/openapi.yaml`](../api/openapi.yaml).

| | Import | Getting started |
|---|---|---|
| Go | `github.com/bryanster/blacklight/sdk/go` | [`go/README.md`](go/README.md) |
| TypeScript / JavaScript | `blacklight-sdk` | [`typescript/README.md`](typescript/README.md) |
| Python | `blacklight` | [`python/README.md`](python/README.md) |
| Rust | `blacklight` | [`rust/README.md`](rust/README.md) |

All four are **generated code and must not be edited**. `make generate` overwrites them, and CI
fails if what is committed differs from a fresh run. Each has exactly one hand-written file (two in
Rust), named in its README, holding the two things the OpenAPI document cannot say: that `/api/v1`
has to be appended to a deployment's origin, and how a service token is presented.

[`docs/sdk.md`](../docs/sdk.md) is how they are built, why each language uses the generator it uses,
and what to do when one needs upgrading.
