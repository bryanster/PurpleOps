package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bryanster/blacklight/internal/config"
)

// TestSecurityHeadersAreOnEveryResponse walks the kinds of response this server
// produces — a handler's, a validator's rejection, a router's 404, a panic's
// 500 — because a header set on the success path only is missing from every
// response an attacker is interested in.
func TestSecurityHeadersAreOnEveryResponse(t *testing.T) {
	t.Parallel()

	healthy, _ := newTestServer(t)
	panicky, _ := newTestServerWith(t, testConfig(t), panickyStore{})

	responses := map[string]*httptest.ResponseRecorder{
		"a handler's response":    get(healthy, BasePath+"/healthz"),
		"a generated 404":         get(healthy, BasePath+"/nope"),
		"a route outside the API": get(healthy, "/"),
		"a wrong method":          do(healthy, httptest.NewRequest(http.MethodPost, BasePath+"/healthz", nil)),
		"a recovered panic":       get(panicky, BasePath+"/healthz"),
		"the version endpoint":    get(healthy, BasePath+"/version"),
	}

	want := map[string]string{
		"X-Content-Type-Options":  "nosniff",
		"Referrer-Policy":         "no-referrer",
		"X-Frame-Options":         "DENY",
		"Content-Security-Policy": contentSecurityPolicy,
	}

	for name, recorder := range responses {
		for header, value := range want {
			if got := recorder.Header().Get(header); got != value {
				t.Errorf("%s: %s = %q, want %q", name, header, got, value)
			}
		}
	}
}

// TestHSTSFollowsTheBaseURL keeps the header off an http deployment. Sending it
// from one either does nothing or, if the host is later reached over https and
// then reverts, locks every browser that saw it out of a server that cannot
// serve TLS.
func TestHSTSFollowsTheBaseURL(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"https://blacklight.example.com": hstsValue,
		"http://localhost:8080":          "",
	}

	for baseURL, want := range tests {
		t.Run(baseURL, func(t *testing.T) {
			var parsed config.URL
			if err := parsed.UnmarshalText([]byte(baseURL)); err != nil {
				t.Fatalf("parsing %q: %v", baseURL, err)
			}
			cfg := testConfig(t)
			cfg.Server.BaseURL = parsed

			server, _ := newTestServerWith(t, cfg, stubStore{})
			recorder := get(server, BasePath+"/healthz")

			if got := recorder.Header().Get("Strict-Transport-Security"); got != want {
				t.Errorf("Strict-Transport-Security = %q, want %q", got, want)
			}
		})
	}
}
