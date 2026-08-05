# M2-009 — Sigma adapter (technique-mapped rules)

**Milestone:** M2 · **Size:** M · **Depends on:** M2-003, M2-005

## Why

Blue needs detection references tied to techniques without Blacklight becoming a SIEM. Sigma rules
are **reference only** — never executed or deployed (`PLAN.md` §3). Ingest only rules that already
carry ATT&CK technique mappings so the library stays relevant.

## Scope

**In**

- `internal/content/sigma` adapter for `kind=sigma`.
- Fetch: HTTPS release archive of SigmaHQ rules (seed URL). Optional secondary sources (Elastic/ESCU)
  are **out** unless the seed is designed for one archive only — stick to one upstream in M2.
- Parse rule YAML; **skip** rules with no ATT&CK technique mapping (count skips in job message).
- Map to `content_detection_rule_ref`:
  - title, description, level, status
  - `technique_external_ids` from tags (`attack.t1059` → `T1059`, subtechniques normalized)
  - logsource JSON
  - full rule body (`rule_yaml`) for display/copy
  - stable `external_id` (path or id field — document)
- Rolling `current` replace semantics (same as Atomic).
- Read API (`content.read`):
  - `GET /content/detection-rules` — filters: `q`, `technique`, `level`, `sourceId`
  - `GET /content/detection-rules/{id}`
- Fixtures: mapped rule, unmapped rule (skipped), broken YAML.
- UI copy later must say reference-only; API description text states never executed.

**Out**

- Rule deployment, conversion to product-specific queries, Elastic/ESCU as a second kind.
- Executing or validating rules against data.

## Acceptance criteria

- [ ] Unmapped fixture rules produce zero rows; job still succeeds with skip count > 0 in message.
- [ ] Mapped rule's technique filter finds it by `Txxxx` / `Txxxx.xxx`.
- [ ] Rule body returned on GET is sufficient to display/copy in UI.
- [ ] No code path "runs" a rule against events (grep/architecture: package has no execution API).
- [ ] CI fixtures only.

## Tests

- Tag→technique normalization cases (`attack.t1059.001`, mixed case).
- Skip counting; integration sync; list filters.

## Notes for the implementer

- Be conservative on technique extraction — wrong links are worse than skips. Document the tag
  patterns accepted.
- Large archives: stream file-by-file from the zip; do not load all rules into memory.
