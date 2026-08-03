// Package authz decides what an authenticated caller may do. It is the single
// place that answers that question: one policy function over (subject, action,
// resource), with no role comparison anywhere else in the codebase.
//
// v1 duplicated role checks per handler with two contradictory definitions of
// "blue", which is how spectators acquired write access (PLAN.md §4).
//
// # The shape of it
//
//	decision := authz.Can(ctx, subject, authz.ActionExecutionWriteRed, resource)
//
// [Can] is a pure function. It performs no I/O, reads nothing from ctx, and
// never looks anything up: every fact it uses arrives in the [Subject] the
// authentication middleware built and the [Resource] the caller loaded. That is
// not tidiness. It is what lets M1-014 assert the entire role × action ×
// resource matrix in milliseconds, and what stops a rule from quietly becoming
// a database query — the moment one can, the exhaustive test becomes impossible
// and the model stops being checkable. If a future rule needs a fact, the fact
// goes in the struct and the caller loads it.
//
// # The model is a table
//
// The permission model is data: [Rules], one row per [Action], evaluated by the
// same twenty lines for every action. A reviewer reads the model rather than
// tracing branches, docs/authz.md is that table rendered by `make generate`, and
// the two cannot drift. Adding an action without a rule fails a test; there is
// no default, and silence is never a permission.
//
// # This package owns the words
//
// [PlatformRole], [EngagementRole] and [Method] are defined here and nowhere
// else, and a test fails the build if a role string appears elsewhere in the Go
// tree. That is the direct fix for the two definitions of "blue": there is one
// vocabulary, so there is nothing to disagree with. The wire enums in
// api/openapi.yaml are generated from the spec and checked against these.
//
// # What this package must never import
//
// No database, no HTTP, no store, no clock. TestAuthzImportsNothingThatCouldMakeItImpure
// enforces it over the transitive dependency graph, because the constraint is
// about what [Can] *could* do, not about what today's code happens to do.
//
// Implemented by M1-012 (this package) and M1-013 (the one middleware that
// calls it).
package authz
