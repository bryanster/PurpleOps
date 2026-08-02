package apierr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/bryanster/purpleops/api"
	"github.com/bryanster/purpleops/internal/httpapi/gen"
)

const testRequestID = "018f3b2c-7a41-7c3e-9b0d-2f1a4c6e8d90"

func TestTranslateMapsAnErrorToItsProblem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		status     int
		code       Code
		detail     string
		fields     []FieldError
		absentFrom string // must not appear anywhere in the serialized document
	}{
		{
			name: "not found hides the identifier",
			// Deliberately unlike testRequestID: the assertion below is that
			// this string is nowhere in the document, and `instance` legitimately
			// carries the request ID.
			err:        NotFound("engagement", "d4a91f60-looked-up"),
			status:     http.StatusNotFound,
			code:       gen.ProblemCodeNotFound,
			detail:     "no such engagement",
			absentFrom: "d4a91f60-looked-up",
		},
		{
			name:       "forbidden says nothing about what was attempted",
			err:        Forbidden("delete engagement 018f3b2c"),
			status:     http.StatusForbidden,
			code:       gen.ProblemCodeForbidden,
			detail:     "you are not permitted to do this",
			absentFrom: "delete engagement",
		},
		{
			name:   "method not allowed",
			err:    MethodNotAllowed(http.MethodPost),
			status: http.StatusMethodNotAllowed,
			code:   gen.ProblemCodeMethodNotAllowed,
			detail: "the method is not allowed on this path",
		},
		{
			name:   "conflict explains itself",
			err:    Conflict("the engagement is closed"),
			status: http.StatusConflict,
			code:   gen.ProblemCodeConflict,
			detail: "the engagement is closed",
		},
		{
			name:   "rate limited",
			err:    RateLimited("too many login attempts", 90*time.Second),
			status: http.StatusTooManyRequests,
			code:   gen.ProblemCodeRateLimited,
			detail: "too many login attempts",
		},
		{
			name:   "validation carries its fields",
			err:    Validation(Field("name", "must not be empty"), Field("members[0].role", "must be one of lead, red, blue, observer")),
			status: http.StatusBadRequest,
			code:   gen.ProblemCodeValidationFailed,
			detail: "the request is not valid",
			fields: []FieldError{
				{Field: "name", Message: "must not be empty"},
				{Field: "members[0].role", Message: "must be one of lead, red, blue, observer"},
			},
		},
		{
			name:       "an unrecognised error is internal and generic",
			err:        errors.New("boom"),
			status:     http.StatusInternalServerError,
			code:       gen.ProblemCodeInternal,
			detail:     internalDetail,
			absentFrom: "boom",
		},
		{
			name:       "a wrapped domain error keeps its code",
			err:        fmt.Errorf("load engagement: %w", NotFound("engagement", "018f3b2c")),
			status:     http.StatusNotFound,
			code:       gen.ProblemCodeNotFound,
			detail:     "no such engagement",
			absentFrom: "load engagement",
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
			if got, want := problem.Title, http.StatusText(tt.status); got != want {
				t.Errorf("Title = %q, want %q", got, want)
			}
			if got := detailOf(problem); got != tt.detail {
				t.Errorf("Detail = %q, want %q", got, tt.detail)
			}
			if got, want := fieldsOf(problem), tt.fields; !equalFields(got, want) {
				t.Errorf("Errors = %v, want %v", got, want)
			}
			// AC: instance carries the request ID, so a user can quote it and an
			// operator can find the log line.
			if problem.Instance == nil || *problem.Instance != testRequestID {
				t.Errorf("Instance = %v, want %q", problem.Instance, testRequestID)
			}

			body := serialize(t, problem)
			if tt.absentFrom != "" && strings.Contains(body, tt.absentFrom) {
				t.Errorf("the problem document contains %q, which is for the log only:\n%s", tt.absentFrom, body)
			}
			validateAgainstSpec(t, body)
		})
	}
}

