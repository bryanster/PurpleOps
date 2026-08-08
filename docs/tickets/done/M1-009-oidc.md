# M1-009 — OIDC login with discovery and group→role mapping

**Milestone:** M1 · **Size:** L · **Depends on:** M1-003, M1-012

## Why

`PLAN.md` §4 replaces v1's "hand-rolled generic OAuth2 flow" with discovery-based OIDC, because
hand-rolled OAuth2 is where authentication bugs come from — and because discovery is what makes
"works with Entra, Okta, Google, Keycloak, Authentik" a configuration exercise rather than five
integrations.

## Scope

**In**

- Config: issuer URL, client ID, client secret, scopes (default `openid profile email groups`),
  optional group claim name, group→role mapping, `auto_provision` flag.
- Discovery via `/.well-known/openid-configuration`, with JWKS fetch and **key caching + rotation
  handling**. Do not pin a key.
- Authorization Code flow with **PKCE**, `state`, and `nonce`.
- Endpoints: `GET /auth/oidc/start` (redirects), `GET /auth/oidc/callback`.
- ID token validation: signature, `iss`, `aud`, `exp`, `nbf`, `nonce`. Use a library
  (`coreos/go-oidc`) — do not verify JWTs by hand.
- Account linking via `identity(provider='oidc', subject=sub)` from `M1-001`. Link by `sub`, and
  by verified email **only** when the ID token's `email_verified` is true. Never by unverified email.
- Provisioning: create a user on first login when `auto_provision` is on; otherwise 403 with a
  message telling them to ask an admin.
- Role mapping: IdP groups → platform role, evaluated on **every** login so a revocation in the IdP
  takes effect here. Unmapped groups → `member`. A mapping that would produce zero admins is
  allowed; a mapping table is not a safety mechanism.
- `docs/sso-oidc.md` with worked configuration for at least two providers.

**Out**

- SCIM provisioning. Out of scope for v1.
- IdP-initiated login. OIDC doesn't really have it; say so.

## Acceptance criteria

- [x] The full flow works end to end against a real IdP in a dev container (Keycloak or Authentik in
      compose, as a dev-only profile). State which you tested against.
- [x] `state` is single-use, bound to the browser (cookie), and expires. A replayed or missing
      `state` is rejected.
- [x] A mismatched `nonce` is rejected.
- [x] An ID token signed by an unknown key, expired, or with the wrong `aud` is rejected — one test
      each. These are the tests that matter; do not skip them because "the library handles it".
- [x] Key rotation at the IdP is handled without a restart (JWKS refetch on unknown `kid`, with
      rate limiting so an attacker can't force unbounded fetches).
- [x] Login as an existing local user with a matching **verified** email links the identity rather
      than creating a duplicate; an unverified email does not link.
- [x] Group changes at the IdP change the platform role on next login, including demotion from
      admin.
- [x] `auto_provision: false` + unknown user → 403 with a clear message, and no user row created.
- [x] The callback cannot be used to redirect to an arbitrary URL (open-redirect check on any
      `return_to` parameter — allowlist relative paths only).
- [x] The client secret never appears in a log, error, or API response.
- [x] With OIDC misconfigured, local login still works and the OIDC button is hidden — a broken IdP
      config must not lock everyone out.

## Tests

- Unit tests against a **mock IdP** with a locally-generated key pair: happy path, each rejection
  case above, key rotation.
- Linking and provisioning tests.
- Role-mapping table test including demotion.
- Open-redirect test.

---

## Implementation notes

Merged as `feat: oidc discovery login and group→role mapping (M1-009)`.

### Tested against

**Keycloak 26.4**, in the `sso` compose profile added by this ticket
(`docker compose --profile sso up keycloak`, realm in `deploy/keycloak/`). Driven as a browser
would: `/auth/oidc/start`, the real Keycloak login form, the callback. What was exercised against
it, beyond the unit tests:

- a first sign-in provisioning an account, with `blacklight-admins` mapping to `admin` and
  `blacklight-users` to `member`;
- **demotion**: removing the account from `blacklight-admins` at Keycloak and signing in again,
  which moved it to `member` and logged the change;
- `auto_provision: false` with an unknown person — `403`, the "ask an administrator" message, and
  no row written;
- an open-redirect attempt on `return_to` — `400` naming the parameter;
- a replayed callback — `401`;
- **a real key rotation**, by adding a second active RSA key to the realm and signing in again with
  nothing restarted.

### Deviations and decisions

**The JWKS refetch interval is 5 seconds, not a minute.** The live rotation above is why. The first
version rate-limited refetches to one a minute, which is what the ticket's "rate limiting" asks for
and which turned a real Keycloak rotation into a full minute of refused sign-ins — the refetch a
rotation needs is the same refetch the limit refuses. Five seconds still bounds an attacker to
twelve requests a minute (with singleflight collapsing bursts), which is less traffic than a health
check, and makes a rotation invisible. The unit test that missed this now runs with the limit *on*
and a fake clock: see `TestARotationIsHonouredAsSoonAsTheLimitAllows`.

**The library's key set is not used.** `go-oidc`'s `RemoteKeySet` refetches on every unrecognised
`kid` with nothing in front of it, so a stream of tokens signed by keys that do not exist turns this
server into a load generator aimed at the identity provider. `internal/authn/oidc/keys.go` is that
type with a rate limit and an algorithm allowlist; everything else about verification is the
library's.

**`GET /auth/providers` was added**, which the ticket does not name. The acceptance criterion "the
OIDC button is hidden" needs an API behind it, and the login page (M1-017) has to know what to draw
before anybody has signed in. It is public, lists local sign-in and every *reachable* provider, and
deliberately does not name the issuer.

**The pending state is a sealed cookie, not a table.** `state`, `nonce` and the PKCE verifier are
AEAD-sealed under a key derived for this purpose alone (`secrets.NewFor`) and travel in `bl_oidc`.
Nothing to sweep, nothing another request can read, and a state this deployment did not issue cannot
be forged. It is `SameSite=Lax` — the one cookie in the application that is not `Strict` — because a
browser does not attach a `Strict` cookie to the top-level navigation the provider sends it on.

**Discovery is lazy and rate-limited, never a startup condition.** A provider that is down must not
stop the server booting, because the login page it would have served is the one that still works.
The attempt at startup runs in the background; `/auth/providers` waits at most two seconds for one
already in flight, and `/auth/oidc/start` waits for it properly.

**A second factor still applies to a federated sign-in.** An account with a confirmed authenticator
is answered the same `mfa_required` a local sign-in gets and lands on the code entry page — it is
the same `completeSignIn` for both paths, so there is no second copy of M1-006's and M1-008's rules
to get wrong. An account with *no local password* remains exempt from being required to enrol, which
is the rule `MFAPolicy.Requires` already had.

**`config.ForeignSecret`** is new: a credential this deployment was given rather than one it
generated. It redacts exactly as `config.Secret` does and enforces none of the strength policy —
refusing to start because the identity provider's secret is short, or contains a word on the weak
list, would be rejecting a credential nobody here can regenerate.

**The development realm drops `groups` from the scope list.** Keycloak defines no scope by that
name, and asking for one it does not define is `invalid_scope` at the provider. The realm attaches
the group mapper to the client instead; `docs/sso-oidc.md` gives both configurations and keeps the
default scope list for the providers where it is right.

### Not done here

- SCIM and IdP-initiated login: out of scope, and `docs/sso-oidc.md` says so and why.
- Single logout: not in the ticket. Recorded in the same section of that document so the next
  reader does not assume it.
- The login page itself. `GET /auth/providers` is the contract it consumes; drawing the button is
  M1-017.
