package attackpin

import (
	"errors"
	"fmt"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

// Sentinel errors for pin-sensitive callers. Handlers and M3 map them once via
// [MapError]; do not branch on Error() strings.
var (
	// ErrVersionNotFound means the pin string does not match any
	// content_source_version.version row for the ATT&CK source.
	ErrVersionNotFound = errors.New("attackpin: version not found")

	// ErrNotReferencable means the version (or its source) cannot accept a new
	// pin or engagement-side reference right now — disabled source, not ready,
	// empty catalog, or still referenced on delete.
	ErrNotReferencable = errors.New("attackpin: version not referencable")
)

// MapError translates pin sentinels into the problem shape the HTTP layer
// serves. Unknown errors pass through unchanged.
func MapError(err error) error {
	if err == nil {
		return nil
	}
	var ae *apierr.Error
	if errors.As(err, &ae) {
		return err
	}
	var ve *versionError
	switch {
	case errors.As(err, &ve) && errors.Is(err, ErrVersionNotFound):
		return apierr.NotFound("attack_version", ve.version)
	case errors.As(err, &ve) && errors.Is(err, ErrNotReferencable):
		detail := ve.detail
		if detail == "" {
			detail = fmt.Sprintf("ATT&CK version %q cannot be referenced", ve.version)
		}
		return apierr.Conflict(detail)
	case errors.Is(err, ErrVersionNotFound):
		return apierr.NotFound("attack_version", versionFrom(err))
	case errors.Is(err, ErrNotReferencable):
		return apierr.Conflict(err.Error())
	default:
		return err
	}
}

type versionError struct {
	sentinal error
	version  string
	detail   string
}

func (e *versionError) Error() string {
	if e.detail != "" {
		return e.detail
	}
	if e.version == "" {
		return e.sentinal.Error()
	}
	return fmt.Sprintf("%s: %s", e.sentinal.Error(), e.version)
}

func (e *versionError) Unwrap() error { return e.sentinal }

func notFound(version string) error {
	return &versionError{sentinal: ErrVersionNotFound, version: version}
}

func notReferencable(version, detail string) error {
	return &versionError{
		sentinal: ErrNotReferencable,
		version:  version,
		detail:   detail,
	}
}

func versionFrom(err error) string {
	var ve *versionError
	if errors.As(err, &ve) && ve.version != "" {
		return ve.version
	}
	return ""
}
