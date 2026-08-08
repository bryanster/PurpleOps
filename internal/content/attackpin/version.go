package attackpin

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// NormalizeVersion is the single ATT&CK version-string rule for pin and catalog
// APIs.
//
// Rules (applied in order):
//  1. Trim surrounding ASCII whitespace.
//  2. Reject empty.
//  3. Reject any remaining whitespace (labels are single tokens).
//  4. Reject reserved tokens (__staging__, current) — those are internal, not
//     MITRE release labels.
//
// There is deliberately no semver coercion and no stripping of a leading "v".
// "15.1" and "v15.1" are different strings; only the former matches an
// installed MITRE release label. Normalize once at this boundary (and at
// ingest via TrimSpace on the collection label) so handlers never invent a
// second spelling.
func NormalizeVersion(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", apierr.Validation(apierr.Field(
			"version", "version must not be empty",
		))
	}
	for _, r := range v {
		if unicode.IsSpace(r) {
			return "", apierr.Validation(apierr.Field(
				"version", "version must not contain whitespace",
			))
		}
	}
	switch v {
	case storecontent.StagingVersion:
		return "", apierr.Validation(apierr.Field(
			"version", fmt.Sprintf("%q is a reserved internal token, not an ATT&CK release label", v),
		))
	case storecontent.VersionCurrent:
		return "", apierr.Validation(apierr.Field(
			"version", fmt.Sprintf("%q is the rolling-source token; ATT&CK pins use release labels like \"15.1\"", v),
		))
	}
	return v, nil
}
