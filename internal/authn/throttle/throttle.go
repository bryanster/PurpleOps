package throttle

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bryanster/blacklight/internal/config"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
)

// backoffSteps is how many times a lockout may double before it stops growing.
// With the default fifteen minutes that is 15m, 30m, 1h, 2h — long enough that
// an attacker gets a handful of guesses a day, short enough that somebody who
// mistyped their password four times over does not have to raise a ticket.
const backoffSteps = 3

// sweepInterval is how often the eviction pass runs, measured on the injected
// clock and driven by the calls that are already holding the lock. A background
// goroutine would be a thing to stop, a thing to leak and a thing a test cannot
// see the end of.
const sweepInterval = time.Minute

// defaultMaxKeys bounds each limiter's table. It is not configurable: it is a
// memory ceiling rather than a policy, and an operator asked to choose one has
// no way to know what a good answer is. At around 120 bytes an entry, fifty
// thousand keys is a few megabytes.
const defaultMaxKeys = 50_000

// throttledDetail is the whole of what a throttled caller is told, and the
// reason it is a constant: an existing account, an address nobody holds and a
// spraying source must all be answered identically, or the 429 becomes the
// enumeration oracle the 401 was carefully built not to be (M1-003).
const throttledDetail = "too many sign-in attempts; wait before trying again"

// Scope names which of the two limiters closed. It reaches the log and the
// activity record (M1-015); it is deliberately not in the response.
type Scope string

const (
	ScopeAccount Scope = "account"
	ScopeSource  Scope = "source"
)

// Rule is one limiter's policy: how many failures are tolerated, and how long
// the first lockout lasts once they are not.
type Rule struct {
	// Failures is the number of failed attempts that closes the key. It must be
	// at least one — zero would lock out the first caller through the door.
	Failures int

	// Lockout is the first cooldown. Each further lockout of the same key
	// doubles it, up to [backoffSteps] doublings.
	Lockout time.Duration
}

// Policy is the whole configuration of a [Limiter].
type Policy struct {
	// Account limits attempts against one email address, and Source limits them
	// from one client address. Both are required: either alone leaves an attack
	// the other one sees.
	Account Rule
	Source  Rule

	// MaxKeys bounds each limiter's table. Zero means [defaultMaxKeys].
	MaxKeys int

	// Now reads the clock. Nil means time.Now.
	Now func() time.Time

	// Log receives the events an operator needs and a response cannot carry: a
	// key being locked, and the table filling up. Nil means slog.Default().
	Log *slog.Logger
}

// PolicyFrom derives the policy from the process configuration.
func PolicyFrom(cfg config.Config) Policy {
	return Policy{
		Account: Rule{
			Failures: cfg.Throttle.AccountFailures,
			Lockout:  cfg.Throttle.AccountLockout,
		},
		Source: Rule{
			Failures: cfg.Throttle.SourceFailures,
			Lockout:  cfg.Throttle.SourceLockout,
		},
	}
}

// Attempt is who is presenting a credential.
//
// Either field may be empty, and an empty one is simply not limited: an
// unparseable peer address (a test's synthetic request, a listener that is not
// TCP) must not become one bucket that every caller in the world shares.
type Attempt struct {
	// Account is the email address being attempted, exactly as the caller
	// spelled it. It is normalized here, the same way the store normalizes it,
	// so that changing the capitalisation is not a way around a lockout.
	Account string

	// Source is the client address, as resolved by the realIP middleware — never
	// a header this server has not been told to trust.
	Source string
}

func (a Attempt) accountKey() string { return strings.ToLower(strings.TrimSpace(a.Account)) }
func (a Attempt) sourceKey() string  { return strings.TrimSpace(a.Source) }

// Lockout is a key this limiter has just closed. It exists for the log and for
// M1-015's activity record — the client is told the same thing whichever key it
// was, and how long it has to wait.
type Lockout struct {
	Scope      Scope
	Key        string
	RetryAfter time.Duration
}

// Limiter is the pair of limiters. It is safe for concurrent use, which is the
// only way it is ever used: every request to a guarded endpoint goes through it.
type Limiter struct {
	account *counter
	source  *counter
}

