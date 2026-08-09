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

- [x] With `IncludeEvidence=false`, output contains no evidence blob bytes and no resolvable
      evidence asset URLs that would stream bytes without authz (placeholders stripped).
- [x] Blue draft preview cannot name unrevealed steps (assert absence of ids/titles from fixture).
- [x] Findings statuses use the same labels as the product UI / analytics.md.
- [x] Walkthrough outcomes use derived outcome labels (`prevented`, `detected`, …).

## Tests

- Fixture engagement with evidence + unrevealed steps; lead vs blue env; evidence on/off matrix.

## Notes for the implementer

- Quota: appendix may be huge — param `limit` or "images only" acceptable; document defaults.
- Do not base64 multi-megabyte images into HTML if it blows memory; prefer asset URLs rewritten at
  publish into version-scoped paths.

## Implementation notes

**Date:** 2026-08-09

### Files created/modified

- `internal/report/block.go` — Added `DomainFacade` interface (6 methods: ListScenarios, ListSteps, ListExecutions, ListFindings, FindingSteps, ListEvidence) and `Domain DomainFacade` field to `RenderEnv`
- `internal/report/blocks/walkthrough.go` — Scenario walkthrough block: `verbosity` param (summary|full), `scenarioIds` filter, scenario→step→execution join, derived outcome from scoring package, blind-scoped step filtering, red/blue notes at full verbosity
- `internal/report/blocks/findings.go` — Findings backlog block: `includeResolved` param, status filter (open + in_progress default), finding→step technique linkage, display labels matching product UI
- `internal/report/blocks/evidence.go` — Evidence appendix block: `limit` + `imagesOnly` params, `NeedsEvidenceOptIn=true`, "Evidence omitted at publish" when IncludeEvidence=false, image MIME allowlist
- `internal/httpapi/server.go` — Replaced 3 stub registrations with real WalkthroughDef/FindingsDef/EvidenceDef + renderers
- `internal/report/blocks/detail_blocks_test.go` — 14 tests: walkthrough (5: full, summary, filter, blind, empty), findings (3: open default, includeResolved, empty), evidence (4: with evidence, omitted, matrix on/off), registry (2: definitions, renderers)

### Design decisions

- **DomainFacade interface** — Defined in `report/block.go` using `storengagement.*` types directly, same pattern as `AnalyticsFacade` with `analytics.*` types. Six methods cover the three blocks' needs. Concrete adapter (`domainDomain`) in tests wraps the real engagement repos.
- **Walkthrough outcome derivation** — Uses `scoring.DeriveOutcome(category, protection)` from the scoring domain package, not a second computation path. Consistent with dashboard and analytics blocks.
- **Blind scope in walkthrough** — Steps are filtered via `ListSteps(ctx, engagementID, env.BlindScope)` which applies the blind WHERE clause at the DB level. `TestWalkthroughBlindHidesUnrevealed` verifies the blue seat cannot see unrevealed step names.
- **Findings status labels** — `findingStatusDisplay()` converts snake_case constants to Title Case labels matching the product UI (Open, In Progress, Resolved, Accepted Risk). No server-side i18n needed per M6-EPIC.
- **Evidence appendix** — Uses `NeedsEvidenceOptIn=true` flag on the Definition. When `IncludeEvidence=false`, renders the placeholder text only. When true, collects all executions → evidence per execution. Uses `allowedImageMIMEs` map for image type filtering.
- **No new SQL** — All three blocks read through `DomainFacade` only; grep for `SELECT` in the new files returns zero matches.

### Deviations from ticket

None. All three blocks match the ticket scope exactly.

### Verification

```
go test ./internal/report/blocks/ -run 'Test(Walkthrough|Findings|Evidence|Detail)' -v -count=1   # 14/14 pass
go test ./internal/report/blocks/ -count=1                                                       # all 60+ pass (narrative + analytics + detail)
go vet ./internal/report/...                                                                     # clean
go build ./...                                                                                   # clean
make generate && git diff --stat                                                                 # only intentional changes (server.go, block.go + new files)
```
