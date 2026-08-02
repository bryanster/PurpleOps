package throttle

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bryanster/purpleops/internal/httpapi/apierr"
)

// The limiter, on a clock the test drives. Nothing here sleeps: a lockout is
// fifteen minutes long and a test that waited one out would be the reason
// nobody runs the suite.

const (
	account = "alice@example.com"
	other   = "bob@example.com"
	source  = "198.51.100.7"
)

// clock is the injected time source. It has a mutex because the concurrency
// test reads it from every goroutine at once.
type clock struct {
	mu sync.Mutex
	at time.Time
}

func newClock() *clock {
	return &clock{at: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)}
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at = c.at.Add(d)
}

// testPolicy is a small, legible policy: three failures locks an account for a
// minute, and ten failures locks a source for the same.
func testPolicy() Policy {
	return Policy{
		Account: Rule{Failures: 3, Lockout: time.Minute},
		Source:  Rule{Failures: 10, Lockout: time.Minute},
		// Discarded: the limiter warns about a full table, and two of these tests
		// fill one on purpose.
		Log: slog.New(slog.DiscardHandler),
	}
}

func newTestLimiter(t *testing.T, p Policy) (*Limiter, *clock) {
	t.Helper()

	c := newClock()
	p.Now = c.now
	limiter, err := New(p)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return limiter, c
}

// attempt is the pair the tests spend most of their time writing.
func attempt(email, ip string) Attempt {
	return Attempt{Account: email, Source: ip}
}

// retryAfter unwraps what Check reported, failing the test if it is not the
// rate-limit error every refusal is supposed to be.
func retryAfter(t *testing.T, err error) time.Duration {
	t.Helper()

	if err == nil {
		t.Fatal("Check allowed the attempt, want it refused")
	}
	if !errors.Is(err, apierr.ErrRateLimited) {
		t.Fatalf("Check = %v, want an apierr.ErrRateLimited", err)
	}
	var problem *apierr.Error
	if !errors.As(err, &problem) {
		t.Fatalf("Check = %v, want an *apierr.Error", err)
	}
	if problem.RetryAfter() <= 0 {
		t.Errorf("the refusal carries Retry-After %v; a 429 with no wait leaves a client guessing",
			problem.RetryAfter())
	}
	return problem.RetryAfter()
}

// mustAllow fails the test if the attempt is refused.
func mustAllow(t *testing.T, limiter *Limiter, a Attempt) {
	t.Helper()

	if err := limiter.Check(a); err != nil {
		t.Fatalf("Check(%q from %q) = %v, want the attempt allowed", a.Account, a.Source, err)
	}
}

// failTimes records n failed attempts, checking each one first the way the
// middleware does.
func failTimes(t *testing.T, limiter *Limiter, a Attempt, n int) {
	t.Helper()

	for i := range n {
		if err := limiter.Check(a); err != nil {
			t.Fatalf("failure %d of %d was refused before it could be recorded: %v", i+1, n, err)
		}
		limiter.Failed(a)
	}
}

func TestNewRejectsARuleThatLimitsNothing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  Policy
		wantErr string
	}{{
		name: "no account threshold",
		policy: Policy{
			Account: Rule{Failures: 0, Lockout: time.Minute},
			Source:  Rule{Failures: 10, Lockout: time.Minute},
		},
		wantErr: "the account failure threshold is 0",
	}, {
		name: "no account lockout",
		policy: Policy{
			Account: Rule{Failures: 3},
			Source:  Rule{Failures: 10, Lockout: time.Minute},
		},
		wantErr: "the account lockout is 0s",
	}, {
		name: "no source threshold",
		policy: Policy{
			Account: Rule{Failures: 3, Lockout: time.Minute},
			Source:  Rule{Failures: -1, Lockout: time.Minute},
		},
		wantErr: "the source failure threshold is -1",
	}, {
		name: "no source lockout",
		policy: Policy{
			Account: Rule{Failures: 3, Lockout: time.Minute},
			Source:  Rule{Failures: 10},
		},
		wantErr: "the source lockout is 0s",
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := New(test.policy)
			if err == nil {
				t.Fatalf("New(%+v) = nil, want an error naming the rule", test.policy)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Errorf("New() = %q, want it to contain %q", err, test.wantErr)
			}
		})
	}
}

