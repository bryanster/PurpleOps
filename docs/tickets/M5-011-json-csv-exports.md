# M5-011 — JSON and CSV exports

**Milestone:** M5 · **Size:** M · **Depends on:** M5-009

## Why

Every purple team eventually wants the data in a spreadsheet. v1 had exports and they were the part
that broke — `export/entire` wrote `export.csv` while `import/entire` read `export.json`
(`PLAN.md` §5), which is what happens when export shapes are decided one endpoint at a time.

These are the flat, tabular exports. The archive is `M5-012`.

## Scope

**In**

- `GET /engagements/{engagementId}/export?format=json|csv&dataset=` — `report.read`, spec first.

  | Dataset | Rows |
  |---|---|
  | `executions` | One row per step: scenario, step, technique, tactic, red status and times, blue category, modifiers, protection, MTTD seconds, derived outcome |
  | `findings` | One row per finding: title, severity, status, owner, linked step ids, created/updated |
  | `coverage` | `M5-004`'s technique rollup, flattened |

- **JSON and CSV render from the same in-memory rows.** One shape, two encoders — the v1 bug was two
  shapes with one name.
- **CSV injection is escaped.** A finding titled `=cmd|' /C calc'!A0` is a formula the moment a
  client opens it in Excel. Any field starting `=`, `+`, `-`, `@`, tab or CR is prefixed with a
  single quote. This is an acceptance criterion, not a judgement call, and it is tested with the
  literal payloads above.
- Streamed, not buffered — `csv.Writer` straight to the response, JSON as a streamed array. An
  engagement with thousands of steps must not be assembled in memory (the same discipline `M5-012`
  needs for evidence).
- `Content-Disposition: attachment` with a filename carrying the engagement name and dataset;
  `Content-Type: text/csv; charset=utf-8` or `application/json`.
- UTF-8 BOM on CSV, or a documented decision not to — Excel mis-renders accented client names
  without it, and this file goes to clients.
- Times as RFC 3339 UTC; durations as integer seconds. **No server-side formatting for display**
  (README § Conventions).
- Blind scoping via `M5-009`'s helper. An unrevealed step is not a row.

**Out**

- The full engagement archive with evidence (`M5-012`).
- Import of any kind (epic decision).
- XLSX, or anything requiring a spreadsheet library.
- Per-user column selection or saved export profiles.

## Acceptance criteria

- [ ] Spec first; drift gate green.
- [ ] JSON and CSV of the same dataset contain the **same values** — a test decodes both and compares
      field by field. This is the regression test for v1's bug and it is the point of the ticket.
- [ ] CSV injection escaping covers `=`, `+`, `-`, `@`, `\t`, `\r` in every text column, tested with
      a fixture finding and a fixture step whose titles start with each.
- [ ] Quoting and embedded newlines are correct — a `red_notes` field containing a comma, a quote and
      a newline round-trips through `encoding/csv` unchanged.
- [ ] Header row present and stable; column order is asserted by a test, because a report or a
      client's macro will depend on it and silent reordering is a support ticket.
- [ ] Empty engagement exports a header row and no data rows, not an empty file.
- [ ] Response is streamed: a test with a large fixture asserts the first bytes arrive before the
      last row is computed, or at minimum that no full slice of rows is materialized.
- [ ] Blue in a blind engagement exports fewer rows, and the file contains no reference to an
      unrevealed step — including in the findings dataset's linked step ids.
- [ ] Derived outcome in the export comes from the same SQL as `M5-005`, not a second Go computation.

## Tests

- JSON/CSV equivalence per dataset.
- Injection payloads, one per dangerous prefix.
- CSV quoting torture case.
- Column order golden assertion.
- Empty engagement.
- Blind seat comparison, including the findings→step-id case.
- Authz: member, observer, non-member, `reports:read` token, `engagements:read` token.

## Notes for the implementer

- The `findings` dataset joins `finding_step`. That is the one place these exports touch step
  identity, so it is the one place the blind filter can be forgotten — `M5-007` deliberately avoided
  the join, this ticket cannot.
- `detection_modifiers` is a JSON array; in CSV it is a documented delimiter-joined string, and the
  delimiter must not be a comma.
- Do not add a "download everything" convenience alias. That is `M5-012`, it has different semantics,
  and one name for two shapes is the bug this ticket exists to close.
