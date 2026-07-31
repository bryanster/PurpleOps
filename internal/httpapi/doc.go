// Package httpapi is the HTTP transport: the server generated from
// api/openapi.yaml, the handlers implementing its interface, and the middleware
// chain (request ID, logging, recovery, request validation, authentication,
// authorization).
//
// Handlers translate between the generated request and response types and the
// domain packages. They contain no authorization checks of their own — that is
// the middleware's job, exactly once (M1-013).
//
// Implemented by M0B-005 (generated server) and M0B-006 (routing, middleware).
package httpapi
