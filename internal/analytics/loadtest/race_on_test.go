//go:build race

package loadtest_test

// raceMult scales CI-gate budgets: -race adds 2-4× overhead on database
// operations and memory allocations; the gates must still pass under -race
// but stay tight enough that a real regression is caught.
const raceMult = 5
