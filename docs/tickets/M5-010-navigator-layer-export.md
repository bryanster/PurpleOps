# M5-010 — ATT&CK Navigator layer export

**Milestone:** M5 · **Size:** M · **Depends on:** M5-004, M5-009

## Why

`PLAN.md` §5: "the ATT&CK Navigator layer export is one query". It is the artefact defenders actually
share — a JSON file they drop into Navigator to see the engagement on the matrix they already use —
and it replaces v1's hexagon graphic with something a client can interrogate.

The version pin is the point. A layer that re-maps itself when ATT&CK updates is the hazard
`engagement.attack_version` exists to prevent (`PLAN.md` §2), so the layer declares the engagement's
pin, not whatever is newest.

## Scope

**In**

- `GET /engagements/{engagementId}/analytics/navigator-layer` — `report.read`, spec first.
- Response is a Navigator layer document:
  - `versions.attack` = **`engagement.attack_version`** (epic decision), never the newest installed.
  - `versions.layer` and `versions.navigator` = **one schema version pinned as a constant in code**
    (currently 4.5), with the constant commented as the thing to change when Navigator moves. Not a
    query parameter.
  - `domain: "enterprise-attack"` — the only domain M2's ATT&CK adapter installs.
  - `name` and `description` from the engagement, `filters`, `gradient`, `legendItems`.
  - `techniques[]`: `techniqueID` (ATT&CK external id), `score` (the detection-category ordinal
    0–4), `color`, `comment`, `enabled`, and `metadata` carrying step count and protection.
- **Gradient and colour ramp documented in `docs/analytics.md`** with the exact hex values, because
  a report and a Navigator screenshot sitting side by side must not disagree about what amber means.
- **Unscored and not-attempted techniques are not scored 0.** A technique nobody looked at and a
  technique that was tested and produced nothing are different facts; `none` scores 0, unscored and
  not-attempted are either omitted or carry `enabled: false` with the reason in `comment`. Pick one,
  document it, test it.
- `Content-Disposition: attachment` with a filename derived from the engagement, and
  `Content-Type: application/json`.
- Blind scoping via `M5-009`'s shared helper. Blue's layer contains only revealed steps' techniques,
  and — as everywhere in M5 — nothing in the file counts what is missing.

**Out**

- Other Navigator domains (mobile, ICS) — M2 installs Enterprise only.
- Multi-layer bundles, or a layer per scenario.
- Configurable layer version (epic decision: pinned in code).
- Uploading a layer back in.

## Acceptance criteria

- [ ] Spec first; drift gate green.
- [ ] `versions.attack` equals `engagement.attack_version`, asserted against two fixture engagements
      pinned to different versions — the single most important assertion in this ticket.
- [ ] The layer opens in ATT&CK Navigator. Verified by hand once and recorded in the PR description;
      the checked-in assertion is a golden-file test against a schema-shaped fixture, since Navigator
      itself is not a CI dependency.
- [ ] `score` equals `scoring.Ordinal` of the technique's category, proven against the same fixture
      `M5-004` uses — not recomputed here.
- [ ] Sub-techniques appear with their own `techniqueID` (`T1059.001` form), and a scored
      sub-technique does not colour its parent.
- [ ] A technique the pinned version does not contain (`unmatchedTechniques` from `M5-004`) is
      omitted from the layer and noted in `description`, so a defender does not silently lose a row.
- [ ] Blue in a blind engagement gets a smaller layer; a test asserts the unrevealed techniques are
      absent by id.
- [ ] Golden file is normalized — no timestamps, no generated ids — per the M6 epic's warning about
      golden files that rot.

## Tests

- Golden-file layer for the fixture engagement, normalized.
- Pin-difference assertion across two engagements.
- Ordinal-to-score agreement with `scoring.Ordinal`.
- Sub-technique independence.
- Blind seat comparison.
- Handler authz: member, observer, non-member, token scopes.

## Notes for the implementer

- Do not add a Navigator client library. The layer is a JSON document with a documented shape; a
  struct and `encoding/json` is the whole implementation, and a dependency here would be a
  dependency on somebody else's release cadence for a format that changes yearly.
- The colour ramp is a product decision that ends up in a client deliverable. Put the hex values in
  `docs/analytics.md` and have `M5-013`'s heatmap read the same table — one ramp, two renderers.
