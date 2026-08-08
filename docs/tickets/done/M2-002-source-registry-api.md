# M2-002 — Source registry API, enable/disable/delete, authz

**Milestone:** M2 · **Size:** M · **Depends on:** M2-001, M1-013

## Why

Admins need a stable HTTP surface to list sources, flip enablement, and remove unused libraries
before any adapter exists. This is also where disable/delete **semantics** become enforceable API
contracts rather than table comments.

## Scope

**In**

- Spec-first endpoints in `api/openapi.yaml` (names indicative):

  | Method | Path | Action |
  |---|---|---|
  | `GET` | `/content/sources` | List sources (filter: kind, enabled) |
  | `GET` | `/content/sources/{sourceId}` | Detail including license + last job summary |
  | `PATCH` | `/content/sources/{sourceId}` | Update name/url/ref (admin); not kind |
  | `POST` | `/content/sources/{sourceId}/enable` | `enabled=true` |
  | `POST` | `/content/sources/{sourceId}/disable` | `enabled=false` |
  | `DELETE` | `/content/sources/{sourceId}` | Hard delete when unreferenced |
  | `GET` | `/content/sources/{sourceId}/versions` | List version snapshots |

- Authz (extend `internal/authz` table + regenerate `docs/authz.md`):
  - Keep `content.read` — any authenticated subject may list/get sources and later library reads.
  - Keep `content.sync` — admin; start sync/reprocess/bundle (owned by later tickets).
  - Add **`content.manage`** — admin; enable/disable/delete/patch source metadata and all custom
    CRUD (`M2-011`). Do **not** overload `content.sync` for non-sync mutations.
  - Token scopes: `content:read`, `content:sync`, and add `content:write` (or document that manage
    maps to `content:sync` — prefer a distinct `content:write` so automation can sync without
    deleting sources). Pick one, encode in the rule table, document in `docs/api-tokens.md`.
- **Disable semantics:**
  - Sets `enabled=false` and updates `status`.
  - Library list/search endpoints (later tickets) **omit** objects from disabled sources by default.
  - APIs that would create a *new* reference to disabled content return **409** with a clear
    problem detail (hook used by M3 pickers; provide a package-level `content.AssertReferencable`
    helper here even if M3 is the first caller).
  - Existing engagement data is not modified (no engagement tables yet).
- **Delete semantics:**
  - Refuse with **409** + counts if any version/object rows exist *and* are referenced by something
    outside a pure content cascade — in M2 the only external refs are none, so delete may remove
    the source and its content subtree when the admin confirms.
  - Builtin upstream seeds **may** be deleted; re-seed is not automatic. Disabling is the normal
    path. Document that.
  - The `custom` seed source cannot be deleted (409) — user content needs a home.
- Activity verbs (platform, `engagement_id` null): `content.source.enabled`,
  `.disabled`, `.updated`, `.deleted`.
- `blctl content sources` list/show; `blctl content enable|disable`.

**Out**

- Starting sync jobs (`M2-003`), bundle upload (`M2-005`), library object APIs (`M2-006`+ / `M2-011`).
- UI (`M2-014`).

## Acceptance criteria

- [x] Spec paths exist; `make generate` produces server + TS client stubs.
- [x] Non-admin receives 403 on enable/disable/delete/patch; member can GET list/detail.
- [x] Disable then GET library helpers (unit on `AssertReferencable`) refuse new refs; object rows
      still readable by id for admin/debug if you expose that, but default lists exclude them.
- [x] Delete of `custom` seed source is 409.
- [x] Delete of a source with only content children succeeds and removes children in one write
      transaction (or documents staged delete); no orphan versions.
- [x] Activity rows written in the same transaction as the mutation (`M1-015` pattern).
- [x] Errors use `M0B-007` problem shape; unknown source id is 404 via authz conceal rules where
      applicable (platform resource — 404 vs 403 consistent with other admin APIs).

## Tests

- Handler tests with real temp DB: enable/disable round-trip, delete custom refused, delete empty
  upstream ok, authz matrix rows for the new action.
- Exhaustiveness: new `Action` has a rule (`M1-012` pattern).

## Notes for the implementer

- Resource type stays `ResourceContent` unless a source-scoped resource is cleaner for conceal;
  default deny still applies.
- Do not allow arbitrary `kind` creation via API in M2 — kinds are closed; only seed + custom.
  If PATCH url/ref is enough for mirrors, skip `POST /content/sources`.

## Implementation notes

- Authz: `content.manage` (admin) with distinct token scope `content:write`. `content.sync` /
  `content:sync` left for job start only (M2-003+). Documented in `docs/api-tokens.md` and
  regenerated `docs/authz.md`.
- Domain: `internal/content.Registry` owns product rules; `AssertReferencable` returns 409 on
  disabled sources for M3 pickers. Storage stays in `internal/store/content`.
- Cascade delete: `Sources.DeleteCascade` clears every content table naming `source_id` (objects,
  jobs, versions) then the registry row in one `store.Write`. No external refs exist in M2; when
  M3 adds engagement refs this method must refuse with 409+counts before cascading. Custom seed
  is refused in the domain layer (409). Builtin upstream seeds may be deleted; re-seed is not
  automatic.
- Activity verbs: `content.source.enabled|disabled|updated|deleted` on object type
  `content_source`, same-txn via store `After` hooks (mirrors identity).
- Disable only flips `enabled`; operational `status` stays independent (per schema comment).
- HTTP: `/api/v1/content/sources` list/detail/patch/delete/enable/disable + versions. No
  `POST /content/sources` — kinds are closed.
- blctl: `content sources [--id|--kind|--enabled]`, `content enable|disable --id`. `content sync`
  remains a stub until M2-003.
- Resource type remains `ResourceContent` (platform). Unknown source id is plain 404 from the
  repository, not conceal — consistent with other platform admin resources.
