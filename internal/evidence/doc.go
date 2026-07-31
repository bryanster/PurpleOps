// Package evidence stores uploaded artefacts — screenshots, logs, packet
// captures — in a content-addressed blob store on local disk, so identical
// uploads deduplicate and a reference cannot outlive its bytes.
//
// Implemented by M3.
package evidence
