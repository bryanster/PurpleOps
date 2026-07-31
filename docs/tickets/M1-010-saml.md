# M1-010 — SAML 2.0 service provider

**Milestone:** M1 · **Size:** L · **Depends on:** M1-009

## Why

`PLAN.md` §4: "Retained for enterprises without OIDC." Nobody chooses SAML in 2026; enterprises
still require it. Build it after OIDC so the identity-linking and role-mapping code is already
shaped and can be shared rather than duplicated.

## Scope

**In**

- SP implementation using `crewjam/saml` — do not hand-roll XML signature validation. This is the
  single most dangerous thing in the codebase to get wrong.
- Config: IdP metadata URL **or** pasted metadata XML, SP entity ID, SP certificate/key, attribute
  names for email / display name / groups, `auto_provision`, group→role mapping.
- Endpoints: `GET /auth/saml/metadata` (SP metadata for the IdP admin), `GET /auth/saml/start`,
  `POST /auth/saml/acs` (assertion consumer), optional `/auth/saml/sls`.
- Assertion validation: signature (assertion **and/or** response — accept a signed assertion, reject
  an unsigned one either way), `Issuer`, `Audience`, `NotBefore`/`NotOnOrAfter`, `Recipient`,
  `InResponseTo` for SP-initiated, and **replay prevention** on assertion ID.
- Identity linking and role mapping reuse the M1-009 code paths — extract shared logic rather than
  copying it.
- SP-initiated and IdP-initiated login both supported (IdP-initiated skips `InResponseTo`; be
  explicit that this is intentional and note the tradeoff).
- `docs/sso-saml.md` with worked setup for one commercial IdP.

**Out**

- SAML single logout as a hard requirement — implement if straightforward, otherwise document that
  logout is local-only.
- Encrypted assertions beyond what the library supports out of the box.

## Acceptance criteria

- [ ] Round-trip works against a test IdP (`crewjam/saml`'s test harness, SimpleSAMLphp, or Keycloak
      in SAML mode). Name what you tested against.
- [ ] An **unsigned** assertion is rejected.
- [ ] An assertion signed by the wrong key is rejected.
- [ ] A tampered assertion (attribute modified after signing) is rejected.
- [ ] An expired assertion and one outside its `NotBefore` window are rejected.
- [ ] A replayed assertion (same ID, within validity) is rejected — implement and test the replay
      cache; the library does not do this for you.
- [ ] Wrong `Audience` or wrong `Recipient` is rejected.
- [ ] SP metadata at `/auth/saml/metadata` is valid XML that a real IdP accepts.
- [ ] Group attributes map to platform roles, including demotion on subsequent login.
- [ ] Clock skew tolerance is bounded and configurable (default ≤ 2 minutes).
- [ ] The SP private key is never logged or exposed via any endpoint.
- [ ] Misconfigured SAML doesn't break local login or OIDC.

## Tests

- Fixture-based tests using checked-in assertions (valid, unsigned, wrong-key, tampered, expired,
  replayed, wrong-audience) — one test per rejection case, each named for the attack it prevents.
- Metadata generation test.
- Shared linking/role-mapping tests exercised through the SAML path.

## Notes for the implementer

- Generate the test fixtures with a script committed alongside them, so they can be regenerated when
  certificates expire. A test suite that dies in two years because a fixture cert expired is a
  predictable and preventable annoyance.
- If any acceptance criterion above is hard to test with the chosen library, say so in the PR rather
  than dropping it. These cases *are* the ticket.
