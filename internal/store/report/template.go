package report

import (
	"encoding/json"
	"time"
)

// Template is one saved report template, owned by an engagement.
type Template struct {
	ID           string
	EngagementID string
	Name         string
	CreatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TemplateBlock is one ordered block instance in a template.
type TemplateBlock struct {
	TemplateID string
	Ordinal    int
	BlockID    string
	Params     json.RawMessage
}

// NewTemplate describes the caller's half of creating a template.
type NewTemplate struct {
	EngagementID string
	Name         string
	CreatedBy    string
}

// TemplateUpdate describes the caller's half of patching a template.
type TemplateUpdate struct {
	Name      *string
	UpdatedBy string
}

// NewTemplateBlock describes the caller's half of creating a template block.
type NewTemplateBlock struct {
	BlockID string
	Params  json.RawMessage
}
