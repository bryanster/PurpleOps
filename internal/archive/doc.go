// Package archive writes a versioned, self-contained engagement archive as a
// streamed ZIP. The format is documented in docs/archive.md; the one number a
// reader MUST check before anything else is [FormatVersion].
//
// Export only. There is no import or restore path in v1 (M5-EPIC).
//
// See [WriteArchive] for the writer.
package archive
