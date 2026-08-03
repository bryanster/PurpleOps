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

- [ ] A non-administrator gets the same answer for another account's tokens as for an account that
      does not exist. Neither endpoint may be a way to find out which accounts are real.
- [ ] An administrator revoking somebody's token stops it at its next request, and the owner's own
      listing shows it as revoked with the same timestamp.
- [ ] No secret appears in either response — the same test as `M1-011`'s, extended over these two
      endpoints.
- [ ] The owner-only endpoints are unchanged: `GET /auth/tokens` still returns the caller's own
      tokens and nobody else's, for an administrator too.

## Tests

- Authorization through the matrix in `M1-014`.
- The cross-account revocation case end to end: administrator revokes, owner's token stops working.
