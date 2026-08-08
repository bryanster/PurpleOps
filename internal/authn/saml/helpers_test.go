package saml

import (
	"encoding/xml"
	"log/slog"
	"testing"

	crewjam "github.com/crewjam/saml"
)

// quietLogger sends this package's warnings to the test log, so a failure shows
// what the provider said about it and a passing run says nothing.
func quietLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Logf("%s", p)
	return len(p), nil
}

// parseOwnMetadata reads back the document this deployment publishes.
//
// Deliberately not [parseMetadata], which is for the *identity provider's*
// metadata and holds it to a standard this document could never meet: an
// IDPSSODescriptor carrying a signing certificate. What is published here is an
// SPSSODescriptor, and running it through the wrong reader would be a test
// asserting something nobody claimed.
func parseOwnMetadata(raw []byte) (*crewjam.EntityDescriptor, error) {
	var descriptor crewjam.EntityDescriptor
	if err := xml.Unmarshal(raw, &descriptor); err != nil {
		return nil, err
	}
	return &descriptor, nil
}
