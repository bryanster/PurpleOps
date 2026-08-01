# PurpleOps v2 — developer entry points.
#
# Every target works from a clean checkout with Go and Node installed.
# `make tools` is the only target that needs network access; `make generate`
# runs entirely from ./bin afterwards.

SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

MODULE   := github.com/bryanster/purpleops
BIN_DIR  := $(CURDIR)/bin
WEB_DIR  := $(CURDIR)/web
E2E_DIR  := $(CURDIR)/e2e

# The Go half of the repo, listed rather than `./...`. web/node_modules is
# inside the module directory and npm packages sometimes ship Go sources of
# their own — `./...` compiles and vets those, so a dependency nobody here
# controls could fail the build. golangci-lint is told the same thing in
# .golangci.yml.
#
# ./web (without /...) is the embed package in web/, and nothing below it.
GO_PACKAGES := ./api/... ./cmd/... ./internal/... ./tools/... ./web

# Pinned outside go.mod deliberately: golangci-lint's dependency tree is large
# and conflicts with ordinary library upgrades, so it is installed with an
# explicit @version instead of via tools/tools.go. Generators, whose output must
# be byte-identical everywhere, are pinned in go.mod — see tools/tools.go.
GOLANGCI_LINT_VERSION := v2.5.0

# Version stamping. Overridable so a release pipeline can pass exact values.
# TestLDFlagsPopulateInfo asserts these -X paths still resolve.
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo v2-dev)
COMMIT     ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X $(MODULE)/internal/version.version=$(VERSION) \
           -X $(MODULE)/internal/version.commit=$(COMMIT) \
           -X $(MODULE)/internal/version.buildDate=$(BUILD_DATE)

# Generators resolve from ./bin before anything in the developer's PATH.
export PATH := $(BIN_DIR):$(PATH)

# The web half of each target is skipped when web/package.json is absent — a
# Go-only checkout, or a container stage that never copies web/, still builds.
# The SPA landed in M0B-008, so in a normal checkout this is always set.
#
# Node is pinned in .prototools and web/.nvmrc; web/package.json states the same
# range in `engines`, so npm fails loudly rather than subtly on an older one.
HAS_WEB := $(wildcard $(WEB_DIR)/package.json)

# The same guard for the Playwright suite, which is a third npm project. It is
# skipped by the container build for the same reason: nothing in the image runs
# it.
HAS_E2E := $(wildcard $(E2E_DIR)/package.json)

# The frontend is embedded in the binary behind the `spa` build tag (web/dist.go
# says why). Only the build that has just run `npm run build` sets it: web/dist
# is gitignored build output, and //go:embed will not compile without it — so
# `go build ./...` in a fresh checkout, and every Go-only stage, must not.
ifneq ($(HAS_WEB),)
GO_TAGS := spa
endif

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.PHONY: tools
tools: $(BIN_DIR)/oapi-codegen $(BIN_DIR)/golangci-lint ## Install pinned tooling into ./bin
ifneq ($(HAS_WEB),)
	npm --prefix $(WEB_DIR) ci
endif
ifneq ($(HAS_E2E),)
	npm --prefix $(E2E_DIR) ci
endif
	@echo 'Browsers for the end-to-end suite are a separate download: make e2e-browsers'

# No @version suffix: the version comes from go.mod, which tools/tools.go pins.
$(BIN_DIR)/oapi-codegen: go.mod go.sum tools/tools.go
	GOBIN=$(BIN_DIR) go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen

$(BIN_DIR)/golangci-lint:
	GOBIN=$(BIN_DIR) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

# Depends on the generator binaries rather than on `tools`, so a present and
# up-to-date ./bin is used offline and only a missing or stale one is fetched.
.PHONY: generate
generate: $(BIN_DIR)/oapi-codegen ## Run every code generator (idempotent, offline after `make tools`)
	go generate $(GO_PACKAGES)
ifneq ($(HAS_WEB),)
	npm --prefix $(WEB_DIR) run generate
endif

.PHONY: lint
lint: $(BIN_DIR)/golangci-lint lint-spec ## Lint Go sources, the API spec, and both TypeScript trees
	$(BIN_DIR)/golangci-lint run
ifneq ($(HAS_WEB),)
	npm --prefix $(WEB_DIR) run lint
endif
ifneq ($(HAS_E2E),)
	npm --prefix $(E2E_DIR) run lint
endif

# The API spec's linter is a Go test rather than a second toolchain: the rules
# need the parsed document, which `go test ./api` already has. It runs here as
# well as in `test` because a spec that breaks its own conventions is a lint
# failure — see docs/api.md.
.PHONY: lint-spec
lint-spec: ## Check api/openapi.yaml is valid and follows the API conventions
	go test -count=1 ./api

