package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bryanster/blacklight/internal/config"
)

// TestClientIPTrustsOnlyConfiguredProxies is the regression case for the
// header this middleware exists to distrust. Until an operator names a proxy,
// X-Forwarded-For is a value the caller chose — and believing it would let
// anyone change which address login throttling counts (M1-004) and which one
// the activity log records (M1-015).
func TestClientIPTrustsOnlyConfiguredProxies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		trusted    string
		remoteAddr string
		forwarded  []string
		realIP     string
		want       string
	}{
		{
			name:       "no proxy configured, header ignored",
			remoteAddr: "203.0.113.9:54321",
			forwarded:  []string{"10.9.9.9"},
			want:       "203.0.113.9",
		},
		{
			name:       "no header at all",
			remoteAddr: "203.0.113.9:54321",
			want:       "203.0.113.9",
		},
		{
			name:       "trusted proxy, one hop",
			trusted:    "10.0.0.0/8",
			remoteAddr: "10.0.0.5:41234",
			forwarded:  []string{"203.0.113.9"},
			want:       "203.0.113.9",
		},
		{
			name:       "untrusted peer claiming to be a proxy",
			trusted:    "10.0.0.0/8",
			remoteAddr: "198.51.100.7:41234",
			forwarded:  []string{"203.0.113.9"},
			want:       "198.51.100.7",
		},
		{
			// Two proxies of ours, and a value the client put there first. The
			// answer is the last address our own infrastructure vouched for.
			name:       "a client-supplied hop is not believed",
			trusted:    "10.0.0.0/8",
			remoteAddr: "10.0.0.5:41234",
			forwarded:  []string{"1.2.3.4, 203.0.113.9, 10.0.0.6"},
			want:       "203.0.113.9",
		},
		{
			name:       "every hop is ours",
			trusted:    "10.0.0.0/8",
			remoteAddr: "10.0.0.5:41234",
			forwarded:  []string{"10.0.0.7, 10.0.0.6"},
			want:       "10.0.0.5",
		},
		{
			name:       "separate headers keep their order",
			trusted:    "10.0.0.0/8",
			remoteAddr: "10.0.0.5:41234",
			forwarded:  []string{"203.0.113.9", "10.0.0.6"},
			want:       "203.0.113.9",
		},
		{
			name:       "X-Real-IP when there is no chain",
			trusted:    "10.0.0.0/8",
			remoteAddr: "10.0.0.5:41234",
			realIP:     "203.0.113.9",
			want:       "203.0.113.9",
		},
		{
			name:       "X-Real-IP is ignored from an untrusted peer",
			trusted:    "10.0.0.0/8",
			remoteAddr: "198.51.100.7:41234",
			realIP:     "203.0.113.9",
			want:       "198.51.100.7",
		},
		{
			name:       "an unparseable hop is skipped",
			trusted:    "10.0.0.0/8",
			remoteAddr: "10.0.0.5:41234",
			forwarded:  []string{"203.0.113.9, unknown"},
			want:       "203.0.113.9",
		},
		{
			name:       "a hop with a port",
			trusted:    "10.0.0.0/8",
			remoteAddr: "10.0.0.5:41234",
			forwarded:  []string{"[2001:db8::1]:9999"},
			want:       "2001:db8::1",
		},
		{
			name:       "an IPv4 peer on a dual-stack listener",
			trusted:    "10.0.0.0/8",
			remoteAddr: "[::ffff:10.0.0.5]:41234",
			forwarded:  []string{"203.0.113.9"},
			want:       "203.0.113.9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var trusted config.CIDRs
			if tt.trusted != "" {
				if err := trusted.UnmarshalText([]byte(tt.trusted)); err != nil {
					t.Fatalf("parsing the trusted proxies: %v", err)
				}
			}

			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = tt.remoteAddr
			for _, value := range tt.forwarded {
				request.Header.Add(forwardedForHeader, value)
			}
			if tt.realIP != "" {
				request.Header.Set(realIPHeader, tt.realIP)
			}

			var got string
			handler := realIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = ClientIP(r.Context())
			}))
			handler.ServeHTTP(httptest.NewRecorder(), request)

			if got != tt.want {
				t.Errorf("ClientIP = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRemoteAddrIsLeftAlone pins the deliberate difference from chi's RealIP:
// the transport's own answer stays readable, so nothing downstream is quietly
// told something the connection did not say.
func TestRemoteAddrIsLeftAlone(t *testing.T) {
	t.Parallel()

	var trusted config.CIDRs
	if err := trusted.UnmarshalText([]byte("10.0.0.0/8")); err != nil {
		t.Fatalf("parsing the trusted proxies: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.5:41234"
	request.Header.Set(forwardedForHeader, "203.0.113.9")

	var remoteAddr string
	handler := realIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		remoteAddr = r.RemoteAddr
	}))
	handler.ServeHTTP(httptest.NewRecorder(), request)

	if want := "10.0.0.5:41234"; remoteAddr != want {
		t.Errorf("RemoteAddr = %q, want %q", remoteAddr, want)
	}
}

// TestTheClientIPIsLogged closes the loop: resolving the address is only useful
// if it reaches the line an operator reads.
func TestTheClientIPIsLogged(t *testing.T) {
	t.Parallel()

	server, logs := newTestServer(t)

	request := httptest.NewRequest(http.MethodGet, BasePath+"/healthz", nil)
	request.RemoteAddr = "203.0.113.9:54321"
	do(server, request)

	if got, want := logs.find(t, "request")["client_ip"], "203.0.113.9"; got != want {
		t.Errorf("client_ip = %v, want %q", got, want)
	}
}
