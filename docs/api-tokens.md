# Service tokens

A service token is how a script, a pipeline or another system talks to Blacklight. The REST API is
the only supported integration surface, and these are the credentials for it.

This file is the operator's and integrator's half. The design decisions behind it are in
[`internal/authn/servicetoken`](../internal/authn/servicetoken/doc.go); what a token is *permitted*
to do is [`docs/authz.md`](authz.md), which is rendered from the rule table the server enforces.

## What you have to know first

**A token can never exceed the person who owns it.** Every request it makes is decided against its
owner's permissions as they stand *at that moment*, not as they stood when the token was created.
Demote somebody and every token they hold narrows immediately. Disable their account and every token
they hold stops working, at its next request, with nothing to revoke.

**The secret is shown once.** The creation response is the only place it exists outside your client.
The server stores a hash and could not produce the value again if asked. A token you did not save is
a token you replace.

**Every token expires.** The maximum is a year, and there is no "never". A credential with no expiry
is a credential nobody remembers to revoke.

**Tokens cannot manage tokens.** Creating and revoking take a signed-in session. A leaked token
therefore cannot mint a longer-lived replacement for itself and outlive the revocation of the
original — the fence the scope list and the owner's role would both let past.

## Creating one

Sign in to the web interface as the account the token should act as, then:

```bash
curl -sS -X POST https://blacklight.example.com/api/v1/auth/tokens \
  -H 'Content-Type: application/json' \
  -H "X-CSRF-Token: $CSRF" \
  --cookie "bl_session=$SESSION; bl_csrf=$CSRF" \
  -d '{
        "name": "nightly coverage export",
        "scopes": ["engagements:read", "reports:read"],
        "expiresAt": "2027-03-01T00:00:00Z"
      }'
```

```json
{
  "serviceToken": {
    "id": "0192f1a0-0000-7000-8000-000000000001",
    "name": "nightly coverage export",
    "prefix": "K7QM4TZB2A",
    "scopes": ["engagements:read", "reports:read"],
    "status": "active",
    "createdAt": "2026-03-01T09:14:22Z",
    "expiresAt": "2027-03-01T00:00:00Z"
  },
  "token": "bl_K7QM4TZB2A_2VHQ5XKPYD3JW8N6RTFA9CBEM4SZUG7LH2QX3KVD5RY"
}
```

Save `token` now. It appears in no other response, no log line and no activity entry.

### The shape of a token

`bl_<prefix>_<secret>`.

- `bl_` marks it, so a secret scanner — GitHub push protection, a pre-commit hook, an operator
  grepping a CI log — can recognise one that has escaped.
- `<prefix>` is public, unique, and printed in the listing. It is how a token found somewhere it
  should not be is matched to a row without anybody handling its secret.
- `<secret>` is the credential.

## Using one

```bash
curl -sS https://blacklight.example.com/api/v1/settings/mfa \
  -H "Authorization: Bearer $BLACKLIGHT_TOKEN"
```

No cookie, and no CSRF header — that check exists for credentials a browser attaches on its own, and
nothing attaches a bearer token for you.

### What the answers mean

| Status | `code` | What to do |
|---|---|---|
| `401` | `unauthenticated` | The token is unknown, malformed, expired, revoked, or its owner has been disabled. **Do not retry** — get a new token. |
| `403` | `forbidden` | The token is fine. Either it lacks the scope this endpoint needs, or its owner is not permitted to do this. Retrying will not help; the fix is a new token with the right scopes, or a permission change for its owner. |
| `429` | `rate_limited` | Too many failed attempts from this token's prefix or this address. `Retry-After` says how long, and retrying sooner does not shorten it. |

The `401`/`403` split is worth building against: `401` means "this credential is dead", `403` means
"this credential is alive and this is not for it".

## Scopes

Scopes are few and blunt on purpose. A scope per endpoint would be a second permission model to keep
in step with the real one.

| Scope | Covers |
|---|---|
| `engagements:read` | Reading engagements, members, executions. |
| `engagements:write` | Creating and changing engagements, members, executions, findings, comments. |
| `content:read` | Reading the shared technique and test-case library. |
| `content:sync` | Syncing that library from upstream. |
| `reports:read` | Reading and exporting reports. |
| `reports:write` | Publishing reports. |
| `admin:read` | Reading accounts, platform settings and the activity log. |
| `admin:write` | Changing accounts and platform settings. |

Which action each one gates is in [`docs/authz.md`](authz.md), in the same table the server decides
from. A scope is never a grant on its own: holding `admin:write` permits nothing your account could
not already do.

### Binding a token to one engagement

