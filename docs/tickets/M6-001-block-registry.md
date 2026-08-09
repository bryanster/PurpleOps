# M6-001 — Block registry (id, params schema, data deps, renderer hook)

**Milestone:** M6 · **Size:** M · **Depends on:** M5-015

## Why

Every report section is a registered block: identity, parameters, which analytics/domain data it
needs, and how it renders to HTML. Without a single registry, M6 grows a pile of ad-hoc sections
and a second aggregation path — the failure `M5-009` exists to prevent.

## Scope

**In**

- Package `internal/report` (or `internal/report/block`) with:
  - `type ID string` — stable block ids (`cover`, `executive_summary`, `scope_roe`, `rich_text`,
    `page_break`, `coverage_heatmap`, `tactic_scorecard`, `detection_distribution`, `detection_gaps`,
    `mttd`, `engagement_compare`, `scenario_walkthrough`, `findings_backlog`, `evidence_appendix`).
  - `type Definition` — `ID`, human `Title`, `Description`, `ParamsSchema` (JSON Schema as Go
    struct tags or embedded schema the API can expose), `DataDeps` (which analytics queries /
    domain reads the block needs), `DefaultParams`, flags (`AllowInTemplate`, `NeedsEvidenceOptIn`).
  - `type Registry` — register at init; `Get(id)`, `List()`, reject duplicate ids.
  - `type Instance` — `InstanceID`, `BlockID`, `Ordinal`, `Params json.RawMessage` (validated).
  - `ValidateParams(id, params) error` — unknown fields rejected; defaults applied.
- **No HTML output yet** — renderer hook is an interface the later tickets implement:
  `type Renderer interface { Render(ctx, env RenderEnv, inst Instance) (HTMLFragment, error) }`.
  Registry may hold optional `Renderer` nil until M6-006+.
- `RenderEnv` sketch (concrete fields filled by M6-009): engagement meta, branding, analytics
  facade, evidence access policy, locale/format helpers, `IncludeEvidence bool`, blind scope
  (always full/lead for publish; seat-scoped only for draft preview).
- Unit tests: register built-in stubs (empty renderers OK), duplicate id panics/fails at init,
  param validation accept/reject table.
- Package doc states: blocks **must not** embed SQL aggregates; data comes from `internal/analytics`
  / domain repos injected via `RenderEnv`.

**Out**

- Persistence (`M6-002`), HTML (`M6-009`), any concrete block body (`M6-006`–`M6-008`), UI.

## Files

- `internal/report/doc.go`, `block.go`, `registry.go`, `params.go`, `registry_test.go`
- Optional `internal/report/blockids.go` constants

## Acceptance criteria

- [ ] All v1 block ids are declared as constants; `List()` returns them in a stable order.
- [ ] Duplicate registration fails loudly at init (test).
- [ ] `ValidateParams` rejects unknown keys and wrong types; applies defaults for omitted keys.
- [ ] Package doc forbids second aggregation path and names `docs/analytics.md` as label source.
- [ ] No HTTP surface yet; no DB migrations.

## Tests

- Table-driven param validation per stub definition (at least one block with required params,
  e.g. `engagement_compare.baselineEngagementId`).
- Registry completeness: every id in the epic catalogue is registered.

## Notes for the implementer

- Keep JSON Schema modest — object with typed properties is enough; no `$ref` gymnastics.
- `engagement_compare` params must include `baselineEngagementId` (UUID); current engagement is
  always the report's engagement.
- Page break has empty params.
