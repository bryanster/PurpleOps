package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"

	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

// api/openapi.yaml has only GETs with no request body until M1, so the
// interesting half of request validation — a body that does not match the
// schema — needs a document that describes one. This is that document, and it
// is deliberately small: the subject is the middleware, not the spec.
//
// The path is under the same /api/v1 server as the real one, so the chain being
// exercised is the chain the process runs.
const bodySpec = `
openapi: 3.1.0
info:
  title: request validation fixture
  version: 1.0.0
servers:
  - url: /api/v1
paths:
  /widgets:
    post:
      operationId: createWidget
      summary: Create a widget.
      tags: [system]
      security: []
      # Required of every operation, fixture or not — newServer refuses to build
      # a chain over a document with a gap in its authorization mapping
      # (M1-013), which is a rule this fixture is subject to like any other. It
      # is public because the subject here is validation and a permission would
      # only be one more thing between the request and the assertion.
      x-authz-public: true
      x-authz-because: a fixture for the request validator, which runs before authorization anyway.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required: [name, members]
              properties:
                name:
                  type: string
                  minLength: 1
                members:
                  type: array
                  items:
                    type: object
                    required: [role]
                    properties:
                      role:
                        type: string
                        enum: [lead, red, blue, observer]
      responses:
        "204":
          description: created
`

// TestABodyThatViolatesTheSpecNeverReachesTheHandler is the acceptance
// criterion in two halves: the client is told which fields are wrong, and the
// code behind the validator does not run. The second half is what makes the
// specification an enforcement point rather than documentation — a handler may
// assume its input matched.
func TestABodyThatViolatesTheSpecNeverReachesTheHandler(t *testing.T) {
	t.Parallel()

	entered := false
	server := validatingServer(t, func(http.ResponseWriter, *http.Request) { entered = true })

	body := strings.NewReader(`{"name":"","members":[{"role":"wizard"}]}`)
	request := httptest.NewRequest(http.MethodPost, BasePath+"/widgets", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := do(server, request)

	if got, want := recorder.Code, http.StatusBadRequest; got != want {
		t.Fatalf("status = %d, want %d\nbody: %s", got, want, recorder.Body.String())
	}
	if entered {
		t.Error("the handler ran; a request the specification rejects reached the code behind it")
	}

	problem := decodeProblem(t, recorder)
	if got, want := problem.Code, gen.ProblemCodeValidationFailed; got != want {
		t.Errorf("code = %q, want %q", got, want)
	}
	if problem.Errors == nil {
		t.Fatalf("errors[] is absent; the client is told a field is wrong but not which: %s", recorder.Body.String())
	}

	fields := make(map[string]string)
	for _, fieldErr := range *problem.Errors {
		fields[fieldErr.Field] = fieldErr.Message
	}
	for _, want := range []string{"name", "members[0].role"} {
		if message, ok := fields[want]; !ok {
			t.Errorf("no errors[] entry for %q, got %v", want, fields)
		} else if strings.TrimSpace(message) == "" {
			t.Errorf("the entry for %q has no message", want)
		}
	}

	// The submitted value is the caller's, and reflecting it back into a
	// response body is how a validation message becomes a way to put content on
	// this origin.
	if strings.Contains(recorder.Body.String(), "wizard") {
		t.Errorf("the problem document echoes the submitted value:\n%s", recorder.Body.String())
	}
}

// TestAValidBodyReachesTheHandler is the other half of the same criterion:
// the validator has to let a good request through, or the test above would pass
// with a middleware that rejects everything.
func TestAValidBodyReachesTheHandler(t *testing.T) {
	t.Parallel()

	entered := false
	server := validatingServer(t, func(w http.ResponseWriter, r *http.Request) {
		entered = true
		w.WriteHeader(http.StatusNoContent)
	})

	body := strings.NewReader(`{"name":"a widget","members":[{"role":"red"}]}`)
	request := httptest.NewRequest(http.MethodPost, BasePath+"/widgets", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := do(server, request)

	if got, want := recorder.Code, http.StatusNoContent; got != want {
		t.Fatalf("status = %d, want %d\nbody: %s", got, want, recorder.Body.String())
	}
	if !entered {
		t.Error("the handler did not run for a request the specification allows")
	}
}

// validatingServer builds the production chain over the fixture document, with
// handler registered on the API router where a generated route would be — so
// everything in front of it, the validator included, is what newServer builds
// for the real specification.
func validatingServer(t *testing.T, handler http.HandlerFunc) http.Handler {
	t.Helper()

	loader := &openapi3.Loader{}
	doc, err := loader.LoadFromData([]byte(bodySpec))
	if err != nil {
		t.Fatalf("loading the fixture spec: %v", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("the fixture spec is not valid: %v", err)
	}

	logs := &logBuffer{}
	deps := Deps{Config: testConfig(t), Store: stubStore{}, Logger: logs.logger()}
	// The path is relative to the mount point: the API router is mounted at
	// BasePath, which is also the server the fixture declares.
	server, err := newServer(deps, doc, func(r chi.Router) {
		r.Post("/widgets", handler)
	})
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	return server
}
