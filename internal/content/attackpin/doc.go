// Package attackpin is the ATT&CK version catalog and pin surface (M2-007).
//
// Engagements pin an opaque ATT&CK release label (for example "15.1"). That
// string is equal to content_source_version.version for the attack source —
// nothing here rewrites semver, strips a leading "v", or falls back to another
// installed version. M3 owns engagement.attack_version; this package is the
// single definition of "version" those columns will call.
//
// Normative contracts:
//
//   - Version string rule: docs/content-attack.md § Version strings
//   - Copy-on-use for steps: docs/content-copy-on-use.md
package attackpin
