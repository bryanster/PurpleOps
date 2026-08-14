package apierr

import "strings"

// shareViewsPrefix is the path segment under which a report share token
// travels. A share token is a credential exactly as much as a password is: it
// is shown once at creation and is the whole key to a published report. Every
// log line that records r.URL.Path for one of these routes without redaction
// publishes it forever.
const shareViewsPrefix = "/report-views/"

// RedactPath returns path with the report-share token segment, if any, replaced
// by "[redacted]". A share token lives in the URL path
// (/report-views/{token}…), unlike the query-string secrets the access logger
// already avoids, so the path itself is the thing to scrub.
//
// Every log site that records a request path — the access logger, authorization
// refusals, problem documents, the throttler, the CSRF and MFA gates, and the
// panic recoverer — must pass the path through here. It is one helper precisely
// so that a new log site has no second definition to remember.
func RedactPath(path string) string {
	i := strings.Index(path, shareViewsPrefix)
	if i < 0 {
		return path
	}
	tail := path[i+len(shareViewsPrefix):]
	if tail == "" {
		// The prefix sits at the end of the path ("…/report-views/"): there is
		// no token segment to redact.
		return path
	}
	end := strings.IndexByte(tail, '/')
	if end == 0 {
		// An empty segment ("…/report-views//claim"): nothing but a slash.
		return path
	}
	if end < 0 {
		end = len(tail)
	}
	return path[:i+len(shareViewsPrefix)] + "[redacted]" + tail[end:]
}
