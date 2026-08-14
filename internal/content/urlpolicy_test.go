package content

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func lookupIPs(addrs ...string) func(context.Context, string) ([]net.IP, error) {
	return func(_ context.Context, _ string) ([]net.IP, error) {
		out := make([]net.IP, len(addrs))
		for i, a := range addrs {
			out[i] = net.ParseIP(a)
		}
		return out, nil
	}
}

func TestURLPolicyValidate(t *testing.T) {
	t.Parallel()

	public := lookupIPs("8.8.8.8")
	tests := []struct {
		name    string
		policy  URLPolicy
		raw     string
		wantErr bool
	}{
		{name: "public https ok", policy: URLPolicy{LookupIP: public}, raw: "https://example.com/catalog.json"},
		{name: "http in production rejected", policy: URLPolicy{LookupIP: public}, raw: "http://example.com/catalog.json", wantErr: true},
		{name: "http in development ok", policy: URLPolicy{AllowHTTP: true, LookupIP: public}, raw: "http://example.com/catalog.json"},
		{name: "loopback rejected", policy: URLPolicy{LookupIP: public}, raw: "http://127.0.0.1/", wantErr: true},
		{name: "loopback https rejected", policy: URLPolicy{LookupIP: public}, raw: "https://127.0.0.1/", wantErr: true},
		{name: "ipv6 loopback rejected", policy: URLPolicy{LookupIP: public}, raw: "http://[::1]/", wantErr: true},
		{name: "rfc1918 rejected", policy: URLPolicy{LookupIP: public}, raw: "http://10.0.0.1/", wantErr: true},
		{name: "link-local rejected", policy: URLPolicy{LookupIP: public}, raw: "http://169.254.169.254/latest/meta-data/", wantErr: true},
		{name: "ipv6 ula rejected", policy: URLPolicy{LookupIP: public}, raw: "http://[fd00::1]/", wantErr: true},
		{name: "file scheme rejected", policy: URLPolicy{LookupIP: public}, raw: "file:///etc/passwd", wantErr: true},
		{name: "gopher scheme rejected", policy: URLPolicy{LookupIP: public}, raw: "gopher://example.com/", wantErr: true},
		{name: "no scheme rejected", policy: URLPolicy{LookupIP: public}, raw: "example.com/x", wantErr: true},
		{name: "empty rejected", policy: URLPolicy{LookupIP: public}, raw: "", wantErr: true},
		{name: "hostname resolving private rejected", policy: URLPolicy{LookupIP: lookupIPs("10.0.0.5")}, raw: "https://internal.example/", wantErr: true},
		{name: "hostname resolving public and private rejected", policy: URLPolicy{LookupIP: lookupIPs("8.8.8.8", "10.0.0.5")}, raw: "https://rebind.example/", wantErr: true},
		{name: "metadata hostname rejected", policy: URLPolicy{LookupIP: public}, raw: "http://metadata.google.internal/", wantErr: true},
		{name: "bare metadata hostname rejected", policy: URLPolicy{LookupIP: public}, raw: "http://metadata/", wantErr: true},
		{name: "unresolvable rejected", policy: URLPolicy{LookupIP: func(context.Context, string) ([]net.IP, error) {
			return nil, errors.New("no such host")
		}}, raw: "https://nope.example/", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.policy.Validate(context.Background(), tc.raw)
			if tc.wantErr && err == nil {
				t.Fatalf("Validate(%q) = nil, want error", tc.raw)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate(%q) = %v, want nil", tc.raw, err)
			}
		})
	}
}

// redirectTrip answers one redirect to target and fails loudly if a second
// request arrives — the whole point is that the redirect must not be followed.
type redirectTrip struct {
	target string
	calls  int
}

func (r *redirectTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	r.calls++
	if r.calls == 1 {
		return &http.Response{
			StatusCode: http.StatusFound,
			Header:     http.Header{"Location": []string{r.target}},
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
		}, nil
	}
	return nil, errors.New("second request: redirect must not be followed")
}

func TestURLPolicyClientRefusesBadRedirect(t *testing.T) {
	t.Parallel()

	policy := URLPolicy{LookupIP: lookupIPs("8.8.8.8")}
	for _, target := range []string{
		"http://169.254.169.254/",  // plain http + private
		"https://169.254.169.254/", // https to link-local
		"https://10.0.0.1/",        // https to RFC1918
	} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			client := policy.NewClient()
			rt := &redirectTrip{target: target}
			client.Transport = rt

			req, err := http.NewRequest(http.MethodGet, "https://example.com/", nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := client.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
			if err == nil {
				t.Fatal("client.Do = nil, want redirect-refused error")
			}
			if rt.calls != 1 {
				t.Fatalf("transport calls = %d, want 1 (redirect must not be followed)", rt.calls)
			}
		})
	}
}

// recordingDoer fails the test the moment Do is called: HTTPSource must reject
// a private URL before any request leaves.
type recordingDoer struct {
	called bool
}

func (r *recordingDoer) Do(*http.Request) (*http.Response, error) {
	r.called = true
	return nil, errors.New("must not be called")
}

func TestHTTPSourceOpenRejectsPrivateBeforeDial(t *testing.T) {
	t.Parallel()

	doer := &recordingDoer{}
	src := HTTPSource{
		URL:    "http://127.0.0.1/",
		Client: doer,
	}
	if _, _, err := src.Open(context.Background()); err == nil {
		t.Fatal("Open = nil error, want rejection")
	}
	if doer.called {
		t.Fatal("HTTP doer was called for a private URL")
	}
}
