// Package attack implements the MITRE ATT&CK Enterprise content adapter (M2-006).
//
// # Fetch
//
// The seeded source row carries:
//
//	url = https://raw.githubusercontent.com/mitre-attack/attack-stix-data/master
//	ref = enterprise-attack/enterprise-attack-{version}.json
//
// A sync with an explicit version substitutes into ref and GETs `{url}/{ref}`.
// A sync without a version discovers the latest Enterprise release from
// `{url}/index.json` (first collection named "Enterprise ATT&CK", versions[0]).
// Tests inject [content.HTTPDoer] / Fetch hooks so CI never hits the network.
//
// # Parse
//
// Accepts a STIX 2.x bundle JSON document, or a zip/tar(.gz) archive that
// contains one. Mobile and ICS paths inside an archive are ignored. The
// collection object's x_mitre_version is the authoritative version label.
//
// # Multi-version isolation
//
// Apply writes into a private staging version token, then promotes to the
// target release label inside one store.Write transaction. A failed re-sync
// leaves the prior ready catalog for that version byte-identical.
//
// # Operator docs
//
// See docs/content-attack.md for the URL template and offline bundle shape.
package attack
