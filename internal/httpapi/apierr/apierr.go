package apierr

import (
	"fmt"

	"github.com/bryanster/purpleops/internal/httpapi/gen"
)

// Error is a failure this API knows how to describe to a client: a [Code], the
// prose that goes with it, and — for a validation failure — which fields were
// wrong.
//
// It splits what the client is told from what the log is told, which is the
// whole reason it exists as a type rather than as a string. Build one with a
// constructor; the fields are unexported so that a code and its status cannot
// drift apart, and so that "put the driver error in the detail" is not
// something a hurried change can do by accident.
type Error struct {
	// code decides the HTTP status and is what a client switches on.
	code Code
	// detail is the client's half: prose about this occurrence, safe to show to
	// whoever made the request.
	detail string
	// fields is the field-level breakdown of a validation failure. Empty for
	// every other code.
	fields []FieldError
	// cause is the log's half: the wrapped error, or context such as the
	// identifier that was not found. It reaches the log and never the response.
	cause error
}

// Code returns the stable machine-readable identifier for this failure.
func (e *Error) Code() Code { return e.code }

// Status returns the HTTP status this failure is reported with.
func (e *Error) Status() int { return Status(e.code) }

// Error implements error. It is the log's view: everything, including the
// cause. Nothing serializes it into a response — [Translate] builds the
// client's view from the fields it is allowed to send.
func (e *Error) Error() string {
	msg := string(e.code) + ": " + e.detail
	if e.cause != nil {
		msg += ": " + e.cause.Error()
	}
	return msg
}

// Unwrap returns both the code's sentinel and the wrapped cause, so
// errors.Is(err, apierr.ErrNotFound) and errors.Is(err, sql.ErrNoRows) can both
// be true of the same error.
func (e *Error) Unwrap() []error {
	if e.cause == nil {
		return []error{sentinel(e.code)}
	}
	return []error{sentinel(e.code), e.cause}
}

// NotFound reports that a resource does not exist, or that the caller may not
// know it does — PLAN.md §4 wants those two indistinguishable, so there is one
// constructor for both.
//
// resource is the kind ("engagement"), which the client is told; id is the one
// that was looked up, which only the log is told. Echoing an identifier back at
// the caller who just sent it tells them nothing, and doing it uniformly is how
// an identifier from somewhere else ends up in a response.
func NotFound(resource, id string) *Error {
	return &Error{
		code:   gen.ProblemCodeNotFound,
		detail: "no such " + resource,
		cause:  fmt.Errorf("id %q", id),
	}
}

// Forbidden reports that an authenticated caller may not do this.
//
// action describes what was attempted ("close engagement 018f…") and is for the
// log. The client gets a fixed detail: what a caller is not allowed to do is
// frequently a description of something they are not allowed to know about.
func Forbidden(action string) *Error {
	return &Error{
		code:   gen.ProblemCodeForbidden,
		detail: "you are not permitted to do this",
		cause:  fmt.Errorf("attempted %s", action),
	}
}

// MethodNotAllowed reports that the path exists but not with this method.
//
// Normally this comes from the request validator rather than from a handler —
// see [Translate] — but the constructor exists so a router that decides it
// first can produce the same response.
func MethodNotAllowed(method string) *Error {
	return &Error{
		code:   gen.ProblemCodeMethodNotAllowed,
		detail: "the method is not allowed on this path",
		cause:  fmt.Errorf("method %s", method),
	}
}

// Conflict reports that the request contradicts the current state of the
// resource.
//
// detail is shown to the client and should say what the conflict is ("the
// engagement is closed"): a caller who cannot tell what state they are fighting
// with has no way to recover, and this status is not about hiding anything.
func Conflict(detail string) *Error {
	return &Error{code: gen.ProblemCodeConflict, detail: detail}
}

// RateLimited reports that the caller has been throttled. detail is shown to
// the client and should say what the limit is on.
//
// The Retry-After header that goes with it belongs to whoever is doing the
// limiting, not here — see M1-004.
func RateLimited(detail string) *Error {
	return &Error{code: gen.ProblemCodeRateLimited, detail: detail}
}

// Validation reports field-level failures — the domain-level kind ("the end
// date is before the start date"), which the request validator cannot see
// because the request matched the specification.
//
// Spec-level failures produce the same code and shape without going through
// here; see translateSpecError.
func Validation(fields ...FieldError) *Error {
	return &Error{
		code:   gen.ProblemCodeValidationFailed,
		detail: "the request is not valid",
		fields: fields,
	}
}

// Field is a single entry for [Validation]. It exists so that the domain layer
// can describe a bad field without importing the generated package.
//
// field is the path within the request body, dotted with bracketed indices:
// "members[0].role". message says what is wrong with it, in English, and is
// shown to the caller — so it describes the requirement, not the value.
func Field(field, message string) FieldError {
	return FieldError{Field: field, Message: message}
}

// Internal reports a failure the caller can do nothing about. cause is wrapped
// and logged; the client gets [internalDetail] and nothing else.
//
// Returning this explicitly is the same as returning cause on its own —
// [Translate] classifies anything it does not recognise this way. Use it where
// saying "this is not the caller's problem" out loud makes the code clearer.
func Internal(cause error) *Error {
	return &Error{code: gen.ProblemCodeInternal, detail: internalDetail, cause: cause}
}

// internalDetail is the only thing a client is ever told about a 500. It is
// deliberately useless: the useful version is in the log, under the request ID
// the client also has.
const internalDetail = "the server could not complete the request"
