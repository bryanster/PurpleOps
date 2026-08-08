package attackpin_test

import (
	"errors"
	"testing"

	"github.com/bryanster/blacklight/internal/content/attackpin"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

func TestNormalizeVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "plain release", in: "15.1", want: "15.1"},
		{name: "trim spaces", in: "  14.1  ", want: "14.1"},
		{name: "leading v kept distinct", in: "v15.1", want: "v15.1"},
		{name: "leading V kept distinct", in: "V15.1", want: "V15.1"},
		{name: "empty", in: "", wantErr: true},
		{name: "whitespace only", in: "   ", wantErr: true},
		{name: "internal space", in: "15. 1", wantErr: true},
		{name: "staging reserved", in: "__staging__", wantErr: true},
		{name: "current reserved", in: "current", wantErr: true},
		{name: "tab inside", in: "15.1\t", want: "15.1"}, // trailing tab trimmed
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := attackpin.NormalizeVersion(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("NormalizeVersion(%q) = %q, want error", tc.in, got)
				}
				if !errors.Is(err, apierr.ErrValidation) {
					t.Fatalf("error = %v, want validation", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("NormalizeVersion(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeVersion(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestLeadingVIsNotFifteenPointOne(t *testing.T) {
	t.Parallel()
	a, err := attackpin.NormalizeVersion("15.1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := attackpin.NormalizeVersion("v15.1")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatalf("15.1 and v15.1 must remain distinct after normalize; both became %q", a)
	}
}
