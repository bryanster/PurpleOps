package httpapi

import (
	"context"
	"net/http"
	"time"
)

// timeout puts BLACKLIGHT_REQUEST_TIMEOUT on every request's context.
//
// A deadline rather than an http.TimeoutHandler, for two reasons. The response
// TimeoutHandler writes on expiry is a plain-text 503 that no `code` in the
// spec describes, so it would be the one error in the application that is not a
// problem document; and it does not stop the handler it gave up on, which keeps
// running with the database transaction it holds. A deadline reaches the
// database driver instead — every query this application makes takes a context
// — so the work is actually abandoned and the failure travels back through the
// one error path as a 500.
//
// A non-positive duration disables it. Nothing configures that (the variable is
// validated as positive) but a zero-valued config.Server in a test should not
// mean "every request expires immediately".
//
// GET /api/v1/events is the one endpoint this cannot apply to: the stream is
// meant to outlive the ordinary request budget. It opts out by path here.
func timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if d <= 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isSSEPath(r) {
				next.ServeHTTP(w, r)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), d)
			// Releases the timer as soon as the handler returns, rather than at
			// the deadline: without it, a fast request holds a timer for the
			// whole timeout, and a busy server holds thousands.
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
