# Single sign-on with SAML 2.0

Nobody chooses SAML in 2026. Enterprises still require it, so Blacklight is a SAML 2.0 service
provider as well as an OpenID Connect relying party, and the two are independent — configure one,
both, or neither.

If you have a choice, use [OpenID Connect](sso-oidc.md). It is simpler to configure, harder to
misconfigure dangerously, and does not require you to manage a certificate. This file is for when
you do not have a choice.

This is the operator's half. The design decisions behind it are in
[`internal/authn/saml`](../internal/authn/saml/doc.go); what a *session* is once somebody has signed
in is [`docs/security.md`](security.md).

## What you have to know first

**Single sign-on never replaces local sign-in.** Passwords keep working, and they are what gets you
in when the identity provider is down. Do not remove your local administrator account.

**A provider that is unreachable is not an outage.** The server starts without it, the login page
renders without it, and the SAML button disappears until the metadata can be read again. Nothing has
to be restarted when it comes back.

**Nothing is provisioned by default.** With `BLACKLIGHT_SAML_AUTO_PROVISION=false` (the default),
somebody the provider vouches for and Blacklight has never seen is refused with a message telling
them to ask an administrator, and no account is created.

**Logout is local only.** See [Single logout](#single-logout) — it is not implemented, and that is a
decision rather than an omission.

## The two URLs and the certificate

Everything an identity provider administrator needs is at one endpoint:

```
<BLACKLIGHT_BASE_URL>/api/v1/auth/saml/metadata
```

It is public, it needs no session, and it answers while the identity provider is unreachable — which
is exactly when somebody is fetching it. Most consoles accept the URL directly; the rest accept the
document you download from it. It carries the public certificate and never the private key.

If your console wants the values typed in by hand:

| Field | Value |
|---|---|
| Entity ID / Audience | `<BLACKLIGHT_BASE_URL>/api/v1/auth/saml/metadata` (or `BLACKLIGHT_SAML_ENTITY_ID` if you set one) |
| Assertion consumer service (ACS) | `<BLACKLIGHT_BASE_URL>/api/v1/auth/saml/acs`, **HTTP-POST binding** |
| NameID format | Persistent (see [How an account is decided](#how-an-account-is-decided)) |
| Signed assertions | Required. Blacklight refuses an unsigned one. |
| Single logout | Leave it empty |

The ACS URL must match byte for byte. It is checked against the assertion's `Recipient` and the
response's `Destination`, so a trailing slash or `http` where you meant `https` is a sign-in that
fails with "the assertion was rejected" and no clue which field was wrong except in the log.

## The service provider key pair

Blacklight signs its authentication requests, so it needs a certificate and a key. Self-signed is
correct here — nothing validates a chain, and what the identity provider stores is the exact
certificate you gave it.

```sh
openssl req -x509 -newkey rsa:2048 -nodes -days 1095 \
  -keyout /etc/blacklight/saml.key -out /etc/blacklight/saml.crt \
  -subj "/CN=blacklight.example.com"
chmod 600 /etc/blacklight/saml.key
```

They are **file paths** rather than pasted PEM, deliberately: a private key in an environment
variable is a private key in `docker inspect`, in a process listing on some systems, and in every
crash report that dumps the environment. The server refuses to start if the key file is readable by
anybody but its owner, and it will not start if the key is anything but RSA — the signature is
RSA-SHA256, which is what every identity provider expects from a service provider.

Note the expiry you chose. When it comes, the symptom is an identity provider refusing your
authentication requests; re-run the command and re-upload the metadata.

## The variables

Every one of these is in [`.env.example`](../.env.example) with the same prose. The identity
provider's metadata is the switch: with neither `BLACKLIGHT_SAML_IDP_METADATA_URL` nor
`BLACKLIGHT_SAML_IDP_METADATA_FILE` there is no SAML, and setting the others without one is a
startup error rather than a silence.

| Variable | Meaning |
|---|---|
| `BLACKLIGHT_SAML_IDP_METADATA_URL` | Where the identity provider publishes its metadata. Preferred: a rotated signing certificate reaches this deployment without anybody editing anything. |
| `BLACKLIGHT_SAML_IDP_METADATA_FILE` | The same document on disk, for a provider that publishes no URL. Set one of the two, never both. |
| `BLACKLIGHT_SAML_ENTITY_ID` | What this deployment calls itself. Unset means the metadata URL above, which is the conventional default. |
| `BLACKLIGHT_SAML_CERT_FILE` / `_KEY_FILE` | The PEM key pair. Both required. |
| `BLACKLIGHT_SAML_EMAIL_ATTRIBUTE` | Which attributes carry the address, best first. The defaults cover Keycloak, Entra ID, Okta, ADFS and the OASIS OIDs. |
| `BLACKLIGHT_SAML_NAME_ATTRIBUTE` | The same, for the display name. |
| `BLACKLIGHT_SAML_GROUPS_ATTRIBUTE` | The same, for group memberships. |
| `BLACKLIGHT_SAML_ROLE_MAP` | `group=role` pairs: `blacklight-admins=admin,staff=member`. Identical in form and meaning to the OIDC one. |
| `BLACKLIGHT_SAML_AUTO_PROVISION` | Whether a first sign-in creates an account. Default `false`. |
| `BLACKLIGHT_SAML_ALLOW_IDP_INITIATED` | Whether to accept a sign-in that starts at the provider's portal. Default `true`; see [below](#portal-sign-in-and-what-it-costs). |
| `BLACKLIGHT_SAML_CLOCK_SKEW` | How far the provider's clock may be from this one. Default `2m`, capped at `5m`. |

### Attribute names

The three attribute variables are **lists**, and the first name that is present wins. Both the
`Name` and the `FriendlyName` of an attribute are matched, and matching is case-insensitive, because
no two directories agree: the same address is `mail` at one, `urn:oid:0.9.2342.19200300.100.1.3` at
the next, and `http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress` at a third. The
defaults cover all of those; you should only need to touch these if your directory sends a name
nobody guessed.

Groups are read from the **first** listed attribute that is present, not the union of all of them —
a directory that sends both `groups` and `memberOf` is sending the same memberships spelled two
ways, and concatenating would map a role from a list with everything in it twice. Reordering the
list is how you choose.

## How an account is decided

Every sign-in, in this order:

1. **By NameID.** The `NameID` is what an existing link is found by, and it is the only value the
   provider promises not to reassign. Configure a **persistent** NameID if you have the choice; a
   transient one changes on every sign-in and every sign-in would provision a new account.
2. **By email.** No link yet, and an account here already holds the address the assertion carries:
   the login is attached to that account.
3. **Provisioning**, if `BLACKLIGHT_SAML_AUTO_PROVISION` is on. The new account has no local
   password, so it can only ever be signed in to through the provider.
4. **Refusal.** A `403` saying to ask an administrator. Nothing is written.

> **On step 2 and `email_verified`.** The OIDC path refuses to link by an *unverified* address,
> because a provider may vouch for one nobody proved. SAML has no such concept and Blacklight does
> not invent one: the assertion is a document signed by the one identity provider this deployment was
> configured against, carrying an attribute that provider's administrator chose to send. Somebody who
> can set the mail attribute in that directory can already mint an assertion for any NameID, so the
> address is exactly as trustworthy as the subject beside it. If your directory lets ordinary users
> edit their own mail attribute, that is worth changing at the directory.

A disabled account is refused whichever door it comes to.

### Roles

`BLACKLIGHT_SAML_ROLE_MAP` is the same mechanism as the OIDC one, applied by the same code, with the
same two deliberate non-behaviours: a group that is not in the mapping contributes nothing, and a
mapping that produces no administrators is allowed. It is evaluated on **every** sign-in, so removing
somebody from a group at the provider demotes them here at their next login — including out of
`admin`, which is the direction that matters.

Where several groups map, the strongest role wins.

### Second factors

Identical to OIDC: somebody who has enrolled an authenticator here is asked for a code whichever door
they came in through, and an account with no local password is exempt from being *required* to enrol
one. [`docs/sso-oidc.md`](sso-oidc.md#second-factors) has the argument.

## Portal sign-in, and what it costs

`BLACKLIGHT_SAML_ALLOW_IDP_INITIATED` is on by default, because clicking a Blacklight tile in the
provider's application portal is how a great many enterprises expect SAML to work.

It is a real tradeoff and worth understanding before you leave it on.

A sign-in that **starts here** is bound to the browser that started it. Blacklight sets a sealed
cookie naming the authentication request it issued, and the assertion has to answer that exact
request — so an attacker who signs in at your identity provider with their own account, captures the
assertion, and delivers it into your browser is refused. That attack is login CSRF, and the outcome
is that you spend the rest of your session signed in as them, entering things into their account.

A sign-in that **starts at the portal** answers no request, so it cannot carry that binding. This is
inherent in the profile and not something an implementation can fix. Everything else still applies —
the signature, the issuer, the audience, the recipient, the validity window, and the replay check —
but that one is gone.

Set it to `false` on a deployment nobody reaches from a portal, and point the tile at
`<BLACKLIGHT_BASE_URL>/api/v1/auth/saml/start` instead, which begins an ordinary sign-in.

## Assertions are single-use

SAML has no nonce. An assertion is a signed document and it stays a valid signed document for its
whole window, so anybody who obtains a copy — from a proxy log, a shared machine, a browser history —
can present it again. No library prevents this, because no library knows where you keep state.

Blacklight records every assertion ID it accepts and refuses one it has seen before. The record is a
table rather than memory, so a restart does not empty it, and rows are swept when they can no longer
be replayed. There is nothing to configure and nothing to schedule.

`BLACKLIGHT_SAML_CLOCK_SKEW` widens every validity window in an assertion, which means it also widens
the period an assertion is worth replaying. Two minutes is generous; the cap is five. If you need
more, the answer is NTP.

## Development: a real Keycloak in one command

```
docker compose --profile sso up keycloak
```

It imports [`deploy/keycloak/blacklight-realm.json`](../deploy/keycloak/README.md) into an in-memory
database on every start, so the realm is exactly what is in that file and nothing survives a restart.
The realm holds a SAML client alongside the OIDC one. The admin console is
<http://localhost:8081/admin> (`admin` / `admin`).

Generate a key pair and run Blacklight against it:

```sh
mkdir -p /tmp/blacklight-saml && cd /tmp/blacklight-saml
openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -keyout saml.key -out saml.crt -subj "/CN=blacklight.localhost"
chmod 600 saml.key

export BLACKLIGHT_ENV=development
export BLACKLIGHT_BASE_URL=http://localhost:8080
export BLACKLIGHT_SAML_IDP_METADATA_URL=http://localhost:8081/realms/blacklight/protocol/saml/descriptor
export BLACKLIGHT_SAML_CERT_FILE=/tmp/blacklight-saml/saml.crt
export BLACKLIGHT_SAML_KEY_FILE=/tmp/blacklight-saml/saml.key
export BLACKLIGHT_SAML_ROLE_MAP=blacklight-admins=admin,blacklight-users=member
export BLACKLIGHT_SAML_AUTO_PROVISION=true
make run
```

`rowan` / `blacklight` arrives as an `admin`, `sam` / `blacklight` as a `member`. The realm's own
README lists the third user and what each is for.

Two things about that realm are worth reading rather than copying:

- **The SAML client's ID is a URL.** In SAML the client ID *is* the entity ID, and Blacklight's
  default entity ID is its metadata URL — so the realm registers the client as
  `http://localhost:8080/api/v1/auth/saml/metadata`. Change `BLACKLIGHT_BASE_URL` and you have to
  change the client too, or the audience will not match.
- **Client signature verification is off in the realm.** Blacklight signs its authentication requests
  regardless, but the development realm cannot know your certificate at import time, so it does not
  check them. In production, upload the metadata and turn *Client signature required* on.

## Worked configuration

### Microsoft Entra ID

Entra is the commercial provider most likely to be the reason you are reading this.

1. **Enterprise applications → New application → Create your own application**, then *Integrate any
   other application you don't find in the gallery*.
2. **Single sign-on → SAML.** In *Basic SAML Configuration*, upload the metadata from
   `<BLACKLIGHT_BASE_URL>/api/v1/auth/saml/metadata` — Entra fills in the identifier and the reply
   URL from it. If you type them:
   - *Identifier (Entity ID)*: `https://blacklight.example.com/api/v1/auth/saml/metadata`
   - *Reply URL (ACS)*: `https://blacklight.example.com/api/v1/auth/saml/acs`
3. **Attributes & Claims.** Entra sends its claims under `http://schemas.xmlsoap.org/...` names,
   which the defaults already match. Add a **group claim**: *Add a group claim → Security groups*,
   and under *Advanced*, set the source attribute. **Group ID** sends object IDs — GUIDs — so your
   mapping maps GUIDs, which is exact and unreadable; `sAMAccountName` sends names and needs group
   writeback from an on-premises directory.
4. **SAML Certificates.** Copy the *App Federation Metadata Url* — that is
   `BLACKLIGHT_SAML_IDP_METADATA_URL`. Use the URL rather than downloading the certificate: Entra
   rotates its signing certificate, and a URL survives that where a file does not.
5. **Users and groups.** Entra refuses a sign-in from anybody not assigned to the application, which
   is a second gate in front of `BLACKLIGHT_SAML_AUTO_PROVISION`.

```sh
BLACKLIGHT_SAML_IDP_METADATA_URL=https://login.microsoftonline.com/<tenant-id>/federationmetadata/2007-06/federationmetadata.xml?appid=<app-id>
BLACKLIGHT_SAML_CERT_FILE=/etc/blacklight/saml.crt
BLACKLIGHT_SAML_KEY_FILE=/etc/blacklight/saml.key
BLACKLIGHT_SAML_ROLE_MAP=<group-object-id>=admin
BLACKLIGHT_SAML_AUTO_PROVISION=false
```

> Entra omits the group claim entirely for a user in more than about 150 groups, sending an overage
> pointer instead. Blacklight does not follow that pointer — it would mean a Graph call on every
> sign-in — so those people map to no group and keep the role they have. Use a filtered group claim
> if that is your directory.

### Okta

1. **Applications → Create App Integration → SAML 2.0.**
2. *Single sign-on URL*: `https://blacklight.example.com/api/v1/auth/saml/acs`.
   *Audience URI*: `https://blacklight.example.com/api/v1/auth/saml/metadata`.
   *Name ID format*: **Persistent**. *Application username*: Okta username or email.
3. **Attribute Statements**: `email` → `user.email`, `displayName` → `user.displayName`.
4. **Group Attribute Statements**: name `groups`, filter *Matches regex* `.*` or a tighter one.
5. **Sign On → View SAML setup instructions** has the *Identity Provider metadata* URL.

```sh
BLACKLIGHT_SAML_IDP_METADATA_URL=https://example.okta.com/app/<app-id>/sso/saml/metadata
BLACKLIGHT_SAML_ROLE_MAP=Blacklight-Admins=admin
```

### Keycloak

1. **Clients → Create client → SAML.** Client ID must be the entity ID:
   `https://blacklight.example.com/api/v1/auth/saml/metadata`.
2. *Valid redirect URIs*: `https://blacklight.example.com/api/v1/auth/saml/acs`.
   *Name ID format*: `persistent`. *Sign assertions*: **on**. *Client signature required*: **on**,
   after you have uploaded the metadata in step 4.
3. **Client scopes → \<client\>-dedicated → Add mapper**: a *User Property* mapper for `email`, a
   *User Attribute* mapper for `displayName`, and a *Group list* mapper named `groups` with
   *Full group path* **off**. Full path off matters: with it on the value is `/blacklight-admins`
   and your mapping has to say so.
4. **Keys → Import key → Certificate PEM**, or upload the metadata document from Blacklight, so
   Keycloak can verify our signed authentication requests.

```sh
BLACKLIGHT_SAML_IDP_METADATA_URL=https://keycloak.example.com/realms/blacklight/protocol/saml/descriptor
BLACKLIGHT_SAML_ROLE_MAP=blacklight-admins=admin,blacklight-users=member
```

## What is not supported

### Single logout

**Not implemented, and the metadata does not advertise it.** `POST /auth/logout` ends the Blacklight
session; the identity provider's session is untouched, so somebody who signs out here and clicks the
tile again is signed straight back in without being asked for anything.

That is the honest behaviour and it is worth saying plainly, because the alternative — a half-working
SLO endpoint — is worse than none. SAML single logout is a large surface (signed logout requests in
both directions, session index tracking, front-channel and back-channel bindings, and a partial
failure mode nobody agrees on) for a guarantee that is weaker than it looks.

To end somebody's access, disable their account. That closes every door at once, including the
sessions they already hold.

### Encrypted assertions

Only what the library decrypts out of the box. Assertions cross a TLS connection and are
audience-restricted, short-lived and single-use; encryption on top of that is a requirement some
organizations have rather than a security property this deployment lacks.

### The artifact binding, and SCIM

Not implemented. The published metadata advertises HTTP-POST at the assertion consumer and nothing
else, so an identity provider is never offered a binding this deployment will not accept. Accounts
arrive at first sign-in or are created by an administrator.

## When it does not work

The log is the first place to look. Every refusal is one line, and it names the check that failed —
the response says only "sign in to continue", deliberately, because a caller who can tell which half
of a forgery failed is a caller mapping the implementation.

| Symptom | Cause |
|---|---|
| No SAML button on the login page | The metadata could not be read. The log says why, at `warn`, naming the URL. It retries on its own. |
| `saml: the identity provider's metadata could not be read` | Wrong URL, or the provider is unreachable from the *server* — check egress, not just your browser. |
| "publishes no signing certificate" | The document is metadata for a service provider, or an aggregate. You want the identity provider's own descriptor. |
| `401` on the assertion consumer, first ever attempt | Almost always the ACS URL. It is compared against the assertion's `Recipient` and the response's `Destination` byte for byte — check the scheme, the host and the trailing slash. |
| `401` and the log says "audience restriction" | The entity ID registered at the provider is not `BLACKLIGHT_SAML_ENTITY_ID` (or the metadata URL, if you did not set one). |
| `401` and the log says "it has already been used" | Working as designed: an assertion is single-use. A browser resubmitting the form, or a bookmarked POST, produces this. Start again from the login page. |
| `401` and the log says "does not match any of the possible request IDs" | The pending-request cookie did not come back. See the cookie note below. |
| `403`, "you have no account on this Blacklight" | Working as configured: `BLACKLIGHT_SAML_AUTO_PROVISION` is off. |
| Everybody is a `member` | The group attribute is not arriving under a name in `BLACKLIGHT_SAML_GROUPS_ATTRIBUTE`. The provider's own "test SAML login" screen usually shows the raw assertion. |
| A new account on every sign-in | The NameID is transient. Configure a persistent one. |

> **The pending-request cookie.** `bl_saml` is the one cookie in this application that is
> `SameSite=None; Secure`, and it has to be: the assertion arrives as a cross-site POST, and browsers
> send neither Strict nor Lax cookies on one. `Secure` is therefore unconditional. Browsers treat
> `http://localhost` as a secure context, so development works; a development deployment served over
> plain `http` from any *other* host will drop the cookie and no sign-in will complete. Serve it over
> TLS, or use `localhost`.
