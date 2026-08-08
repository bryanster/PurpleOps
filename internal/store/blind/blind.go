// Package blind is the query-layer half of blind mode.
//
// A blind engagement is one where red executes without blue being told what to
// expect (PLAN.md §4). The policy withholds an unrevealed step from the blue
// seat — [authz.GuardBlindMode] — and that is the first fence. This is the
// second: a repository that reads steps filters them here, in its WHERE clause,
// so that an endpoint added without the right action, or a rule that forgot the
// guard, still cannot return a step blue has not been shown.
//
// Both, deliberately. M1-013's words are "so no endpoint can leak them even if a
// rule is missed", and a single fence is one edit away from not being there. The
// two are checked against each other in this package's tests: a reader the
// policy would refuse is a reader this filter hides the row from, across every
// combination of seat and blind flag, so the fences cannot drift into
// disagreeing about who blue is — which is exactly how v1 ended up with two
// definitions of that word.
//
// # What it is not
//
// It is not a permission check. A [Scope] is built from facts somebody else
// loaded — whether the engagement runs blind, and what seat the reader holds —
// and it decides only which rows a query returns. Whether the reader may run the
// query at all is [authz.Can]'s answer, in the one middleware that asks it.
package blind

import "github.com/bryanster/blacklight/internal/authz"

// Scope is what one reader may see of one engagement's steps.
//
// The zero value is the widest scope there is: an engagement that does not run
// blind, read by somebody with no seat in it. That is the right default here and
// would be the wrong one in a permission check — nothing reaches this package
// without having already been allowed, and a filter that hid everything from a
// caller nobody had described would break every read in an engagement that is
// not blind at all.
type Scope struct {
	// Blind is whether the engagement withholds unrevealed steps.
	Blind bool

	// Seat is the reader's role in that engagement, and is empty for somebody
	// who is not a member — a platform administrator reading from outside it.
	//
	// It is the *seat* and not "how they were granted the read", for the same
	// reason [authz.GuardBlindMode] binds to one: an administrator sitting in
	// the blue chair of a blind engagement is sitting in the blue chair, and an
	// administrator who wants the unblinded view can have it by not sitting in
	// it.
	Seat authz.EngagementRole
}

// Withholds reports whether this reader is held to blind mode at all. It is true
// only for the blue seat of a blind engagement; everybody else sees every row,
// and the predicate below says so.
func (s Scope) Withholds() bool {
	return s.Blind && s.Seat == authz.EngagementRoleBlue
}

// Where returns the SQL predicate that hides what this reader may not see, over
// the boolean column that says whether a row has been revealed.
//
// It is "TRUE" for a reader who is withholding nothing, rather than the empty
// string, so that a caller concatenates it the same way in both cases — a
// predicate that is sometimes absent is a WHERE clause somebody eventually
// assembles with a dangling AND, and the failure mode of that is a query that
// returns everything.
//
//	rows, err := db.Read().QueryContext(ctx,
//		`SELECT … FROM app.step WHERE engagement_id = ? AND `+scope.Where("revealed"),
//		engagementID)
//
// revealedColumn is concatenated into SQL, so it must be a constant belonging to
// the repository that owns the table. Nothing here ever sees a value from a
// request; a repository that passed one would be putting a caller's string into
// its own query, which is a bug no signature can prevent and this comment is the
// warning about.
func (s Scope) Where(revealedColumn string) string {
	if !s.Withholds() {
		return "TRUE"
	}
	return revealedColumn
}

// Permits reports whether one row survives the filter, for the callers that have
// rows rather than a query to build: an event about to be pushed to a subscriber
// (M4), or a projection assembled in Go.
//
// It answers exactly what [Scope.Where] answers, so that the two cannot disagree
// about a row — TestWhereAndPermitsAgree holds them to it against a real
// database rather than against each other's source.
func (s Scope) Permits(revealed bool) bool {
	return revealed || !s.Withholds()
}
