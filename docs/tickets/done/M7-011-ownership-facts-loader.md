# M7-011 — Replace the production Ownership.Facts stub

**Milestone:** M7 · **Size:** L · **Depends on:** M3-002, M1-013  
**Finding:** [BL-003](../SECURITY_FINDINGS.md#bl-003--production-ownershipfacts-is-still-the-m1-stub) · **Severity:** High

## Why

`authz.Can` is pure: it decides from facts the middleware loads. Production never loads them.

`cmd/blacklight` does not set `Deps.Ownership`. `newServer` therefore installs
`membershipOwnership`, whose `Facts` accepts any path string as an engagement and hard-codes
`Blind=false`, `Revealed=true`. The type comment still says “until M3”. M3 shipped.

Consequences:

- `GuardBlindMode` in middleware never fires.
- OpenAPI mappings that set `x-authz-resource.engagement` to `evidenceId`, `findingId`,
  `executionId`, `reportId`, `templateId`, `versionId`, or `shareId` cause `Seat` to look up
  membership on a UUID that is not an engagement.
- Nested IDORs in M7-012 cannot be closed at the middleware layer until this loader is real.

This is the root control failure. Fix it before, or in the same PR as, M7-012 — not after.

## Scope

**In**

- Implement `Ownership.Facts` by loading the named resource and returning:
  - the **owning engagement id** (not the path string that happened to be labelled `engagement`)
  - `Blind` from that engagement’s mode
  - `Revealed` for step/execution/evidence-shaped resources (true when not applicable)
  - `NotFound` when the row does not exist (so concealed denial and missing id stay indistinguishable)
- When the operation also names a path `engagementId`, refuse (404) if the loaded owner ≠ that path id.
- Point `x-authz-resource.engagement` at the actual engagement path param **or** teach the loader
  a dedicated owner field so child-only routes (`/evidence/{evidenceId}`) resolve correctly.
- Wire the real loader from `newServer` / `cmd/blacklight` so tests and production share it.
- Delete or rewrite the “until M3” comment. A stub that ships is not a stub.

**Out**

- Handler-side parent-bind of every nested ByID (M7-012) — do that next, as defense in depth.
- Engagement *list* filtering (M7-010).
- Changing `authz.Can` itself unless a fact it needs cannot be expressed on `Resource`.

## Files

- `internal/httpapi/ownership.go` — replace `membershipOwnership.Facts`
- `internal/httpapi/server.go` — stop defaulting to the stub as the production path
- `cmd/blacklight/main.go` — only if Deps must be explicit
- `api/openapi.yaml` — fix `x-authz-resource.engagement` on child-id routes (spec-first)
- `internal/httpapi/authorize.go` / `authorize_test.go` — keep fail-closed if Facts is nil
- Store packages already have the ByID walks (`engagement`, `scenario`, `step`, `execution`,
  `evidence`, `finding`, `report`)

## Acceptance criteria

- [x] Production `Facts` opens the named row. A missing row is `NotFound`, not “exists, not blind, revealed”.
- [x] `Facts` for a scenario/step/execution/evidence/finding/report/template/share returns that
      object’s owning engagement id.
- [x] A nested route whose path `engagementId` disagrees with the loaded owner is 404.
- [x] `GuardBlindMode` can fire: a blue member of a blind engagement is 404’d by middleware on
      `execution.read` / `evidence.read` for an unrevealed step (handlers may still re-check).
- [x] `cmd/blacklight` no longer depends on the M1 “engagements do not exist yet” loader.
- [x] `TestNewServerDefaultsOwnershipForEngagementOps` is rewritten or deleted so it cannot
      re-bless the stub as the production path.

## Tests

- Unit: `Facts` for each resource type — existing row, missing row, path engagement mismatch.
- Blind: blue + blind + unrevealed step → middleware 404 on an `evidence.read` / `execution.read`
  operation that previously relied on the stub.
- Admin who is *not* a member of a blind engagement is unaffected (existing policy).

## Notes for the implementer

Do not special-case “if the path param looks like an engagement id, trust it”. Walk the parent
chain (evidence → execution → step → scenario → engagement, etc.). Cache nothing across requests.

`Seat` stays on the *owning* engagement id Facts returned, never on the child UUID.

This ticket is a ship gate for `M7-009`. High findings do not defer.

## Implementation notes

- `ownership` (in `internal/httpapi/ownership.go`) replaces `membershipOwnership`. `NewOwnership(store.Store)`
  builds the engagement and report repositories and walks the parent chain per resource type; `newServer`
  installs it when `Deps.Ownership` is nil, and `cmd/blacklight` needs no change.
- Added `x-authz-resource.kind` (`api/authz.go`, both `ResourceRef` types, `checkKind`) to disambiguate the
  report family (report/template/version/share) and `evidence` upload (addressed by its `execution`). Spec
  mappings on the child-id routes were corrected to name `engagementId` where the path has one, and to drop
  the bogus child-id `engagement:` label where it does not.
- `Facts` normalises a store miss to `apierr.NotFound`: the store packages disagree on whether a missing row
  is `sql.ErrNoRows` or `apierr.ErrNotFound`, and concealed denial must be indistinguishable from a missing id.
- Tests: `ownership_test.go` covers every walk, missing rows, and cross-engagement mismatch; the blind-mode
  middleware regression and the admin-non-member case drive the default loader end to end.
