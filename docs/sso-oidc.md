# Single sign-on with OpenID Connect

Blacklight signs people in through any provider that publishes an OpenID Connect discovery
document: Keycloak, Entra ID, Okta, Google, Authentik, Auth0. There is one integration and no
per-provider code — configure the issuer, and discovery finds the rest.

This file is the operator's half. The design decisions behind it are in
[`internal/authn/oidc`](../internal/authn/oidc/doc.go); what a *session* is once somebody has signed
in is [`docs/security.md`](security.md) and [`docs/http.md`](http.md).

## What you have to know first

**Single sign-on never replaces local sign-in.** Passwords keep working, and they are what gets you
in when the identity provider is down. Do not remove your local administrator account.

**A provider that is unreachable is not an outage.** The server starts without it, the login page
renders without it, and the sign-on button disappears until it answers again. Nothing has to be
restarted when it comes back.

**Nothing is provisioned by default.** With `BLACKLIGHT_OIDC_AUTO_PROVISION=false` (the default),
somebody the provider vouches for and Blacklight has never seen is refused with a message telling
them to ask an administrator, and no account is created. Turn it on only if everybody who can sign
in at the provider *should* have an account here.

## The variables

Every one of these is in [`.env.example`](../.env.example) with the same prose. `BLACKLIGHT_OIDC_ISSUER`
is the switch: with it unset there is no single sign-on, and setting any of the others without it is
a startup error rather than a silence.

| Variable | Meaning |
|---|---|
| `BLACKLIGHT_OIDC_ISSUER` | The issuer identifier. Discovery is `<issuer>/.well-known/openid-configuration`, and every ID token's `iss` must equal this **byte for byte** — do not add or remove a trailing slash. |
| `BLACKLIGHT_OIDC_CLIENT_ID` | This deployment's client registration. Not a secret. |
| `BLACKLIGHT_OIDC_CLIENT_SECRET` | The secret the provider issued. Leave unset for a public client; PKCE protects the code either way. |
| `BLACKLIGHT_OIDC_SCOPES` | Default `openid profile email groups`. Must include `openid`. |
| `BLACKLIGHT_OIDC_GROUPS_CLAIM` | Which claim carries group memberships. Default `groups`. |
| `BLACKLIGHT_OIDC_ROLE_MAP` | `group=role` pairs, comma-separated: `blacklight-admins=admin,staff=member`. |
| `BLACKLIGHT_OIDC_AUTO_PROVISION` | Whether a first-time sign-in creates an account. Default `false`. |

The redirect URI to register at the provider is built from `BLACKLIGHT_BASE_URL`:

```
<BLACKLIGHT_BASE_URL>/api/v1/auth/oidc/callback
```

It must match exactly — the provider compares it as a string, and so does the token exchange.

## How an account is decided

Every sign-in, in this order:

1. **By subject.** The `sub` claim is the only identifier a provider promises never to reuse or
   reassign, so it is the only thing an existing link is found by.
2. **By verified email.** No link yet, and the provider says the address is verified *and* an account
   here already holds it: the login is attached to that account. An **unverified** address links
   nothing — anybody who can type an address at the provider would otherwise be able to claim the
   matching account here.
3. **Provisioning**, if `BLACKLIGHT_OIDC_AUTO_PROVISION` is on. The new account has no local
   password, so it can only ever be signed in to through the provider.
4. **Refusal.** A `403` saying to ask an administrator. Nothing is written.

A disabled account is refused whichever door it comes to. Disabling somebody in Blacklight closes
single sign-on for them too.

### Roles

`BLACKLIGHT_OIDC_ROLE_MAP` is evaluated on **every** sign-in, not only the first. Removing somebody
from a group at the provider demotes them here at their next login, including out of `admin` — which
is the direction that matters, because an integration that only promotes is one where revoking access
at the directory does nothing.

Two deliberate non-behaviours:

- **A group that is not in the mapping contributes nothing.** Somebody whose groups are all unmapped
  keeps the role they have. "None of your groups is mapped" is not the same fact as "your groups say
  member", and treating it as one would demote every administrator on a deployment with no mapping.
- **A mapping that produces no administrators is allowed.** It is a mapping, not a safety mechanism;
  your local administrator account is the fallback.

Where several groups map, the strongest role wins, so the order the provider lists them in decides
nothing.

### Second factors

Somebody who has **enrolled an authenticator here** is asked for a code whichever door they came in
through: the provider sends them back, and they land on the code entry page rather than signed in.
Enrolling one was a decision they made, and single sign-on is not a way around it
([`docs/security.md`](security.md)).

