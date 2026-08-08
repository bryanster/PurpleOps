package analytics

import "github.com/bryanster/blacklight/internal/store/blind"

// Scope is the engagement + seat view that every rollup reads through.
//
// A rollup that forgot the seat would compile while leaking a blind
// engagement's totals to blue. Making [Scope] the one parameter every
// exported rollup takes is how the package stays safe by construction —
// a new rollup cannot be written without choosing a scope, and every
// scope carries blind mode.
type Scope struct {
	// EngagementID is the engagement being aggregated.
	EngagementID string

	// Blind is the reader's blind-mode scope for this engagement.
	Blind blind.Scope
}

// stepPredicate returns the SQL fragment that hides unrevealed steps
// from a blue reader in a blind engagement. It calls [blind.Scope.Where]
// with the column expression that says whether a step row is visible.
//
// The column argument to Where is a constant expression owned by this
// package — NEVER a value from a request. blind.Scope.Where concatenates
// it into SQL; passing caller-supplied text would be an injection bug
// no signature can prevent.
func (s Scope) stepPredicate() string {
	return s.Blind.Where("revealed_at IS NOT NULL")
}
