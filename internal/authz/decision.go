package authz

import (
	"context"
	"log/slog"
)

// Decision is [Can]'s answer, and always says why.
//
// The reason is not decoration. v1's authorization failures were hard to see
// precisely because a denial was a bare 403 with no record of which rule
// produced it — so nobody could tell an intended refusal from a missing rule,
// and the missing rules survived. Every value here carries prose naming the
// role, the action and, where it matters, the engagement.
//
// The reason is for operators, not for callers. It names roles and identifiers
// the requester may not be entitled to know about, so M1-013 logs it and
// answers the request with the flat problem shape from M0B-007. Nothing here
// is ever written into a response body.
type Decision struct {
	// Allowed is the answer. The zero Decision is a denial with no reason,
	// which no constructor here produces — so a Decision nobody filled in
	// fails closed.
	Allowed bool

	// Reason is one line of prose. Non-empty on every Decision this package
	// returns, allowed or denied.
	Reason string

	// Conceal marks a denial that must not admit the resource exists: a
	// non-member asking about an engagement, or the blue side asking about an
	// unrevealed step. M1-013 turns these into 404 and the rest into 403.
	//
	// It is decided here rather than there because the policy is what knows
	// *why* it refused, and "403 or 404" is a question about the reason. A
	// middleware inferring it from the status of the subject would be a second
	// place making a permission decision, which is the thing this package
	// exists to prevent.
	Conceal bool
}

// allow permits the action, recording which role did it.
func allow(reason string) Decision {
	return Decision{Allowed: true, Reason: reason}
}

// deny refuses, in a way that may admit the resource exists.
func deny(reason string) Decision {
	return Decision{Reason: reason}
}

// concealed refuses without admitting the resource exists.
func concealed(reason string) Decision {
	return Decision{Reason: reason, Conceal: true}
}

// LogValue renders a decision for a log line.
func (d Decision) LogValue() slog.Value {
	attrs := []slog.Attr{
		slog.Bool("allowed", d.Allowed),
		slog.String("reason", d.Reason),
	}
	if d.Conceal {
		attrs = append(attrs, slog.Bool("conceal", true))
	}
	return slog.GroupValue(attrs...)
}

// Log records one decision at debug level: who asked, for what, on what, and
// what the policy said.
//
// It is here rather than inside [Can] because Can does no I/O — that is what
// makes the M1-014 matrix possible — and it is here rather than in the
// middleware so that every caller logs the same fields under the same keys. A
// decision somebody has to reconstruct from three log lines is a decision
// nobody reconstructs.
//
// Debug for both outcomes. A denial that matters to an operator is an activity
// log entry (M1-015), not a warning here: on a busy installation the ordinary
// denial is a browser probing an endpoint the user cannot see, and a level that
// cries wolf about that is a level people turn off.
func Log(ctx context.Context, log *slog.Logger, subject Subject, action Action, resource Resource, decision Decision) {
	if log == nil {
		return
	}
	log.DebugContext(ctx, "authorization decision",
		slog.Any("subject", subject),
		slog.String("action", action.String()),
		slog.Any("resource", resource),
		slog.Any("decision", decision))
}
