// Package authz decides what an authenticated caller may do. It is the single
// place that answers that question: one policy function over (subject, action,
// resource), with no role comparison anywhere else in the codebase.
//
// v1 duplicated role checks per handler with two contradictory definitions of
// "blue", which is how spectators acquired write access (PLAN.md §4).
//
// Implemented by M1-012 (policy) and M1-013 (middleware).
package authz
