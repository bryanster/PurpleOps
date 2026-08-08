package store

import (
	"context"
	"sync"
	"sync/atomic"
)

// PostCommitFunc runs after a write transaction commits successfully.
// It must not access the database transaction — the tx is already closed.
type PostCommitFunc func()

// FanoutQueue collects post-commit callbacks (M4-002 activity→SSE fan-out)
// and flushes them after the write transaction commits. Because writes are
// serialised through [DB.Write], a single queue guarded by a mutex is correct
// even without per-transaction tagging.
type FanoutQueue struct {
	mu  sync.Mutex
	fns []PostCommitFunc
}

// Push appends fn to the queue. Safe for concurrent use.
func (q *FanoutQueue) Push(fn PostCommitFunc) {
	q.mu.Lock()
	q.fns = append(q.fns, fn)
	q.mu.Unlock()
}

// Flush executes every queued callback and drains the queue. Panics are
// recovered so one broken hook cannot poison later flushes.
func (q *FanoutQueue) Flush() {
	q.mu.Lock()
	queue := q.fns
	q.fns = nil
	q.mu.Unlock()
	for _, fn := range queue {
		//nolint:errcheck // recover in defer, value intentionally discarded
		func() {
			defer func() { _ = recover() }()
			fn()
		}()
	}
}

// Clear drops every pending callback without executing them.
// Used on rollback — events for uncommitted rows must never be published.
func (q *FanoutQueue) Clear() {
	q.mu.Lock()
	q.fns = nil
	q.mu.Unlock()
}

// PostCommitFanout is the default fan-out queue flushed by [DB.Write].
// Set by the server wiring; nil means no fan-out (tests default).
var PostCommitFanout atomic.Pointer[FanoutQueue]

type postCommitKey struct{}

// WithPostCommit registers fn to run after the current write transaction
// commits. For use when the caller does not have access to [PostCommitFanout].
func WithPostCommit(ctx context.Context, fn PostCommitFunc) context.Context {
	//nolint:errcheck // type assertion to known concrete type, second return is always ok
	existing, _ := ctx.Value(postCommitKey{}).([]PostCommitFunc)
	return context.WithValue(ctx, postCommitKey{}, append(existing, fn))
}

// RunPostCommit executes every callback registered via [WithPostCommit] on ctx.
func RunPostCommit(ctx context.Context) {
	//nolint:errcheck // type assertion to known concrete type, second return is always ok
	fns, _ := ctx.Value(postCommitKey{}).([]PostCommitFunc)
	if len(fns) == 0 {
		return
	}
	for _, fn := range fns {
		//nolint:errcheck // recover in defer, value intentionally discarded
		func() {
			defer func() { _ = recover() }()
			fn()
		}()
	}
}