.PHONY: test
test: ## Run Go and web tests
	go test $(GO_PACKAGES)
ifneq ($(HAS_WEB),)
	npm --prefix $(WEB_DIR) test
endif

# What CI runs for the Go half (.github/workflows/ci.yml). Separate from `test`
# because -race roughly triples the wall clock, which is the wrong default for
# the loop you run every few minutes — but the right one before merging. The
# profile is a build artifact, so *.out is gitignored.
COVERPROFILE ?= coverage.out

.PHONY: test-race
test-race: ## Run the Go tests with the race detector and a coverage profile
	go test -race -covermode=atomic -coverprofile=$(COVERPROFILE) $(GO_PACKAGES)

.PHONY: build
build: ## Build the SPA and the server and CLI binaries into ./bin
ifneq ($(HAS_WEB),)
	npm --prefix $(WEB_DIR) run build
endif
	CGO_ENABLED=1 go build -tags "$(GO_TAGS)" -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/purpleops ./cmd/purpleops
	# No tag: popsctl does not import web, so it embeds nothing either way.
	CGO_ENABLED=1 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/popsctl ./cmd/popsctl

# Separate from `test` because it only compiles once web/dist exists. CI runs it
# after `make build`; locally, so can you.
.PHONY: test-spa
test-spa: ## Check the built web/dist is what got embedded (run after `make build`)
	go test -count=1 -tags spa ./web

.PHONY: run
run: build ## Build, then run the server
	$(BIN_DIR)/purpleops

# --- End to end --------------------------------------------------------------
#
# Playwright drives a real ./bin/purpleops against a real DuckDB file, so `e2e`
# builds first: a suite run against yesterday's binary is worse than no suite,
# because it is green about the wrong thing. docs/testing.md is the rest of this
# story — headed runs, the trace viewer, and how to keep a failed run's database.
#
# The harness starts and stops its own servers. Set BASE_URL to point it at one
# that is already running instead; if nothing answers there, the run fails
# rather than skipping (M0B-013 exists because v1's skipped).

# `npm run`, not `npm exec`: only the former runs the command with e2e/ as the
# working directory. `npm exec` stays in the caller's, where Playwright finds no
# playwright.config.ts and helpfully collects every *.test.ts in web/ instead.
.PHONY: e2e
e2e: build ## Build, then run the Playwright suite (E2E_ARGS passes flags through)
	npm --prefix $(E2E_DIR) test -- $(E2E_ARGS)

.PHONY: e2e-browsers
e2e-browsers: ## Download the browser Playwright drives (once, and after an upgrade)
	npm --prefix $(E2E_DIR) run browsers

.PHONY: e2e-report
e2e-report: ## Open the report from the last e2e run
	npm --prefix $(E2E_DIR) run report

# --- Container ---------------------------------------------------------------
#
# The image is the supported deployment artifact (PLAN.md §8), so it is built
# from here rather than only by CI. It needs no other target to have run: the
# build context is the repository, and both the SPA and the binary are compiled
# inside it. docs/deploy.md is the operator-facing half of this.

IMAGE     ?= purpleops
IMAGE_TAG ?= local

.PHONY: docker-build
docker-build: ## Build the container image (IMAGE, IMAGE_TAG, PLATFORM override)
	docker build \
		--file deploy/Dockerfile \
		--tag $(IMAGE):$(IMAGE_TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_DATE=$(BUILD_DATE) \
		$(if $(PLATFORM),--platform $(PLATFORM) --load,) \
		.

.PHONY: docker-smoke
docker-smoke: ## Build the image, run it, and check it answers (CI runs this too)
	IMAGE=$(IMAGE):$(IMAGE_TAG) deploy/smoke.sh

.PHONY: clean
clean: ## Remove build output
	rm -rf $(BIN_DIR) $(WEB_DIR)/dist $(E2E_DIR)/test-results $(E2E_DIR)/playwright-report

# Consumed by internal/version's ldflags test, which re-stamps these flags with
# known values to prove they still reach the version package.
.PHONY: print-ldflags
print-ldflags:
	@echo '$(LDFLAGS)'

# Consumed by the CI lint job, which installs golangci-lint from a prebuilt
# release rather than with `go install`. This keeps the version pinned in one
# place instead of two that drift apart quietly.
.PHONY: print-golangci-version
print-golangci-version:
	@echo '$(GOLANGCI_LINT_VERSION)'
