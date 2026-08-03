package authz

// docs/authz.md is rendered from the rule table, so the documented permission
// model and the enforced one are the same artifact. `make generate` runs this,
// and CI's drift gate fails a change to the table that was not accompanied by
// the regenerated document (M0B-012).
//
// The renderer lives in its own package below this one so that this one keeps
// importing nothing — see TestAuthzImportsNothingThatCouldMakeItImpure.

//go:generate go run ./authzdoc -out ../../docs/authz.md
