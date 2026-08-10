# Contributing

The short version: branch, make the change, `make lint test build`, open a pull request against the
working branch, and let CI answer. Nothing merges that CI has not agreed with.

## The local loop

```sh
make tools     # once: pinned generators and linters into ./bin
make lint      # golangci-lint, the OpenAPI spec linter, tsc + eslint + prettier
make test      # Go and web unit tests
make build     # SPA, then both binaries into ./bin
```

`make help` lists every target. Two more matter before you push:

```sh
make test-race      # what CI runs: -race and a coverage profile. Slower.
make generate       # then `git status` — a dirty tree here fails CI
```

If you touched anything a browser sees — a route, a screen, the shell, the API behind one — run the
end-to-end suite too. It drives the real binary against a real database:

```sh
make e2e-browsers   # once, and again after a Playwright upgrade
make e2e            # builds, then runs the Playwright suite
```

[`docs/testing.md`](testing.md) covers headed runs, the trace viewer, and how to seed a spec.

`make generate` is not optional bookkeeping. `internal/httpapi/gen/server.gen.go` and
`web/src/api/schema.d.ts` are committed, so editing [`api/openapi.yaml`](../api/openapi.yaml)
without regenerating leaves a tree that still compiles and is still wrong. See
[`docs/api.md`](api.md).

The container is the supported artifact, so if you changed anything it depends on — the Dockerfile,
the entrypoint, the embedded SPA, startup — run its checks too:

```sh
make docker-smoke   # builds the image and makes 14 assertions about it
```

## What CI runs

[`.github/workflows/ci.yml`](../.github/workflows/ci.yml), on every push to `main` and on
every pull request. No secrets are used anywhere, so a fork's pull request gets the same verdict as
a branch here.

| Job | What it proves |
|---|---|
| `Lint` | `golangci-lint`, the spec's own conventions, `tsc --noEmit`, eslint, prettier |
| `Go tests` | `go test -race`, with the coverage number in the run summary |
| `Web tests` | `vitest` |
| `Generated code is current` | `make generate` leaves the tree clean |
| `Build linux/amd64`, `Build linux/arm64` | The CGO build works on both, and produces a binary of the architecture it claims |
| `Container smoke test` | The image builds, boots with no network, persists data, and still has a working Chromium |
| `End-to-end` | A browser reaches the real binary's UI and the API behind it — and the run fails, rather than skipping, if no server ever answers |
| `CI` | Every job above succeeded |

Coverage is reported, never gated. A percentage target makes people test getters; the per-layer
requirements in [`PLAN.md`](../PLAN.md) §9 are the real bar.

## Branch protection

These are the settings on the default branch. They are recorded here because GitHub's UI is not in
version control, and a protection rule that silently disappears is the same as never having had one.

**Require a pull request before merging** — no direct pushes, including for administrators.

- Required approvals: 1.
- Dismiss stale approvals when new commits are pushed.

**Require status checks to pass before merging**, with branches required to be up to date.

- Required check: **`CI`** — and only that one.

  The `CI` job depends on every other job in the workflow and fails if any of them reported
  anything other than success, including *skipped* and *cancelled*. Requiring it rather than the
  six individual jobs means adding a job (M0B-013 adds an e2e job) does not need a settings change,
  and a job that never started cannot be mistaken for a job that passed.

**Require linear history** — merge commits are rejected. Squash or rebase.

**Require conversation resolution before merging.**

**Do not allow force pushes**, and **do not allow deletions**.

Administrator bypass is off for all of the above. If a rule is wrong, change the rule.

## Pull requests

The backlog in [`docs/tickets/`](tickets/) is the unit of work. Its
[README](tickets/README.md#definition-of-done) has the definition of done that every ticket assumes;
the ticket file itself has the acceptance criteria a reviewer will run down.

A pull request description says **what was tested and how**, and links the ticket file. "CI is
green" is not an answer to that question — CI runs unit tests, a container smoke test and an
end-to-end suite, and none of them knows whether the feature does what the ticket asked for.

When the implementation had to deviate from the ticket, the reason goes in the ticket file under
**Implementation notes** before it moves to `tickets/done/`. The next person to touch that area
reads those notes instead of rediscovering the constraint.

## Dependencies

Dependabot opens one grouped pull request per ecosystem per month
([`.github/dependabot.yml`](../.github/dependabot.yml)), a week after a release so a bad publish has
time to be yanked. Actions are pinned to commit SHAs with the version in a trailing comment;
Dependabot updates both, so leave the comment in place.


## Releasing

Maintainers: see [`docs/releasing.md`](releasing.md) for the tag→release workflow,
version policy, and verification steps.