package apierr

import "testing"

func TestRedactPathRedactsShareTokens(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"non-share path is untouched", "/api/v1/version", "/api/v1/version"},
		{"share info", "/api/v1/report-views/0123abcd", "/api/v1/report-views/[redacted]"},
		{"share claim", "/api/v1/report-views/0123abcd/claim", "/api/v1/report-views/[redacted]/claim"},
		{"share password", "/api/v1/report-views/0123abcd/password", "/api/v1/report-views/[redacted]/password"},
		{"share html", "/api/v1/report-views/0123abcd/html", "/api/v1/report-views/[redacted]/html"},
		{"share pdf", "/api/v1/report-views/0123abcd/pdf", "/api/v1/report-views/[redacted]/pdf"},
		{"trailing slash", "/api/v1/report-views/0123abcd/", "/api/v1/report-views/[redacted]/"},
		{"bare prefix untouched", "/report-views/", "/report-views/"},
		{"empty token untouched", "/report-views//claim", "/report-views//claim"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := RedactPath(tt.in); got != tt.want {
				t.Errorf("RedactPath(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
