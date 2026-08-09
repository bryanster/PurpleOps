package report

import (
	"context"
	"encoding/json"
)

// ID is a stable block identifier from the v1 catalogue.
type ID string

// Definition describes one registered block: identity, parameter shape, and
// the data it needs to render.
type Definition struct {
	// ID is the stable block identifier.
	ID ID

	// Title is the human-readable name shown in the block picker.
	Title string

	// Description explains what the block adds to a report.
	Description string

	// ParamsSchema declares the JSON Schema for block parameters.
	// When nil, the block accepts no parameters.
	ParamsSchema ParamSchema

	// DefaultParams are applied when params are omitted or empty.
	DefaultParams json.RawMessage

	// DataDeps lists the analytics or domain queries this block needs.
	// Consumers (M6-009) use this to pre-fetch data before rendering.
	DataDeps []DataDep

	// AllowInTemplate controls whether this block may appear in an
	// engagement-scoped report template (M6-003).
	AllowInTemplate bool

	// NeedsEvidenceOptIn controls whether this block must respect the
	// publish-time evidence-inclusion flag (M6-011).
	NeedsEvidenceOptIn bool

	// HTMLParamKeys lists the param keys whose values are HTML and
	// must be sanitized on write (M6-005). An empty slice means
	// no params contain HTML.
	HTMLParamKeys []string
}

// DataDep names a data dependency a block needs at render time.
type DataDep string

// Instance is one block placed in a report draft. It pairs a block ID with
// validated parameters at a specific ordinal position.
type Instance struct {
	// InstanceID is unique per report, generated client-side.
	InstanceID string `json:"instanceId"`

	// BlockID identifies which block definition this instance uses.
	BlockID ID `json:"blockId"`

	// Ordinal is the zero-based position in the report's block list.
	Ordinal int `json:"ordinal"`

	// Params is the validated, defaults-applied parameter JSON.
	Params json.RawMessage `json:"params"`
}

// HTMLFragment is a block's rendered output — a complete, self-contained
// piece of HTML that the report assembler concatenates.
type HTMLFragment string

// Renderer turns a block Instance into an HTML fragment. Each concrete
// block type implements this interface (M6-006 through M6-008).
//
// A nil Renderer is valid — blocks that have not been implemented yet
// produce an empty fragment with no error.
type Renderer interface {
	Render(ctx context.Context, env RenderEnv, inst Instance) (HTMLFragment, error)
}

// RenderEnv is the rendering context injected into every block renderer.
// Fields are filled by the report assembler (M6-009) from the engagement,
// branding config, analytics facade, and publish-time flags.
type RenderEnv struct {
	// EngagementID is the engagement this report belongs to.
	EngagementID string

	// EngagementName is the human-readable engagement name.
	EngagementName string

	// EngagementClient is the client organisation name.
	EngagementClient string

	// Branding holds the resolved branding (install defaults merged with
	// per-report overrides).
	Branding BrandingConfig

	// Analytics provides read access to analytics rollups. Blocks MUST
	// NOT embed SQL; they read through this facade only.
	// Set in M6-009.
	Analytics AnalyticsFacade

	// Evidence provides access to evidence blobs. Set in M6-009.
	Evidence EvidenceAccess

	// IncludeEvidence is the publish-time evidence opt-in flag. When false,
	// blocks that depend on evidence must omit binary content.
	IncludeEvidence bool

	// BlindScope carries the seat scope for draft previews.
	// Published versions always use the full/lead scope (M6-EPIC).
	BlindScope string

	// Locale and format helpers (fixed for v1: ISO dates, en-US grouping).
	// Set in M6-009.
	Format FormatHelpers
}

// AnalyticsFacade is the interface report blocks use to read analytics data.
// Concrete type defined in M6-009; declared here so blocks can reference it.
type AnalyticsFacade interface {
	// Methods defined by M6-009.
}

// EvidenceAccess is the interface report blocks use to read evidence.
// Concrete type defined in M6-009; declared here so blocks can reference it.
type EvidenceAccess interface {
	// Methods defined by M6-009.
}

// BrandingConfig holds the resolved branding for a report.
// Concrete type defined by M6-004; declared here so RenderEnv can carry it.
type BrandingConfig struct {
	// LogoRef is a reference to the logo blob.
	LogoRef string

	// PrimaryColor is the primary brand colour (hex).
	PrimaryColor string

	// SecondaryColor is the secondary brand colour (hex).
	SecondaryColor string

	// FirmName is the firm or team name.
	FirmName string

	// ClientName overrides the engagement client name for this report.
	ClientName string
}

// FormatHelpers provides locale/format functions for report rendering.
// Concrete type defined in M6-009; declared here so RenderEnv can carry it.
type FormatHelpers struct {
	// Methods defined by M6-009.
}
