//go:build tools

// Package tools pins the versions of the code generators this repository uses.
//
// The imports below are never compiled into a binary — the `tools` build tag
// excludes this file from every normal build. Their only job is to keep the
// generators in `go.mod`, so `make tools` can install them with `go install`
// (no `@version` suffix) and every developer and CI runner produces
// byte-identical generated code. Without this, the codegen-drift gate in
// M0B-012 fails depending on whatever version happens to be in someone's PATH.
//
// To add a generator: import its `cmd/...` package here, run `go mod tidy`,
// and add an install rule to the Makefile.
package tools

import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)