// New returns a Limiter enforcing p, or an error describing a rule that would
// not limit anything.
//
// The checks are startup checks. A threshold of zero locks out everybody and a
// lockout of zero locks out nobody; both are configuration mistakes that must
// stop the process rather than quietly change what the server enforces.
func New(p Policy) (*Limiter, error) {
	if err := errors.Join(
		validate("account", p.Account),
		validate("source", p.Source),
	); err != nil {
		return nil, err
	}

	now := p.Now
	if now == nil {
		now = time.Now
	}
	log := p.Log
	if log == nil {
		log = slog.Default()
	}
	maxKeys := p.MaxKeys
	if maxKeys <= 0 {
		maxKeys = defaultMaxKeys
	}

	return &Limiter{
		account: newCounter(ScopeAccount, p.Account, maxKeys, now, log),
		source:  newCounter(ScopeSource, p.Source, maxKeys, now, log),
	}, nil
}

func validate(name string, r Rule) error {
	switch {
	case r.Failures < 1:
		return fmt.Errorf("throttle: the %s failure threshold is %d; "+
			"it must be at least 1, or the first attempt is locked out", name, r.Failures)
	case r.Lockout <= 0:
		return fmt.Errorf("throttle: the %s lockout is %s; "+
			"it must be positive, or nothing is ever locked out", name, r.Lockout)
	}
	return nil
}

// Check reports whether an attempt may proceed. A nil error means yes.
//
// A non-nil error is an [apierr.Error] carrying the rate_limited code, one fixed
// detail and the Retry-After the response must send — built here rather than by
// the caller, so that the two endpoints and one middleware that call this cannot
// answer three different ways.
func (l *Limiter) Check(a Attempt) error {
	// The account first, and the order decides nothing a client can see: the two
	// refusals are the same document, and only the wait differs.
	if wait := l.account.retryAfter(a.accountKey()); wait > 0 {
		return apierr.RateLimited(throttledDetail, wait)
	}
	if wait := l.source.retryAfter(a.sourceKey()); wait > 0 {
		return apierr.RateLimited(throttledDetail, wait)
	}
	return nil
}

// Failed records an attempt that presented the wrong credential, and returns
// the keys that closed because of it — none, one or both.
//
// An attempt that arrives while its key is already locked changes nothing.
// [Check] refuses those before they reach a handler, so it happens only in a
// race; allowing it to extend a lockout would let an attacker keep an account
// shut for as long as they cared to keep sending requests.
func (l *Limiter) Failed(a Attempt) []Lockout {
	var locked []Lockout
	// Both, always: an attempt that closes an account is also one more failure
	// from wherever it came from, and short-circuiting after the first would let
	// an attacker spend their source budget for free.
	if wait := l.account.fail(a.accountKey()); wait > 0 {
		locked = append(locked, Lockout{
			Scope: ScopeAccount, Key: a.accountKey(), RetryAfter: wait,
		})
	}
	if wait := l.source.fail(a.sourceKey()); wait > 0 {
		locked = append(locked, Lockout{
			Scope: ScopeSource, Key: a.sourceKey(), RetryAfter: wait,
		})
	}
	return locked
}

// Succeeded records an attempt that presented the right credential. It forgets
// the account entirely — the failure count and its place on the doubling ladder
// — so somebody who mistypes twice, or who sat out a lockout and then signed in,
// starts from nothing next time. Whoever it was is holding the password, which
// is the strongest evidence this package ever gets that it is the owner.
//
// It deliberately does not clear the source: an attacker who holds one valid
// account would otherwise reset their spraying budget whenever it ran low, which
// is precisely the attack the source limiter exists for.
func (l *Limiter) Succeeded(a Attempt) {
	l.account.succeed(a.accountKey())
}

// entry is one key's state.
type entry struct {
	// failures counts consecutive failures since the last success or lockout.
	failures int
	// lockouts counts how many times this key has been locked since it was last
	// clean. It is what the cooldown doubles on, rather than the failure count,
	// so that a lockout served in full and then re-earned is what lengthens the
	// next one.
	lockouts int
	// lastAt is when this entry last changed. Eviction is measured from it.
	lastAt time.Time
	// until is when the current lockout ends. Zero means not locked.
	until time.Time
}

// counter is one limiter: a table of keys and the rule applied to each.
type counter struct {
	scope      Scope
	rule       Rule
	maxLockout time.Duration
	maxKeys    int
	now        func() time.Time
	log        *slog.Logger

	mu      sync.Mutex
	entries map[string]*entry
	sweptAt time.Time
	// full is whether the table was at capacity the last time somebody tried to
	// add to it, so that the warning is written on the way in and not once per
	// refused attempt.
	full bool
}

func newCounter(scope Scope, rule Rule, maxKeys int, now func() time.Time, log *slog.Logger) *counter {
	return &counter{
		scope: scope,
		rule:  rule,
		// The longest a lockout can be, and — not by coincidence — how long an
		// entry is kept. See stale.
		maxLockout: rule.Lockout << backoffSteps,
		maxKeys:    maxKeys,
		now:        now,
		log:        log,
		entries:    make(map[string]*entry),
	}
}

