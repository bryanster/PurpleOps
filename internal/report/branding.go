package report

import (
	"context"
	"encoding/json"
	"fmt"

	storereport "github.com/bryanster/blacklight/internal/store/report"
)

// BrandingResolver resolves the effective branding for a report by merging
// per-report overrides with install-wide defaults and built-in fallbacks.
//
// Resolution order (first wins):
//  1. Per-report override (report.client_name, logo_blob_ref, colours)
//  2. Install-wide default (platform settings)
//  3. Built-in neutral fallback
//
// The resolver is used by the report renderer (M6-009) to populate
// RenderEnv.Branding.
type BrandingResolver struct {
	settings *BrandingSettingsService
}

// NewBrandingResolver returns a resolver backed by the given settings service.
func NewBrandingResolver(settings *BrandingSettingsService) *BrandingResolver {
	return &BrandingResolver{settings: settings}
}

// Resolve returns the effective BrandingConfig for a report by merging
// install defaults with per-report overrides.
func (r *BrandingResolver) Resolve(ctx context.Context, rep storereport.Report) (BrandingConfig, error) {
	def, err := r.settings.Get(ctx)
	if err != nil {
		return BrandingConfig{}, fmt.Errorf("report: branding resolve: %w", err)
	}

	cfg := BrandingConfig{
		FirmName:      def.FirmName,
		PrimaryColor:   def.PrimaryColor,
		SecondaryColor: def.SecondaryColor,
		LogoRef:        def.LogoBlobRef,
	}

	// Per-report overrides.
	if rep.ClientName != nil {
		cfg.ClientName = *rep.ClientName
	}
	if rep.LogoBlobRef != nil {
		cfg.LogoRef = *rep.LogoBlobRef
	}
	if len(rep.Colours) > 0 {
		var c struct {
			Primary   string `json:"primary"`
			Secondary string `json:"secondary"`
		}
		if err := json.Unmarshal(rep.Colours, &c); err != nil {
			return BrandingConfig{}, fmt.Errorf("report: branding resolve: colours: %w", err)
		}
		if c.Primary != "" {
			cfg.PrimaryColor = c.Primary
		}
		if c.Secondary != "" {
			cfg.SecondaryColor = c.Secondary
		}
	}

	return cfg, nil
}
