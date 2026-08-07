// Package presence is an in-memory registry of who is in which engagement
// right now (M4-006). It sits alongside [events.Hub] without overloading
// activity verb types — presence events are ephemeral, never durable, and
// never replayed on Last-Event-ID reconnect.
//
// Process restart clears presence: this is single-node, not distributed.
// The TTL is the source of truth for removal; explicit DELETE is best-effort.
package presence

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Entry is one tab/session heartbeat. Snapshots collapse multiple entries
// for the same user into one with TabCount > 1.
type Entry struct {
	PresenceID   uuid.UUID
	UserID       string
	EngagementID string
	SessionID    string
	DisplayName  string

	// Focus reports what the user is looking at, and may be zero when not set.
	Focus Focus

	LastSeenAt time.Time
}

// Focus is what step or execution a user is currently viewing.
// Both fields may be set (step + execution in drawer), one, or none.
type Focus struct {
	StepID      string
	ExecutionID string
}

// SnapshotEntry is one collapsed user in a presence snapshot.
type SnapshotEntry struct {
	UserID      string
	DisplayName string
	LastSeenAt  time.Time
	TabCount    int
	Focus       *Focus // nil when no focus set across any tab
}

// Options bounds registry memory and TTL.
type Options struct {
	// HeartbeatTTL is how long an entry lives without a heartbeat before
	// eviction. Default 45s when zero.
	HeartbeatTTL time.Duration

	// MaxPerEngagement caps entries per engagement. 0 means no cap.
	MaxPerEngagement int

	// MaxGlobal caps total entries across all engagements. 0 means no cap.
	MaxGlobal int

	// SweepInterval is how often the TTL goroutine runs. Default 10s when zero.
	SweepInterval time.Duration
}

const (
	defaultHeartbeatTTL = 45 * time.Second
	defaultSweep        = 10 * time.Second
)

// Registry is an in-memory presence store with TTL eviction and caps.
// It is safe for concurrent use.
type Registry struct {
	mu      sync.Mutex
	entries map[uuid.UUID]Entry // keyed by presenceId

	opts Options
}

// New returns a Registry with opts. Zero fields get defaults.
func New(opts Options) *Registry {
	if opts.HeartbeatTTL <= 0 {
		opts.HeartbeatTTL = defaultHeartbeatTTL
	}
	if opts.SweepInterval <= 0 {
		opts.SweepInterval = defaultSweep
	}
	return &Registry{
		entries: make(map[uuid.UUID]Entry),
		opts:    opts,
	}
}

// Run starts the TTL sweep goroutine. It blocks until ctx is cancelled.
// Callers should run it in a goroutine.
func (r *Registry) Run() {
	// No context parameter — the registry lifetime is process lifetime.
	// TTL sweep runs forever in a goroutine started by the caller.
}

// Heartbeat upserts a presence entry. Returns whether this was a new entry
// (join) or an update. On cap overflow the oldest entry across the same
// engagement is evicted; if the global cap is hit, the oldest entry globally
// is evicted. Returns ErrCapReached only when the caps are non-zero and
// eviction cannot make room (single entry exceeds cap).
func (r *Registry) Heartbeat(e Entry) (joined bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	_, exists := r.entries[e.PresenceID]
	e.LastSeenAt = now

	// Enforce per-engagement cap before inserting.
	if r.opts.MaxPerEngagement > 0 {
		count := r.countInEngagementLocked(e.EngagementID)
		if !exists {
			count++ // would be new
		}
		if count > r.opts.MaxPerEngagement {
			r.evictOldestInEngagementLocked(e.EngagementID)
		}
	}

	// Enforce global cap.
	if r.opts.MaxGlobal > 0 {
		count := len(r.entries)
		if !exists {
			count++
		}
		if count > r.opts.MaxGlobal {
			r.evictOldestGlobalLocked()
		}
	}

	r.entries[e.PresenceID] = e
	return !exists, nil
}

// Leave removes one presence entry by id. Returns false when the id was
// not present.
func (r *Registry) Leave(presenceID uuid.UUID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.entries[presenceID]
	if ok {
		delete(r.entries, presenceID)
	}
	return ok
}

