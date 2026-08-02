package apierr

// This file is in the package rather than beside it because the thing under
// test is the table itself: that it covers the spec's enum exactly, and that no
// two codes claim the same status. Read from outside, a missing entry and the
// deliberate fallback in Status look the same.

import (
	"errors"
	"net/http"
	"testing"

	"github.com/bryanster/blacklight/api"
)

// specCodes returns the ProblemCode enum as api/openapi.yaml declares it. The
// spec is the source of truth for the set of codes; this package is the source
// of truth for what each one means over HTTP.
func specCodes(t *testing.T) []Code {
	t.Helper()

	doc, err := api.Load()
	if err != nil {
		t.Fatalf("load the embedded spec: %v", err)
	}
	schema := doc.Components.Schemas["ProblemCode"]
	if schema == nil || schema.Value == nil {
		t.Fatal("components.schemas.ProblemCode is missing from the spec; it is the client's half of this table")
	}
	if len(schema.Value.Enum) == 0 {
		t.Fatal("ProblemCode declares no enum; without one a client cannot know what it might have to handle")
	}

	list := make([]Code, 0, len(schema.Value.Enum))
	for _, value := range schema.Value.Enum {
		name, ok := value.(string)
		if !ok {
			t.Fatalf("ProblemCode enum contains %v (%T), want a string", value, value)
		}
		list = append(list, Code(name))
	}
	return list
}

func TestEveryCodeInTheSpecHasAStatus(t *testing.T) {
	t.Parallel()

	for _, code := range specCodes(t) {
		entry, ok := codes[code]
		if !ok {
			t.Errorf("the spec declares the code %q and this package has no entry for it; it would be reported as a 500 with the wrong code", code)
			continue
		}
		if entry.status < 400 || entry.status > 599 {
			t.Errorf("code %q maps to status %d, want an error status", code, entry.status)
		}
		if entry.sentinel == nil {
			t.Errorf("code %q has no sentinel; errors.Is(err, Err...) would never be true of it", code)
		}
	}
}

func TestEveryCodeInTheTableIsInTheSpec(t *testing.T) {
	t.Parallel()

	declared := map[Code]bool{}
	for _, code := range specCodes(t) {
		declared[code] = true
	}
	for code := range codes {
		if !declared[code] {
			t.Errorf("this package can produce the code %q, which the spec does not declare; a generated client has no type for it", code)
		}
	}
}

// TestEachStatusBelongsToOneCode is the 1:1 half of the pairing that
// api/openapi.yaml and docs/api.md both promise. Two codes sharing a status
// would leave a client unable to tell them apart without parsing prose.
func TestEachStatusBelongsToOneCode(t *testing.T) {
	t.Parallel()

	byStatus := map[int]Code{}
	for _, code := range specCodes(t) {
		status := Status(code)
		if first, taken := byStatus[status]; taken {
			t.Errorf("%q and %q both report %d (%s); a client cannot distinguish them from the status line",
				first, code, status, http.StatusText(status))
		}
		byStatus[status] = code
	}
}

func TestEachCodeHasItsOwnSentinel(t *testing.T) {
	t.Parallel()

	bySentinel := map[error]Code{}
	for _, code := range specCodes(t) {
		s := sentinel(code)
		if first, taken := bySentinel[s]; taken {
			t.Errorf("%q and %q share the sentinel %v; errors.Is could not tell them apart", first, code, s)
		}
		bySentinel[s] = code
	}
}

// TestAnUnknownCodeReportsInternal pins the deliberate fallback. It can only be
// reached by a bug — the tests above are what stop the spec and the table
// diverging — and the failure mode of a bug here should be a 500 nobody
// believes, not a 200 the client does.
func TestAnUnknownCodeReportsInternal(t *testing.T) {
	t.Parallel()

	if got, want := Status(Code("not_a_code")), http.StatusInternalServerError; got != want {
		t.Errorf("Status(unknown) = %d, want %d", got, want)
	}
	// errors.Is, not !=, because that is how a caller asks the question — and
	// the sentinel is returned bare here, so the two agree either way.
	if got := sentinel(Code("not_a_code")); !errors.Is(got, ErrInternal) {
		t.Errorf("sentinel(unknown) = %v, want %v", got, ErrInternal)
	}
}
