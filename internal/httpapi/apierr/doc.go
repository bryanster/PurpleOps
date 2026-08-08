// Package apierr is this API's error vocabulary and the single point where a Go
// error becomes an HTTP response.
//
// The domain layer returns an [Error] built by one of the constructors —
// apierr.NotFound("engagement", id), apierr.Conflict(...) — and wraps it as
// usual on the way up:
//
//	if err := repo.Load(ctx, id); err != nil {
//	    return fmt.Errorf("load engagement: %w", err)
//	}
//
// The transport calls [Responder.Write], which translates whatever it is given
// into the one problem shape described by api/openapi.yaml, serves it as
// application/problem+json, and logs. Translation uses errors.As, so wrapping
// does not change the answer.
//
// # The rule
//
// An error this package does not recognise becomes 500 with the code "internal"
// and a generic detail. The real error goes to the log with the request ID
// echoed in the problem's `instance`, and never to the client. v1 returned raw
// driver errors to the browser; the shape of this package is what makes that
// impossible rather than merely discouraged.
//
// The consequence for a handler author: an error that should tell the caller
// something has to say so through a constructor here. There is no path by which
// an unclassified error reaches a user, so "it will surface in the response"
// is never true.
//
// # Codes
//
// Every code in the ProblemCode enum in api/openapi.yaml maps to exactly one
// HTTP status, and the table in codes.go is the mapping. Adding a code means
// editing the spec and that table in the same change; TestEveryCodeInTheSpecHasAStatus
// fails otherwise.
package apierr