```json
{ "name": "acme retest runner",
  "scopes": ["engagements:write"],
  "engagementId": "0192f1a0-0000-7000-8000-00000000abcd",
  "expiresAt": "2026-09-01T00:00:00Z" }
```

A bound token reaches that engagement and nothing else — not another engagement, and not the
installation. Requests naming a different engagement answer `404`, the same as an engagement that
does not exist, so a bound token cannot be used to find out which engagements are real.

Binding only ever subtracts. It cannot let a token into an engagement its owner is not a member of.

## Rotating

There is no rotate endpoint, and that is deliberate: rotation is a create followed by a revoke, and
doing it in that order means no gap where the integration has no working credential.

1. Create the replacement.
2. Deploy it wherever the old one is configured.
3. Confirm the integration is working on the new one.
4. Revoke the old one.

`lastUsedAt` in the listing is how you check step 3 — but it is written back at most once a minute,
so give it a minute before concluding a token is idle.

## Revoking

```bash
curl -sS -X DELETE "https://blacklight.example.com/api/v1/auth/tokens/$TOKEN_ID" \
  -H "X-CSRF-Token: $CSRF" \
  --cookie "bl_session=$SESSION; bl_csrf=$CSRF"
```

`$TOKEN_ID` is the `id` from `GET /auth/tokens` — never the prefix and never the secret. Revocation
takes effect on the token's next request; nothing is cached anywhere.

Revoking a token that is already revoked answers `204` and keeps the original revocation time.

## Listing

```bash
curl -sS https://blacklight.example.com/api/v1/auth/tokens \
  --cookie "bl_session=$SESSION"
```

Your own tokens, newest first, including expired and revoked ones — you cannot decide what to rotate
without seeing what ended and when. `status` is `active`, `expired` or `revoked`.

There is no parameter here that names another account, for an administrator either. Administrators
have their own pair of endpoints; see below.

## For administrators: somebody else's tokens

These two are the incident case, and they need a signed-in administrator — a service token cannot
call either of them, whatever it is scoped for and however senior its owner. A leaked credential
must not be able to end every other credential in the installation.

```bash
# What does this account hold, and when was each last used?
curl -sS "https://blacklight.example.com/api/v1/users/$USER_ID/tokens" \
  --cookie "bl_session=$SESSION"

# End one of them.
curl -sS -X DELETE "https://blacklight.example.com/api/v1/users/$USER_ID/tokens/$TOKEN_ID" \
  -H "X-CSRF-Token: $CSRF" \
  --cookie "bl_session=$SESSION; bl_csrf=$CSRF"
```

The listing is the same shape as `GET /auth/tokens` and reads the same rows, so an administrator and
an owner never disagree about what is revoked. It carries no secret, because none is stored.

`$TOKEN_ID` has to belong to `$USER_ID`. A token identifier belonging to a different account answers
`404`, exactly as an invented one does — this is not a way to revoke by identifier alone.

Revoking here is the same act the owner's own revocation is: one row, one timestamp, taking effect on
the token's next request. What differs is the record of it. The token's `revokedBy` names the account
that ended it, so an administrator stepping in is tellable from a routine rotation afterwards, and
the activity log files it under `token.admin_revoked` rather than `token.revoked` — an incident
review can filter for one without wading through every rotation in the installation.

**This is narrower than disabling the account, on purpose.** Disabling stops every token the account
holds *and* stops the person; this stops one credential and leaves them able to work. Reach for
`POST /users/{userId}/disable` when the account itself is the problem, and for this when one
credential is.

## If a token leaks

1. **Revoke it.** That is the whole of the immediate fix; there is no propagation delay. If it is not
   yours, an administrator revokes it with `DELETE /users/{userId}/tokens/{tokenId}` — they do not
   need to disable the account to stop one credential.
2. Search the server log for its `prefix`. Every request the token made is logged with the
   authorization decision it produced, so the prefix is what tells you what was done with it. The
   secret is in no log — the type that carries one renders as `[redacted]` under every printf verb,
   log attribute and JSON encoder, so it cannot reach a log by accident.
3. Create a replacement with the narrowest scopes the integration actually needs. A token that
   leaked once will leak again.

## Where the secrets live

Token hashes are keyed with a value derived from `BLACKLIGHT_ENCRYPTION_KEY`, and deliberately not
from `BLACKLIGHT_SESSION_SECRET`. Rotating the session secret is the documented way to sign every
browser out ([`docs/security.md`](security.md)) and must not also break every integration in the
deployment.

Rotating `BLACKLIGHT_ENCRYPTION_KEY` **does** invalidate every service token, along with every
enrolled authenticator and recovery code. Treat it as a rebuild of the deployment's credentials, not
as routine hygiene.