Being **required** to enrol one is a different question, and an account with no local password is
exempt from it. There is no local sign-in for a local second factor to stand behind, so for SSO-only
accounts the second factor is the provider's business. If your provider enforces MFA, that is where
to enforce it.

## Development: a real Keycloak in one command

```
docker compose --profile sso up keycloak
```

It imports [`deploy/keycloak/blacklight-realm.json`](../deploy/keycloak/README.md) into an in-memory
database on every start, so the realm is exactly what is in that file and nothing survives a restart.
The admin console is <http://localhost:8081/admin> (`admin` / `admin`).

Then run Blacklight against it:

```sh
export BLACKLIGHT_ENV=development
export BLACKLIGHT_BASE_URL=http://localhost:8080
export BLACKLIGHT_OIDC_ISSUER=http://localhost:8081/realms/blacklight
export BLACKLIGHT_OIDC_CLIENT_ID=blacklight
export BLACKLIGHT_OIDC_CLIENT_SECRET=development-only-client-secret
export BLACKLIGHT_OIDC_SCOPES="openid profile email"
export BLACKLIGHT_OIDC_ROLE_MAP=blacklight-admins=admin,blacklight-users=member
export BLACKLIGHT_OIDC_AUTO_PROVISION=true
make run
```

`rowan` / `blacklight` arrives as an `admin`, `sam` / `blacklight` as a `member`. The realm's own
README lists the third user and what each is for.

Two things about that environment are worth reading rather than copying:

- **`BLACKLIGHT_OIDC_SCOPES` drops `groups`.** Keycloak has no built-in scope by that name, and
  asking for a scope a Keycloak realm does not define is `invalid_scope` — the sign-in fails at the
  provider with an error that does not say which scope was wrong. The development realm attaches the
  group mapper to the *client* instead, so the claim arrives without a scope being requested for it.
  On a realm where you have created a `groups` client scope, keep the default list.
- **The issuer is `localhost`, not a container name.** It is compared against what a browser types
  *and* what this server fetches, so both have to reach the provider by the same URL. This is the
  single most common cause of "the issuer did not match" when Keycloak runs in Docker.

## Worked configuration

### Keycloak

In the realm you are using:

1. **Clients → Create client.** Client ID `blacklight`, client authentication **on** (confidential),
   standard flow only — turn off direct access grants and implicit.
2. **Valid redirect URIs**: `https://blacklight.example.com/api/v1/auth/oidc/callback`, exactly.
3. **Advanced → Proof Key for Code Exchange**: `S256`. Blacklight always sends PKCE; requiring it at
   the provider means a client that does not is refused rather than downgraded.
4. **Credentials** tab: copy the secret into `BLACKLIGHT_OIDC_CLIENT_SECRET`.
5. **Groups.** Either create a `groups` client scope with an *oidc-group-membership-mapper* (claim
   name `groups`, full path **off**) and keep `groups` in `BLACKLIGHT_OIDC_SCOPES`, or add the same
   mapper directly to the client and drop `groups` from the scopes, as the development realm does.
   Full path off matters: with it on, the claim is `/blacklight-admins` and your mapping has to say
   so.

```sh
BLACKLIGHT_OIDC_ISSUER=https://keycloak.example.com/realms/blacklight
BLACKLIGHT_OIDC_CLIENT_ID=blacklight
BLACKLIGHT_OIDC_CLIENT_SECRET=…
BLACKLIGHT_OIDC_ROLE_MAP=blacklight-admins=admin,blacklight-users=member
```

### Microsoft Entra ID

1. **App registrations → New registration.** Redirect URI of type *Web*:
   `https://blacklight.example.com/api/v1/auth/oidc/callback`.
2. **Certificates & secrets → New client secret.** Copy the *value*, not the ID. Note the expiry:
   Entra secrets expire, and the symptom is every sign-in failing at the token exchange on a date
   nobody wrote down.
3. **Token configuration → Add groups claim.** Choose **Security groups** and, under *ID*, select
   **Group ID** or **sAMAccountName**. Group ID gives you object IDs — GUIDs — in the claim, and your
   mapping then maps GUIDs, which is exact and unreadable. sAMAccountName gives you names and needs
   group writeback from an on-premises directory.
4. The issuer is the **v2.0** one. The v1 endpoint issues tokens whose `iss` does not match it, and
   they are refused.

