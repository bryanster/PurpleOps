package apierr

import (
	"errors"
	"regexp"
	"strings"
	"unicode"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"

	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

// The request validator (M0B-006) rejects anything api/openapi.yaml does not
// describe, before a handler is entered. What it returns is kin-openapi's own
// error types, and this file is where they become the same problem documents as
// everything else — otherwise the validator would be a second error model
// sitting in front of the first, which is the thing M0B-007 exists to prevent.

// specValidationDetail is what a client is told when its request failed the
// specification and the failure had no field-level breakdown to report.
const specValidationDetail = "the request does not match the API specification"

// translateSpecError recognises the errors the OpenAPI request validator
// produces, and returns nil for anything else so that [classify] can carry on.
func translateSpecError(err error) *Error {
	if err == nil {
		return nil
	}

	// Routing failures come first: they are sentinel comparisons, and the
	// request never reached the stage that produces the richer errors below.
	switch {
	case errors.Is(err, routers.ErrPathNotFound):
		return &Error{
			code: gen.ProblemCodeNotFound,
			// "endpoint", not "resource": the path is not in the specification
			// at all, which is a different problem from an id that is not in the
			// database, and a client debugging a typo needs to tell them apart.
			detail: "no such endpoint",
			cause:  err,
		}
	case errors.Is(err, routers.ErrMethodNotAllowed):
		return &Error{
			code:   gen.ProblemCodeMethodNotAllowed,
			detail: "the method is not allowed on this path",
			cause:  err,
		}
	}

	// A security requirement that no credential satisfied. Until M1 there are no
	// authenticated operations, so in practice this means the validator was
	// handed an operation with a security requirement and no AuthenticationFunc
	// — see docs/api.md. Reporting it as 403 keeps that mistake visible without
	// inventing a code the spec does not have; M1-003 revisits it when there is
	// an authenticated status to distinguish.
	var securityErr *openapi3filter.SecurityRequirementsError
	if errors.As(err, &securityErr) {
		return &Error{
			code:   gen.ProblemCodeForbidden,
			detail: "you are not permitted to do this",
			cause:  err,
		}
	}

	var requestErr *openapi3filter.RequestError
	if errors.As(err, &requestErr) {
		fields, reason := requestFailures(err)
		if len(fields) == 0 {
			// err is a RequestError that requestFailures could not walk from the
			// top — something wrapped it. The one it found still describes the
			// failure.
			fields = fieldErrors(requestErr)
		}
		if reason == "" {
			reason = requestErr.Reason
		}
		detail := specValidationDetail
		// With no field-level breakdown — a malformed body, an unsupported
		// content type — the generic detail would leave the caller with nothing
		// to act on, so the validator's own reason stands in. It describes the
		// specification, not the value that failed it.
		if len(fields) == 0 && reason != "" {
			detail = reason
		}
		return &Error{
			code:   gen.ProblemCodeValidationFailed,
			detail: detail,
			fields: fields,
			cause:  err,
		}
	}

	return nil
}

// requestFailures walks the error tree the validator built and returns every
// field failure in it, along with the first reason it saw.
//
// The two functions below switch on the concrete type instead of using
// errors.As, and that is the point rather than an oversight: the shape of the
// tree is the information. An openapi3.MultiError at the top is a list of
// failed *requirements* — one per parameter, plus the body — while the same
// type inside a RequestError is a list of failed *fields*, and a SchemaError's
// wrapped cause is the branch-by-branch detail of a oneOf that a caller is
// better off not seeing. errors.As unwraps straight through all three
// distinctions and reports whatever it reaches first.
//
//nolint:errorlint // deliberate: see above.
func requestFailures(err error) ([]FieldError, string) {
	switch e := err.(type) {
	case openapi3.MultiError:
		var fields []FieldError
		var reason string
		for _, member := range e {
			memberFields, memberReason := requestFailures(member)
			fields = append(fields, memberFields...)
			if reason == "" {
				reason = memberReason
			}
		}
		return fields, reason
	case *openapi3filter.RequestError:
		return fieldErrors(e), e.Reason
	default:
		return nil, ""
	}
}

// fieldErrors flattens a validation failure into the errors[] array of the
// problem document: one entry per field the request got wrong, rather than one
// string containing all of them, so a form can put each message next to its
// input.
func fieldErrors(requestErr *openapi3filter.RequestError) []FieldError {
	// A parameter failure carries no path of its own — the parameter name is
	// the whole path. A body failure has a JSON pointer and no prefix.
	prefix := ""
	if requestErr.Parameter != nil {
		prefix = requestErr.Parameter.Name
	}

	var fields []FieldError
	collectFieldErrors(requestErr.Err, prefix, &fields)
	return fields
}

// collectFieldErrors appends one [FieldError] per schema failure it can find.
// Anything else is left to the caller's generic detail: an error with no field
// attached has nothing to say in a per-field list.
//
//nolint:errorlint // deliberate: see the comment above requestFailures.
func collectFieldErrors(err error, prefix string, fields *[]FieldError) {
	switch e := err.(type) {
	case openapi3.MultiError:
		for _, member := range e {
			collectFieldErrors(member, prefix, fields)
		}
	case *openapi3.SchemaError:
		// A schema error that carries a collection is a summary of the errors
		// in it — reporting the summary as one field failure would collapse
		// every field the request got wrong into a single entry with no field
		// name. See schemaFieldError for where that shape comes from.
		if nested := nestedErrors(e); nested != nil {
			for _, member := range nested {
				collectFieldErrors(member, prefix, fields)
			}
			return
		}
		*fields = append(*fields, schemaFieldError(e, prefix))
	}
}

// nestedErrors returns the collection a schema error summarises, or nil if it
// stands alone.
func nestedErrors(schemaErr *openapi3.SchemaError) openapi3.MultiError {
	if schemaErr.Origin == nil {
		return nil
	}
	var multi openapi3.MultiError
	if errors.As(schemaErr.Origin, &multi) {
		return multi
	}
	return nil
}

// The OpenAPI 3.1 path — the one api/openapi.yaml takes — validates against
// JSON Schema 2020-12 with santhosh-tekuri/jsonschema, and kin-openapi flattens
// that library's result back into its own SchemaError: the structured instance
// location is dropped and JSONPointer() comes back empty. What survives is a
// Reason of the form
//
//	error at "/members/0/role": at '/members/0/role': value must be one of …
//
// The patterns below recover the path from that sentence. They are pinned by
// the tests in validation_test.go, which drive the real validator rather than a
// remembered fixture: an upgrade that rewords this fails there, loudly, instead
// of quietly serving errors[] entries with no field in them.
var (
	instanceLocationPattern = regexp.MustCompile(`(?s)^(?:error at "[^"]*": )?at '([^']*)': (.*)$`)
	missingPropertyPattern  = regexp.MustCompile(`^missing property '([^']+)'$`)
)

// schemaFieldError renders one schema failure as the client sees it.
//
// It reads only Reason, which kin-openapi documents as never containing the
// offending value. SchemaError.Error() does embed the value in some cases, and
// echoing a request's own contents back into a response body is how a
// validation message becomes a way to reflect content at a browser.
func schemaFieldError(schemaErr *openapi3.SchemaError, prefix string) FieldError {
	pointer := schemaErr.JSONPointer()
	message := schemaErr.Reason

	if match := instanceLocationPattern.FindStringSubmatch(schemaErr.Reason); match != nil && len(pointer) == 0 {
		pointer = splitInstanceLocation(match[1])
		message = match[2]
	}
	// "missing property 'members'" is about a field the path does not reach,
	// because the value that is missing has no location of its own. Naming it is
	// the difference between a form highlighting the empty input and a form
	// showing a sentence at the top.
	if match := missingPropertyPattern.FindStringSubmatch(message); match != nil {
		pointer = append(pointer, match[1])
		message = "is required"
	}

	switch {
	case message != "":
	case schemaErr.SchemaField != "":
		message = "does not satisfy " + schemaErr.SchemaField
	default:
		message = "is not valid"
	}

	// An empty path is not a missing one: it means the body as a whole is wrong
	// ("got array, want object"), which is a thing a client needs to be told.
	return FieldError{Field: fieldPath(prefix, pointer), Message: message}
}

// splitInstanceLocation turns a JSON Schema instance location — /members/0/role
// — into pointer segments. Segments are used verbatim: RFC 6901 escaping (~0,
// ~1) only matters for a property name containing a slash or a tilde, which no
// schema in this API has.
func splitInstanceLocation(location string) []string {
	location = strings.TrimPrefix(location, "/")
	if location == "" {
		return nil
	}
	return strings.Split(location, "/")
}

// fieldPath renders a JSON pointer the way the spec's FieldError example does —
// members[0].role — because that is what a client has to match against its own
// form fields.
func fieldPath(prefix string, pointer []string) string {
	var b strings.Builder
	b.WriteString(prefix)
	for _, segment := range pointer {
		if isIndex(segment) {
			b.WriteByte('[')
			b.WriteString(segment)
			b.WriteByte(']')
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(segment)
	}
	return b.String()
}

// isIndex reports whether a pointer segment is an array index rather than a
// property name. A property whose name is all digits is legal JSON and would be
// rendered as an index; that is a wrong-looking path for an object nobody has,
// and not worth carrying the schema around to rule out.
func isIndex(segment string) bool {
	if segment == "" {
		return false
	}
	for _, r := range segment {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
