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

- [x] Round-trip works against a test IdP (`crewjam/saml`'s test harness, SimpleSAMLphp, or Keycloak
      in SAML mode). Name what you tested against.
- [x] An **unsigned** assertion is rejected.
- [x] An assertion signed by the wrong key is rejected.
- [x] A tampered assertion (attribute modified after signing) is rejected.
- [x] An expired assertion and one outside its `NotBefore` window are rejected.
- [x] A replayed assertion (same ID, within validity) is rejected — implement and test the replay
      cache; the library does not do this for you.
- [x] Wrong `Audience` or wrong `Recipient` is rejected.
- [x] SP metadata at `/auth/saml/metadata` is valid XML that a real IdP accepts.
- [x] Group attributes map to platform roles, including demotion on subsequent login.
- [x] Clock skew tolerance is bounded and configurable (default ≤ 2 minutes).
- [x] The SP private key is never logged or exposed via any endpoint.
- [x] Misconfigured SAML doesn't break local login or OIDC.

## Tests

- [x] Fixture-based tests using checked-in assertions (valid, unsigned, wrong-key, tampered, expired,
  replayed, wrong-audience) — one test per rejection case, each named for the attack it prevents.
  *Deviated: a live signing harness instead of checked-in fixtures. See the implementation notes.*
- [x] Metadata generation test.
- [x] Shared linking/role-mapping tests exercised through the SAML path.

## Notes for the implementer

- Generate the test fixtures with a script committed alongside them, so they can be regenerated when
  certificates expire. A test suite that dies in two years because a fixture cert expired is a
  predictable and preventable annoyance.
- If any acceptance criterion above is hard to test with the chosen library, say so in the PR rather
  than dropping it. These cases *are* the ticket.

---

## Implementation notes

Everything in **Scope** landed. Four decisions deviate from the ticket as written, and one of them
is a genuine disagreement with it rather than a detail.

### The fixture corpus became a live signing harness

**The ticket asked for checked-in assertions; there are none.** What exists instead is
[`internal/authn/saml/samltest`](../../../internal/authn/saml/samltest/samltest.go): a real identity
provider that generates a key pair, publishes metadata over an `httptest` server, reads the
authentication request out of the redirect, and mints a signed assertion — or, on request, any of the
ways of minting one that must be refused. Each attack is one field of `samltest.Assertion` away from
the correct document beside it.

The ticket's own note is the argument for this. It says a fixture corpus needs a regeneration script
"so they can be regenerated when certificates expire", and that "a test suite that dies in two years
because a fixture cert expired is a predictable and preventable annoyance". A live harness does not
merely make that regeneration easy; it removes the failure. It also removes two others a corpus has:
a blob and the code that produced it drifting apart, and a signature that has to be recomputed by
hand whenever the assertion's shape changes.

The intent of the **Tests** section is met in full — one test per rejection case, each named for the
attack it prevents, in `internal/authn/saml/saml_test.go`:

| Test | Attack |
|---|---|
| `TestAnUnsignedAssertionIsRejected` | a document nothing signed |
| `TestAnAssertionSignedByTheWrongKeyIsRejected` | a perfect signature from a key the IdP does not publish |
| `TestATamperedAssertionIsRejected` | an attribute edited after signing |
| `TestAnExpiredAssertionIsRejected` | a captured assertion presented later |
| `TestAnAssertionOutsideItsNotBeforeWindowIsRejected` | one presented early |
| `TestAReplayedAssertionIsRejected` | the same assertion, twice |
| `TestAnAssertionForAnotherAudienceIsRejected` | one valid at a different service provider |
| `TestAnAssertionForAnotherRecipientIsRejected` | one addressed to a different ACS |
| `TestAnAssertionSentToAnotherDestinationIsRejected` | the same, at the response level |
| `TestAnAssertionAnsweringAnotherRequestIsRejected` | login CSRF |
| `TestACorruptPendingCookieIsRejectedRatherThanIgnored` | removing the browser binding by corrupting the cookie |

Plus `TestAPortalSignInIsStillSubjectToEveryOtherCheck`, which reruns four of them with
IdP-initiated sign-in enabled, because the library's own switch for that gives up more than it looks.

### Pasted metadata XML is a file path

**Scope** says "IdP metadata URL **or** pasted metadata XML". It is `BLACKLIGHT_SAML_IDP_METADATA_URL`
or `BLACKLIGHT_SAML_IDP_METADATA_FILE`, and pasting means saving the download to a file. A multi-line
XML document does not survive an environment variable — shells, compose files and orchestrators each
mangle it differently, and the failure mode is a signature that will not verify, which is the worst
possible symptom for a mangled trust anchor. Setting both is a startup error rather than a
precedence rule: they can describe different identity providers, and picking one silently would mean
trusting a certificate the operator believes is not trusted.

The service provider key pair is a pair of paths for a related reason (a private key in an
environment variable is a private key in `docker inspect`), and the server refuses to start if the
key file is readable by anybody but its owner.

### The library's clock skew is a package-level variable