```sh
BLACKLIGHT_OIDC_ISSUER=https://login.microsoftonline.com/<tenant-id>/v2.0
BLACKLIGHT_OIDC_CLIENT_ID=<application-client-id>
BLACKLIGHT_OIDC_CLIENT_SECRET=…
BLACKLIGHT_OIDC_SCOPES="openid profile email"
BLACKLIGHT_OIDC_ROLE_MAP=<group-object-id>=admin
```

`groups` is dropped from the scopes for the same reason as Keycloak: it is not a scope Entra defines,
and the claim is configured in step 3 rather than requested. If your tenant emits **app roles**
instead of groups, set `BLACKLIGHT_OIDC_GROUPS_CLAIM=roles` and map the role names.

> Entra omits the `groups` claim entirely for a user in more than about 150 groups, sending an
> `_claim_names` overage pointer instead. Blacklight does not follow that pointer — it would mean a
> Graph call on every sign-in — so those people map to no group and keep the role they have. Use app
> roles, or a filtered groups claim, if that is your directory.

### Okta

1. **Applications → Create App Integration → OIDC, Web Application**, authorization code only.
   Sign-in redirect URI: `https://blacklight.example.com/api/v1/auth/oidc/callback`.
2. **Security → API → Authorization Servers**. The issuer is that server's URI — the `default` one
   is `https://<org>.okta.com/oauth2/default`, and the org-level one is `https://<org>.okta.com`.
   Use whichever you registered the app against; they issue different `iss` values.
3. Add a **groups claim** to the ID token on that authorization server: name `groups`, value type
   *Groups*, filter *Matches regex* `.*` (or a tighter expression).

```sh
BLACKLIGHT_OIDC_ISSUER=https://example.okta.com/oauth2/default
BLACKLIGHT_OIDC_CLIENT_ID=…
BLACKLIGHT_OIDC_CLIENT_SECRET=…
BLACKLIGHT_OIDC_ROLE_MAP=Blacklight-Admins=admin
```

### Google Workspace

Google issues no group claim at all, so role mapping cannot be used: leave `BLACKLIGHT_OIDC_ROLE_MAP`
unset and manage roles in Blacklight. Everything else is ordinary.

```sh
BLACKLIGHT_OIDC_ISSUER=https://accounts.google.com
BLACKLIGHT_OIDC_CLIENT_ID=….apps.googleusercontent.com
BLACKLIGHT_OIDC_CLIENT_SECRET=…
BLACKLIGHT_OIDC_SCOPES="openid profile email"
```

## What is not supported

**IdP-initiated login.** OpenID Connect has no such flow, and an unsolicited assertion is precisely
what `state` and `nonce` exist to reject. Point the provider's application tile at
`<BLACKLIGHT_BASE_URL>/api/v1/auth/oidc/start`, which begins an ordinary sign-in.

**SCIM provisioning.** Out of scope for v1. Accounts arrive at first sign-in or are created by an
administrator.

**Single logout.** Signing out of Blacklight ends the Blacklight session and nothing else. Signing
out at the provider does not end sessions here; revoke them by disabling the account.

## When it does not work

The log is the first place to look — every refusal is one line, and it says which check failed.

| Symptom | Cause |
|---|---|
| No sign-on button on the login page | The provider was not discovered. The log says why, at `warn`, naming the issuer. It retries on its own. |
| `oidc: the identity provider could not be reached` | The issuer URL is wrong, or the provider is unreachable from the *server* — check egress, not just your browser. |
| Discovery fails with an issuer mismatch | `BLACKLIGHT_OIDC_ISSUER` is not byte-for-byte the `issuer` in the discovery document. Usually a trailing slash, or Keycloak behind a hostname it does not know it has. |
| `invalid_scope` at the provider | A scope in `BLACKLIGHT_OIDC_SCOPES` is not one the provider defines. See the Keycloak and Entra notes above. |
| `invalid_redirect_uri` | The registered URI is not `<BLACKLIGHT_BASE_URL>/api/v1/auth/oidc/callback` exactly. |
| Everyone is a `member` | The groups claim is not arriving. Decode an ID token and look; then check `BLACKLIGHT_OIDC_GROUPS_CLAIM`, and whether the provider is sending full group paths. |
| `403`, "you have no account on this Blacklight" | Working as configured: `BLACKLIGHT_OIDC_AUTO_PROVISION` is off and nobody has created the account. |
| `401` on the callback, "sign in to continue" | The callback did not match a sign-in this browser started: an expired attempt, a bookmarked callback URL, or a state cookie a browser refused. Start again from the login page. |
