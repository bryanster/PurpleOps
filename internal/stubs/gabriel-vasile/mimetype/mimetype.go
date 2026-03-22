// Package mimetype is a stub replacement for github.com/gabriel-vasile/mimetype.
// It provides the minimal interface needed to compile github.com/go-playground/validator/v10.
package mimetype

import "io"

// MIME represents a MIME type.
type MIME struct {
	mime string
}

func (m *MIME) String() string {
	if m == nil {
		return "application/octet-stream"
	}
	return m.mime
}

// DetectReader detects the MIME type of the content from r.
func DetectReader(r io.Reader) (*MIME, error) {
	return &MIME{mime: "application/octet-stream"}, nil
}
