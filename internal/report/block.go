package report

import (
	"context"
	"encoding/json"
	"fmt"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	"io"
	"math"
	"strings"
	"time"

	"github.com/bryanster/blacklight/internal/analytics"
	"github.com/bryanster/blacklight/internal/store/blind"
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

	// EngagementStartsOn is the engagement start date.
	EngagementStartsOn time.Time

	// EngagementEndsOn is the engagement end date.
	EngagementEndsOn time.Time

	// Branding holds the resolved branding (install defaults merged with
	// per-report overrides).
	Branding BrandingConfig

	// Analytics provides read access to analytics rollups. Blocks MUST
	// NOT embed SQL; they read through this facade only.
	// Set in M6-009.
	Analytics AnalyticsFacade

	// Evidence provides access to evidence blobs. Set in M6-009.
	Evidence EvidenceAccess

	// Domain provides access to engagement domain data: scenarios, steps,
	// executions, findings, and evidence. Blocks MUST read through this
	// facade only — no direct DB access. Set in M6-009.
	Domain DomainFacade

	// IncludeEvidence is the publish-time evidence opt-in flag. When false,
	// blocks that depend on evidence must omit binary content.
	IncludeEvidence bool

	// BlindScope carries the seat scope for draft previews.
	// Published versions always use the full/lead scope (M6-EPIC).
	BlindScope blind.Scope

	// Locale and format helpers (fixed for v1: ISO dates, en-US grouping).
	// Set in M6-009.
	Format FormatHelpers
}

// AnalyticsFacade is the interface report blocks use to read analytics data.
// M6-007 fills the concrete methods; *analytics.Queries satisfies it directly.
// M6-009 sets it on RenderEnv during assembly.
type AnalyticsFacade interface {
	TechniqueCoverage(ctx context.Context, scope analytics.Scope) (*analytics.TechniqueCoverageResult, error)
	TacticCoverage(ctx context.Context, scope analytics.Scope) (*analytics.TacticCoverageResult, error)
	CategoryDistribution(ctx context.Context, scope analytics.Scope) (*analytics.DistributionResult, error)
	ProtectionRate(ctx context.Context, scope analytics.Scope) (*analytics.DistributionResult, error)
	OutcomeMix(ctx context.Context, scope analytics.Scope) (*analytics.DistributionResult, error)
	MTTD(ctx context.Context, scope analytics.Scope) (*analytics.MTTDResult, error)
	Compare(ctx context.Context, scope analytics.CompareScope) (*analytics.CompareResult, error)
}

// EvidenceAccess provides read access to evidence blobs.
// Methods are defined by M6-009.
type EvidenceAccess interface {
	// OpenEvidence returns a reader for the blob identified by its SHA-256
	// hex digest. The caller must close the reader.
	OpenEvidence(sha256hex string) (io.ReadCloser, error)
}

// DomainFacade is the interface report blocks use to read engagement domain
// data: scenarios, steps, executions, findings, and evidence metadata.
// The concrete type wraps the engagement store repositories.
// Set in M6-009.
type DomainFacade interface {
	ListScenarios(ctx context.Context, engagementID string) ([]storengagement.Scenario, error)
	ListSteps(ctx context.Context, engagementID string, scope blind.Scope) ([]storengagement.Step, error)
	ListExecutions(ctx context.Context, engagementID string) ([]storengagement.Execution, error)
	ListFindings(ctx context.Context, engagementID string) ([]storengagement.Finding, error)
	FindingSteps(ctx context.Context, findingID string) ([]storengagement.Step, error)
	ListEvidence(ctx context.Context, executionID string) ([]storengagement.Evidence, error)
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
// Methods are defined here (M6-007) — fixed for v1: ISO dates, en-US grouping.
type FormatHelpers struct{}

// Count formats an integer with en-US grouping (thousands separator).
func (FormatHelpers) Count(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + (len(s)-1)/3)
	rem := len(s) % 3
	if rem == 0 {
		rem = 3
	}
	b.WriteString(s[:rem])
	for i := rem; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// Duration formats an integer number of seconds as a human-readable string.
// Values < 60s render as "Ns"; >= 60s render as "Xm Ys" with zero seconds
// omitted. Zero renders as "0s".
func (FormatHelpers) Duration(sec int) string {
	if sec == 0 {
		return "0s"
	}
	if sec < 0 {
		return fmt.Sprintf("-%s", FormatHelpers{}.Duration(-sec))
	}
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	m := sec / 60
	s := sec % 60
	if s == 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%dm %ds", m, s)
}

// Date formats a time as ISO 8601 date (YYYY-MM-DD) in UTC.
func (FormatHelpers) Date(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(time.UTC).Format("2006-01-02")
}

// Percent formats a fraction as a whole-number percentage string.
// When denominator is 0 the result is "—".
func (FormatHelpers) Percent(num, denom int) string {
	if denom == 0 {
		return "—"
	}
	pct := int(math.Round(float64(num) / float64(denom) * 100))
	return fmt.Sprintf("%d%%", pct)
}
