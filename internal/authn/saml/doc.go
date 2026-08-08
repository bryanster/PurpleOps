// Package saml is a SAML 2.0 service provider: it publishes this deployment's
// metadata, starts a sign-in at an identity provider, and turns the assertion
// that comes back into claims that have been *proved* (M1-010).
//
// Nobody chooses SAML in 2026. Enterprises still require it, and it is built
// after OIDC so that everything downstream of a verified identity — which
// account this is, whether one may be created, what role it maps to, whether a
// second factor still applies — is code that already existed. This package
// produces an [Identity]; internal/authn decides what it means, exactly as it
// does for internal/authn/oidc. There is one copy of those decisions and this is
// not it.
//
// # What is dangerous here
//
// XML signature validation is the single most dangerous thing in this
// repository to get wrong, and none of it is written here. It is
// github.com/crewjam/saml over github.com/russellhaering/goxmldsig, which is the
// pairing the Go ecosystem has actually reviewed. The XML signature wrapping
// attacks — moving a signed element so that the signature covers one thing and
// the parser reads another — are exactly the class of bug that hand-rolled
// validation produces and that a canonicalizing, reference-resolving
// implementation refuses.
//
// What this package adds around the library is the part the library cannot do
// for anybody:
//
//   - Replay prevention. SAML has no nonce. An assertion is a signed document
//     and it stays valid for its whole window, so anybody who obtains a copy can
//     present it again — and no library refuses that, because no library knows
//     where you keep state. See [Assertions] and 0007_saml_assertion.sql.
//   - The browser binding. An SP-initiated sign-in carries a sealed cookie
//     naming the request the assertion must answer, so an assertion minted for
//     somebody else cannot be delivered into your browser. See state.go.
//   - A bounded, configured clock skew. The library's is a package-level
//     variable with a three-minute default; see [maxAllowedSkew] for what is
//     done about that and why.
//   - Reading attributes out of five directories that all disagree about how to
//     spell an email address. See attributes.go.
//
// # What is deliberately not here
//
// Single logout. `POST /auth/logout` ends the Blacklight session and leaves the
// identity provider's alone, which is documented in docs/sso-saml.md rather than
// left for somebody to discover. Encrypted assertions beyond what the library
// decrypts out of the box, and the artifact binding, are also out — the
// published metadata advertises HTTP-POST at the assertion consumer and nothing
// else, so an identity provider is never offered a binding this deployment will
// not accept.
package saml
