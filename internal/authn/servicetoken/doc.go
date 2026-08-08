// Package servicetoken is the bearer credential somebody automates against
// this deployment with (M1-011): what one is, how one is minted, and how a
// presented one is turned back into the row it stands for.
//
// PLAN.md §4, on v1: "API keys authenticate nothing." That is the defect this
// package exists to close, and closing it is mostly about where the checking
// happens rather than about the cryptography. So:
//
//   - Nothing here decides what a token may *reach*. [Manager.Resolve] answers
//     "which token is this, and is it still usable"; everything after that is
//     [authz.Can]'s, through the fences M1-012 built. A package that both
//     issued credentials and judged them would be the second opinion this
//     codebase is written against.
//   - Resolution is on the same middleware as the session cookie
//     (internal/httpapi/authn.go), producing the same [authn.Subject]. There is
//     no second authentication path for tokens to be forgotten on, which is how
//     v1's ended up checking nothing.
//   - The two fences are not here either, but the facts they need are: the
//     scopes as stored and the engagement binding as stored, neither inferred.
//
// What *is* decided here is the shape of the credential — see token.go, which
// argues for each of its three parts — and the rules about time: an expiry is
// required, it is bounded, and a use is recorded off the request that made it.
package servicetoken
