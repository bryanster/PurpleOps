# M1-018 — Administrative service token management

**Milestone:** M1 · **Size:** S · **Depends on:** M1-011, M1-016

## Why

`M1-011` built service tokens with owner-only endpoints: `/auth/tokens` reads and writes the
caller's own, and there is no way for anybody — administrator included — to see or revoke somebody
else's. That was the right shape for that ticket, because `authz.Can` is role-based and has no
"owner of this row" concept to build an "admin *or* owner" rule out of; see its **Implementation
notes**.

What is missing is the incident case. An administrator who learns that an account's credentials have
leaked can today disable the account, which stops every token it holds — but that also stops the
person, and there is no way to answer "what tokens does this account hold, and when were they last
used?" without a SQL console. That question comes up during an incident, which is the worst time to
be reaching for one.

## Scope

**In**

- `GET /users/{userId}/tokens` — an administrator reads one account's tokens. Mapped to the existing
  `user.read` action, on `type: user`, so no new row in the rule table.
- `DELETE /users/{userId}/tokens/{tokenId}` — an administrator revokes one. `user.manage`.
- Both reuse `identity.ServiceTokens` and the `gen.ServiceToken` wire shape. The listing is the same
  renderer as `GET /auth/tokens`, so the two cannot describe a token differently.
- `revoked_by` on `app.service_token`, in a new migration, so that "who ended this, and were they
  its owner?" is answerable. `M1-011` left the column out because there was only one answer.
- Activity log entries for an administrative revocation, distinguished from an owner's own
  (`M1-015`'s vocabulary — the verb should say who did it to whose).

**Out**

- Creating a token *on behalf of* another account. `created_by` exists in the schema for it, and it
  is a different decision: a credential somebody else minted in your name, spending your permissions,
  is one you did not agree to hold. Argue it separately or not at all.
- Any change to `authz.GuardSessionOnly`. A service token still cannot manage tokens, including an
  administrator's.

## Acceptance criteria

- [x] A non-administrator gets the same answer for another account's tokens as for an account that
      does not exist. Neither endpoint may be a way to find out which accounts are real.
- [x] An administrator revoking somebody's token stops it at its next request, and the owner's own
      listing shows it as revoked with the same timestamp.
- [x] No secret appears in either response — the same test as `M1-011`'s, extended over these two
      endpoints.
- [x] The owner-only endpoints are unchanged: `GET /auth/tokens` still returns the caller's own
      tokens and nobody else's, for an administrator too.

## Tests

- Authorization through the matrix in `M1-014`.
- The cross-account revocation case end to end: administrator revokes, owner's token stops working.

---

## Implementation notes

Read these before starting a ticket that depends on this one.

### The endpoints carry their own actions, not `user.read` and `user.manage`

**This is where the implementation deviates from the ticket, and the ticket is inconsistent with
itself.** The scope maps both endpoints to `user.read`/`user.manage` "so no new row in the rule
table". Those two rules carry `Guard: GuardNone`. The Out section says, in the same breath, "a
service token still cannot manage tokens, including an administrator's" — which under that mapping
is false: a token owned by an administrator and carrying `admin:write` would have reached
`DELETE /users/{userId}/tokens/{tokenId}` and been able to end every other credential in the
installation. That is worse than the hole `M1-011`'s `GuardSessionOnly` was added to close, because
a leaked token cannot mint a sibling but could have revoked everybody's.

Resolved in favour of the Out section, and confirmed with the ticket's author. `internal/authz`
gains `token.admin_read` and `token.admin_manage` — `Platform: admins`, `Resource: user`,
`Guard: GuardSessionOnly`. Two new rows, which the scope asked to avoid, and:

- `GuardSessionOnly` itself is untouched, which is what the Out section actually forbids.
- `user.read` and `user.manage` are untouched, so a service token can still do user administration
  exactly as it could before. Adding the guard to *those* would have closed the hole and broken
  `M1-014`'s sweep, which asserts that a session and a token get the same answer for
  `PATCH /users/{userId}`.
- The pair keeps meaning what it says. `token.read` and `token.manage` are held by *everybody*, over
  their own rows; folding an administrative case into them would have made that sentence false, and
  it is the sentence the endpoints' safety rests on.

The resource is `user` and not `service_token`, because what the request names is an account: the
question is "what does this account hold", and the answer is a set rather than a row.

### The concealment is structural, and so is the enumeration test

`token.admin_read` acts on a platform-owned resource, so `authorize` decides it without loading
anything. A platform member is refused before any code has looked to see whether the account in the
path exists — which is why both endpoints answer a non-administrator identically for a real account
and an invented one, and why `TestAnAccountsTokensAreNotAWayToFindOutWhichAccountsExist` compares the
two whole problem documents rather than just the statuses.

For an *administrator*, `AccountTokens` reads the account first so that an identifier naming nobody
is a `404` rather than an empty list. That is safe only because an administrator may already read the
account through `GET /users/{userId}`; it would be an enumeration oracle on any other endpoint in
this file, and the comment there says so.

### `revoked_by` is on the wire, which the scope did not ask for

`0010_service_token_revoked_by.sql` adds the column, as the scope says. It is also rendered as
`revokedBy` on `gen.ServiceToken`, which the scope did not say — it says the two endpoints "reuse …
the `gen.ServiceToken` wire shape".

The column exists to make "who ended this, and were they its owner?" answerable, and the Why section
is about not having to reach for a SQL console during an incident. Storing it and then requiring a
SQL console to read it would have delivered half the ticket. It is one optional field on the shared
renderer, so the owner's own listing carries it too — which is a feature: an owner can see that
somebody else ended their token. It is an opaque account identifier and not a secret.

The renderer is shared with `GET /auth/tokens` deliberately (`serviceToken` in
`internal/httpapi/tokenhandlers.go`), so the two listings cannot describe a token differently. The
`revokedAt` equality assertion in
`TestAnAdministratorRevokesSomebodyElsesTokenAndItStopsAtItsNextRequest` is what holds that: there is
one row and one revocation, not an administrative copy of one.

### `Revoke` grew an argument rather than a sibling

`identity.ServiceTokens.Revoke` is now `(id, ownerUserID, revokedBy, at, after...)`, and
`servicetoken.Manager.Revoke` mirrors it. One function, not two: an owner ending their own passes
their identifier twice, and an administrator passes the account named in the path and then
themselves. A second function would have been a second definition of "revoked" to keep in step, and
the ownership clause in the `WHERE` — the thing that makes a token belonging to another account
indistinguishable from one that does not exist — would have had to be written twice.

A second revocation keeps the *first* revoker as well as the first timestamp. Whoever arrived second
stopped nothing.

### `token.admin_revoked` is a verb, not a delta field

`M1-015`'s vocabulary gains one entry. An incident review filters for "an administrator ended
somebody's credential", and a filter that also returns every routine rotation in the installation is
a filter nobody uses — the same reason `M1-016` kept `user.sessions_revoked` apart from
`session.logout`. The delta carries `owner_user_id`, because an entry naming only the token is one a
reader has to go and look up at the moment they can least afford to.

### Not in scope, and still missing

**No UI.** The ticket's In section is two endpoints, a column, and an activity verb; `M1-017`'s admin
screens have no service-token panel on an account, and this ticket did not add one. An administrator
uses `curl` (`docs/api-tokens.md` has both calls) until somebody opens a follow-up.

**Creating a token on behalf of another account** is still out, per the scope, and `created_by`
still has only one possible value on every path that exists.
