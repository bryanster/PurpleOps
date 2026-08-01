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

# The Go half of the repo, listed rather than `./...`. web/node_modules is
# inside the module directory and npm packages sometimes ship Go sources of
# their own — `./...` compiles and vets those, so a dependency nobody here
# controls could fail the build. golangci-lint is told the same thing in
# .golangci.yml.
GO_PACKAGES := ./api/... ./cmd/... ./internal/... ./tools/...

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

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.PHONY: tools
tools: $(BIN_DIR)/oapi-codegen $(BIN_DIR)/golangci-lint ## Install pinned tooling into ./bin
ifneq ($(HAS_WEB),)
	npm --prefix $(WEB_DIR) ci
endif

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
lint: $(BIN_DIR)/golangci-lint lint-spec ## Lint Go sources, the API spec, and web sources
	$(BIN_DIR)/golangci-lint run
ifneq ($(HAS_WEB),)
	npm --prefix $(WEB_DIR) run lint
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

.PHONY: build
build: ## Build the SPA and the server and CLI binaries into ./bin
ifneq ($(HAS_WEB),)
	npm --prefix $(WEB_DIR) run build
endif
	CGO_ENABLED=1 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/purpleops ./cmd/purpleops
	CGO_ENABLED=1 go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/popsctl ./cmd/popsctl

.PHONY: run
run: build ## Build, then run the server
	$(BIN_DIR)/purpleops

.PHONY: clean
clean: ## Remove build output
	rm -rf $(BIN_DIR) $(WEB_DIR)/dist

# Consumed by internal/version's ldflags test, which re-stamps these flags with
# known values to prove they still reach the version package.
.PHONY: print-ldflags
print-ldflags:
	@echo '$(LDFLAGS)'