// TestAnAccountLocksAtTheThresholdAndOpensAfterTheCooldown is the shape of the
// whole ticket: the threshold closes it, the right password does not open it,
// and the clock does.
func TestAnAccountLocksAtTheThresholdAndOpensAfterTheCooldown(t *testing.T) {
	t.Parallel()

	limiter, clock := newTestLimiter(t, testPolicy())
	a := attempt(account, source)

	failTimes(t, limiter, a, 2)
	mustAllow(t, limiter, a) // two failures is not three

	limiter.Failed(a)
	if got := retryAfter(t, limiter.Check(a)); got != time.Minute {
		t.Errorf("Retry-After immediately after the lockout = %v, want %v", got, time.Minute)
	}

	// Accurate as it counts down, not just when it starts.
	clock.advance(40 * time.Second)
	if got, want := retryAfter(t, limiter.Check(a)), 20*time.Second; got != want {
		t.Errorf("Retry-After 40s into a 1m lockout = %v, want %v", got, want)
	}

	// A success cannot end it. This is the acceptance criterion the ticket
	// argues for at length: a lockout that lets the right password through is
	// not a lockout, so Succeeded — which the handler could only reach by
	// answering 200 — must not clear one either.
	limiter.Succeeded(a)
	if retryAfter(t, limiter.Check(a)) != 20*time.Second {
		t.Error("a successful attempt cleared a live lockout")
	}

	clock.advance(20 * time.Second)
	mustAllow(t, limiter, a)

	// And the count started again: three more failures, not one, to close it.
	failTimes(t, limiter, a, 2)
	mustAllow(t, limiter, a)
}

func TestASuccessBeforeTheThresholdClearsTheCount(t *testing.T) {
	t.Parallel()

	limiter, _ := newTestLimiter(t, testPolicy())
	a := attempt(account, source)

	failTimes(t, limiter, a, 2)
	limiter.Succeeded(a)

	// Without the reset, one more failure would be the third and would lock.
	failTimes(t, limiter, a, 2)
	mustAllow(t, limiter, a)
}

func TestEachLockoutIsLongerThanTheLastUpToACap(t *testing.T) {
	t.Parallel()

	policy := testPolicy()
	// This test is about the account limiter, and it spends fifteen failures
	// getting there. The source rule is put out of its way rather than the test
	// inventing a new address every few attempts, which would test nothing.
	policy.Source = Rule{Failures: 1_000, Lockout: time.Minute}
	limiter, clock := newTestLimiter(t, policy)
	a := attempt(account, source)

	// Three doublings and then a ceiling: an attacker who keeps coming back gets
	// fewer guesses per hour each time, and somebody who has genuinely forgotten
	// their password is never locked out for more than the cap.
	want := []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 8 * time.Minute}
	for i, want := range want {
		failTimes(t, limiter, a, 3)
		got := retryAfter(t, limiter.Check(a))
		if got != want {
			t.Fatalf("lockout %d lasted %v, want %v", i+1, got, want)
		}
		clock.advance(got - time.Second)
		if err := limiter.Check(a); err == nil {
			t.Fatalf("lockout %d ended a second early", i+1)
		}
		clock.advance(time.Second)
		mustAllow(t, limiter, a)
	}

	// The ladder is not permanent. Serving the longest lockout in full and then
	// going quiet for it leaves an entry that says nothing about now — it is
	// evicted, and the next lockout starts at the bottom again. It costs the
	// same three guesses per eight minutes either way, and it means somebody who
	// spent a bad afternoon guessing their own password is not still being
	// punished for it a week later.
	clock.advance(8 * time.Minute)
	failTimes(t, limiter, a, 3)
	if got, want := retryAfter(t, limiter.Check(a)), time.Minute; got != want {
		t.Errorf("the lockout after a long quiet spell lasted %v, want %v", got, want)
	}
}

// TestSigningInAfterALockoutStartsTheLadderAgain is the other way down from the
// doubling: whoever just signed in is holding the password, which is the best
// evidence this package gets that they own the account.
func TestSigningInAfterALockoutStartsTheLadderAgain(t *testing.T) {
	t.Parallel()

	limiter, clock := newTestLimiter(t, testPolicy())
	a := attempt(account, source)

	failTimes(t, limiter, a, 3)
	clock.advance(time.Minute)
	mustAllow(t, limiter, a)
	limiter.Succeeded(a)

	failTimes(t, limiter, a, 3)
	if got, want := retryAfter(t, limiter.Check(a)), time.Minute; got != want {
		t.Errorf("the lockout after signing in lasted %v, want %v — the doubling survived a success",
			got, want)
	}
}

