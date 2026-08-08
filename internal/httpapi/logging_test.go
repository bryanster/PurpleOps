package httpapi

import (
	"net/http"
	"strings"
	"testing"
)

// TestEveryRequestIsLoggedOnce asserts the fields an operator actually greps
// for. A line missing any of them is a line that cannot answer "what did this
// user do, when, and what did we say".
func TestEveryRequestIsLoggedOnce(t *testing.T) {
	t.Parallel()

	server, logs := newTestServer(t)
	recorder := get(server, BasePath+"/version")

	line := logs.find(t, "request")
	for field, want := range map[string]any{
		"method": http.MethodGet,
		"path":   BasePath + "/version",
		"status": float64(http.StatusOK), // JSON numbers decode as float64.
	} {
		if got := line[field]; got != want {
			t.Errorf("%s = %v, want %v", field, got, want)
		}
	}
	if got := line["request_id"]; got != recorder.Header().Get(RequestIDHeader) {
		t.Errorf("request_id = %v, want the ID the client was given", got)
	}
	if bytes, ok := line["bytes"].(float64); !ok || bytes <= 0 {
		t.Errorf("bytes = %v, want the size of a response that had a body", line["bytes"])
	}
	if _, ok := line["duration"]; !ok {
		t.Error("the line has no duration; a slow endpoint is invisible")
	}
}

// TestALoggedPanicReportsTheStatusTheClientSaw is why the recoverer sits inside
// the logger rather than outside it, which is where M0B-006 suggested putting
// it. With the recoverer outside, this line is written while the panic is still
// unwinding — before any status exists — and every panic in the system appears
// in the access log as a 200.
func TestALoggedPanicReportsTheStatusTheClientSaw(t *testing.T) {
	t.Parallel()

	server, logs := newTestServerWith(t, testConfig(t), panickyStore{})
	recorder := get(server, BasePath+"/healthz")

	if got, want := recorder.Code, http.StatusInternalServerError; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := logs.find(t, "request")["status"], float64(http.StatusInternalServerError); got != want {
		t.Errorf("the request was logged as status %v, but the client received %v", got, want)
	}
}

// TestTheLogDoesNotCarryTheQueryString keeps secrets that travel in a URL — a
// report share token (M6), a password-reset link — out of the one file that is
// shipped to a log aggregator and kept for a year.
func TestTheLogDoesNotCarryTheQueryString(t *testing.T) {
	t.Parallel()

	server, logs := newTestServer(t)
	get(server, BasePath+"/version?token=sup3rs3cret")

	if got := logs.find(t, "request")["path"]; got != BasePath+"/version" {
		t.Errorf("path = %v, want the path without its query string", got)
	}
	if body := logs.String(); strings.Contains(body, "sup3rs3cret") {
		t.Errorf("the query string reached the log:\n%s", body)
	}
}
