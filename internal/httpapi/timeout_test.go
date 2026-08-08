package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/store"
)

// TestTheRequestDeadlineEndsTheHandlersContext is what makes the timeout real:
// the deadline is on the context every query is issued with, so a handler that
// outruns it is abandoned rather than left running against a client that has
// already gone.
func TestTheRequestDeadlineEndsTheHandlersContext(t *testing.T) {
	t.Parallel()

	var handlerErr error
	handler := timeout(50 * time.Millisecond)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		handlerErr = r.Context().Err()
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !errors.Is(handlerErr, context.DeadlineExceeded) {
		t.Errorf("the handler's context ended with %v, want %v", handlerErr, context.DeadlineExceeded)
	}
}

// TestAZeroTimeoutIsNoTimeout keeps a zero-valued config from meaning "every
// request expires immediately", which is the failure mode a caller would find
// hardest to explain.
func TestAZeroTimeoutIsNoTimeout(t *testing.T) {
	t.Parallel()

	var deadline time.Time
	var hasDeadline bool
	handler := timeout(0)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		deadline, hasDeadline = r.Context().Deadline()
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if hasDeadline {
		t.Errorf("the request has a deadline of %v, want none", deadline)
	}
}

// TestTheServerAppliesTheConfiguredTimeout proves the middleware is in the
// chain and reads the configuration, rather than being a constant somewhere.
func TestTheServerAppliesTheConfiguredTimeout(t *testing.T) {
	t.Parallel()

	cfg := testConfig(t)
	cfg.Server.RequestTimeout = 5 * time.Minute

	db := deadlineStore{deadlines: make(chan time.Time, 1)}
	server, _ := newTestServerWith(t, cfg, db)

	if got, want := get(server, BasePath+"/healthz").Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}

	select {
	case deadline := <-db.deadlines:
		if deadline.IsZero() {
			t.Fatal("the store was called with no deadline; the timeout middleware is not in the chain")
		}
		if remaining := time.Until(deadline); remaining < 4*time.Minute {
			t.Errorf("the deadline is %v away, want about the configured 5m", remaining)
		}
	default:
		t.Fatal("the health check never reached the store")
	}
}

// deadlineStore reports the deadline the request context carried by the time it
// reached the database, which is the only place the timeout has to be real.
type deadlineStore struct {
	store.Store

	deadlines chan time.Time
}

func (d deadlineStore) Health(ctx context.Context) error {
	deadline, _ := ctx.Deadline()
	d.deadlines <- deadline
	return nil
}
