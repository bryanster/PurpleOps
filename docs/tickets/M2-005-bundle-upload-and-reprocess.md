# M2-005 — Offline bundle upload + reprocess-from-raw

**Milestone:** M2 · **Size:** M · **Depends on:** M2-003, M2-004

## Why

Air-gapped purple-team installs are normal for this audience. Online fetch and offline install must
be the **same parse path** with different bytes-in. Reprocess-from-raw is how an adapter bugfix
repairs a catalog without another download.

## Scope

**In**

- `POST /content/sources/{sourceId}/bundle` — multipart upload of a release archive (zip/tar.gz)
  matching what the adapter's HTTPS fetch would have produced. Creates a `bundle_import` job,
  runs Parse→Normalize→Apply (no Fetch). Enforces `BLACKLIGHT_CONTENT_MAX_BYTES`.
- `POST /content/sources/{sourceId}/reprocess` — body optional `{ "version": "…" }`. Creates a
  `reprocess` job that opens the last raw snapshot for that version and runs Parse→Normalize→Apply.
  409 if no raw snapshot exists.
- Both paths take the global job slot (`M2-003`).
- Bundle format contract documented in `docs/` (short): per-kind expected layout / filename
  patterns, how to obtain the archive on a connected machine (release URL list).
- `blctl content import-bundle --source <id|kind> --file <path> [--version] [--wait]`
- `blctl content reprocess --source <id|kind> [--version] [--wait]`
- Activity: `content.bundle.imported` (terminal via sync verbs is fine if delta carries kind),
  reprocess uses `content.sync.*` with `kind=reprocess` in delta.
- Progress over the hub exactly like online sync.

**Out**

- Building bundles inside Blacklight.
- Partial/chunked upload resume (single request only).
- Auto-detect kind from archive (source row already knows kind).

## Acceptance criteria

- [ ] Uploading a fixture bundle for the test/fake adapter produces the same DB rows as a Fetch of
      identical bytes (hash-equal raw snapshot).
- [ ] Upload over the configured max size fails before the job runs, with a problem detail naming
      the limit.
- [ ] Reprocess after intentionally breaking normalized rows (test-only) restores them from raw
      without calling HTTP.
- [ ] Reprocess with missing raw → 409.
- [ ] Air-gap proof: with network blocked (test httptest or pull adapter's HTTP client), bundle import
      still succeeds.
- [ ] Concurrent bundle import while another job runs → 409.

## Tests

- Golden fixture zip committed under `internal/content/testdata/`.
- Size-limit test, reprocess test, parity test (fetch path vs bundle path) using the fake adapter.

## Notes for the implementer

- Spool uploads to a temp file under `CONTENT_DIR`, never hold whole archives in memory.
- Virus scanning is out of scope; treat upload as trusted admin input (same as sync URL).
- Do not invent a different Apply for reprocess — if Normalize changes, Apply must upsert cleanly.
