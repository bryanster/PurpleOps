# Offline content bundles

Air-gapped deployments install reference content the same way connected ones do —
**one parse path, different bytes-in**. Online sync fetches a release archive
over HTTPS; offline install uploads that same archive. Reprocess re-reads the
last successful raw snapshot with no network at all.

This page is the operator contract for obtaining and uploading bundles. Adapter
tickets (`M2-006`…`M2-010`) own the per-kind layout details; until a kind's
adapter lands, that kind cannot be synced or imported.

## Limits

| Knob | Env | Default |
|---|---|---|
| Archive / download ceiling | `BLACKLIGHT_CONTENT_MAX_BYTES` | `512MiB` |
| Job wall-clock | `BLACKLIGHT_CONTENT_JOB_TIMEOUT` | `30m` |
| Content data root | `BLACKLIGHT_CONTENT_DIR` | `./content` |

Uploads over the ceiling are refused **before** a job is enqueued. The problem
detail names the limit.

## How to obtain a bundle (connected machine)

1. Identify the source kind and, for ATT&CK, the release label you want pinned.
2. Download the upstream release archive with the same URL the source row carries
   (see `blctl content sources` / the Sources admin UI). Builtin seed URLs:

   | Kind | What to download |
   |---|---|
   | `attack` | MITRE ATT&CK Enterprise STIX bundle for one release — see [`content-attack.md`](content-attack.md) for the exact URL template (`enterprise-attack-{version}.json`, optionally inside zip/tar). |
   | `atomic` | Atomic Red Team atomics release archive (yaml tree). |
   | `sigma` | SigmaHQ rules release that includes ATT&CK-mapped detections — see [`content-sigma.md`](content-sigma.md). |
   | `ctid` | CTID adversary-emulation plan catalog archive — see [`content-ctid.md`](content-ctid.md). |

3. Copy the file to the air-gapped host unchanged. Do not re-pack or rename
   internals — the adapter's Parse step expects the upstream layout.

Exact filename patterns and nested paths are locked when each adapter ticket
lands. Until then, treat the online Fetch output as the golden shape: whatever
bytes a successful `POST .../sync` stored under
`{CONTENT_DIR}/raw/{source_id}/{version}/{sha256}` is a valid bundle for that
source/version.

## Install offline

### UI / API

```http
POST /api/v1/content/sources/{sourceId}/bundle
Content-Type: multipart/form-data

file: <archive bytes>
version: 15.1          # optional; ATT&CK pin
```

Requires `content.sync` (platform admin, or a service token scoped
`content:sync`). Returns `202` with the `bundle_import` job. Progress streams on
`GET /api/v1/events?topics=content.jobs` (session only).

### CLI

```sh
# Stop the server first — DuckDB is single-process.
docker compose stop
docker compose run --rm blacklight \
  blctl content import-bundle --source atomic --file /bundles/atomics.zip --wait
```

## Reprocess from raw

After an adapter bugfix, re-parse the last successful snapshot without another
download:

```http
POST /api/v1/content/sources/{sourceId}/reprocess
{ "version": "15.1" }   # required for ATT&CK; omit → current for rolling sources
```

```sh
blctl content reprocess --source atomic --wait
blctl content reprocess --source attack --version 15.1 --wait
```

Answers `409` when no raw snapshot exists for that source/version.

## Shared pipeline

```
Fetch (online) ─┐
Bundle upload  ─┼─→ Parse → Normalize → Apply → raw snapshot on disk
Reprocess raw  ─┘
```

- **One global job slot.** A second start (sync, bundle, or reprocess) while any
  job is queued/running/cancelling is `409` and names the active `jobId`.
- **Failure leaves the prior catalog intact.** Only a successful Apply replaces
  objects and the raw snapshot for that version.
- **Activity.** Jobs use `content.sync.started|finished|failed|cancelled` with
  `kind` (`sync` / `bundle_import` / `reprocess`) in the delta.

## Security note

Bundle upload is trusted admin input, the same as configuring a sync URL. There
is no virus scanning in M2. Treat the content data directory as sensitive.
