package apierr

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

func TestEachConstructorCarriesItsCodeStatusAndSentinel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *Error
		code     Code
		status   int
		sentinel error
	}{
		{
			name:     "not found",
			err:      NotFound("engagement", "018f3b2c-7a41-7c3e-9b0d-2f1a4c6e8d90"),
			code:     gen.ProblemCodeNotFound,
			status:   http.StatusNotFound,
			sentinel: ErrNotFound,
		},
		{
			name:     "forbidden",
			err:      Forbidden("close engagement 018f3b2c"),
			code:     gen.ProblemCodeForbidden,
			status:   http.StatusForbidden,
			sentinel: ErrForbidden,
		},
		{
			name:     "method not allowed",
			err:      MethodNotAllowed(http.MethodDelete),
			code:     gen.ProblemCodeMethodNotAllowed,
			status:   http.StatusMethodNotAllowed,
			sentinel: ErrMethodNotAllowed,
		},
		{
			name:     "conflict",
			err:      Conflict("the engagement is closed"),
			code:     gen.ProblemCodeConflict,
			status:   http.StatusConflict,
			sentinel: ErrConflict,
		},
		{
			name:     "rate limited",
			err:      RateLimited("too many login attempts", time.Minute),
			code:     gen.ProblemCodeRateLimited,
			status:   http.StatusTooManyRequests,
			sentinel: ErrRateLimited,
		},
		{
			name:     "validation",
			err:      Validation(Field("name", "must not be empty")),
			code:     gen.ProblemCodeValidationFailed,
			status:   http.StatusBadRequest,
			sentinel: ErrValidation,
		},
		{
			name:     "internal",
			err:      Internal(errors.New("boom")),
			code:     gen.ProblemCodeInternal,
			status:   http.StatusInternalServerError,
			sentinel: ErrInternal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.err.Code(); got != tt.code {
				t.Errorf("Code() = %q, want %q", got, tt.code)
			}
			if got := tt.err.Status(); got != tt.status {
				t.Errorf("Status() = %d, want %d", got, tt.status)
			}
			if !errors.Is(tt.err, tt.sentinel) {
				t.Errorf("errors.Is(%v, %v) = false, want true", tt.err, tt.sentinel)
			}
			// Wrapping is the normal way one of these travels up a call stack,
			// and it must not change any of the answers above.
			wrapped := fmt.Errorf("load engagement: %w", tt.err)
			if !errors.Is(wrapped, tt.sentinel) {
				t.Errorf("errors.Is(wrapped, %v) = false, want true", tt.sentinel)
			}
			var unwrapped *Error
			if !errors.As(wrapped, &unwrapped) {
				t.Fatal("errors.As(wrapped, *Error) = false, want true")
			}
			if got := unwrapped.Code(); got != tt.code {
				t.Errorf("wrapped Code() = %q, want %q", got, tt.code)
			}
		})
	}
}

// TestASentinelDoesNotMatchAnotherCode guards the thing a shared sentinel would
// quietly break: a caller that handles "not found" specially must not also
// swallow a conflict.
func TestASentinelDoesNotMatchAnotherCode(t *testing.T) {
	t.Parallel()

	if errors.Is(NotFound("engagement", "x"), ErrConflict) {
		t.Error("a not-found error matches ErrConflict; the sentinels are not distinct")
	}
	if errors.Is(Conflict("closed"), ErrNotFound) {
		t.Error("a conflict matches ErrNotFound; the sentinels are not distinct")
	}
}

// TestInternalKeepsItsCauseReachable is why Unwrap returns two errors: the
// sentinel is for "what kind of failure was this", the cause is for "what
// actually went wrong", and a caller may reasonably ask either.
func TestInternalKeepsItsCauseReachable(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("count engagements: %w", Internal(sql.ErrConnDone))

	if !errors.Is(err, ErrInternal) {
		t.Error("errors.Is(err, ErrInternal) = false, want true")
	}
	if !errors.Is(err, sql.ErrConnDone) {
		t.Error("errors.Is(err, sql.ErrConnDone) = false, want true; the cause left the chain")
	}
}

// TestTheLogSeesWhatTheClientDoesNot is the split the Error type exists for.
// Error() is the operator's view; the identifier and the attempted action are
// in it and nowhere near the response body — Translate is tested separately for
// the other half.
func TestTheLogSeesWhatTheClientDoesNot(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  *Error
		want string
	}{
		{
			name: "not found names the identifier",
			err:  NotFound("engagement", "018f3b2c"),
			want: "018f3b2c",
		},
		{
			name: "forbidden names the action",
			err:  Forbidden("close engagement 018f3b2c"),
			want: "close engagement",
		},
		{
			name: "internal names the cause",
			err:  Internal(errors.New("dial tcp 10.0.0.5:5432: connection refused")),
			want: "connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			text := tt.err.Error()
			if !strings.Contains(text, tt.want) {
				t.Errorf("Error() = %q, want it to contain %q — the log line is all an operator gets", text, tt.want)
			}
			if !strings.Contains(text, string(tt.err.Code())) {
				t.Errorf("Error() = %q, want it to name the code %q", text, tt.err.Code())
			}
		})
	}
}
