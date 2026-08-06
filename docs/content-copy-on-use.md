# Copy-on-use contract (content → engagement steps)

Normative for M3. M2 ships the pin surface and this contract so engagement
code cannot invent a second definition of "use a technique from the library".

## Why

Catalog syncs must never rewrite war-room history. An engagement pinned to
ATT&CK `14.1` keeps seeing `14.1` objects even after `15.1` is installed. A step
created from a procedure template must keep the wording the operator saw when
they created it, even if the template is later edited or the source re-synced.

## Rules

1. **Pin is opaque.** `engagement.attack_version` (M3) stores the exact string
   returned by the pin catalog — equal to `content_source_version.version` for
   the ATT&CK source (for example `15.1`). No semver rewriting, no leading-`v`
   strip, no implicit "latest". See [content-attack.md](content-attack.md)
   § Version strings.

2. **Resolve never crosses versions.** Library pickers and step-create paths
   call:

   - `attackpin.AssertPinned(ctx, engagement.attack_version)` before offering
     or accepting a technique from the catalog.
   - `attackpin.ResolveTechnique(ctx, engagement.attack_version, externalID)`
     (or the HTTP equivalent
     `GET /content/attack/versions/{version}/techniques/{externalId}`) to load
     the object. A miss is not-found even when the external id exists under
     another installed version.

3. **Steps snapshot display fields.** When a step is created from a technique
   and/or procedure template, the step row stores **copies** of the fields the
   UI showed and the operator will execute against, at minimum:

   | Snapshot field | Source |
   |---|---|
   | Display name | technique / template name |
   | Description / procedure body | technique description and/or template procedure JSON |
   | `technique_external_id` | MITRE id (lineage only) |
   | `template_id` | procedure template id when used (lineage only) |
   | `attack_version` | engagement pin at create time |

   Lineage ids are optional breadcrumbs for "open in library". They are **not**
   live foreign keys. Subsequent catalog syncs, template edits, version
   deletes, and source disables must not alter stored step snapshots.

4. **Disable and delete stay out of history.** Disabling a source or deleting
   a version refuses **new** references (`AssertPinned` / `AssertReferencable`,
   delete 409 via `References.AttackVersion`). Existing steps keep their
   snapshots.

## Out of scope here

- The `engagement.attack_version` column and engagement CRUD (M3-001).
- UI pickers (M3 / M2-013 may list versions; they must still call AssertPinned
  when the engagement pin is set).
- Custom tactics/techniques (not a product feature).

## CTID plan → scenario (M3-012)

Emulation-plan import snapshots plan/step fields the same way procedure
templates do. Field mapping lives in [content-ctid.md](content-ctid.md)
§ M3 import contract. Lineage may keep `plan_id` / step external ids; they are
not live foreign keys. Plan `metadata.attack_version` is advisory only — the
engagement pin is authoritative for technique resolve.

## Related

- Pin API: `internal/content/attackpin`
- HTTP: `GET/DELETE /content/attack/versions…`
- ATT&CK adapter & version coexistence: [content-attack.md](content-attack.md)
- CTID catalog & import mapping: [content-ctid.md](content-ctid.md)
