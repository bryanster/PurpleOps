// Package api embeds the OpenAPI document that defines this service's HTTP API
// and is the only place that parses it.
//
// openapi.yaml is the source of truth: the Go server interface in
// internal/httpapi/gen, the TypeScript client in web/, and the four published
// SDKs in sdk/ are all generated from it, and the request-validation middleware
// (M0B-006) enforces it at runtime against the copy embedded here — never
// against a file on disk, which a deployed binary has no reason to have.
//
// The two //go:generate lines below are the server and the Go SDK: the same
// generator at the same version, so both sides of the wire agree about what a
// nullable field or a prefixed enum constant is. docs/sdk.md covers the other
// three.
package api

import (
	_ "embed"
	"fmt"

	"github.com/getkin/kin-openapi/openapi3"
)

//go:generate oapi-codegen --config=codegen-server.yaml openapi.yaml
//go:generate oapi-codegen --config=codegen-sdk-go.yaml openapi.yaml

//go:embed openapi.yaml
var specYAML []byte

// Spec returns the raw bytes of the embedded OpenAPI document. Callers that want
// a parsed document should use [Load]; this is for serving the spec verbatim and
// for tests that need to compare it against the file on disk.
func Spec() []byte {
	// Copied: the caller must not be able to mutate the embedded document that
	// every later Load reads.
	return append([]byte(nil), specYAML...)
}

// Load parses and validates the embedded OpenAPI document.
//
// It parses on every call rather than caching, because the returned document is
// a mutable tree that the kin-openapi validator writes into; a shared copy would
// make two callers' state each other's problem. Callers load once at startup.
func Load() (*openapi3.T, error) {
	return load(specYAML)
}

// load is the body of [Load], split out so tests can feed it a deliberately
// broken document and prove that validation is real rather than assumed.
func load(document []byte) (*openapi3.T, error) {
	loader := &openapi3.Loader{
		// Everything lives in one file. Allowing external references would let a
		// $ref reach the filesystem or the network of whatever machine happens to
		// be parsing the spec.
		IsExternalRefsAllowed: false,
	}

	doc, err := loader.LoadFromData(document)
	if err != nil {
		return nil, fmt.Errorf("parse openapi document: %w", err)
	}

	// Format validation is on deliberately. Without it a `format` that no
	// generator understands — a typo, or one someone invented — passes silently
	// and turns into a plain string on both sides of the wire. With it, using a
	// format outside the OpenAPI-defined set is an error here, and adding one
	// means registering it (openapi3.DefineStringFormat) rather than hoping.
	if err := doc.Validate(loader.Context, openapi3.EnableSchemaFormatValidation()); err != nil {
		return nil, fmt.Errorf("validate openapi document: %w", err)
	}

	return doc, nil
}
