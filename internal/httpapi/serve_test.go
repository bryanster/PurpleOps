package httpapi

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/config"
)

// TestShutdownLetsAnInFlightRequestFinish is the guarantee a rolling restart
// depends on: the signal arrives while somebody is halfway through saving a
// scoring decision, and they do not lose it.
func TestShutdownLetsAnInFlightRequestFinish(t *testing.T) {
	t.Parallel()

	// The handler blocks until the test says so, which is how "in flight when
	// the signal arrived" is made deterministic rather than a race with a sleep.
	started := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		if _, err := io.WriteString(w, "finished"); err != nil {
			t.Errorf("writing the slow response: %v", err)
		}
	})

	ctx, stop := context.WithCancel(t.Context())
	server := startServing(t, ctx, config.Server{ShutdownTimeout: 5 * time.Second}, handler)

	// Fire the slow request and wait until the server is actually inside it.
	type result struct {
		body string
		err  error
	}
	responses := make(chan result, 1)
	go func() {
		body, err := fetch(server.url)
		responses <- result{body, err}
	}()
	<-started

	// The signal. Nothing is allowed to finish yet.
	stop()

	// A new connection is refused from here on: the listener is closed even
	// though the in-flight request is still running.
	waitUntilRefused(t, server.url)

	close(release)

	got := <-responses
	if got.err != nil {
		t.Fatalf("the in-flight request failed instead of finishing: %v", got.err)
	}
	if got.body != "finished" {
		t.Errorf("the in-flight response was %q, want %q", got.body, "finished")
	}

	if err := <-server.done; err != nil {
		t.Errorf("ListenAndServe returned %v, want nil for a clean shutdown", err)
	}
	if logged := server.logs.String(); !strings.Contains(logged, "shutting down") {
		t.Errorf("the shutdown was not logged:\n%s", logged)
	}
}

// TestShutdownGivesUpOnAHungRequest is the other half. A handler that never
// returns must not be able to keep the process alive: an orchestrator that
// asked for a stop will kill it, and the store then never closes cleanly.
func TestShutdownGivesUpOnAHungRequest(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	hang := make(chan struct{})
	// Released when the test ends, so the goroutine does not outlive it.
	t.Cleanup(func() { close(hang) })

	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-hang
	})

	ctx, stop := context.WithCancel(t.Context())
	// A grace period short enough to observe, long enough not to be flaky on a
	// loaded machine.
	server := startServing(t, ctx, config.Server{ShutdownTimeout: 250 * time.Millisecond}, handler)

	// The response never arrives — the connection is cut off — so the error is
	// collected rather than ignored, and the goroutine has somewhere to end.
	hungRequest := make(chan error, 1)
	go func() {
		_, err := fetch(server.url)
		hungRequest <- err
	}()
	<-started

	stop()

	select {
	case err := <-server.done:
		if err != nil {
			t.Errorf("ListenAndServe returned %v, want nil: a cut-off request is a warning, not a failed run", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the server never returned; a hung handler is holding the process open")
	}

	if logged := server.logs.String(); !strings.Contains(logged, "closing connections") {
		t.Errorf("the connections were cut off without saying so:\n%s", logged)
	}
	if err := <-hungRequest; err == nil {
		t.Error("the hung request returned a response; it was not cut off")
	}
}

func TestListenReportsAnUnusableAddress(t *testing.T) {
	t.Parallel()

	err := ListenAndServe(t.Context(), config.Server{Addr: "127.0.0.1:not-a-port"}, http.NotFoundHandler(), nil)
	if err == nil {
		t.Fatal("ListenAndServe on a bad address = nil, want the failure to bind")
	}
	if !strings.Contains(err.Error(), "not-a-port") {
		t.Errorf("error = %v, want it to name the address that could not be bound", err)
	}
}

// runningServer is a server under test, with the address it actually bound.
type runningServer struct {
	url  string
	done <-chan error
	logs *logBuffer
}

// startServing binds port 0 on the loopback interface and serves handler until
// ctx is cancelled. Port 0 rather than a fixed one: two of these run in
// parallel, and a test that fails because a developer had something else on
// 8080 is a test nobody trusts.
func startServing(t *testing.T, ctx context.Context, cfg config.Server, handler http.Handler) runningServer {
	t.Helper()

	listener, err := listen(ctx, "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	logs := &logBuffer{}
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg, listener, handler, logs.logger()) }()

	return runningServer{
		url:  "http://" + listener.Addr().String() + "/",
		done: done,
		logs: logs,
	}
}

// fetch performs a real HTTP request over the loopback listener, because what
// is being tested is the connection handling rather than the handler.
func fetch(url string) (string, error) {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", err
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(response.Body)
	return string(body), err
}

// waitUntilRefused blocks until the listener has stopped accepting, which is
// what "stops accepting new connections" means from the outside.
func waitUntilRefused(t *testing.T, url string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		// A fresh client each time, with no keep-alive: a pooled connection
		// would be reused, and the question is about new ones. The timeout is
		// for the moment between the signal and Shutdown closing the listener,
		// where a probe connects and then waits on the handler that is holding
		// everything up.
		client := &http.Client{
			Transport: &http.Transport{DisableKeepAlives: true},
			Timeout:   200 * time.Millisecond,
		}
		request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("building the probe request: %v", err)
		}
		response, err := client.Do(request)
		switch {
		case err == nil:
			if err := response.Body.Close(); err != nil {
				t.Errorf("closing the probe response: %v", err)
			}
		case os.IsTimeout(err):
			// Connected, and now queued behind the slow handler. Not an answer
			// either way; try again.
		default:
			return // refused: the listener is closed
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the listener was still accepting connections after the shutdown began")
}
