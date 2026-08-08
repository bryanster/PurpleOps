package content

import (
	"fmt"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storecontent "github.com/bryanster/blacklight/internal/store/content"
)

// AssertReferencable refuses a *new* reference to content that is not currently
// installable for use.
//
// Disable semantics (M2-EPIC / M2-002): rows from a disabled source stay on
// disk and remain readable by id for admin/debug, but browse/search/pickers
// omit them and any API that would create a new engagement-side reference must
// call this first. A disabled source answers 409 with a clear problem detail
// rather than 404, so a client can tell "turn it back on" from "it never
// existed".
//
// M3 pickers are the first production callers; the helper is here so the rule
// has one home before those land.
func AssertReferencable(src storecontent.Source) error {
	if src.Enabled {
		return nil
	}
	return apierr.Conflict(fmt.Sprintf(
		"content source %q is disabled; enable it before creating new references",
		src.Name,
	))
}
