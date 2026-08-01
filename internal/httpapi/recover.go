package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/bryanster/purpleops/internal/httpapi/apierr"
)

// recoverer turns a panicking handler into a 500 problem document and a logged
// stack trace.
//
// The stack goes to the log and nothing else: a Go stack names types, file
// paths and package versions, and v1's habit of returning internals to the
// browser is exactly what M0B-007 exists to end. The client gets the request
// ID, and an operator greps for it.
func recoverer(log *slog.Logger, responder *apierr.Responder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Captured before the handler runs so that the deferred closure logs
			// against the context the request arrived with, whatever the handler
			// did with its own copy.
			ctx := r.Context()

			defer func() {
				recovered := recover()
				if recovered == nil {
					return
				}
				// net/http documents this value as "the handler deliberately
				// gave up on this connection, quietly". Swallowing it would
				// turn an abandoned connection into a 500 nobody can act on.
				if recovered == http.ErrAbortHandler { //nolint:errorlint // the sentinel is compared by identity, as net/http documents.
					panic(recovered)
				}

				log.ErrorContext(ctx, "panic serving request",
					// Formatted, not slog.Any: a panic value can be anything at
					// all, including something a JSON handler refuses to
					// marshal, and this line must not itself fail.
					slog.String("panic", fmt.Sprint(recovered)),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("request_id", middleware.GetReqID(ctx)),
					slog.String("stack", string(debug.Stack())),
				)

				// A handler that panicked after writing has already sent a
				// status line, and a second one would be a mangled response and
				// a "superfluous WriteHeader" in the log. The panic is recorded
				// above either way.
				if wrapped, ok := w.(middleware.WrapResponseWriter); ok && wrapped.Status() != 0 {
					return
				}
				responder.Write(w, r, apierr.Internal(fmt.Errorf("panic: %v", recovered)))
			}()

			next.ServeHTTP(w, r)
		})
	}
}
