package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAGeneratedRequestIDIsAUUIDv7AndReachesEverything(t *testing.T) {
	t.Parallel()

	server, logs := newTestServer(t)
	recorder := get(server, BasePath+"/healthz")

	id := recorder.Header().Get(RequestIDHeader)
	if id == "" {
		t.Fatal("no " + RequestIDHeader + " on the response; the client has nothing to quote in a bug report")
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		t.Fatalf("the generated request ID %q is not a UUID: %v", id, err)
	}
	if got, want := parsed.Version(), uuid.Version(7); got != want {
		t.Errorf("the generated request ID is UUIDv%d, want v%d — identifiers here sort by creation time", got, want)
	}

	// The other half of the acceptance criterion: the same value is on the log
	// line, or the ID the client was given leads nowhere.
	if got := logs.find(t, "request")["request_id"]; got != id {
		t.Errorf("the request was logged under request_id %v, but the client was given %q", got, id)
	}
}

func TestAClientsRequestIDIsEchoed(t *testing.T) {
	t.Parallel()

	server, logs := newTestServer(t)

	const id = "trace-0123456789abcdef"
	request := httptest.NewRequest(http.MethodGet, BasePath+"/healthz", nil)
	request.Header.Set(RequestIDHeader, id)
	recorder := do(server, request)

	if got := recorder.Header().Get(RequestIDHeader); got != id {
		t.Errorf("%s = %q, want the client's %q — a caller correlating its own traces gets nothing back",
			RequestIDHeader, got, id)
	}
	if got := logs.find(t, "request")["request_id"]; got != id {
		t.Errorf("the request was logged under request_id %v, want the client's %q", got, id)
	}
}

// TestAnUnusableRequestIDIsReplaced covers what an echoed header can be used
// for: a value with a newline splits the log line it is written into, and an
// unbounded one writes as much of the log as the sender likes. Both are
// answered by generating an ID instead — the request itself is fine.
func TestAnUnusableRequestIDIsReplaced(t *testing.T) {
	t.Parallel()

	server, _ := newTestServer(t)

	tests := map[string]string{
		"a newline":     "abc\ndef",
		"a space":       "abc def",
		"a quote":       `abc"def`,
		"markup":        "<script>alert(1)</script>",
		"a header ":     "abc\r\nX-Evil: 1",
		"too long":      strings.Repeat("a", maxRequestIDLength+1),
		"a null byte":   "abc\x00def",
		"a comma chain": "abc,def",
	}

	for name, id := range tests {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, BasePath+"/healthz", nil)
			// Set the header without net/http's own validation, which rejects
			// some of these before they could reach the middleware.
			request.Header[RequestIDHeader] = []string{id}
			recorder := do(server, request)

			got := recorder.Header().Get(RequestIDHeader)
			if got == id {
				t.Errorf("%s = %q, want a generated ID: this one was repeated verbatim", RequestIDHeader, got)
			}
			if _, err := uuid.Parse(got); err != nil {
				t.Errorf("%s = %q, want a generated UUID: %v", RequestIDHeader, got, err)
			}
		})
	}
}

func TestAcceptableRequestIDKeepsWhatIsSafe(t *testing.T) {
	t.Parallel()

	for _, id := range []string{
		"018f3b2c-7a41-7c3e-9b0d-2f1a4c6e8d90",
		"trace_1.2:3",
		strings.Repeat("a", maxRequestIDLength),
	} {
		if got := acceptableRequestID(id); got != id {
			t.Errorf("acceptableRequestID(%q) = %q, want it kept", id, got)
		}
	}
}
