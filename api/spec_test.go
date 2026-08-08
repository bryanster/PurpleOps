package api

import (
	"bytes"
	"strings"
	"testing"
)

// TestLoadAcceptsTheEmbeddedSpec is the gate everything else stands on. If
// openapi.yaml stops being a valid OpenAPI document then the generated server,
// the generated client and the runtime request validator are all describing
// something that does not exist — and this is the cheapest place to find out.
func TestLoadAcceptsTheEmbeddedSpec(t *testing.T) {
	doc, err := Load()
	if err != nil {
		t.Fatalf("the embedded spec is not a valid OpenAPI document: %v", err)
	}

	if !doc.IsOpenAPI31OrLater() {
		t.Errorf("openapi = %q, want 3.1 or later; the conventions in this package (type arrays for null, SPDX licence identifiers) are 3.1 spellings", doc.OpenAPI)
	}
	if doc.Paths == nil || doc.Paths.Len() == 0 {
		t.Fatal("the spec declares no paths, so nothing can be generated from it")
	}
}

// TestLoadRejectsABrokenSpec is the acceptance criterion "verify by temporarily
// breaking the spec", made permanent. Each case breaks the *real* document —
// not a toy fixture — in a way a careless edit plausibly would, and asserts that
// Load says so instead of returning a half-understood document.
//
// A mutation that no longer applies fails the test rather than silently passing:
// a rule that stops being exercised is worse than one that was never written.
func TestLoadRejectsABrokenSpec(t *testing.T) {
	cases := map[string]struct {
		break_ func(spec string) string
		want   string // substring of the error, so the failure names the cause
	}{
		"a reference to a schema that does not exist": {
			break_: func(spec string) string {
				return strings.Replace(spec, `"#/components/schemas/Problem"`, `"#/components/schemas/Problm"`, 1)
			},
			want: "Problm",
		},
		"a response with no description": {
			// The indentation is part of the match: dropping the key but leaving
			// its leading spaces produces a YAML error instead of the OpenAPI
			// error this case is about.
			break_: func(spec string) string {
				return strings.Replace(spec, "          description: Every dependency is reachable.\n", "", 1)
			},
			want: "description",
		},
		"a property whose type is a typo": {
			break_: func(spec string) string {
				return strings.Replace(spec, "type: integer", "type: interger", 1)
			},
			want: "interger",
		},
		"a document that is not YAML at all": {
			break_: func(spec string) string {
				return spec + "\n\tthis: [is not, yaml\n"
			},
			want: "parse openapi document",
		},
	}

	original := string(specYAML)

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			broken := tc.break_(original)
			if broken == original {
				t.Fatal("this mutation no longer changes the spec — the wording it targets has moved, so the rule is untested until the mutation is updated")
			}

			_, err := load([]byte(broken))
			if err == nil {
				t.Fatal("load() accepted a broken spec")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("load() error = %q, want it to mention %q so the reader knows what to fix", err, tc.want)
			}
		})
	}
}

// TestSpecReturnsACopy guards the one piece of shared mutable state in this
// package. The embedded bytes back every future Load; a caller that serves them,
// or hands them to a parser that scribbles on its input, must not be able to
// corrupt the spec for the rest of the process.
func TestSpecReturnsACopy(t *testing.T) {
	first := Spec()
	if len(first) == 0 {
		t.Fatal("Spec() is empty")
	}

	first[0] = 'x'

	if second := Spec(); bytes.Equal(first, second) {
		t.Error("Spec() handed out the embedded slice itself; a caller can corrupt the spec for every later Load")
	}
	if _, err := Load(); err != nil {
		t.Fatalf("mutating a Spec() result broke Load: %v", err)
	}
}
