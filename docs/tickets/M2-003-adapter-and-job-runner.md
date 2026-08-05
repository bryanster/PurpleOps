# M2-003 — Adapter interface + global DB-backed job runner

**Milestone:** M2 · **Size:** L · **Depends on:** M2-001, M2-002

## Why

Every upstream kind shares one pipeline: **Fetch → Parse → Normalize → Upsert**, with progress,
cancellation, and a single global job slot so sync cannot starve the war-room write path. Get the
interface and runner wrong and every adapter ticket reinvents concurrency and error handling.

## Scope

**In**

- `internal/content` package:
  - `Adapter` interface roughly:
    ```go
    type Adapter interface {
        Kind() Kind
        // Fetch retrieves bytes (HTTPS). Bundle/reprocess supply a Source instead.
        Fetch(ctx context.Context, req FetchRequest) (Bundle, error)
        Parse(ctx context.Context, bundle Bundle) (Ast, error)
        Normalize(ctx context.Context, ast Ast) ([]Object, error)
        // Apply upserts in batches via the provided Writer; reports progress.
        Apply(ctx context.Context, w Writer, objects []Object, prog Progress) error
    }
    ```
  - Adapt names to taste; properties are fixed: fetch is optional when a bundle is already local;
    parse/normalize are pure-ish; apply does all writes.
  - `Source` abstraction for bytes: `HTTPFetch`, `Filesystem` (raw snapshot), `Upload` (temp file).
- **Global single job runner:**
  - At most one content job `running` in the installation.
  - `POST /content/sources/{id}/sync` enqueues (`queued`) or 409 if one is active/queued.
  - Worker picks jobs, sets `running`, runs adapter pipeline, terminal status + source/version
    bookkeeping.
  - Cancellation: `POST /content/jobs/{id}/cancel` flips `cancelling`; adapter steps observe
    `ctx.Done()`. Final status `cancelled`.
  - Progress fields updated at phase boundaries and periodically during apply (batch N).
  - Job timeout from config (default 30m); on timeout mark `failed` with clear error.
  - Download size limit (default 512 MiB) enforced in Fetch.
- Endpoints (spec-first):
  - `POST /content/sources/{sourceId}/sync` — body may include `{ "version": "15.1" }` for ATT&CK
    when selecting a known release; omit to mean "latest discoverable" per adapter.
  - `GET /content/jobs/{jobId}`
  - `GET /content/jobs?status=&sourceId=` (admin)
  - `POST /content/jobs/{jobId}/cancel`
- Authz: `content.sync` for start/cancel; `content.read` for GET job if the subject could read the
  source (admin-only is acceptable for job list in M2 — document).
- Writes during `Apply`:
  - Batched `store.Write` transactions (batch size configurable, default large enough to be fast
    and small enough not to hold the lock for seconds).
  - Never hold the write lock across network I/O.
  - Interactive requests must remain healthy during sync — proven later in `M2-016`; design for it
    now.
- Upsert rules:
  - ATT&CK multi-version: apply into the target version key; never mutate another version's rows.
  - Rolling sources: replace `current` objects atomically from the caller's POV (stage table or
    delete+insert in one transaction per batch family — document; prefer no half-visible catalog).
- On success: update `content_source.last_synced_at`, `item_count`, clear `error`; write/replace
  version row; persist **raw snapshot** (last successful only — delete previous raw for that
  version).
- On failure: set source/version `error` message; leave prior successful catalog intact.
- Activity: `content.sync.started`, `.finished`, `.failed`, `.cancelled`.
- `blctl content sync --source <id|kind> [--version] [--wait]`.
- Config: `BLACKLIGHT_CONTENT_DIR`, `BLACKLIGHT_CONTENT_MAX_BYTES` (512 MiB),
  `BLACKLIGHT_CONTENT_JOB_TIMEOUT` (30m), `BLACKLIGHT_CONTENT_WRITE_BATCH` .

**Out**

- Concrete adapters (`M2-006`…`M2-010`) — ship a `noop` or fixture adapter used only in tests.
- SSE stream (`M2-004`) — runner must expose an in-process progress subscribe hook (channel/callback
  registry) so M2-004 is wiring, not a rewrite.
- Bundle upload endpoint (`M2-005`) — runner accepts a pre-seated bundle path/kind for jobs created
  by that ticket.

## Acceptance criteria

- [ ] Starting a second sync while one runs returns 409 with the active `jobId`.
- [ ] Cancel during a slow test adapter stops before further Apply batches; job ends `cancelled`.
- [ ] Process restart: jobs left `running` become `interrupted` (helper from `M2-001`); source not
      stuck forever in `syncing`.
- [ ] Fetch failure does not delete existing catalog rows.
- [ ] Successful apply with the test adapter writes objects and a raw snapshot file whose sha256
      matches the DB.
- [ ] No network I/O inside `store.Write`.
- [ ] Context cancel while waiting for the write lock aborts the batch (`M0B-003` contract).
- [ ] Oversized download fails with a problem detail naming the limit.

## Tests

- Runner unit/integration tests with a fake adapter (phases, progress, cancel, fail, success).
- Global mutex/slot test: two parallel start calls → one job, one 409.
- Batch write test: Apply of N objects uses multiple Write calls when N > batch size.
- Boot reconciliation test for interrupted jobs.

## Notes for the implementer

- Do not use a general job framework. One worker goroutine + DB rows is enough for single-node.
- Progress callback signature should carry `jobId`, `phase`, counters, optional message — M2-004
  maps these to SSE event types.
- Adapters register in a kind→adapter map at server wiring; unknown kind on a source is a hard
  error at sync start.
