package apierr

import (
	"errors"
	"net/http"

	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

// Code and FieldError are aliases rather than new types: the generated
// definitions come from api/openapi.yaml, so a value built here is exactly the
// value the client is documented to receive, with no conversion in between to
// get wrong. The alias is what lets the domain layer name them without
// importing the generated transport package.
type (
	Code       = gen.ProblemCode
	FieldError = gen.FieldError
)

// The sentinels. They exist so that a caller who only wants to know *what kind*
// of failure this was can ask errors.Is(err, apierr.ErrNotFound) without a type
// assertion, and so that a test can say which failure it expects by name.
//
// They are never returned on their own — every one of them arrives as part of
// an [Error], which also carries the detail and the field list.
var (
	ErrValidation = errors.New("validation failed")
	// ErrUnauthenticated covers both halves of 401: a request that carried no
	// usable session, and a sign-in attempt that failed. They are one code
	// because they are one instruction to the client — sign in — and because
	// telling them apart is exactly what a login endpoint must not help with.
	ErrUnauthenticated  = errors.New("unauthenticated")
	ErrForbidden        = errors.New("forbidden")
	ErrNotFound         = errors.New("not found")
	ErrMethodNotAllowed = errors.New("method not allowed")
	ErrConflict         = errors.New("conflict")
	ErrRateLimited      = errors.New("rate limited")
	ErrInternal         = errors.New("internal error")
)

// codes is the whole code→(status, sentinel) table, and the reason it is one
// table rather than two maps: half an entry is not a thing anyone can add.
//
// The pairing is 1:1 in both directions, and TestEveryStatusIsUsedByOneCode
// keeps it that way. Two codes sharing a status would leave a client unable to
// tell them apart from the status line alone, which is the whole point of
// having a code.
var codes = map[Code]struct {
	status   int
	sentinel error
}{
	gen.ProblemCodeValidationFailed: {http.StatusBadRequest, ErrValidation},
	gen.ProblemCodeUnauthenticated:  {http.StatusUnauthorized, ErrUnauthenticated},
	gen.ProblemCodeForbidden:        {http.StatusForbidden, ErrForbidden},
	gen.ProblemCodeNotFound:         {http.StatusNotFound, ErrNotFound},
	gen.ProblemCodeMethodNotAllowed: {http.StatusMethodNotAllowed, ErrMethodNotAllowed},
	gen.ProblemCodeConflict:         {http.StatusConflict, ErrConflict},
	gen.ProblemCodeRateLimited:      {http.StatusTooManyRequests, ErrRateLimited},
	gen.ProblemCodeInternal:         {http.StatusInternalServerError, ErrInternal},
}

// Status returns the HTTP status a code is reported with.
//
// A code with no entry in the table is a programming error — the spec and the
// table disagree, which the tests are there to prevent — and it reports 500
// rather than 200, so the failure mode of a mistake here is a loud error and
// not a success the client will believe.
func Status(code Code) int {
	if c, ok := codes[code]; ok {
		return c.status
	}
	return http.StatusInternalServerError
}

// sentinel returns the errors.Is target for a code, or [ErrInternal] for a code
// the table does not know, matching [Status].
func sentinel(code Code) error {
	if c, ok := codes[code]; ok {
		return c.sentinel
	}
	return ErrInternal
}
