package apierr

import (
	"errors"
	"fmt"
	"time"

	"github.com/bryanster/blacklight/internal/httpapi/gen"
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
	// retryAfter is how long the caller must wait before trying again. Set only
	// by [RateLimited]; the [Responder] turns it into the header.
	retryAfter time.Duration
}

// Code returns the stable machine-readable identifier for this failure.
func (e *Error) Code() Code { return e.code }

// Status returns the HTTP status this failure is reported with.
func (e *Error) Status() int { return Status(e.code) }

// Fields returns the field-level breakdown of a validation failure, and nothing
// for every other code.
//
// It exists for the callers that are not HTTP responses: blctl reports a
// password that breaks the policy as a sentence rather than as a problem
// document, and it should be the same sentence the API would have sent.
func (e *Error) Fields() []FieldError { return e.fields }

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

// Unauthenticated reports that the request carried no usable session: none at
// all, or one that has expired, timed out or been revoked.
//
// reason says which, for the log only. The client is told the same sentence
// whichever it was — a caller with no session cannot act differently on the
// difference, and an attacker probing with a stolen cookie can.
func Unauthenticated(reason string) *Error {
	return &Error{
		code:   gen.ProblemCodeUnauthenticated,
		detail: "sign in to continue",
		cause:  errors.New(reason),
	}
}

// BadCredentials is the answer to every failed sign-in, and the reason it is
// its own constructor: the response cannot vary with what was wrong, because
// this constructor gives the caller no way to make it vary. A wrong password, an
// address nobody holds and a disabled account produce byte-identical bodies
// (M1-003).
//
// reason is the log's half — "no such email", "password mismatch", "account
// disabled" — and is the only record of which it was.
func BadCredentials(reason string) *Error {
	return &Error{
		code:   gen.ProblemCodeUnauthenticated,
		detail: "the email address or password is incorrect",
		cause:  errors.New(reason),
	}
}

// BadSecondFactor is the answer to every failed second-factor verification, and
// exists for the same reason [BadCredentials] does: the caller is given no way
// to make the response vary. A wrong code, a code already spent, an expired
// pending state and no pending state at all produce byte-identical bodies
// (M1-006), so nothing about the answer says how close a guess was.
//
// reason is the log's half, and is the only record of which it was.
func BadSecondFactor(reason string) *Error {
	return &Error{
		code:   gen.ProblemCodeUnauthenticated,
		detail: "the code is incorrect or the sign-in has expired; sign in again",
		cause:  errors.New(reason),
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

// MFAEnrolmentRequired reports that the caller holds a session which may do
// exactly one thing: enrol a second factor (M1-008).
//
// It refines [Forbidden] rather than reusing it because the two are different
// instructions. "You may not do this" is final; this one says the caller is one
// enrolment away from being able to, and names the endpoints — so an interface
// can put a person in front of a QR code instead of an apology.
//
// detail is shown to the client and says what to do, which is safe here in a way
// it is not for [Forbidden]: nothing about the requirement is a secret, and the
// caller is being told about their own account.
//
// action is the log's half: what they were trying to reach when they were
// stopped.
func MFAEnrolmentRequired(action string) *Error {
	return &Error{
		code: gen.ProblemCodeMfaEnrolmentRequired,
		detail: "a second factor is required of your account; " +
			"enrol an authenticator to continue",
		cause: fmt.Errorf("attempted %s", action),
	}
}

// SignInRefused reports that somebody proved who they are and still may not in:
// an identity provider vouched for them and this deployment has no account for
// them, or the one it has is disabled (M1-009).
//
// It is a [Forbidden] with a detail the caller is actually told, which is the
// difference between it and that constructor. Forbidden hides its reason because
// what a caller may not do is often a description of something they may not know
// about; here there is nothing to hide — the person has authenticated, the
// subject is their own account, and "ask an administrator to create one" is the
// only thing that gets them any further. A fixed sentence instead would send
// somebody who signed in perfectly well to a support queue.
//
// detail is that sentence. reason is the log's half: which subject, at which
// provider, and why.
func SignInRefused(detail, reason string) *Error {
	return &Error{
		code:   gen.ProblemCodeForbidden,
		detail: detail,
		cause:  errors.New(reason),
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

// PayloadTooLarge reports that the request body exceeds the configured size
// limit. detail is shown to the client and should name the limit ("file exceeds
// 25 MiB upload limit").
func PayloadTooLarge(detail string) *Error {
	return &Error{code: gen.ProblemCodePayloadTooLarge, detail: detail}
}

// RateLimited reports that the caller has been throttled. detail is shown to
// the client and should say what the limit is on — and, where the limit is on a
// credential, it should say nothing else: the sign-in throttle answers a real
// account and an invented one identically (M1-004).
//
// retryAfter travels with the error rather than being set by the caller, so that
// a 429 cannot be sent without the header that says when to come back. The
// [Responder] rounds it up to whole seconds, which is all RFC 9110 allows.
func RateLimited(detail string, retryAfter time.Duration) *Error {
	return &Error{
		code:       gen.ProblemCodeRateLimited,
		detail:     detail,
		retryAfter: retryAfter,
	}
}

// RetryAfter returns how long the caller must wait, or zero for every failure
// that is not a rate limit.
func (e *Error) RetryAfter() time.Duration { return e.retryAfter }

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
