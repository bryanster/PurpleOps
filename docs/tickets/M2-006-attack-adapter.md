# M2-006 — ATT&CK Enterprise adapter (multi-version)

**Milestone:** M2 · **Size:** L · **Depends on:** M2-003, M2-005

## Why

ATT&CK is the spine of the product (`PLAN.md` §2–3). Multiple versions must coexist so an engagement
pinned to 14.1 does not silently remap when 15.1 is installed. This adapter is the first real
content load and sets the quality bar for failures and fixtures.

## Scope

**In**

- `internal/content/attack` adapter registered for `kind=attack`.
- **Enterprise domain only.** Ignore mobile/ics bundles if present in an archive.
- Fetch: HTTPS download of the published STIX bundle for a version (default base URL on the seed
  source — typically `mitre-attack/attack-stix-data` release assets or equivalent stable JSON URLs).
  Discover latest and list known versions via a documented mechanism (GitHub releases API **or**
  a static index file URL on the source — prefer something fixture-friendly; no git).
- Parse STIX 2.x bundle; map to normalized objects:
  - tactics (`x-mitre-tactic`)
  - techniques + sub-techniques (`attack-pattern` + `x-mitre-is-subtechnique`)
  - data sources / data components as upstream provides
  - mitigations (`course-of-action`)
  - groups (`intrusion-set`)
  - software (`malware`, `tool`)
- Relationships needed for library UX: technique↔tactic, technique↔mitigation, technique↔subtechnique
  parent. Store enough to answer "tactics for T1059.001 in v15.1" without re-parsing STIX.
- **Multi-version:** each sync targets one `version` string (e.g. `15.1`). Never update rows for
  another version. Installing 15.1 while 14.1 exists leaves 14.1 byte-identical.
- Sync request without version → latest stable Enterprise release the adapter can resolve.
- Fail loudly on schema drift: unknown required fields, empty technique table after parse, version
  label mismatch → job `failed` with operator-readable error (include phase + object id when
  possible). No partial "best effort" catalog for that version — leave previous successful snapshot
  for that version if re-sync fails mid-way (transactional replace per version).
- Checked-in fixtures: trimmed STIX snippets covering tactic, parent technique, sub-technique,
  mitigation, group, software, and one deliberate broken fixture for error paths. CI never hits the
  network (`PLAN.md` §9).
- Library read APIs (spec-first), all `content.read`, default exclude disabled sources:
  - `GET /content/techniques` — filters: `version`, `q`, `tactic`, `isSubtechnique`
  - `GET /content/techniques/{id}`
  - `GET /content/tactics`, mitigations, groups, software (list+get) — same version filter pattern
- Indexes supporting those filters and substring search on `external_id`, `name`, `description`.

**Out**

- Pin helpers for engagements (`M2-007`).
- Mobile/ICS.
- Navigator layer export (M5).
- UI (`M2-013`).

## Acceptance criteria

- [ ] Syncing fixture vA then fixture vB yields two versions; queries for vA never return vB rows.
- [ ] Re-syncing vA after a fixture edit replaces vA rows; vB unchanged.
- [ ] Broken fixture fails the job and leaves the prior good vA catalog intact.
- [ ] Sub-technique has parent link; list filter `isSubtechnique=true` works.
- [ ] Technique list substring query finds by external id (`T1059`) and by name case-insensitively.
- [ ] Disabled ATT&CK source: list endpoints return empty for non-admin default views.
- [ ] No test in CI performs real network I/O (httptest/forced client injection asserted).

## Tests

- Parse/normalize unit tests on fixtures.
- Full sync integration via job runner + storetest DB.
- Version isolation test (the important one).
- Handler tests for list filters.

## Notes for the implementer

- STIX IDs are stored as attributes if useful for debug, not as PKs (`M2-001`).
- External ids are MITRE IDs (`TA0001`, `T1059`, `T1059.001`).
- Watch memory: stream/parse without holding multiple full bundles.
- Document the exact default URL template on the seed row and in `docs/` so air-gap operators fetch
  the same artifact.
