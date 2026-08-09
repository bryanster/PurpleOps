package report

import (
	"fmt"
	"sort"
	"sync"
)

// Registry holds every known block Definition and Renderer, keyed by ID.
// Register blocks during init — the registry is safe for concurrent reads
// after init but writing after init is a programmer error that panics.
type Registry struct {
	mu        sync.RWMutex
	items     map[ID]Definition
	renderers map[ID]Renderer
}

// NewRegistry returns an empty Registry ready for registration.
func NewRegistry() *Registry {
	return &Registry{
		items:     make(map[ID]Definition),
		renderers: make(map[ID]Renderer),
	}
}

// Register adds def to the registry. It panics on a duplicate ID — duplicate
// registration is a programmer error that must be caught at init time.
//
// Register is safe for concurrent use with other Register calls during init
// (it takes a write lock), but callers should finish registration before
// serving reads.
func (r *Registry) Register(def Definition) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[def.ID]; exists {
		panic(fmt.Sprintf("report: duplicate block id %q", def.ID))
	}
	r.items[def.ID] = def
}

// Get returns the Definition for id, or false when it is not registered.
func (r *Registry) Get(id ID) (Definition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.items[id]
	return def, ok
}

// MustGet returns the Definition for id, panicking when it is not registered.
func (r *Registry) MustGet(id ID) Definition {
	def, ok := r.Get(id)
	if !ok {
		panic(fmt.Sprintf("report: block id %q not registered", id))
	}
	return def
}

// Renderer returns the renderer for id, or false when none is set.
func (r *Registry) Renderer(id ID) (Renderer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rend, ok := r.renderers[id]
	return rend, ok
}

// SetRenderer registers a renderer for an existing block id.
// It panics if the block is not already registered via Register.
func (r *Registry) SetRenderer(id ID, rend Renderer) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[id]; !exists {
		panic(fmt.Sprintf("report: SetRenderer: block id %q not registered", id))
	}
	if _, exists := r.renderers[id]; exists {
		panic(fmt.Sprintf("report: duplicate renderer for block id %q", id))
	}
	r.renderers[id] = rend
}

// List returns every registered Definition, in the stable catalogue order
// defined by AllBlockIDs. Blocks registered outside that order (custom or
// future blocks) are appended at the end, alphabetically by ID.
func (r *Registry) List() []Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	catalogue := AllBlockIDs()
	seen := make(map[ID]bool, len(r.items))
	out := make([]Definition, 0, len(r.items))

	// Catalogue order first.
	for _, id := range catalogue {
		if def, ok := r.items[id]; ok {
			out = append(out, def)
			seen[id] = true
		}
	}

	// Remaining (non-catalogue) blocks, alphabetically.
	var extra []ID
	for id := range r.items {
		if !seen[id] {
			extra = append(extra, id)
		}
	}
	sort.Slice(extra, func(i, j int) bool { return string(extra[i]) < string(extra[j]) })
	for _, id := range extra {
		out = append(out, r.items[id])
	}

	return out
}
