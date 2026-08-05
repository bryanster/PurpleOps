# M2-007 — ATT&CK version catalog & pin surface

**Milestone:** M2 · **Size:** M · **Depends on:** M2-006

## Why

Pinning `attack_version` on an engagement is what stops a report re-mapping itself when ATT&CK
updates mid-engagement (`PLAN.md` §2). M3 owns the engagement column; M2 must ship the **catalog,
resolve helpers, and invariants** so M3 cannot invent a second definition of "version".

Review this with the same care as `M1-012`.

## Scope

**In**

- `internal/content/attackpin` (or `internal/content/version`) API used by HTTP and later M3:
  - `ListVersions(ctx) []VersionInfo` — installed ATT&CK versions with item counts and synced_at.
  - `Resolve(ctx, version string) (VersionInfo, error)` — exact match; unknown → typed not-found.
  - `ResolveTechnique(ctx, version, externalID) (Technique, error)` — the object in that version or
    not-found. **Never** falls back to another version.
  - `AssertPinned(ctx, version) error` — version exists, source enabled, status healthy enough to
    pin (define "healthy": last sync succeeded and item_count > 0).
- HTTP (spec-first), `content.read`:
  - `GET /content/attack/versions` — installed versions.
  - `GET /content/attack/versions/{version}` — detail + counts by object type.
  - `GET /content/attack/versions/{version}/techniques/{externalId}` — resolve by natural key.
- Invariants (tests + code structure):
  1. A pin string is opaque text equal to `content_source_version.version` for the attack source —
     no semver rewriting (`15.1` ≠ `v15.1` unless you normalize **once** at ingest and document it).
  2. Resolve never crosses versions.
  3. Deleting a version that is not referenced is allowed via admin API
     `DELETE /content/attack/versions/{version}` (`content.manage`) with the same 409-if-referenced
     rule. In M2 "referenced" means only content-internal; document the M3 extension point
     (`References.AttackVersion(version) (count, error)` interface M3 implements).
  4. Sync of version X never mutates version Y (restate `M2-006` as a pin-level guarantee).
- **Copy-on-use contract** written in `docs/` (short, normative for M3):
  - When a step is created from a technique/template, the step stores **snapshots** of display
    fields and procedure JSON plus optional lineage ids (`technique_external_id`, `template_id`,
    `attack_version`).
  - Subsequent catalog syncs must not alter stored steps.
  - Library pickers in M3 call `AssertPinned` / `ResolveTechnique` using the engagement's pin.
- Activity on version delete: `content.version.deleted`.

**Out**

- `engagement.attack_version` column and engagement CRUD (M3-001).
- UI beyond what library/sources pages already show (`M2-013`/`M2-014` may list versions).

## Acceptance criteria

- [x] `ResolveTechnique("15.1", "T1059.001")` after installing 14.1 and 15.1 fixtures returns the
      15.1 row only; wrong version → not-found even if the external id exists elsewhere.
- [x] `AssertPinned` fails for missing, empty, or disabled-source versions.
- [x] Version delete with zero refs succeeds and removes that version's object rows only.
- [x] There is exactly one normalization rule for version strings, tested with the confusing cases
      you choose to accept or reject.
- [x] `docs/` contract page exists and is linked from the epic / contributing content section.
- [x] M3 extension interface for ref counts compiles with a stub that always returns 0.

## Tests

- Multi-version fixture matrix for Resolve/Assert/List.
- Delete isolation test.
- Import-check or doc test that the copy-on-use contract file exists.

## Notes for the implementer

- Do not let handlers query technique tables with a default "latest" when version is omitted on
  pin-sensitive endpoints — require the version path param. Broader library search may default to
  latest **only if** the response includes the version used.
- Prefer typed errors (`ErrVersionNotFound`, `ErrNotReferencable`) mapped once to problem codes.

## Implementation notes

- Package: `internal/content/attackpin` — `ListVersions`, `Resolve`/`ResolveDetail`,
  `ResolveTechnique`, `AssertPinned`, `DeleteVersion`, `NormalizeVersion`,
  `References` + `NopReferences`, typed errors + `MapError`.
- Version rule: TrimSpace only; reject empty/internal whitespace/`__staging__`/`current`.
  **No** leading-`v` strip — `15.1` ≠ `v15.1` (tested). Documented in
  `docs/content-attack.md` § Version strings.
- Healthy pin: source enabled + status `ready` + `item_count > 0`.
- Store: `Objects.TechniqueByExternal`, `Objects.CountFamilies`,
  `Versions.DeleteAttackCatalog` (objects + version row, one txn, After hook).
- HTTP (spec-first): `GET/DELETE /content/attack/versions…` under `content.read` /
  `content.manage`. Pin-sensitive technique resolve requires version path param.
- Activity: `content.version.deleted` on `content_source_version`.
- Copy-on-use contract: `docs/content-copy-on-use.md` (linked from
  `docs/content-attack.md` and M2-EPIC pin-surface row).
- M3 ref extension: `attackpin.References.AttackVersion`; M2 wires `NopReferences`.
