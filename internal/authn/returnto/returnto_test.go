package returnto_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/bryanster/blacklight/internal/authn/returnto"
)

// The open-redirect check. It moved here from internal/authn/oidc when SAML
// became its second caller (M1-010), and it keeps its own tests for the reason
// it moved: it is the rule, and it should not be provable only through whichever
// protocol happens to call it today.
//
// Every rejection below is a way a redirect on the login path becomes a credible
// phishing page on your own domain.

func TestASafePathIsAccepted(t *testing.T) {
	t.Parallel()

	for _, safe := range []string{
		"/",
		"/engagements",
		"/engagements/018f9c2e-1234",
		"/engagements?tab=findings",
		"/engagements#summary",
		"/engagements?q=a%20b&sort=name",
	} {
		got, err := returnto.Safe(safe)
		if err != nil {
			t.Errorf("Safe(%q) = %v, want it accepted", safe, err)
			continue
		}
		if got != safe {
			t.Errorf("Safe(%q) = %q; the approved value is what ends up in the Location header, "+
				"so it has to be what was checked", safe, got)
		}
	}
}

func TestAnEmptyPathIsNotAnError(t *testing.T) {
	t.Parallel()

	// A sign-in that named nowhere to go. The caller decides where that lands;
	// it is not a malformed request.
	got, err := returnto.Safe("")
	if err != nil || got != "" {
		t.Errorf(`Safe("") = %q, %v; want "", nil`, got, err)
	}
}

func TestAnUnsafePathIsRefused(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"an absolute URL":               "https://evil.example.com/",
		"an absolute URL to this host":  "https://blacklight.example.com/engagements",
		"a scheme-relative URL":         "//evil.example.com/path",
		"a scheme-relative URL, padded": "//evil.example.com",
		// Browsers normalize a backslash to "/" *after* a naive check has
		// approved the string, which is how "/\evil.example.com" becomes
		// "//evil.example.com".
		"a backslash":              `/\evil.example.com`,
		"a backslash anywhere":     `/engagements\..\..`,
		"a carriage return":        "/engagements\rLocation: https://evil.example.com",
		"a newline":                "/engagements\nSet-Cookie: bl_session=x",
		"a tab":                    "/engagements\ttab",
		"a null byte":              "/engagements\x00",
		"a relative path":          "engagements",
		"a javascript URL":         "javascript:alert(1)",
		"a data URL":               "data:text/html,<script>alert(1)</script>",
		"credentials in a URL":     "//user:password@evil.example.com/",
		"longer than the limit":    "/" + strings.Repeat("a", 512),
		"an empty scheme and host": "http://evil.example.com",
	}
	for name, unsafe := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := returnto.Safe(unsafe)
			if err == nil {
				t.Fatalf("Safe(%q) = %q, want it refused", unsafe, got)
			}
			if !errors.Is(err, returnto.ErrUnsafe) {
				t.Errorf("Safe(%q) failed with %v, want it to wrap ErrUnsafe — callers switch on "+
					"that to answer 400 rather than 500", unsafe, err)
			}
			if got != "" {
				t.Errorf("Safe(%q) returned %q alongside its error; a caller that ignored the "+
					"error would redirect there", unsafe, got)
			}
		})
	}
}

// TestARefusalIsTotalRatherThanAFallback. Substituting "/" would hide somebody
// else's link from whoever is reading the logs, and the one thing that sets this
// value is our own login page.
func TestARefusalIsTotalRatherThanAFallback(t *testing.T) {
	t.Parallel()

	got, err := returnto.Safe("https://evil.example.com/")
	if err == nil {
		t.Fatal("an absolute URL was accepted")
	}
	if got == "/" {
		t.Error(`Safe fell back to "/" instead of refusing; the caller then cannot tell a bad ` +
			"link from a sign-in that named nowhere")
	}
}
