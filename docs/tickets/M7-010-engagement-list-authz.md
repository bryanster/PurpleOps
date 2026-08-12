# M7-010 — Engagement list leaks every engagement

**Milestone:** M7 · **Size:** S · **Depends on:** M3-002, M1-013  
**Finding:** [BL-001](../SECURITY_FINDINGS.md#bl-001--get-engagements-enumerates-every-engagement) · **Severity:** High

## Why

`GET /engagements` is declared `x-authz-self` on the premise that the handler returns only
engagements the caller may see. The handler never reads the caller. The store query is
`WHERE 1=1`. Any signed-in platform member — including a newly invited account with no seats —
enumerates every engagement’s id, client, description, dates, ATT&CK pin, and blind/standard
mode. That is the existence leak the rest of the model conceals with 404.

M7-007 marked this surface PASS. The contract and the code disagree.

## Scope

**In**

- Pass the authenticated caller into `engagement.Service.List`.
- Platform admin: keep the current unscoped query.
- Platform member: restrict to rows in `app.engagement_member` for `user_id = caller`.
- Empty list (not 403) for a member with no seats — same concealment as `GET /engagements/{id}` → 404 for a non-member.
- Status / cursor / limit still apply *after* the membership filter.
- Service tokens: same split as a session. A token with `engagements:read` whose owner is a
  member sees that owner’s engagements only; an admin owner sees all.

**Out**

- Changing `x-authz-self` to an action (not required if the handler actually filters).
- Nested child-id binding (M7-012) or replacing `Ownership.Facts` (M7-011).
- UI changes beyond whatever already consumes the list.

## Files

- `internal/httpapi/engagementhandlers.go` — read subject, pass it down
- `internal/engagement/service.go` — `List` takes the caller (or an explicit admin/member mode derived from the subject)
- `internal/store/engagement/engagements.go` — membership-scoped `List` / `ListFilter`
- `internal/httpapi/*_test.go` or `internal/engagement/*_test.go` — regression
- `docs/SECURITY_FINDINGS.md` — mark BL-001 resolved in the PR, not in this ticket’s first landing

## Acceptance criteria

- [ ] A platform member who is not in engagement `EA` gets `EA` omitted from `GET /engagements` and still gets 404 from `GET /engagements/{EA}`.
- [ ] A platform admin sees every engagement, including ones they are not a member of.
- [ ] A member sees every engagement they belong to, regardless of seat (lead/red/blue/observer).
- [ ] Cursor pagination does not skip or leak rows across the membership fence.
- [ ] The domain comment that “the handler chooses the filter” is deleted; the filter lives in one place.

## Tests

- HTTP or service test: two members, one engagement owned only by A; B’s list is empty; admin’s list contains it.
- Pagination: more than one page of *visible* engagements still pages correctly when invisible rows exist between them.

## Notes for the implementer

Do not leave a `MemberID` field on `ListFilter` that handlers can forget to set. Take `authn.Subject` (or platform role + user id) on `Service.List` and decide there.

This ticket is a ship gate for `M7-009`. High findings do not defer.
