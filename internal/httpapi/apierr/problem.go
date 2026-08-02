package apierr

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/bryanster/purpleops/internal/httpapi/gen"
)

// MediaType is the content type of every error this API produces (RFC 9457).
const MediaType = "application/problem+json"

// problemType is the `type` member of every problem document this API produces.
// RFC 9457 reserves about:blank for "the status code is the whole story", which
// is true here because the machine-readable half is `code` — a URI per code
// would be a second identifier for the same thing, and one nobody would host.
const problemType = "about:blank"

// Translate maps any error onto the problem document that describes it. It is
// the only function that decides what a client is told about a failure.
//
// instance is the request ID; it is omitted when empty rather than sent as an
// empty string, so a document that has one always means it.
//
// Anything unrecognised becomes 500 "internal" with a generic detail. See the
// package comment: that rule is the point of this package.
func Translate(err error, instance string) gen.Problem {
	e := classify(err)

	problem := gen.Problem{
		Type:   problemType,
		Status: e.Status(),
		Code:   e.code,
	}
	// RFC 9457: with type about:blank, the title is the status phrase. It is
	// prose for a human — a client that needs to branch reads Code.
	problem.Title = http.StatusText(problem.Status)
	if detail := e.detail; detail != "" {
		problem.Detail = &detail
	}
	if fields := e.fields; len(fields) > 0 {
		problem.Errors = &fields
	}
	if instance != "" {
		problem.Instance = &instance
	}
	return problem
}

// classify finds the [Error] describing err, inventing one where err is not
// something this API models.
//
// errors.As, not a type assertion: a domain error wrapped on its way up the
// stack — fmt.Errorf("load engagement: %w", apierr.NotFound(...)) — is still a
// 404, and staying a 404 through refactoring is why wrapping is allowed at all.
func classify(err error) *Error {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		return apiErr
	}
	if specErr := translateSpecError(err); specErr != nil {
		return specErr
	}
	return Internal(err)
}

// Responder writes problem documents, and is the other half of the rule: every
// error that reaches a client goes through Write, so there is one place that
// decides what is said and one place that logs what is not.
//
// M0B-006 installs it as the error handler for the generated router, for the
// strict handler and for the request validator, so those three cannot disagree.
type Responder struct {
	log *slog.Logger
}

// NewResponder returns a Responder logging to log. A nil log means
// slog.Default(), for a caller that has not built one yet — a test, or popsctl.
func NewResponder(log *slog.Logger) *Responder {
	if log == nil {
		log = slog.Default()
	}
	return &Responder{log: log}
}

// Write translates err, logs it, and sends it as the response.
//
// Its signature is the one the generated server and the request validator both
// want for their error hooks, which is what lets it be the single translation
// point rather than the one people remember to call.
//
// It is the caller's job not to have written a response already; nothing here
// can undo one.
func (rs *Responder) Write(w http.ResponseWriter, r *http.Request, err error) {
	ctx := r.Context()
	// The request ID lives under chi's context key so that this package, the
	// logging middleware and any chi middleware all read the same value. M0B-006
	// supplies it — with its own middleware, since chi's neither echoes the
	// header back nor generates the UUIDv7 this project uses elsewhere.
	instance := middleware.GetReqID(ctx)
	problem := Translate(err, instance)

	// err.Error(), not slog.Any("error", err): a JSONHandler marshals an error
	// value with encoding/json, and the standard error types have no exported
	// fields, so the cause would be logged as {} — exactly the information this
	// line exists to preserve.
	attrs := []any{
		slog.String("code", string(problem.Code)),
		slog.Int("status", problem.Status),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.String("request_id", instance),
		slog.String("error", errorText(err)),
	}
	if problem.Status >= http.StatusInternalServerError {
		// The client was told nothing useful, so this line is the only record
		// of what happened. It carries the request ID the client can quote.
		rs.log.ErrorContext(ctx, "request failed", attrs...)
	} else {
		// The client was told what was wrong and can act on it. Logging every
		// 404 at info level would drown the ones that matter.
		rs.log.DebugContext(ctx, "request rejected", attrs...)
	}

	w.Header().Set("Content-Type", MediaType)
	// Set from the error rather than by whoever is doing the limiting: a 429
	// that does not say when to come back leaves a client to guess, and every
	// 429 this API sends goes through here.
	if retryAfter := classify(err).retryAfter; retryAfter > 0 {
		w.Header().Set("Retry-After", retryAfterSeconds(retryAfter))
	}
	w.WriteHeader(problem.Status)
	if err := json.NewEncoder(w).Encode(problem); err != nil {
		// The status line is already on the wire, so there is no second chance
		// at a response. Record it: a problem document that will not encode is
		// a bug in this package, not a transient write failure.
		rs.log.ErrorContext(ctx, "write problem document",
			slog.String("request_id", instance),
			slog.String("error", err.Error()))
	}
}

// retryAfterSeconds renders a wait as the whole number of seconds RFC 9110
// allows, rounded *up* — a client that comes back at the rounded-down second is
// still locked out, and would spend its retry learning that.
func retryAfterSeconds(d time.Duration) string {
	seconds := int64(d / time.Second)
	if d%time.Second > 0 {
		seconds++
	}
	if seconds < 1 {
		seconds = 1
	}
	return strconv.FormatInt(seconds, 10)
}

// errorText renders err for the log, tolerating the nil that a mistaken caller
// might pass rather than panicking inside error handling.
func errorText(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}
