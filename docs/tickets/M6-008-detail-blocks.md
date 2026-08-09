# M6-008 — Detail blocks: scenario walkthrough, findings backlog, evidence appendix

**Milestone:** M6 · **Size:** L · **Depends on:** M6-001, M3-009, M5-009

## Why

Narrative proof under the numbers: what was run, what remains open, and optional screenshots. Evidence
is the sensitive part — renderers must honor `RenderEnv.IncludeEvidence`.

## Scope

**In**

- Renderers for:
  | Block | Behavior |
  |---|---|
  | `scenario_walkthrough` | Params: optional `scenarioIds[]` (default all), `verbosity` (summary\|full). Lists scenarios → steps → execution outcome (derived), detection category, protection; optional red/blue notes at full verbosity. Respect blind scope in draft preview. |
  | `findings_backlog` | Open + in_progress (+ optional resolved via param). Severity, title, status, linked techniques/steps. Not a second burndown chart — table/list. Burndown stays analytics block territory if ever added; v1 catalogue uses backlog only. |
  | `evidence_appendix` | Index of evidence metadata (caption, side, step linkage). **Binaries/images inlined or linked only when `IncludeEvidence`**. When false: section shows "Evidence omitted at publish" or omits block content per publish rules. |
- Evidence HTML: use safe `<img>` only for allowlisted image MIME; others listed as filename +
  caption. Never inline HTML evidence. URLs must go through report asset routes (defined fully in
  M6-011/M6-012) — fragments may emit placeholders `cid:` or `report-asset:` tokens resolved at
  assembly time.
- Findings/steps read via existing domain repos with blind fence — no new query holes.

**Out**

- Zip export of evidence (M5 archive already covers bulk); comments-in-report; full packet-capture
  inline.

## Files

- `internal/report/blocks/walkthrough.go`, `findings.go`, `evidence.go`

## Acceptance criteria

- [ ] With `IncludeEvidence=false`, output contains no evidence blob bytes and no resolvable
      evidence asset URLs that would stream bytes without authz (placeholders stripped).
- [ ] Blue draft preview cannot name unrevealed steps (assert absence of ids/titles from fixture).
- [ ] Findings statuses use the same labels as the product UI / analytics.md.
- [ ] Walkthrough outcomes use derived outcome labels (`prevented`, `detected`, …).

## Tests

- Fixture engagement with evidence + unrevealed steps; lead vs blue env; evidence on/off matrix.

## Notes for the implementer

- Quota: appendix may be huge — param `limit` or "images only" acceptable; document defaults.
- Do not base64 multi-megabyte images into HTML if it blows memory; prefer asset URLs rewritten at
  publish into version-scoped paths.
