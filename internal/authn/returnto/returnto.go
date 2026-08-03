// Package returnto decides whether a caller-supplied path is one this
// application may redirect a browser to after signing in.
//
// It is its own package because two sign-on protocols need exactly the same
// answer. It arrived with OIDC (M1-009) and moved here when SAML (M1-010)
// became its second caller: an open-redirect check that exists twice is one
// that gets fixed once.
//
// The rule is an allowlist of a single shape — a relative path within this
// application — and it is applied twice on every sign-in: once when the value
// arrives, and again when it comes back out of the sealed cookie it travelled
// in. A check that only runs on the way in is a check that trusts what came
// back.
package returnto

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// maxBytes caps the path a caller may ask to be sent back to. It is a path
// within this application; anything longer is somebody exploring what the
// cookie will hold.
const maxBytes = 512

// ErrUnsafe reports a return path that is not a path within this application.
var ErrUnsafe = errors.New("returnto: the return path is not a path within this application")

// Safe returns the path a completed sign-in should land on, or [ErrUnsafe].
//
// Everything that is not a relative path is refused — an absolute URL, a
// scheme-relative "//evil.example", a backslash (which browsers normalize to
// "/" *after* a naive check has approved it), and a control character (which is
// how a header gets split).
//
// The refusal is total rather than a fallback to "/" because the caller is
// asking for something specific and getting it wrong should be visible. The one
// place that sets this is our own login page, and a value it did not produce is
// somebody else's link — on the login path, which is where an open redirect
// becomes a credible phishing page on your own domain.
func Safe(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if len(raw) > maxBytes {
		return "", fmt.Errorf("%w: it is %d bytes, and the limit is %d", ErrUnsafe, len(raw), maxBytes)
	}
	if strings.ContainsAny(raw, "\\\x00\r\n\t") {
		return "", fmt.Errorf("%w: it contains a backslash or a control character", ErrUnsafe)
	}
	if !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		// "//evil.example/path" is a URL to another origin that reads as a path.
		return "", fmt.Errorf("%w: it must start with a single %q", ErrUnsafe, "/")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrUnsafe, err)
	}
	if parsed.Scheme != "" || parsed.Host != "" || parsed.User != nil {
		return "", fmt.Errorf("%w: it names a host or a scheme", ErrUnsafe)
	}
	// Re-rendered from the parsed form rather than passed through, so what ends
	// up in the Location header is what was actually parsed and approved.
	return parsed.String(), nil
}
