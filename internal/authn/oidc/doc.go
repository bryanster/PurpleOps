// Package oidc is the OpenID Connect half of single sign-on (M1-009): finding
// out what a provider's endpoints are, sending a browser to one of them, and
// turning what comes back into a verified set of claims.
//
// It does no I/O against this deployment's database and knows nothing about
// users, sessions or roles. What it produces is an [Identity] — "the provider at
// this issuer says this subject, with this address, is in these groups" — and
// what happens to that is internal/authn's decision. The split is the same one
// M1-006 made for TOTP: the protocol lives where it can be tested against a mock
// provider with a generated key pair, and the account rules live where they can
// be tested against a database.
//
// # What is not hand-rolled
//
// PLAN.md §4 replaces v1's hand-written OAuth2 flow, because that is where
// authentication bugs come from. So: discovery finds the endpoints, and
// github.com/coreos/go-oidc verifies the ID token's signature, issuer, audience
// and expiry. Nothing here parses a JWT by hand.
//
// What this package does own is the part a library cannot decide for you:
//
//   - The pending state. [Provider.Start] mints `state`, `nonce` and a PKCE
//     verifier, seals all three into one AEAD blob with an expiry, and hands it
//     back as a cookie. [Provider.Complete] opens it, compares `state` in
//     constant time, and refuses anything that has expired. The state is bound
//     to the browser because it is *in* the browser and nowhere else — there is
//     no server-side table to grow, and a state this deployment never issued
//     cannot be forged without the encryption key.
//   - The key set. See keys.go: keys are cached, an unknown key ID refetches
//     them so a rotation at the provider is handled without a restart, and the
//     refetch is rate limited so a stream of tokens signed by keys that do not
//     exist cannot turn this server into a load generator pointed at the
//     provider. That last property is why the library's own key set is not used.
//   - Discovery timing. A provider that is down must not stop this server from
//     starting or take local login with it, so discovery is attempted in the
//     background and retried on demand rather than being a startup requirement.
//
// # What it deliberately does not do
//
// IdP-initiated login. OpenID Connect has no such flow — an unsolicited
// assertion is exactly what `state` and `nonce` exist to reject, and accepting
// one would mean accepting a sign-in this deployment never started. A provider's
// "application tile" should link to <base>/api/v1/auth/oidc/start, which begins
// an ordinary flow.
package oidc
