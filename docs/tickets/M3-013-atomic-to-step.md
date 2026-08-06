# M3-013 — Atomic / procedure template → Step

**Milestone:** M3 · **Size:** M · **Depends on:** M3-005, M2-008, `docs/content-copy-on-use.md`

## Why

Atomic structure must survive into the workbook — platform, executor, command, cleanup, args —
not a flattened `actions` string (`PLAN.md` §3). Copy-on-use snapshots the template at insert time.

## Scope

**In**

- Spec-first (either dedicated endpoint or documented POST `/steps` body variant — prefer explicit):

  | Method | Path | Action |
  |---|---|---|
  | `POST` | `/engagements/{engagementId}/scenarios/{scenarioId}/steps/from-template` | `workbook.write` |

- Body: `template_id` (content procedure template UUID), optional name/objective/target overrides,
  optional arg values map for template input_args.
- Resolve template from content store; refuse disabled source (`AssertReferencable`).
- Snapshot into step.procedure JSON: platforms, executor, command, cleanup, resolved args,
  dependency metadata as available — **preserve structure**.
- Technique ids on template → resolve against engagement pin when present; snapshot technique /
  subtechnique / tactic external ids onto the step.
- `template_id` lineage only (no FK). `attack_version` = engagement pin.
- Creates pending execution in same txn.
- Activity: `step.created` with template lineage in delta.

**Out**

- Editing templates (M2 custom API).
- Running executors (Blacklight does not execute attacks).
- UI (`M3-014` library “add to scenario”).

## Acceptance criteria

- [ ] Import Atomic fixture template → step.procedure has distinct command and cleanup when both
      exist in template.
- [ ] Template edit/re-sync after create does not change step snapshot.
- [ ] Unknown template → 404; disabled source → 409.
- [ ] Technique resolve uses engagement pin only.

## Tests

- Fixture template → step round-trip field assertions.
- Snapshot isolation after content update.

## Notes for the implementer

- Share code paths with manual step create where possible.
- Arg substitution: simple `#{key}` replacement is enough if Atomic uses that; do not build a
  scripting engine. Document unresolved placeholders left as-is or 400 — prefer 400 on required
  missing args when template marks required.