// TestTranslateLeaksNothingFromAnUnrecognisedError is the leak test the ticket
// asks for: a driver error with a connection string in it is exactly what v1
// put on the screen.
func TestTranslateLeaksNothingFromAnUnrecognisedError(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("open store: %w", errors.New("dial duckdb://pops:s3cr3t@db.internal:5432/pops: connection refused"))
	body := serialize(t, Translate(err, testRequestID))

	for _, secret := range []string{"s3cr3t", "db.internal", "connection refused", "duckdb"} {
		if strings.Contains(body, secret) {
			t.Errorf("the problem document contains %q:\n%s", secret, body)
		}
	}
	validateAgainstSpec(t, body)
}

// TestTranslateOmitsAnAbsentRequestID keeps `instance` meaningful: a document
// that has one always carries a request ID an operator can search for.
func TestTranslateOmitsAnAbsentRequestID(t *testing.T) {
	t.Parallel()

	problem := Translate(NotFound("engagement", "018f3b2c"), "")
	if problem.Instance != nil {
		t.Errorf("Instance = %q, want it omitted when there is no request ID", *problem.Instance)
	}
	validateAgainstSpec(t, serialize(t, problem))
}

func TestWriteServesTheProblemMediaTypeAndStatus(t *testing.T) {
	t.Parallel()

	responder, _ := newTestResponder()
	rec := httptest.NewRecorder()
	responder.Write(rec, newTestRequest(t, testRequestID), Conflict("the engagement is closed"))

	if got, want := rec.Code, http.StatusConflict; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	if got, want := rec.Header().Get("Content-Type"), MediaType; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}

	var problem gen.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decode the response body: %v\n%s", err, rec.Body.String())
	}
	if got, want := problem.Code, gen.ProblemCodeConflict; got != want {
		t.Errorf("code = %q, want %q", got, want)
	}
	if got := problem.Instance; got == nil || *got != testRequestID {
		t.Errorf("instance = %v, want %q", got, testRequestID)
	}
	validateAgainstSpec(t, rec.Body.String())
}

// TestWriteSendsRetryAfterWithARateLimit is why the wait travels on the error
// rather than being set by whoever is doing the limiting: a 429 that does not
// say when to come back leaves a client guessing, and this is the one place that
// can promise every one of them carries it (M1-004).
func TestWriteSendsRetryAfterWithARateLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{{
		name: "whole seconds",
		err:  RateLimited("too many sign-in attempts", 90*time.Second),
		want: "90",
	}, {
		// Rounded up. A client that came back at the rounded-down second would
		// still be locked out, and would spend its retry finding that out.
		name: "a part second rounds up",
		err:  RateLimited("too many sign-in attempts", 1500*time.Millisecond),
		want: "2",
	}, {
		name: "the last moment of a lockout is still a second away",
		err:  RateLimited("too many sign-in attempts", time.Microsecond),
		want: "1",
	}, {
		name: "every other failure carries none",
		err:  Conflict("the engagement is closed"),
		want: "",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			responder, _ := newTestResponder()
			rec := httptest.NewRecorder()
			responder.Write(rec, newTestRequest(t, testRequestID), test.err)

			if got := rec.Header().Get("Retry-After"); got != test.want {
				t.Errorf("Retry-After = %q, want %q", got, test.want)
			}
		})
	}
}

// TestWriteLogsTheCauseItDoesNotSend asserts both halves of the rule at once:
// the operator gets the error, the client does not.
func TestWriteLogsTheCauseItDoesNotSend(t *testing.T) {
	t.Parallel()

	responder, log := newTestResponder()
	rec := httptest.NewRecorder()
	responder.Write(rec, newTestRequest(t, testRequestID), fmt.Errorf("count engagements: %w", errors.New("boom")))

	if got, want := rec.Code, http.StatusInternalServerError; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	if body := rec.Body.String(); strings.Contains(body, "boom") {
		t.Errorf("the response body contains the cause:\n%s", body)
	}

	logged := log.String()
	if !strings.Contains(logged, "boom") {
		t.Errorf("the log does not contain the cause; nothing else records it:\n%s", logged)
	}
	// Without the request ID the log line and the client's copy of the problem
	// cannot be joined up, which is the only reason instance exists.
	if !strings.Contains(logged, testRequestID) {
		t.Errorf("the log does not contain the request ID:\n%s", logged)
	}
	if !strings.Contains(logged, `"level":"ERROR"`) {
		t.Errorf("a 500 was not logged at ERROR:\n%s", logged)
	}
}

