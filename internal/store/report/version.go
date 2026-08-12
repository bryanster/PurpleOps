package report

import (
	"time"
)

// ReportVersion is one immutable published snapshot of a report (M6-011).
// Content columns (BlocksJSON, BrandingJSON, HTML, ContentSHA256) are
// never updated after insert — the store enforces this.
type ReportVersion struct {
	ID              string
	ReportID        string
	Ordinal         int
	Title           string
	PublishedBy     string
	PublishedAt     time.Time
	IncludeEvidence bool
	BlindScope      string
	BlocksJSON      string
	BrandingJSON    string
	HTML            string
	ContentSHA256   *string
	PDFSHA256       *string
}

// NewVersion describes the caller's half of creating a published version.
type NewVersion struct {
	ReportID        string
	Ordinal         int
	Title           string
	PublishedBy     string
	IncludeEvidence bool
	BlindScope      string
	BlocksJSON      string
	BrandingJSON    string
	HTML            string
	ContentSHA256   string
}
