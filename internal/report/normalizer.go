package report

import (
	"regexp"
	"strings"
)

// GoldenNormalizer strips volatile content from rendered HTML so that
// golden-file tests are stable across runs. It replaces:
//
//   - UUIDv7 strings with __UUID__
//   - RFC 3339 timestamps with __TIMESTAMP__
//   - Consecutive whitespace (3+) with a single space
func GoldenNormalizer(html string) string {
	// UUIDv7: 36-char dash-separated hex, 8-4-4-4-12
	uuidRE := regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}`)
	html = uuidRE.ReplaceAllString(html, "__UUID__")

	// RFC 3339 timestamps: 2024-01-15T08:30:00Z or 2024-01-15T08:30:00.000000Z
	tsRE := regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z`)
	html = tsRE.ReplaceAllString(html, "__TIMESTAMP__")

	// Collapse runs of whitespace (3+ spaces/tabs/newlines) into one space
	// for deterministic comparison.
	wsRE := regexp.MustCompile(`[ \t\n]{3,}`)
	html = wsRE.ReplaceAllString(html, " ")

	return strings.TrimSpace(html)
}

// NormalizeAndCompare normalizes actual and expected HTML through
// [GoldenNormalizer] and reports whether they match.
func NormalizeAndCompare(actual, expected string) bool {
	return GoldenNormalizer(actual) == GoldenNormalizer(expected)
}
