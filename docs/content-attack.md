# ATT&CK Enterprise content

Blacklight installs **MITRE ATT&CK for Enterprise** from the published STIX 2.1
bundles. Multiple versions coexist so an engagement pinned to `14.1` is not
silently remapped when `15.1` is installed.

## Default source row

Seeded by migration `0011_content` (disabled until an admin enables it):

| Field | Value |
|---|---|
| Kind | `attack` |
| URL | `https://raw.githubusercontent.com/mitre-attack/attack-stix-data/master` |
| Ref | `enterprise-attack/enterprise-attack-{version}.json` |

Effective fetch URL for a pin:

```
{url}/{ref with {version} substituted}
```

Example for 15.1:

```
https://raw.githubusercontent.com/mitre-attack/attack-stix-data/master/enterprise-attack/enterprise-attack-15.1.json
```

## Version discovery

A sync **without** `version` loads `{url}/index.json` and takes the latest
version from the collection named **Enterprise ATT&CK** (`versions[0]` in
MITRE's index layout). Mobile and ICS collections in the same index are ignored.

`GET /content/attack/releases` reads the same index and returns *every*
Enterprise release, merged with what is installed here — the version picker in
the first-run wizard is built on it, and so is anything else that wants to offer
a choice rather than assume "latest".

| Field | Meaning |
|---|---|
| `items[].version` | Upstream's label, unmodified, and what to send back as a pin |
| `items[].released` | Upstream's `modified` date, when it parses; absent otherwise |
| `items[].installed` / `status` | Whether this installation holds that label, and in what state |
| `items[].latest` | Upstream's newest. Absent from every item when the index was not read |
| `reachable` / `unreachable` | Whether MITRE answered, and the error if not |

It reaches upstream while the request is open, takes no job slot, and writes
nothing. **An unreachable index answers `200`, not `502`**: air-gapped
installations are supported and for them that is the normal case, so the answer
still lists what is installed and says why there is nothing to choose from.
Order is upstream's own, newest first — ATT&CK labels (`4.0`, `10.0`, `17.1`)
sort correctly under neither string comparison nor semver, so nothing here
invents an order. Installed labels upstream no longer offers are appended.

## What is ingested

Enterprise domain only:

- tactics (`x-mitre-tactic`)
- techniques and sub-techniques (`attack-pattern`)
- mitigations (`course-of-action` with `M####` external ids)
- groups (`intrusion-set`)
- software (`malware`, `tool`)
- data sources / components (`x-mitre-data-source`, `x-mitre-data-component`)

Relationships stored for library UX:

- technique ↔ tactic (join table)
- technique ↔ mitigation (join table)
- sub-technique → parent (column on the technique row)

## Offline bundles

Same bytes as online fetch. Acceptable shapes:

1. The STIX JSON document itself (`enterprise-attack-15.1.json`).
2. A zip or tar(.gz) archive containing that file. Paths under `mobile-attack/`
   or `ics-attack/` are skipped.

```sh
# Connected host
curl -LO https://raw.githubusercontent.com/mitre-attack/attack-stix-data/master/enterprise-attack/enterprise-attack-15.1.json

# Air-gapped host (server stopped — DuckDB is single-process)
docker compose run --rm blacklight \
  blctl content import-bundle --source attack --file enterprise-attack-15.1.json --version 15.1 --wait
```

Or `POST /api/v1/content/sources/{id}/bundle` with the same file.

## Library API

All require `content.read`. Objects from **disabled** sources are omitted
(list empty / get `404`).

| Method | Path | Filters |
|---|---|---|
| GET | `/content/techniques` | `version`, `q`, `tactic`, `isSubtechnique`, `limit` |
| GET | `/content/techniques/{id}` | — (includes `tactics` + `mitigations`) |
| GET | `/content/tactics` | `version`, `q`, `limit` |
| GET | `/content/tactics/{id}` | |
| GET | `/content/mitigations` | `version`, `q`, `limit` |
| GET | `/content/mitigations/{id}` | |
| GET | `/content/groups` | `version`, `q`, `limit` |
| GET | `/content/groups/{id}` | |
| GET | `/content/software` | `version`, `q`, `limit` |
| GET | `/content/software/{id}` | |


## Version strings

ATT&CK version labels are **opaque text** equal to
`content_source_version.version` (for example `15.1`).

Single normalization rule (pin APIs and docs; ingest uses the same TrimSpace on
the STIX collection label):

1. Trim surrounding whitespace.
2. Reject empty or internal whitespace.
3. Reject reserved tokens `__staging__` and `current`.
4. **Do not** strip a leading `v` / `V`, and **do not** rewrite semver.
   `15.1` and `v15.1` are different strings; only the former matches a MITRE
   release label installed by the adapter.

## Version pin surface (M2-007)

Engagements store `attack_version`. M2 shipped the catalog and resolve
helpers so that column has one definition:

| Method | Path | Authz |
|---|---|---|
| GET | `/content/attack/releases` | `content.read` — upstream's catalog merged with what is installed |
| GET | `/content/attack/versions` | `content.read` |
| GET | `/content/attack/versions/{version}` | `content.read` (includes per-family counts) |
| GET | `/content/attack/versions/{version}/techniques/{externalId}` | `content.read` — **never** cross-version |
| DELETE | `/content/attack/versions/{version}` | `content.manage` — 409 if referenced |

Domain package: `internal/content/attackpin`
(`ListVersions`, `ListReleases`, `Resolve`, `ResolveTechnique`, `AssertPinned`,
`DeleteVersion`).

`AssertPinned` requires: version exists, ATT&CK source enabled, status `ready`,
`item_count > 0`.

Delete isolation: removing version X never mutates version Y. External ref
counts go through `attackpin.References.AttackVersion`. Activity verb:
`content.version.deleted`.

Copy-on-use for steps: [content-copy-on-use.md](content-copy-on-use.md).

## Failure behaviour

- Schema drift, empty technique table, or version label mismatch → job `failed`
  with an operator-readable error (phase + object id when known).
- Apply stages into a private version token, then promotes in one write
  transaction. A failed re-sync leaves the prior ready catalog for that version
  intact.

## Related

- Offline bundle contract: [`content-bundles.md`](content-bundles.md)
- Copy-on-use (steps snapshot catalog fields): [`content-copy-on-use.md`](content-copy-on-use.md)
