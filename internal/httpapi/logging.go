package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

// requestLogger writes one line per request, after it has been answered.
//
// It sits *outside* the recoverer rather than inside it, which is the one place
// this chain departs from the order M0B-006 proposed. The reason is what the
// line says: the logger records the status when the handler beneath it returns,
// so with the recoverer inside, a panicking request is logged as the 500 the
// client actually received. With the recoverer outside, the line is written
// while the panic is still unwinding — before any status exists — and every
// panic appears in the access log as a success. Panics are still logged with
// their stack either way; that is the recoverer's own line, not this one.
// TestALoggedPanicReportsTheStatusTheClientSaw pins it.
func requestLogger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrapped := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			started := time.Now()
			// Captured here rather than read inside the deferred closure: it is
			// the same context either way, and taking it now says so.
			ctx := r.Context()

			// Deferred so that the line is written even when the handler panics
			// or the connection dies mid-response.
			defer func() {
				log.LogAttrs(ctx, slog.LevelInfo, "request",
					slog.String("method", r.Method),
					// Path, not RequestURI: a query string can carry a token, a
					// share secret or a filter nobody meant to publish, and the
					// log is the one place they would then live forever.
					slog.String("path", apierr.RedactPath(r.URL.Path)),
					slog.Int("status", statusOf(wrapped)),
					slog.Int("bytes", wrapped.BytesWritten()),
					slog.Duration("duration", time.Since(started)),
					slog.String("request_id", middleware.GetReqID(ctx)),
					slog.String("client_ip", ClientIP(ctx)),
					// M1 adds the authenticated user here, from the context the
					// authentication middleware fills in.
				)
			}()

			next.ServeHTTP(wrapped, r)
		})
	}
}

// statusOf reports the status the client saw. A handler that returns without
// calling WriteHeader has left net/http to send a 200, so that is what the log
// says rather than the zero the wrapper still holds.
func statusOf(w middleware.WrapResponseWriter) int {
	if status := w.Status(); status != 0 {
		return status
	}
	return http.StatusOK
}