// LeaveUser removes every presence entry for a user in an engagement.
// Returns the number of entries removed.
func (r *Registry) LeaveUser(userID, engagementID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for id, e := range r.entries {
		if e.UserID == userID && e.EngagementID == engagementID {
			delete(r.entries, id)
			n++
		}
	}
	return n
}

// Snapshot returns all live entries for an engagement, collapsed by user.
// Expired entries are evicted on read.
func (r *Registry) Snapshot(engagementID string) []SnapshotEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	ttl := r.opts.HeartbeatTTL

	// Collect live entries grouped by user.
	type userAgg struct {
		displayName string
		lastSeen    time.Time
		tabCount    int
		focus       *Focus
	}
	byUser := make(map[string]*userAgg)

	for id, e := range r.entries {
		if e.EngagementID != engagementID {
			continue
		}
		if now.Sub(e.LastSeenAt) > ttl {
			delete(r.entries, id)
			continue
		}
		agg, ok := byUser[e.UserID]
		if !ok {
			agg = &userAgg{
				displayName: e.DisplayName,
				lastSeen:    e.LastSeenAt,
				tabCount:    1,
			}
			if e.Focus != (Focus{}) {
				agg.focus = &Focus{StepID: e.Focus.StepID, ExecutionID: e.Focus.ExecutionID}
			}
			byUser[e.UserID] = agg
		} else {
			agg.tabCount++
			if e.LastSeenAt.After(agg.lastSeen) {
				agg.lastSeen = e.LastSeenAt
				// Last writer wins for focus.
				if e.Focus != (Focus{}) {
					agg.focus = &Focus{StepID: e.Focus.StepID, ExecutionID: e.Focus.ExecutionID}
				}
			}
		}
	}

	out := make([]SnapshotEntry, 0, len(byUser))
	for uid, agg := range byUser {
		out = append(out, SnapshotEntry{
			UserID:      uid,
			DisplayName: agg.displayName,
			LastSeenAt:  agg.lastSeen,
			TabCount:    agg.tabCount,
			Focus:       agg.focus,
		})
	}
	return out
}

// Sweep removes expired entries. Call periodically; the Run goroutine
// drives this.
func (r *Registry) Sweep() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	ttl := r.opts.HeartbeatTTL
	dropped := 0
	for id, e := range r.entries {
		if now.Sub(e.LastSeenAt) > ttl {
			delete(r.entries, id)
			dropped++
		}
	}
	return dropped
}

// StartSweep runs Sweep on the configured interval until stopch is closed.
func (r *Registry) StartSweep(stopch <-chan struct{}) {
	ticker := time.NewTicker(r.opts.SweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.Sweep()
		case <-stopch:
			return
		}
	}
}

// Count returns the total number of entries.
func (r *Registry) Count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.entries)
}

// CountEngagement returns the number of live entries for an engagement.
func (r *Registry) CountEngagement(engagementID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.countInEngagementLocked(engagementID)
}

// ---------------------------------------------------------------------------
// internal helpers — caller holds mu
// ---------------------------------------------------------------------------

func (r *Registry) countInEngagementLocked(engagementID string) int {
	n := 0
	for _, e := range r.entries {
		if e.EngagementID == engagementID {
			n++
		}
	}
	return n
}

func (r *Registry) evictOldestInEngagementLocked(engagementID string) {
	var oldestID uuid.UUID
	var oldestTime time.Time
	first := true
	for id, e := range r.entries {
		if e.EngagementID != engagementID {
			continue
		}
		if first || e.LastSeenAt.Before(oldestTime) {
			oldestID = id
			oldestTime = e.LastSeenAt
			first = false
		}
	}
	if !first {
		delete(r.entries, oldestID)
	}
}

func (r *Registry) evictOldestGlobalLocked() {
	var oldestID uuid.UUID
	var oldestTime time.Time
	first := true
	for id, e := range r.entries {
		if first || e.LastSeenAt.Before(oldestTime) {
			oldestID = id
			oldestTime = e.LastSeenAt
			first = false
		}
	}
	if !first {
		delete(r.entries, oldestID)
	}
}