func TestHammeringALockedAccountDoesNotExtendTheLockout(t *testing.T) {
	t.Parallel()

	policy := testPolicy()
	// As above: the twenty refused attempts below are still attack traffic and
	// the source limiter is right to count them, but this test is about what
	// they do to the account.
	policy.Source = Rule{Failures: 1_000, Lockout: time.Minute}
	limiter, clock := newTestLimiter(t, policy)
	a := attempt(account, source)

	failTimes(t, limiter, a, 3)

	// A caller who ignores the 429 and keeps sending. If this extended the
	// lockout, anybody who knew an address could keep its owner signed out for
	// as long as they cared to keep a script running.
	for range 20 {
		clock.advance(time.Second)
		limiter.Failed(a)
	}
	if got, want := retryAfter(t, limiter.Check(a)), 40*time.Second; got != want {
		t.Errorf("Retry-After after 20s of hammering a 1m lockout = %v, want %v", got, want)
	}

	clock.advance(40 * time.Second)
	mustAllow(t, limiter, a)
}

func TestLockingOneAccountLeavesTheOthersAlone(t *testing.T) {
	t.Parallel()

	limiter, _ := newTestLimiter(t, testPolicy())

	failTimes(t, limiter, attempt(account, source), 3)

	if err := limiter.Check(attempt(account, source)); err == nil {
		t.Fatal("the account that failed three times is not locked")
	}
	mustAllow(t, limiter, attempt(other, source))
}

// TestOneSourceSprayingManyAccountsIsStopped is the attack the account limiter
// cannot see: one password, many addresses, never enough failures against any
// one of them to close it.
func TestOneSourceSprayingManyAccountsIsStopped(t *testing.T) {
	t.Parallel()

	limiter, _ := newTestLimiter(t, testPolicy())

	for i := range 10 {
		a := attempt(fmt.Sprintf("user%d@example.com", i), source)
		if err := limiter.Check(a); err != nil {
			t.Fatalf("attempt %d was refused before the source threshold: %v", i, err)
		}
		limiter.Failed(a)
	}

	// The eleventh account has never been touched, and this source has run out.
	if err := limiter.Check(attempt("nobody@example.com", source)); err == nil {
		t.Error("ten failures across ten accounts from one address did not trip the source limiter")
	}
	// A different address is unaffected: the limit is on the source, not on the
	// server.
	mustAllow(t, limiter, attempt("nobody@example.com", "203.0.113.9"))
}

// TestASuccessDoesNotRefillASource is the other half of spraying: an attacker
// who holds one valid account must not be able to top their budget up with it.
func TestASuccessDoesNotRefillASource(t *testing.T) {
	t.Parallel()

	limiter, _ := newTestLimiter(t, testPolicy())

	for i := range 9 {
		a := attempt(fmt.Sprintf("user%d@example.com", i), source)
		mustAllow(t, limiter, a)
		limiter.Failed(a)
	}
	limiter.Succeeded(attempt("theirs@example.com", source))

	a := attempt("user9@example.com", source)
	mustAllow(t, limiter, a)
	limiter.Failed(a)
	if err := limiter.Check(attempt("user10@example.com", source)); err == nil {
		t.Error("a successful sign-in reset the source's failure count")
	}
}

func TestTheAccountKeyIsNormalisedTheWayTheStoreNormalisesIt(t *testing.T) {
	t.Parallel()

	limiter, _ := newTestLimiter(t, testPolicy())

	// The store looks users up by lower(trim(email)) — see internal/store/
	// identity/users.go. Anything less here would make the shift key a way
	// around a lockout.
	spellings := []string{"Alice@Example.com", " alice@example.com ", "ALICE@EXAMPLE.COM"}
	for _, spelling := range spellings {
		a := attempt(spelling, source)
		mustAllow(t, limiter, a)
		limiter.Failed(a)
	}

	if err := limiter.Check(attempt(account, source)); err == nil {
		t.Error("three failures spelled three ways did not lock the one account they were against")
	}
}

