# M0B-001 — Repo skeleton, Go module, Makefile, pinned tooling

**Milestone:** M0b · **Size:** M · **Depends on:** nothing

## Why

Every other ticket assumes a layout and a `make` target. `PLAN.md` §6 fixes the layout; this ticket
creates it, and pins the code generators so two developers generate byte-identical output. Without
pinning, the CI codegen-drift gate (`M0B-012`) produces false failures and gets disabled — which is
how spec-first rebuilds quietly die.

## Scope

**In**

- `go.mod` with module `github.com/bryanster/purpleops`, Go 1.24.
- The directory tree from `PLAN.md` §6, each package holding a real (possibly trivial) Go file so
  the tree compiles. No empty directories, no `.gitkeep`.
- `Makefile` with the targets below.
- `tools/tools.go` (build-tagged `//go:build tools`) importing every code generator so
  `go.mod` pins their versions.
- `.golangci.yml`, `.gitignore`, `.editorconfig`, `LICENSE` (carry over v1's licence — recover it
  with `git show <v1-tip>:LICENSE`).
- `README.md` stub: what this is, how to run it, one paragraph. Full rewrite is `M7`.

**Out**

- Any HTTP, DB, or React code. Those are separate tickets.
- CI workflows (`M0B-012`).

## Files

```
go.mod  go.sum  Makefile  .golangci.yml  .gitignore  .editorconfig  README.md  LICENSE
tools/tools.go
api/                          # openapi.yaml lands in M0B-005
cmd/purpleops/main.go         # prints version, exits 0 for now
cmd/popsctl/main.go           # M0B-014 fills this in
internal/config/              internal/store/
internal/domain/              internal/content/
internal/httpapi/             internal/authn/        internal/authz/
internal/evidence/            internal/events/
internal/analytics/           internal/report/
internal/version/version.go   # Version, Commit, BuildDate — set via -ldflags
web/                          # M0B-008
deploy/  docs/
```

## Make targets

Each must work from a clean checkout on Linux with Go and Node installed.

| Target | Does |
|---|---|
| `make tools` | Installs pinned generators into `./bin` via `go install` using versions from `go.mod` |
| `make generate` | Runs every generator: Go server stubs, TS client, anything else. Idempotent |
| `make lint` | `golangci-lint run` + `npm run lint` in `web/` |
| `make test` | `go test ./...` + `npm test` in `web/` |
| `make build` | Builds `web/` then `CGO_ENABLED=1 go build -o bin/purpleops ./cmd/purpleops` with version ldflags |
| `make run` | `make build` then runs the binary |
| `make clean` | Removes `bin/`, `web/dist/` |

`make generate` must not require network access after `make tools` has run once.

## Acceptance criteria

- [x] `go build ./...` and `go vet ./...` pass on a clean clone.
- [x] `make tools && make generate` leaves the working tree unchanged (`git diff --exit-code`).
- [x] Running `make generate` twice in a row produces identical output both times.
- [x] `./bin/purpleops --version` prints a version, commit SHA and build date populated by ldflags,
      not placeholder strings.
- [x] `.golangci.yml` enables at minimum: `errcheck`, `govet`, `staticcheck`, `ineffassign`,
      `unused`, `gosec`, `bodyclose`, `sqlclosecheck`, `rowserrcheck`, `contextcheck`.
- [x] `.gitignore` covers `bin/`, `web/node_modules/`, `web/dist/`, `*.duckdb*`, `evidence/`, `.env`.
- [x] No directory in the tree is empty.

## Tests

- `internal/version` has a test asserting the struct is populated (guards against ldflags path typos
  by failing when values are the zero string in a release build).

## Notes for the implementer

- `tools/tools.go` pattern:
  ```go
  //go:build tools
  package tools
  import _ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
  ```
  This is how the generator version lives in `go.mod` instead of in someone's `$PATH`.
- Do **not** add `github.com/duckdb/duckdb-go/v2` yet — `M0B-003` owns that dependency and its
  build constraints.

---

## Implementation notes (added on completion)

Three decisions deviate from the text above. Later tickets depend on them.

1. **`go.mod` says `go 1.25.0`, not 1.24.** `oapi-codegen` v2.8.0 and its whole transitive tree
   (`kin-openapi`, `x/tools`, `x/net`) declare `go 1.25`, so `go mod tidy` raises the directive.
   Holding at 1.24 would have meant pinning the generator several releases back. `PLAN.md` does not
   name a Go version, so there is no conflict with it. **`M0B-011` (Docker base image) and
   `M0B-012` (CI build matrix) must use 1.25.**
2. **`golangci-lint` is pinned in the Makefile (`GOLANGCI_LINT_VERSION`), not in `tools/tools.go`.**
   It is a linter, not a generator, so it does not need to produce byte-identical output across
   machines, and its dependency tree is large enough to constrain ordinary library upgrades if it
   sits in `go.mod`. It is installed with an explicit `@version`, which keeps CI reproducible.
   `tools/tools.go` holds generators only.
3. **`govet`'s `shadow` check is disabled** in `.golangci.yml`. It flags `if err := f(); err != nil`,
   which is the idiom rather than a defect. `fieldalignment` is disabled for the same class of
   reason. Every other `govet` analyser is on.

`make generate` depends on the generator binaries rather than on `make tools`, so a present and
up-to-date `./bin` is used offline and only a missing or stale one triggers a fetch.

The ldflags test (`TestLDFlagsPopulateInfo`) reads the flags from `make print-ldflags` so the
Makefile stays the single source of truth for how a build is stamped. Both halves were verified to
fail on purpose: a typo'd `-X` path fails the static check, and renaming a stamped variable — which
the linker ignores silently — fails the probe assertion.
