package httpapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
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

		// The validator refuses to serve an operation carrying a security
		// requirement unless it is given one of these, so it exists; it
		// deliberately allows everything.
		//
		// This is not a stub standing in for a check that should be here. The
		// document's `security` says which credential an operation is *for*, and
		// the validator sees a request before the authentication middleware has
		// resolved the cookie — so the only answer it could give is "there is a
		// cookie header", which is not authentication. Rejecting here would also
		// mean the 401 came from the validator rather than from the one place
		// that decides (M1-013), with a body that is not this API's.
		//
		// Authentication happens in the middleware immediately after this one;
		// authorization happens after that.
		AuthenticationFunc: func(context.Context, *openapi3filter.AuthenticationInput) error {
			return nil
		},
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
