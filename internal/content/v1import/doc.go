// Package v1import parses PurpleOps/Blacklight v1 custom content files into
// structured records ready for upsert under the singleton custom source.
//
// Supported layouts (PLAN.md §3 / M2-012):
//
//  1. testcases.json — the array (or {testcases:[…]}) the repo shipped
//  2. testcases/*.yaml — the directory glob the v1 seeder expected
//  3. knowledgebase/*.yaml — KB notes (overview/advice → markdown body)
//  4. a zip mixing any of the above, plus the M2-011 custom export shape
//
// The package never writes to the database. Callers (content.Importer) apply
// the parsed records. Re-import is safe when external ids are taken from
// [ExternalIDForTestcase] / [ExternalIDForNote].
package v1import
