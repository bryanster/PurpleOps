# The development realm

`blacklight-realm.json` is a Keycloak realm to develop single sign-on against —
OpenID Connect (M1-009) and SAML 2.0 (M1-010), side by side, so a deployment with
both configured can be exercised. It is imported on every start of the `sso`
compose profile, into an in-memory database:

```
docker compose --profile sso up keycloak
```

So the file is the whole truth about what exists, and nothing survives a
restart. That is what makes it reproducible, and it is also why **this must
never be deployed**: every credential in it is written down here, in a public
repository.

It deliberately mirrors a real registration rather than the easiest one that
works — a confidential client with a secret, PKCE required, one exact redirect
URI, and group memberships carried in a `groups` claim by a client scope of the
same name.

| What | Value |
|---|---|
| Realm | `blacklight`, at <http://localhost:8081/realms/blacklight> |
| Keycloak admin | `admin` / `admin`, at <http://localhost:8081/admin> |
| OIDC client | `blacklight`, secret `development-only-client-secret` |
| OIDC redirect URI | `http://localhost:8080/api/v1/auth/oidc/callback` |
| SAML client | `http://localhost:8080/api/v1/auth/saml/metadata` — in SAML the client ID *is* the entity ID |
| SAML ACS | `http://localhost:8080/api/v1/auth/saml/acs`, HTTP-POST |
| SAML metadata | <http://localhost:8081/realms/blacklight/protocol/saml/descriptor> |
| Groups | `blacklight-admins`, `blacklight-users` |

Three people to sign in as, all with the password `blacklight`:

| Username | Address | Verified | Group |
|---|---|---|---|
| `rowan` | rowan@example.com | yes | `blacklight-admins` |
| `sam` | sam@example.com | yes | `blacklight-users` |
| `jules` | jules@example.com | **no** | none |

`jules` is the pair of cases the rules turn on: an unverified address, which
must not link to an existing local account, and no group at all, which must map
to no role rather than to a weak one.

The SAML client sends `email`, `displayName` and `groups` as Basic-format
attributes, and a **persistent** NameID — which is what an account is linked by,
and what stops every sign-in provisioning a new one.

*Client signature required* is **off** on the SAML client. Blacklight signs its
authentication requests either way, but this file cannot know the certificate you
generated, so the realm does not check them. A production Keycloak should have it
on, with the service provider metadata uploaded — see the walkthroughs.

[`docs/sso-oidc.md`](../../docs/sso-oidc.md) and
[`docs/sso-saml.md`](../../docs/sso-saml.md) are the walkthroughs, including the
environment to run Blacklight with.
