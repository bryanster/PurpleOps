package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// RequestIDHeader is both read and written: a client that supplies one gets it
// back, and a client that does not is given one. It is what ties a response, a
// problem document's `instance` and every log line for that request together.
const RequestIDHeader = "X-Request-Id"

// maxRequestIDLength bounds an inbound identifier. The value is echoed into
// every log line and every problem document, so an unbounded one is a way to
// write a megabyte into the log by sending a header.
const maxRequestIDLength = 64

// requestID puts a request identifier in the context and on the response.
//
// It stores it under chi's middleware.RequestIDKey so that this middleware,
// apierr.Responder and any chi middleware read one value (M0B-007). chi's own
// RequestID is not used: it neither echoes the header back nor generates the
// UUIDv7 this project uses for identifiers, and it trusts an inbound value
// whatever it contains.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := acceptableRequestID(r.Header.Get(RequestIDHeader))
		if id == "" {
			id = newRequestID()
		}

		// Set before the handler runs: a handler that writes a response, and a
		// panic that produces one, both need the header already on the writer.
		w.Header().Set(RequestIDHeader, id)

		ctx := context.WithValue(r.Context(), middleware.RequestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// acceptableRequestID returns the client's identifier if it is one this server
// is willing to repeat, and "" otherwise.
//
// A caller correlating its own traces has a good reason to choose the value, so
// it is accepted — but it is echoed to other clients in a response header and
// written to the log, so it is restricted to characters that cannot break
// either. Rejection is silent: the caller gets a generated ID, which is a
// better answer than a 400 for a request that is otherwise fine.
func acceptableRequestID(id string) string {
	if id == "" || len(id) > maxRequestIDLength {
		return ""
	}
	for _, c := range []byte(id) {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '-', c == '_', c == '.', c == ':':
		default:
			return ""
		}
	}
	return id
}

// newRequestID mints an identifier. UUIDv7, like every other identifier in this
// system (docs/tickets/README.md), so a request ID sorts by time and is
// recognisable as ours in someone else's log aggregator.
func newRequestID() string {
	if id, err := uuid.NewV7(); err == nil {
		return id.String()
	}
	// crypto/rand failed, which no request can do anything about and which a
	// 500 would not explain. A request with no identifier at all loses the
	// thread between the client's response and the server's log line, so this
	// falls back to the clock: unique enough to correlate one request, and
	// visibly not a UUID.
	return "t" + strconv.FormatInt(time.Now().UnixNano(), 36)
}