`github.com/crewjam/saml` keeps `MaxClockSkew` and `MaxIssueDelay` in package variables. Setting
those from configuration would be a global mutated at construction — two providers or two parallel
tests would fight over it, and the race detector would be right about that.

So `internal/authn/saml` pins them **once, in `init`, to constants**: a five-minute ceiling, wide
enough that the library never refuses an assertion this deployment might accept. The *configured*
skew is then enforced by `Provider.checkFreshness`, which runs after the library and is strictly
narrower. `BLACKLIGHT_SAML_CLOCK_SKEW` defaults to 2m and is capped at 5m by `config.validate`. The
effective tolerance is always the configured one; the library is never the thing deciding it.

### IdP-initiated sign-in is per assertion, not per provider

`crewjam/saml`'s `AllowIDPInitiated` disables the `InResponseTo` checks on *every* assertion — both
the response's and the subject confirmation's. A deployment that wanted portal sign-in would
therefore also lose the browser binding on sign-ins that started here, which is the one check that
refuses login CSRF.

`Provider.consumer` builds a **copy** of the service provider per assertion and sets the flag from
whether a sealed pending-request cookie arrived. Sign-ins that can have the strong check keep it;
only the ones that structurally cannot give it up. A cookie that is present and will not open is
refused rather than demoted to IdP-initiated — otherwise the binding would be removable by anybody
who can make a browser send a corrupt cookie (`TestACorruptPendingCookieIsRejectedRatherThanIgnored`).

`BLACKLIGHT_SAML_ALLOW_IDP_INITIATED` defaults to `true`, because **Scope** asks for both flows to be
supported. The tradeoff is written out in `.env.example`, in `config.SAML` and in
[`docs/sso-saml.md`](../../sso-saml.md).

## Other things worth knowing before the next ticket

**The `bl_saml` cookie is `SameSite=None; Secure`, unconditionally.** It is the only cookie in the
application that is neither Strict nor Lax, and it is not a relaxation anybody chose: the assertion
arrives as a cross-site **POST**, and browsers send neither Strict nor Lax cookies on one — Lax covers
top-level GETs, which is what the OIDC callback is and this is not. `Secure` is unconditional because
browsers refuse `SameSite=None` without it. That works on `http://localhost` (a secure context) and
does not work on a development deployment served over plain http from another host. Documented rather
than papered over.

**Replay prevention is `app.saml_assertion` (migration 0007), swept by its own writes.** A table
rather than a map, because the window an assertion stays replayable in is comfortably longer than a
restart. `identity.SAMLAssertions.Consume` inserts and lets the primary key decide, inside the write
transaction — a read-then-write would let two copies of one assertion arriving together both find
nothing. Any failure to record is a refusal: a cache that cannot say "no" must not be treated as
having said "yes".

**`SafeReturnTo` moved to `internal/authn/returnto`.** M1-010 asked for shared logic to be extracted
rather than copied; this was the one piece of M1-009 that was genuinely duplicable. `oidc.SafeReturnTo`
and `oidc.ErrUnsafeReturnTo` remain as thin aliases, so nothing in the OIDC path changed. Identity
linking and role mapping needed no extraction at all — `authn.SignInWithFederatedIdentity` and
`config.RoleMap` were already protocol-agnostic, and SAML became a second caller.

**`EmailVerified` is `true` on the SAML path**, and `internal/httpapi/samlhandlers.go` argues it at
length. SAML has no such concept; the assertion is signed by the one configured identity provider,
and somebody who can set a mail attribute in that directory can already mint an assertion for any
NameID. Treating it as unverified would close no hole and would stop an enterprise's existing local
accounts from ever being reachable by single sign-on.

**The published metadata advertises HTTP-POST only.** The library adds an artifact binding by
default; `trimArtifactBinding` removes it, because the assertion consumer takes a form with a
`SAMLResponse` in it and nothing else. Advertising a binding the endpoint does not accept would let
an identity provider pick a flow that never reaches any code written here.

**Single logout is out, as **Scope** permits.** `docs/sso-saml.md` says so plainly, with the reason:
a half-working SLO endpoint is worse than none.

## Tested against

**Keycloak 26.4**, in SAML mode, from `deploy/keycloak/blacklight-realm.json` (the realm gained a
SAML client beside the existing OIDC one). Verified by hand, end to end, against a running server:

- `GET /auth/saml/start` → 302 to Keycloak with a signed `AuthnRequest` and the sealed cookie.
- Signing in as `rowan` (in `blacklight-admins`) → Keycloak POSTs the assertion to
  `/auth/saml/acs` → 302 to `/engagements`, `bl_session` and `bl_csrf` set, `bl_saml` cleared.
- `GET /auth/me` reports `rowan@example.com`, display name "Rowan Ash", `platformRole: admin` — the
  group mapping applied.
- Re-posting the same assertion → `401`, and the log names the replay cache.
- Feeding `/auth/saml/metadata` to Keycloak's own `client-description-converter` — the endpoint the
  admin console uses when you upload service provider metadata — produces a correct client: the
  entity ID, the ACS URL and the signing certificate all read back.
- The private key appears nowhere in the log or in any response.
