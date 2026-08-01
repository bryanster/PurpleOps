package httpapi

import (
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/bryanster/purpleops/internal/httpapi/apierr"
)

// requestValidator rejects anything api/openapi.yaml does not describe, before
// a handler is entered. The specification is therefore enforced rather than
// documented: a handler can assume its input matched the schema the client was
// given, and an endpoint cannot quietly start accepting a field the spec does
// not have.
//
// The document is the one embedded in the binary (api.Load), never a file on
// disk — a deployed binary has no api/openapi.yaml beside it, and a validator
// that read one could be pointed at a different API than the one it serves.
//
// This is the middleware M0B-006 calls OapiRequestValidator, written out rather
// than taken from oapi-codegen/nethttp-middleware: it is fifteen lines around
// the same two kin-openapi calls, and it hands apierr the library's own error
// values rather than that package's re-wrapped strings — which is what the
// translation in M0B-007 is written and tested against.
func requestValidator(doc *openapi3.T, responder *apierr.Responder) (func(http.Handler) http.Handler, error) {
	// The gorillamux router, which is the one kin-openapi documents, and the
	// only one of the two that returns routers.ErrPathNotFound and
	// routers.ErrMethodNotAllowed themselves — the legacy router returns fresh
	// values carrying the same text, which errors.Is cannot match, so every
	// unknown path would be reported as a 500. It resolves this document's
	// "/api/v1" server prefix; the paths in the spec are relative to it.
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("httpapi: build the request router from the API specification: %w", err)
	}

	options := &openapi3filter.Options{
		// One response naming every problem with the request, rather than the
		// client fixing them one round trip at a time — the same argument
		// config.Load makes for environment variables.
		MultiError: true,

		// AuthenticationFunc is deliberately absent. Both operations in the
		// document are `security: []`, and the validator only asks for an
		// authenticator when an operation carries a security requirement — so
		// M1's first authenticated endpoint has to arrive together with the
		// middleware that authenticates it (docs/api.md). Stubbing one in now
		// would make that impossible to notice.
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Routing failures are the 404 and the 405: the path is not in the
			// specification, or it is but not with this method. apierr knows
			// both sentinels, so they arrive as problem documents like
			// everything else.
			route, pathParams, err := router.FindRoute(r)
			if err != nil {
				responder.Write(w, r, err)
				return
			}

			input := &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: pathParams,
				Route:      route,
				Options:    options,
			}
			// ValidateRequest reads the body and puts it back, so the handler
			// still gets one it can read.
			if err := openapi3filter.ValidateRequest(r.Context(), input); err != nil {
				responder.Write(w, r, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}, nil
}
