package apierr

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"

	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

// These tests drive the real kin-openapi validator rather than hand-built error
// values, because the thing being asserted is that this package understands
// what that library actually returns. A fixture of the shape we remember it
// having would keep passing after an upgrade changed it.
//
// The document is a local one with a request body, since api/openapi.yaml has
// only GETs until M1. It is small on purpose: the subject is the translation,
// not the spec.
const validatorTestSpec = `
openapi: 3.1.0
info:
  title: validator fixture
  version: 1.0.0
paths:
  /widgets:
    get:
      operationId: listWidgets
      parameters:
        - name: limit
          in: query
          schema:
            type: integer
            minimum: 1
            maximum: 200
      responses:
        "200":
          description: ok
    post:
      operationId: createWidget
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

func TestASpecViolationNamesEveryFieldThatFailed(t *testing.T) {
	t.Parallel()

	body := `{"name":"","members":[{"role":"wizard"}]}`
	err := validateFixture(t, http.MethodPost, "/widgets", body)

	problem := Translate(err, testRequestID)

	if got, want := problem.Status, http.StatusBadRequest; got != want {
		t.Errorf("Status = %d, want %d", got, want)
	}
	if got, want := problem.Code, gen.ProblemCodeValidationFailed; got != want {
		t.Errorf("Code = %q, want %q", got, want)
	}
	if got, want := detailOf(problem), specValidationDetail; got != want {
		t.Errorf("Detail = %q, want %q", got, want)
	}

	fields := fieldsOf(problem)
	if len(fields) != 2 {
		t.Fatalf("Errors = %v, want one entry per failed field (name, members[0].role)", fields)
	}
	// The paths are what a form matches against its own inputs, so the array
	// index has to be rendered the way the spec's example says.
	wantPaths := map[string]bool{"name": false, "members[0].role": false}
	for _, field := range fields {
		if _, ok := wantPaths[field.Field]; !ok {
			t.Errorf("unexpected field path %q, want one of name, members[0].role", field.Field)
			continue
		}
		wantPaths[field.Field] = true
		if strings.TrimSpace(field.Message) == "" {
			t.Errorf("field %q has no message; there is nothing to show next to the input", field.Field)
		}
	}
	for path, seen := range wantPaths {
		if !seen {
			t.Errorf("no entry for the field %q", path)
		}
	}

	validateAgainstSpec(t, serialize(t, problem))
}

// TestASpecViolationSaysNothingAboutTheValue guards the one leak a validation
// message can produce: echoing the request's own contents back into a response
// body.
func TestASpecViolationSaysNothingAboutTheValue(t *testing.T) {
	t.Parallel()

	err := validateFixture(t, http.MethodPost, "/widgets", `{"name":"","members":[{"role":"sup3rs3cret"}]}`)
	body := serialize(t, Translate(err, testRequestID))

	if strings.Contains(body, "sup3rs3cret") {
		t.Errorf("the problem document echoes the submitted value:\n%s", body)
	}
}

// TestAMissingFieldIsNamedAsTheField covers the most common validation failure
// there is. The validator reports it against the enclosing object, because the
// value that is missing has no location of its own; a client that has to parse
// the field name out of an English sentence cannot highlight the input.
func TestAMissingFieldIsNamedAsTheField(t *testing.T) {
	t.Parallel()

	err := validateFixture(t, http.MethodPost, "/widgets", `{"name":"a widget"}`)
	problem := Translate(err, testRequestID)

	fields := fieldsOf(problem)
	if len(fields) != 1 {
		t.Fatalf("Errors = %v, want a single entry", fields)
	}
	if got, want := fields[0].Field, "members"; got != want {
		t.Errorf("Field = %q, want %q", got, want)
	}
	if got, want := fields[0].Message, "is required"; got != want {
		t.Errorf("Message = %q, want %q", got, want)
	}
	validateAgainstSpec(t, serialize(t, problem))
}

// TestABodyOfTheWrongShapeReportsItself pins the one case with no field to
// name: the body itself is wrong. The entry has an empty path, which means the
// document rather than a field in it.
func TestABodyOfTheWrongShapeReportsItself(t *testing.T) {
	t.Parallel()

	err := validateFixture(t, http.MethodPost, "/widgets", `[1,2]`)
	problem := Translate(err, testRequestID)

	fields := fieldsOf(problem)
	if len(fields) != 1 {
		t.Fatalf("Errors = %v, want a single entry", fields)
	}
	if got := fields[0].Field; got != "" {
		t.Errorf("Field = %q, want empty: the body as a whole is what is wrong", got)
	}
	if got := fields[0].Message; !strings.Contains(got, "want object") {
		t.Errorf("Message = %q, want it to say what shape was expected", got)
	}
	validateAgainstSpec(t, serialize(t, problem))
}

func TestAFailedParameterIsNamedAsTheField(t *testing.T) {
	t.Parallel()

	err := validateFixture(t, http.MethodGet, "/widgets?limit=1000", "")
	problem := Translate(err, testRequestID)

	if got, want := problem.Code, gen.ProblemCodeValidationFailed; got != want {
		t.Errorf("Code = %q, want %q", got, want)
	}
	fields := fieldsOf(problem)
	if len(fields) != 1 || fields[0].Field != "limit" {
		t.Fatalf("Errors = %v, want a single entry for the parameter %q", fields, "limit")
	}
	validateAgainstSpec(t, serialize(t, problem))
}

// TestAMalformedBodyStillExplainsItself covers the case with no field to blame:
// the caller gets the validator's reason rather than a generic sentence it
// cannot act on.
func TestAMalformedBodyStillExplainsItself(t *testing.T) {
	t.Parallel()

	err := validateFixture(t, http.MethodPost, "/widgets", `{"name":`)
	problem := Translate(err, testRequestID)

	if got, want := problem.Code, gen.ProblemCodeValidationFailed; got != want {
		t.Errorf("Code = %q, want %q", got, want)
	}
	if detail := detailOf(problem); strings.TrimSpace(detail) == "" {
		t.Error("Detail is empty; a caller with neither a detail nor a field list has nothing to fix")
	}
	validateAgainstSpec(t, serialize(t, problem))
}

func TestRoutingFailuresBecomeProblems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		status int
		code   Code
	}{
		{
			name:   "no such path",
			err:    routers.ErrPathNotFound,
			status: http.StatusNotFound,
			code:   gen.ProblemCodeNotFound,
		},
		{
			name:   "wrong method",
			err:    routers.ErrMethodNotAllowed,
			status: http.StatusMethodNotAllowed,
			code:   gen.ProblemCodeMethodNotAllowed,
		},
		{
			// The validator middleware wraps what it returns; the answer must
			// not depend on that.
			name:   "wrapped",
			err:    fmt.Errorf("validate request: %w", routers.ErrPathNotFound),
			status: http.StatusNotFound,
			code:   gen.ProblemCodeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			problem := Translate(tt.err, testRequestID)
			if got := problem.Status; got != tt.status {
				t.Errorf("Status = %d, want %d", got, tt.status)
			}
			if got := problem.Code; got != tt.code {
				t.Errorf("Code = %q, want %q", got, tt.code)
			}
			validateAgainstSpec(t, serialize(t, problem))
		})
	}
}

// TestAnUnsatisfiedSecurityRequirementIsForbidden pins the behaviour M1 will
// revisit. Until then it matters that this is a 403 with the shared shape and
// not an unrecognised error reported as a 500.
func TestAnUnsatisfiedSecurityRequirementIsForbidden(t *testing.T) {
	t.Parallel()

	err := &openapi3filter.SecurityRequirementsError{
		SecurityRequirements: openapi3.SecurityRequirements{{"cookieSession": {}}},
		Errors:               []error{errors.New("no session cookie")},
	}

	problem := Translate(err, testRequestID)
	if got, want := problem.Status, http.StatusForbidden; got != want {
		t.Errorf("Status = %d, want %d", got, want)
	}
	if got, want := problem.Code, gen.ProblemCodeForbidden; got != want {
		t.Errorf("Code = %q, want %q", got, want)
	}
	if body := serialize(t, problem); strings.Contains(body, "no session cookie") {
		t.Errorf("the problem document explains which credential was missing:\n%s", body)
	}
	validateAgainstSpec(t, serialize(t, problem))
}

// validateFixture runs the real request validator against validatorTestSpec and
// returns the error it produced.
//
// The route is built by hand rather than by a Router: a router would add a
// dependency for the sake of resolving one known path, and the validator only
// needs the operation it is validating against.
func validateFixture(t *testing.T, method, target, body string) error {
	t.Helper()

	loader := &openapi3.Loader{}
	doc, err := loader.LoadFromData([]byte(validatorTestSpec))
	if err != nil {
		t.Fatalf("load the fixture spec: %v", err)
	}
	if err := doc.Validate(loader.Context); err != nil {
		t.Fatalf("the fixture spec is not valid: %v", err)
	}

	var reader *strings.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	var req *http.Request
	if reader == nil {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, reader)
		req.Header.Set("Content-Type", "application/json")
	}

	pathItem := doc.Paths.Find("/widgets")
	if pathItem == nil {
		t.Fatal("the fixture spec has no /widgets path")
	}
	operation := pathItem.GetOperation(method)
	if operation == nil {
		t.Fatalf("the fixture spec has no %s /widgets operation", method)
	}

	err = openapi3filter.ValidateRequest(t.Context(), &openapi3filter.RequestValidationInput{
		Request: req,
		Route: &routers.Route{
			Spec:      doc,
			Path:      "/widgets",
			PathItem:  pathItem,
			Method:    method,
			Operation: operation,
		},
		// One response listing every problem with the request beats a client
		// fixing them one round trip at a time — the same argument config.Load
		// makes for environment variables.
		Options: &openapi3filter.Options{MultiError: true},
	})
	if err == nil {
		t.Fatal("the validator accepted a request the fixture spec forbids")
	}
	return err
}
