# M3-009 — Evidence blob store + upload/download API

**Milestone:** M3 · **Size:** L · **Depends on:** M3-006, M3-001

## Why

Evidence is content-addressed so dedup is free and deletes don’t clobber shared blobs (`PLAN.md`
§2). Safe serving matters: no inline HTML execution. Quotas stop a single engagement filling the
disk (`M3-EPIC`: 25 MiB/file, 2 GiB/engagement defaults).

## Scope

**In**

- Config (`M0B-002` style): `BLACKLIGHT_EVIDENCE_DIR`, `BLACKLIGHT_EVIDENCE_MAX_UPLOAD_BYTES`
  (default 25 MiB), `BLACKLIGHT_EVIDENCE_MAX_ENGAGEMENT_BYTES` (default 2 GiB), MIME allowlist
  (e.g. png/jpeg/gif/webp/pdf/txt/csv/json/zip + common log types — document; reject
  `text/html`, `application/xhtml+xml`, executable types).
- Disk layout: `evidence/blobs/{sha256[0:2]}/{sha256}` (or equivalent); DB holds relative path + hash.
- Package `internal/evidence`: `Put(ctx, r io.Reader, size, mime) (sha256, err)`, `Open(sha256)`,
  `AddRef` / `ReleaseRef` (refcount on `evidence_blob`), engagement quota accounting.
- New authz **`evidence.write`** (`ActionEvidenceWrite`): lead + red + blue (not observer). Token
  `engagements:write`. Domain enforces **side**: red seat → `side=red` only; blue → `blue`; lead →
  either; admin-as-member follows seat; admin without seat may write either for support.
- Spec-first:

  | Method | Path | Action |
  |---|---|---|
  | `POST` | `/executions/{executionId}/evidence` | `evidence.write` | multipart: file, caption, side |
  | `GET` | `/executions/{executionId}/evidence` | `execution.read` |
  | `GET` | `/evidence/{evidenceId}` | `execution.read` | metadata |
  | `GET` | `/evidence/{evidenceId}/content` | `execution.read` | bytes |
  | `DELETE` | `/evidence/{evidenceId}` | `evidence.write` | uploader, lead, or admin |

- **Download safety:** `Content-Type` from stored mime (allowlist),
  `Content-Disposition: attachment` (filename starred/UTF-8),
  `X-Content-Type-Options: nosniff`, **no** `inline` for HTML-ish types (already rejected on
  upload). Do not sniff magic to a looser type than stored without re-validation.
- Upload: hash while streaming to temp file; rename into blob path; if sha exists, delete temp and
  increment ref_count; insert `evidence` row; enforce per-file and per-engagement quotas **before**
  commit (engagement total = sum of evidence sizes linked to executions in that engagement —
  counting unique blobs once is nicer but sum-of-links is acceptable if documented; prefer
  **unique blob bytes per engagement**).
- Delete evidence row: decrement ref; if ref_count=0 remove blob file in After hook / same txn
  bookkeeping.
- Blind: evidence on unrevealed execution follows execution.read conceal.
- Activity: `evidence.uploaded`, `evidence.deleted`.
- Closed engagement: no new uploads (409); download still allowed for members.

**Out**

- Comment-attached evidence may be phased: support `comment_id` parent if schema has it; else
  execution-only in M3 and leave comment link for later — **prefer execution-only** to shrink
  scope; schema column can remain unused.
- Virus scanning.
- UI lightbox (`M3-014` may show links).

## Acceptance criteria

- [ ] Same bytes uploaded twice → one blob file, two evidence rows, ref_count=2; delete one → file
      remains; delete both → file gone.
- [ ] Oversize file → 413/400; over engagement quota → 409/413.
- [ ] `text/html` upload rejected.
- [ ] Content response carries attachment disposition + nosniff.
- [ ] Observer cannot upload; can download if they can read the execution.
- [ ] Red cannot upload `side=blue`.

## Tests

- Store unit tests for refcount GC and quota.
- Handler upload/download/delete with temp dirs.
- Authz side matrix.

## Notes for the implementer

- Never load whole file into memory for hashing if avoidable — stream.
- Paths: reject `..` like content raw paths (`M2-001`).
- Coordinate engagement delete (`M3-002`) to release all evidence refs.
