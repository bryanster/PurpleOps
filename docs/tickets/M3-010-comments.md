# M3-010 — Comments on executions + edit history

**Milestone:** M3 · **Size:** S · **Depends on:** M3-006

## Why

Observers’ only write is comment (`PLAN.md` §4, `comment.write`). Edit history keeps the audit trail
honest when someone “fixes” a note after the fact.

## Scope

**In**

- Spec-first:

  | Method | Path | Action |
  |---|---|---|
  | `GET` | `/executions/{executionId}/comments` | `execution.read` |
  | `POST` | `/executions/{executionId}/comments` | `comment.write` |
  | `PATCH` | `/comments/{commentId}` | `comment.write` | author or lead |
  | `GET` | `/comments/{commentId}/revisions` | `execution.read` |

- Create: `body` (markdown/plain text; max length e.g. 16 KiB). `author_id` = subject.
- Edit: updates `body`, sets `edited_at`; appends **previous** body to `comment_revision` (so history
  is complete). Only **author** or engagement **lead** (or admin) may edit. No delete in v1
  (or soft-delete empty body — prefer **no delete**).
- Blind: cannot comment on unrevealed execution (same conceal as read).
- Closed engagement: allow comment? **Allow** — notes after close are useful; document. Or 409 —
  prefer **allow**.
- Activity: `comment.created`, `comment.edited`.

**Out**

- Real-time threads UI polish (M4-006). Mentions/reactions.
- Evidence on comments (unless already free from M3-009).

## Acceptance criteria

- [ ] Observer can POST comment; cannot PATCH someone else’s.
- [ ] Edit creates revision row with prior body; GET revisions ordered by time.
- [ ] Unrevealed blind → 404 conceal on list/create.
- [ ] Body over max length → 400.

## Tests

- Handler create/edit/list/revisions.
- Authz: observer write; red/blue/lead write; non-member deny.

## Notes for the implementer

- Sanitize nothing into HTML on server — store text; UI escapes.
