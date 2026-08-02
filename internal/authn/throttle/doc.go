// Package throttle rations failed sign-in attempts, so that a password cannot
// be guessed at the speed of the network (M1-004).
//
// Two limiters run over every attempt and both must allow it:
//
//   - The account limiter, keyed on the normalized email address. It closes an
//     account for a cooldown after a few consecutive failures, and the cooldown
//     grows each time it closes again. This is what stops one password being
//     guessed.
//   - The source limiter, keyed on the client address. Its threshold is much
//     higher and a success does not clear it. This is what stops one host
//     trying one password against every account, which the account limiter
//     cannot see.
//
// The state is in memory, in this process, and is gone when it restarts. That
// is correct rather than convenient: PLAN.md §1 deploys one node with an
// embedded database, so there is no second process to disagree with, and a
// restart is not something an attacker can cause. A deployment that ever grows a
// second node needs this state shared, and nothing here will notice on its own.
//
// The clock is injected ([Policy.Now]) so that lockouts and eviction can be
// reached in a test without sleeping, and there is no background goroutine:
// eviction happens on the calls that are already taking the lock.
package throttle