// TestWriteDoesNotLogAClientErrorAsAnError keeps the error log usable: a 404 is
// the client's problem and the client was told about it.
func TestWriteDoesNotLogAClientErrorAsAnError(t *testing.T) {
	t.Parallel()

	responder, log := newTestResponder()
	responder.Write(httptest.NewRecorder(), newTestRequest(t, testRequestID), NotFound("engagement", "018f3b2c"))

	logged := log.String()
	if strings.Contains(logged, `"level":"ERROR"`) {
		t.Errorf("a 404 was logged at ERROR:\n%s", logged)
	}
	if !strings.Contains(logged, "018f3b2c") {
		t.Errorf("the identifier that was not found is in neither the response nor the log:\n%s", logged)
	}
}

func TestWriteWithoutARequestIDStillAnswers(t *testing.T) {
	t.Parallel()

	responder, _ := newTestResponder()
	rec := httptest.NewRecorder()
	// No RequestID middleware ran — a handler mounted outside the chain, or a
	// test. The response is still a well-formed problem document.
	responder.Write(rec, httptest.NewRequest(http.MethodGet, "/api/v1/engagements", nil), NotFound("engagement", "x"))

	if got, want := rec.Code, http.StatusNotFound; got != want {
		t.Errorf("status = %d, want %d", got, want)
	}
	validateAgainstSpec(t, rec.Body.String())
}

func newTestResponder() (*Responder, *bytes.Buffer) {
	log := &bytes.Buffer{}
	handler := slog.NewJSONHandler(log, &slog.HandlerOptions{Level: slog.LevelDebug})
	return NewResponder(slog.New(handler)), log
}

func newTestRequest(t *testing.T, requestID string) *http.Request {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/engagements/018f3b2c", nil)
	// The same context key chi's middleware uses, which is how Write finds it
	// whichever middleware set it.
	return req.WithContext(context.WithValue(req.Context(), middleware.RequestIDKey, requestID))
}

// detailOf and fieldsOf flatten the optional members of a problem document, so
// that a test can compare "absent" and "empty" without a nil check per
// assertion. Whether a member is absent is asserted where it matters — see
// TestTranslateOmitsAnAbsentRequestID.
func detailOf(problem gen.Problem) string {
	if problem.Detail == nil {
		return ""
	}
	return *problem.Detail
}

func fieldsOf(problem gen.Problem) []FieldError {
	if problem.Errors == nil {
		return nil
	}
	return *problem.Errors
}

func equalFields(got, want []FieldError) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func serialize(t *testing.T, problem gen.Problem) string {
	t.Helper()

	raw, err := json.Marshal(problem)
	if err != nil {
		t.Fatalf("marshal the problem document: %v", err)
	}
	return string(raw)
}

// validateAgainstSpec checks a serialized problem document against the Problem
// schema in api/openapi.yaml. Producing a body a generated client cannot parse
// is the failure this whole package is meant to make impossible, so every test
// that builds one ends here.
func validateAgainstSpec(t *testing.T, body string) {
	t.Helper()

	schema, err := problemSchema()
	if err != nil {
		t.Fatalf("load the Problem schema from the embedded spec: %v", err)
	}

	var value any
	if err := json.Unmarshal([]byte(body), &value); err != nil {
		t.Fatalf("the problem document is not JSON: %v\n%s", err, body)
	}
	if err := schema.VisitJSON(value); err != nil {
		t.Errorf("the problem document does not validate against the spec: %v\n%s", err, body)
	}
}

// problemSchema parses the embedded spec once for the whole test binary; Load
// walks and validates the entire document, which is not something to do per
// assertion.
var problemSchema = sync.OnceValues(func() (*openapi3.Schema, error) {
	doc, err := api.Load()
	if err != nil {
		return nil, err
	}
	schema := doc.Components.Schemas["Problem"]
	if schema == nil || schema.Value == nil {
		return nil, errors.New("components.schemas.Problem is missing from the spec")
	}
	return schema.Value, nil
})
