package engagement

import (
	"context"

	"github.com/bryanster/blacklight/internal/content/attackpin"
)

// References implements [attackpin.References] by counting engagements that pin
// a given ATT&CK version. DeleteVersion refuses to cascade when the count is
// non-zero, so a version that any engagement references cannot be removed.
type References struct {
	engagements *Engagements
}

// NewReferences returns a References backed by the engagement store.
func NewReferences(engagements *Engagements) *References {
	return &References{engagements: engagements}
}

// AttackVersion returns how many engagements currently pin version.
func (r *References) AttackVersion(ctx context.Context, version string) (int64, error) {
	return r.engagements.CountByAttackVersion(ctx, version)
}

// Compile-time check.
var _ attackpin.References = (*References)(nil)
