package report

import "github.com/bryanster/blacklight/internal/httpapi/apierr"

// requireSameEngagement returns apierr.NotFound when a report-family resource's
// owning engagement (got) is not the authorized path engagement (want). A
// report or template id from another engagement is indistinguishable from a
// missing id, exactly like a non-member's 404 (M7-012).
func requireSameEngagement(resource, id, got, want string) error {
	if got != want {
		return apierr.NotFound(resource, id)
	}
	return nil
}