// retryAfter reports how long key must wait, or zero if it may proceed.
func (c *counter) retryAfter(key string) time.Duration {
	if key == "" {
		return 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	e, ok := c.entries[key]
	if !ok || c.stale(e, now) || !now.Before(e.until) {
		return 0
	}
	return e.until.Sub(now)
}

// fail records a failure and returns the cooldown if that failure closed the
// key, or zero.
func (c *counter) fail(key string) time.Duration {
	if key == "" {
		return 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := c.now()
	if now.Sub(c.sweptAt) >= sweepInterval {
		c.sweep(now)
	}

	e, ok := c.entries[key]
	if ok && c.stale(e, now) {
		// Older than any lockout it could still be serving, so it says nothing
		// about what is happening now.
		delete(c.entries, key)
		ok = false
	}
	if !ok {
		if e = c.admit(key, now); e == nil {
			return 0
		}
	}

	if now.Before(e.until) {
		return 0 // already locked; see Limiter.Failed
	}

	e.failures++
	e.lastAt = now
	if e.failures < c.rule.Failures {
		return 0
	}

	// Locked. The failure count starts again, so re-earning a lockout after
	// serving one costs the attacker the same number of guesses as the first
	// time — at twice the wait.
	e.failures = 0
	e.lockouts++
	e.until = now.Add(c.lockoutFor(e.lockouts))
	return e.until.Sub(now)
}

// succeed forgets a key, unless it is locked: a lockout that the right password
// ends is not a lockout (M1-004), and Check refuses those attempts before a
// handler can report one as a success anyway.
func (c *counter) succeed(key string) {
	if key == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if e, ok := c.entries[key]; ok && c.now().Before(e.until) {
		return
	}
	delete(c.entries, key)
}

// lockoutFor returns the cooldown for a key that has now been locked n times.
func (c *counter) lockoutFor(n int) time.Duration {
	if n > backoffSteps+1 {
		n = backoffSteps + 1
	}
	return c.rule.Lockout << (n - 1)
}

// stale reports whether an entry can be dropped without shortening a lockout.
//
// The test is the whole eviction policy, and it is safe by arithmetic rather
// than by care: an entry's lockout ends at lastAt plus at most maxLockout, and
// an entry is kept for exactly maxLockout after lastAt, so nothing that is still
// locked is ever stale. Eviction can therefore never be used to unlock an
// account by flooding the table.
//
// It is also what stops the doubling being permanent. A key that has been quiet
// for as long as the longest lockout it could have been serving is
// indistinguishable from one nobody has ever attempted, and is treated as one:
// the ladder starts at the bottom again. That is no more generous than the cap
// already is — the same few guesses per cap-length window — and it means a bad
// afternoon spent guessing your own password is not still costing you next week.
func (c *counter) stale(e *entry, now time.Time) bool {
	return now.Sub(e.lastAt) >= c.maxLockout
}

// admit adds a key, or returns nil when the table is full.
//
// Full means this limiter stops seeing new keys rather than growing without
// bound. It fails open, which is the lesser of the two: refusing every new key
// would turn a full table into a total outage that anybody could cause, whereas
// the other limiter — the source one, whose key space is bounded by the number
// of hosts that can reach this server — is still counting.
func (c *counter) admit(key string, now time.Time) *entry {
	// One sweep on the way to full, and then no more until the periodic one:
	// sweeping is O(n), and a table that is full of entries too young to evict
	// would otherwise pay for a full scan on every single attempt.
	if len(c.entries) >= c.maxKeys && !c.full {
		c.sweep(now)
		if len(c.entries) >= c.maxKeys {
			c.full = true
			c.log.Warn("the login throttle table is full and is no longer tracking new keys",
				slog.String("scope", string(c.scope)),
				slog.Int("keys", len(c.entries)))
		}
	}
	if len(c.entries) >= c.maxKeys {
		return nil
	}

	e := &entry{lastAt: now}
	c.entries[key] = e
	return e
}

// sweep drops every stale entry. It is O(n) and runs at most once per
// [sweepInterval], plus once more when the table hits its ceiling.
func (c *counter) sweep(now time.Time) {
	c.sweptAt = now
	for key, e := range c.entries {
		if c.stale(e, now) {
			delete(c.entries, key)
		}
	}
	if c.full && len(c.entries) < c.maxKeys {
		c.full = false
		c.log.Info("the login throttle table has room again",
			slog.String("scope", string(c.scope)),
			slog.Int("keys", len(c.entries)))
	}
}
