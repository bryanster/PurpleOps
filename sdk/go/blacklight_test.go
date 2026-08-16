package blacklight_test

// What these cover is the seam between the hand-written wrapper and the
// generated client: the path a request actually goes to, the header it actually
// carries, and that a documented status code arrives as its own typed field
// rather than as bytes the caller has to guess about.
//
// They do not re-test the generator. Whether `GET /engagements` builds its query
// string correctly is oapi-codegen's business, and asserting it here would mean
// writing the request builder out a second time by hand.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	blacklight "github.com/bryanster/blacklight/sdk/go"
)

// healthyBody is what a deployment with every dependency up answers /healthz
// with. Small enough to write out, and typed on the way back in.
const healthyBody = `{"status":"ok","checks":{"db":"ok"}}`

// serveOnce stands up a server that answers one canned response and hands back
// the single request it saw. The request arrives over a channel rather than a
// shared variable because the handler runs on the server's goroutine.
func serveOnce(t *testing.T, status int, body string) (baseURL string, seen <-chan *http.Request) {
	t.Helper()

	requests := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- r.Clone(context.Background())
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	return server.URL, requests
}

// requestFor calls /healthz through a client built the way callers build one,
// and returns what reached the server.
func requestFor(t *testing.T, baseURL string, opts ...blacklight.ClientOption) *http.Request {
	t.Helper()

	url, seen := serveOnce(t, http.StatusOK, healthyBody)
	if baseURL == "" {
		baseURL = url
	} else {
		// The caller is testing how a base URL is joined, so it supplies the
		// shape and this supplies the host.
		baseURL = url + baseURL
	}

	client, err := blacklight.New(baseURL, opts...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := client.GetHealthWithResponse(context.Background()); err != nil {
		t.Fatalf("GetHealth: %v", err)
	}

	select {
	case r := <-seen:
		return r
	default:
		t.Fatal("the server was never called")
		return nil
	}
}

// TestNewAppendsTheAPIPath is the reason New exists: the document's one server
// is the relative URL /api/v1, so a caller who passed their origin straight to
// the generated constructor would be talking to the SPA's index.html.
func TestNewAppendsTheAPIPath(t *testing.T) {
	got := requestFor(t, "")

	if want := blacklight.APIPath + "/healthz"; got.URL.Path != want {
		t.Errorf("request path is %q, want %q", got.URL.Path, want)
	}
}

// TestNewToleratesATrailingSlash: an operator's BLACKLIGHT_URL very often ends
// in one, and `//api/v1` is redirected by some reverse proxies and 404ed by
// others.
func TestNewToleratesATrailingSlash(t *testing.T) {
	got := requestFor(t, "/")

	if want := blacklight.APIPath + "/healthz"; got.URL.Path != want {
		t.Errorf("request path is %q, want %q", got.URL.Path, want)
	}
}

func TestNewRejectsAnEmptyBaseURL(t *testing.T) {
	if _, err := blacklight.New("  "); err == nil {
		t.Fatal("New accepted an empty base URL; the failure would otherwise surface as a request to a relative path")
	}
}

func TestWithServiceTokenSetsTheBearerHeader(t *testing.T) {
	got := requestFor(t, "", blacklight.WithServiceToken("bl_abcd_secret"))

	if want := "Bearer bl_abcd_secret"; got.Header.Get("Authorization") != want {
		t.Errorf("Authorization is %q, want %q", got.Header.Get("Authorization"), want)
	}
}

// TestWithServiceTokenRefusesAnEmptyToken: the editor runs per request, so an
// empty token fails at the call rather than at construction — but it must fail,
// not send `Bearer ` and let the call arrive anonymous.
func TestWithServiceTokenRefusesAnEmptyToken(t *testing.T) {
	baseURL, _ := serveOnce(t, http.StatusOK, healthyBody)

	client, err := blacklight.New(baseURL, blacklight.WithServiceToken(""))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if _, err := client.GetHealthWithResponse(context.Background()); err == nil {
		t.Fatal("an empty service token produced a request; it would reach the server as an anonymous call")
	}
}

// TestEachDocumentedStatusParsesIntoItsOwnField is the whole argument for a
// generated client over a hand-rolled one: /healthz answers 200 and 503 with the
// same shape and 500 with a problem document, and the caller does not work out
// which it got by reading the bytes.
func TestEachDocumentedStatusParsesIntoItsOwnField(t *testing.T) {
	baseURL, _ := serveOnce(t, http.StatusServiceUnavailable,
		`{"status":"error","checks":{"db":"error"}}`)

	client, err := blacklight.New(baseURL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := client.GetHealthWithResponse(context.Background())
	if err != nil {
		t.Fatalf("GetHealth: %v", err)
	}

	if got.StatusCode() != http.StatusServiceUnavailable {
		t.Fatalf("status is %d, want 503", got.StatusCode())
	}
	if got.JSON200 != nil {
		t.Error("the 503 body parsed into JSON200; the typed fields are per status code")
	}
	if got.JSON503 == nil {
		t.Fatal("the 503 body did not parse into JSON503")
	}
	if got.JSON503.Status != blacklight.HealthStateError {
		t.Errorf("status is %q, want %q", got.JSON503.Status, blacklight.HealthStateError)
	}
	if got.JSON503.Checks.Db != blacklight.HealthStateError {
		t.Errorf("the db check is %q, want %q", got.JSON503.Checks.Db, blacklight.HealthStateError)
	}
}