func TestAnAttemptWithNoKeyToLimitIsAllowed(t *testing.T) {
	t.Parallel()

	limiter, _ := newTestLimiter(t, testPolicy())

	// An unparseable peer address, which is what a synthetic request and a
	// non-TCP listener produce. The alternative — one bucket keyed on "" — would
	// have every such caller in the world locking each other out.
	for range 50 {
		limiter.Failed(Attempt{Account: "", Source: ""})
	}
	mustAllow(t, limiter, Attempt{Account: "", Source: ""})
	mustAllow(t, limiter, attempt(account, source))
}

// TestFailuresAreCountedExactly is the -race test. Nothing about a limiter is
// worth much if two simultaneous guesses can count as one.
func TestFailuresAreCountedExactly(t *testing.T) {
	t.Parallel()

	const parallel = 64
	policy := testPolicy()
	policy.Account = Rule{Failures: parallel, Lockout: time.Minute}
	policy.Source = Rule{Failures: parallel, Lockout: time.Minute}
	limiter, _ := newTestLimiter(t, policy)

	var (
		start   sync.WaitGroup
		done    sync.WaitGroup
		mu      sync.Mutex
		lockups []Lockout
	)
	start.Add(1)
	for range parallel {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()

			locked := limiter.Failed(attempt(account, source))
			mu.Lock()
			defer mu.Unlock()
			lockups = append(lockups, locked...)
		}()
	}
	start.Done()
	done.Wait()

	// Exactly one goroutine saw the count reach the threshold, on each limiter.
	// One too few would mean a lost update; one too many would mean two
	// goroutines both believed they were the last.
	if len(lockups) != 2 {
		t.Fatalf("%d attempts against a threshold of %d produced %d lockouts, want 2 (one per limiter): %+v",
			parallel, parallel, len(lockups), lockups)
	}
	scopes := map[Scope]bool{}
	for _, lockout := range lockups {
		scopes[lockout.Scope] = true
	}
	if !scopes[ScopeAccount] || !scopes[ScopeSource] {
		t.Errorf("the lockouts were %+v, want one of each scope", lockups)
	}
}

// TestTheTableIsBounded is the memory half of the ticket. The eviction policy is
// stated on counter.stale: an entry is kept for as long as the longest lockout
// it could still be serving, and dropped after that.
func TestTheTableIsBounded(t *testing.T) {
	t.Parallel()

	const distinct = 100_000
	policy := testPolicy()
	policy.MaxKeys = 1_000
	limiter, clock := newTestLimiter(t, policy)

	for i := range distinct {
		// One failure each, from a fresh address as well, so both tables are
		// asked to grow.
		a := attempt(fmt.Sprintf("user%d@example.com", i), fmt.Sprintf("192.0.2.%d", i%256))
		limiter.Failed(a)
		clock.advance(time.Millisecond)
	}

	for _, c := range []*counter{limiter.account, limiter.source} {
		c.mu.Lock()
		size := len(c.entries)
		c.mu.Unlock()
		if size > policy.MaxKeys {
			t.Errorf("the %s table holds %d entries after %d distinct keys, want at most %d",
				c.scope, size, distinct, policy.MaxKeys)
		}
	}
}

// TestEvictionCannotUnlockAnAccount is why the ceiling refuses new keys rather
// than dropping old ones: if a full table evicted whatever was to hand, flooding
// it would be the way out of a lockout.
func TestEvictionCannotUnlockAnAccount(t *testing.T) {
	t.Parallel()

	policy := testPolicy()
	policy.MaxKeys = 16
	limiter, clock := newTestLimiter(t, policy)

	a := attempt(account, source)
	failTimes(t, limiter, a, 3)

	for i := range 1_000 {
		limiter.Failed(attempt(fmt.Sprintf("filler%d@example.com", i), source))
		clock.advance(time.Millisecond)
	}

	if err := limiter.Check(a); err == nil {
		t.Error("filling the table released a live lockout")
	}
}

// TestAStaleEntryIsForgotten pins the other end of the policy: a table that only
// ever grew would be a slow memory leak on a public endpoint.
func TestAStaleEntryIsForgotten(t *testing.T) {
	t.Parallel()

	limiter, clock := newTestLimiter(t, testPolicy())

	limiter.Failed(attempt(account, source))
	// The longest lockout the account rule can produce; after it, the entry can
	// say nothing about what is happening now.
	clock.advance(time.Minute << backoffSteps)
	limiter.Failed(attempt(other, source))

	limiter.account.mu.Lock()
	_, kept := limiter.account.entries[account]
	limiter.account.mu.Unlock()
	if kept {
		t.Error("an entry older than the longest possible lockout was kept")
	}
}
