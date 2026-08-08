# M5-012 — Engagement archive export

**Milestone:** M5 · **Size:** L · **Depends on:** M5-011, M3-009

## Why

The self-contained record of an engagement: structure, scores, findings, comments, activity and the
evidence files themselves, in one versioned artefact an operator can keep after the engagement is
archived or the install is retired.

**Export only** (epic decision). There is no import endpoint in v1 — restore is the DuckDB file
backup (`M7-005`). What v1 actually got wrong was not the absence of import but the *format
disagreement* between the two halves, and the fix for that is one documented, versioned, tested
format. The round-trip test is export → re-parse → compare the object graph.

## Scope

**In**

- `GET /engagements/{engagementId}/archive` — `report.read`, spec first,
  `Content-Type: application/zip`.
- **Archive layout**, documented in `docs/analytics.md` (or a new `docs/archive.md` if it outgrows a
  section):

  ```
  manifest.json          formatVersion, exportedAt, tool version, engagement header, attack pin
  engagement.json        scenarios → steps → executions, comments, findings + finding_step
  analytics.json         the M5-004…M5-008 rollups as exported, so the numbers are frozen with the data
  activity.jsonl         the engagement's activity rows, newest last
  evidence/<sha256>       the blob files, content-addressed exactly as on disk
  evidence.json          evidence metadata: filename, caption, side, parent, uploader, size, mime
  ```

- **`formatVersion` is an integer at the top of `manifest.json`**, and a Go constant. A reader that
  does not recognise it must be able to say so before parsing anything else.
- **Streamed.** `archive/zip` writing directly to the response, blobs copied from
  `evidence.Store.Open` with `io.Copy`. The per-engagement evidence quota is **2 GiB**
  (`M3-EPIC`), so buffering an archive in memory takes the process down — this is the ticket's main
  failure mode.
- **What goes in about people:** user id and display name on every authored row. **No email, no
  password hash, no session token, no service token, no MFA secret, no recovery code.** The archive
  leaves the building; treat it as a document handed to a client.
- **What is deliberately absent:** content library rows (the archive names the ATT&CK pin, it does
  not carry ATT&CK), other engagements, platform settings, and any user record beyond the id/name
  pairs above.
- Blind scoping via `M5-009`'s helper. A blue caller in a blind engagement gets an archive without
  unrevealed steps, their executions, their evidence or their activity rows. The manifest says
  `blindFiltered: true` so nobody mistakes a partial archive for the whole record — and, as
  everywhere, it does not say how much is missing.
- `blctl` subcommand for the same export, so an operator can archive without a browser
  (`M0B-014` pattern).

**Out**

- Import, restore, or engagement creation from an archive (epic decision).
- Encryption or password protection of the zip. If that is wanted it is a product decision with its
  own ticket — and `M6-015`'s share links are the more likely place for it.
- Scheduling, retention, or automatic archival on engagement close.
- Cross-engagement or whole-install archives.

## Acceptance criteria

- [ ] Spec first; drift gate green.
- [ ] **Round-trip test:** export the fixture engagement, re-parse the zip in the test, and assert the
      reconstructed object graph equals what the repositories return — every scenario, step,
      execution, comment, finding, finding_step and evidence metadata row. This is the criterion the
      whole ticket exists for.
- [ ] Evidence blobs in the archive match their `sha256` filename — recomputed from the archived
      bytes, not trusted from the metadata.
- [ ] Evidence shared by two executions (content-addressed dedup, `M3-009`) appears **once** in
      `evidence/` and twice in `evidence.json`.
- [ ] `formatVersion` present and asserted; a test documents what a future reader does with an
      unknown value.
- [ ] **No secret material anywhere in the archive.** A test greps every JSON member for the field
      names `events.secretKey` already refuses (`internal/events/activity.go`) plus `email`, and
      fails on a hit. Extend the redaction list rather than the test's exceptions.
- [ ] Streaming proven: a fixture with a large blob exports without the process resident size
      tracking the archive size. At minimum, no test can find a `[]byte` holding a whole blob.
- [ ] Blue in a blind engagement gets a smaller archive, `blindFiltered: true` in the manifest, and no
      unrevealed step id anywhere in any member — including `activity.jsonl` and `evidence.json`.
- [ ] `blctl` produces a byte-identical archive to the HTTP endpoint for the same engagement and
      seat, modulo `exportedAt`.
- [ ] A missing blob on disk (possible after a botched restore) fails the export with a clear problem
      response naming the sha256 — it does not produce a silently incomplete archive.

## Tests

- The round-trip comparison above.
- Blob hash verification and the dedup case.
- Secret-field sweep.
- Blind seat comparison across every archive member.
- Missing-blob failure.
- `blctl` / HTTP equivalence.
- Authz: member, observer, non-member, `reports:read` token.

## Notes for the implementer

- `evidence.Store` gives you `Open(sha256hex)` and `BlobRoot()`; use `Open`, not the path, so the
  layout stays the store's business.
- `activity.jsonl` is newline-delimited JSON precisely so it streams and so a large engagement's
  activity is not one enormous array a reader must hold entirely.
- The manifest is the first thing written to the zip and the first thing a reader parses. Put
  `formatVersion` first in the struct so it is first in the file even to a reader that gives up early.
- This endpoint is on `M7-007`'s security-review list before it ships. Assume it will be read
  adversarially, because a client will receive one.
