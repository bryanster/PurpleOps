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

`q` is a case-insensitive substring over `externalId`, `name`, and `description`.

## Failure behaviour

- Schema drift, empty technique table, or version label mismatch → job `failed`
  with an operator-readable error (phase + object id when known).
- Apply stages into a private version token, then promotes in one write
  transaction. A failed re-sync leaves the prior ready catalog for that version
  intact.

## Related

- Offline bundle contract: [`content-bundles.md`](content-bundles.md)
- Version pin surface for engagements: ticket `M2-007`
