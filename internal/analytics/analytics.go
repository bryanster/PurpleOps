package analytics

import "github.com/bryanster/blacklight/internal/store"

// Queries holds the read-side database handle that every rollup runs
// against. It is constructor-injected ([NewQueries]); there is no
// package-level database global (DoD 5).
type Queries struct {
	db store.Store
}

// NewQueries creates an analytics query set backed by the given store.
// The store is used for reads only — analytics never calls
// [store.Store.Write].
func NewQueries(db store.Store) *Queries {
	return &Queries{db: db}
}
