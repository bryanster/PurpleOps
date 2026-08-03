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

- [ ] The full flow works end to end against a real IdP in a dev container (Keycloak or Authentik in
      compose, as a dev-only profile). State which you tested against.
- [ ] `state` is single-use, bound to the browser (cookie), and expires. A replayed or missing
      `state` is rejected.
- [ ] A mismatched `nonce` is rejected.
- [ ] An ID token signed by an unknown key, expired, or with the wrong `aud` is rejected — one test
      each. These are the tests that matter; do not skip them because "the library handles it".
- [ ] Key rotation at the IdP is handled without a restart (JWKS refetch on unknown `kid`, with
      rate limiting so an attacker can't force unbounded fetches).
- [ ] Login as an existing local user with a matching **verified** email links the identity rather
      than creating a duplicate; an unverified email does not link.
- [ ] Group changes at the IdP change the platform role on next login, including demotion from
      admin.
- [ ] `auto_provision: false` + unknown user → 403 with a clear message, and no user row created.
- [ ] The callback cannot be used to redirect to an arbitrary URL (open-redirect check on any
      `return_to` parameter — allowlist relative paths only).
- [ ] The client secret never appears in a log, error, or API response.
- [ ] With OIDC misconfigured, local login still works and the OIDC button is hidden — a broken IdP
      config must not lock everyone out.

## Tests

- Unit tests against a **mock IdP** with a locally-generated key pair: happy path, each rejection
  case above, key rotation.
- Linking and provisioning tests.
- Role-mapping table test including demotion.
- Open-redirect test.
