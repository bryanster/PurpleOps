// Package httpapi is the HTTP transport: the server generated from
// api/openapi.yaml, the handlers implementing its interface, and the middleware
// chain (request ID, logging, recovery, request validation, authentication,
// authorization).
//
// Handlers translate between the generated request and response types and the
// domain packages. They contain no authorization checks of their own — that is
// the middleware's job, exactly once (M1-013).
//
// The generated half is the gen subpackage, produced from api/openapi.yaml by
// `make generate`. Adding an endpoint starts there and not here: see docs/api.md.
//
// Errors are the apierr subpackage: the vocabulary a handler returns, and the
// single point where any error becomes a response. Nothing here writes an error
// body of its own.
//
// Implemented by M0B-005 (generated server), M0B-007 (errors) and M0B-006
// (routing, middleware).
package httpapi
