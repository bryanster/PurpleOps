package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/store/storetest"
	"github.com/bryanster/blacklight/internal/version"
)

func TestHealthzReportsAHealthyDatabase(t *testing.T) {
	t.Parallel()

	server, _ := newTestServer(t)
	recorder := get(server, BasePath+"/healthz")

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d\nbody: %s", got, want, recorder.Body.String())
	}
	if got, want := recorder.Header().Get("Content-Type"), "application/json"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}

	health := decodeJSON[gen.Health](t, recorder)
	if got, want := health.Status, gen.HealthStateOk; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
	if got, want := health.Checks.Db, gen.HealthStateOk; got != want {
		t.Errorf("checks.db = %q, want %q", got, want)
	}
}

// TestHealthzReportsADeadDatabase is the half that is worth asserting: a health
// check that cannot fail is a health check that tells an orchestrator nothing.
// The database is closed rather than mocked, so what is proved is that the real
// store reports its real state through the real handler.
func TestHealthzReportsADeadDatabase(t *testing.T) {
	t.Parallel()

	db := storetest.New(t)
	server, logs := newTestServerWith(t, testConfig(t), db)

	if err := db.Close(); err != nil {
		t.Fatalf("closing the database: %v", err)
	}

	recorder := get(server, BasePath+"/healthz")

	if got, want := recorder.Code, http.StatusServiceUnavailable; got != want {
		t.Fatalf("status = %d, want %d\nbody: %s", got, want, recorder.Body.String())
	}
	health := decodeJSON[gen.Health](t, recorder)
	if got, want := health.Status, gen.HealthStateError; got != want {
		t.Errorf("status = %q, want %q", got, want)
	}
	if got, want := health.Checks.Db, gen.HealthStateError; got != want {
		t.Errorf("checks.db = %q, want %q", got, want)
	}

	// The response says which check failed; only the log says why.
	if failure := logs.find(t, "health check failed"); failure["error"] == nil {
		t.Errorf("the health failure was logged without the reason: %v", failure)
	}
}

func TestVersionReportsTheBuild(t *testing.T) {
	t.Parallel()

	server, _ := newTestServer(t)
	recorder := get(server, BasePath+"/version")

	if got, want := recorder.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d\nbody: %s", got, want, recorder.Body.String())
	}

	got := decodeJSON[gen.Version](t, recorder)
	want := version.Get()
	if got.Version != want.Version || got.Commit != want.Commit || got.BuildDate != want.BuildDate {
		t.Errorf("version = %+v, want %+v", got, want)
	}
	// Every field is populated even in an unstamped test binary; a client
	// showing this in a footer must never have to render an empty string.
	if got.Version == "" || got.Commit == "" || got.BuildDate == "" {
		t.Errorf("version = %+v, want every field populated", got)
	}
}

// TestAnUnknownPathIsAProblemDocument covers both sides of the mount: a path
// the specification does not describe, and a path outside the API prefix
// entirely. chi answers the second one, the request validator the first, and
// the client must not be able to tell — nor get chi's plain-text default.
func TestAnUnknownPathIsAProblemDocument(t *testing.T) {
	t.Parallel()

	server, _ := newTestServer(t)

	for _, target := range []string{BasePath + "/nope", BasePath + "/healthz/extra", "/nope", "/"} {
		t.Run(target, func(t *testing.T) {
			recorder := get(server, target)

			if got, want := recorder.Code, http.StatusNotFound; got != want {
				t.Fatalf("status = %d, want %d\nbody: %s", got, want, recorder.Body.String())
			}
			problem := decodeProblem(t, recorder)
			if got, want := problem.Code, gen.ProblemCodeNotFound; got != want {
				t.Errorf("code = %q, want %q", got, want)
			}
			if body := recorder.Body.String(); strings.Contains(body, "404 page not found") {
				t.Errorf("chi's default answered instead of the problem writer:\n%s", body)
			}
		})
	}
}

func TestAWrongMethodIsMethodNotAllowed(t *testing.T) {
	t.Parallel()

	server, _ := newTestServer(t)
	recorder := do(server, httptest.NewRequest(http.MethodPost, BasePath+"/healthz", nil))

	if got, want := recorder.Code, http.StatusMethodNotAllowed; got != want {
		t.Fatalf("status = %d, want %d\nbody: %s", got, want, recorder.Body.String())
	}
	problem := decodeProblem(t, recorder)
	if got, want := problem.Code, gen.ProblemCodeMethodNotAllowed; got != want {
		t.Errorf("code = %q, want %q — a path that exists with another method is not a 404", got, want)
	}
}

// TestAPanicIsA500ThatLeaksNothing is the regression case for v1's habit of
// returning internals to the browser: the panic message here contains a
// filesystem path, and the stack contains package and type names.
func TestAPanicIsA500ThatLeaksNothing(t *testing.T) {
	t.Parallel()

	server, logs := newTestServerWith(t, testConfig(t), panickyStore{})
	recorder := get(server, BasePath+"/healthz")

	if got, want := recorder.Code, http.StatusInternalServerError; got != want {
		t.Fatalf("status = %d, want %d\nbody: %s", got, want, recorder.Body.String())
	}
	problem := decodeProblem(t, recorder)
	if got, want := problem.Code, gen.ProblemCodeInternal; got != want {
		t.Errorf("code = %q, want %q", got, want)
	}

	body := recorder.Body.String()
	for _, leak := range []string{
		"/secret/blacklight.duckdb", // the panic value
		"goroutine",                 // the stack
		"httpapi",                   // a package name from the stack
		"panic",                     // even the word
	} {
		if strings.Contains(body, leak) {
			t.Errorf("the response contains %q:\n%s", leak, body)
		}
	}

	// The other half: it is not lost, it is in the log with the stack and the
	// request ID the client was given.
	panicLine := logs.find(t, "panic serving request")
	if stack, ok := panicLine["stack"].(string); !ok || !strings.Contains(stack, "goroutine") {
		t.Errorf("the panic was logged without a usable stack: %v", panicLine)
	}
	if got, want := panicLine["request_id"], recorder.Header().Get(RequestIDHeader); got != want {
		t.Errorf("the panic was logged under request_id %v, but the client was given %q", got, want)
	}
	if instance := problem.Instance; instance == nil || *instance != recorder.Header().Get(RequestIDHeader) {
		t.Errorf("problem.instance = %v, want the request ID the client can quote", instance)
	}
}

// TestNewServerRefusesToBuildWithoutAStore keeps the failure at startup, where
// it is a message, rather than at the first health check, where it is a panic.
func TestNewServerRefusesToBuildWithoutAStore(t *testing.T) {
	t.Parallel()

	if _, err := NewServer(Deps{Config: testConfig(t)}); err == nil {
		t.Error("NewServer with no store = nil error, want a refusal")
	}
}
