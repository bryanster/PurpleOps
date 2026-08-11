package httpapi

import (
	"context"
	"fmt"

	"github.com/oapi-codegen/nullable"

	"github.com/bryanster/blacklight/internal/analytics"
	"github.com/bryanster/blacklight/internal/authz"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
)

// Analytics endpoint handlers (M5-009).

// analyticsScope builds a [analytics.Scope] from the authorization context
// and engagement, returning the blind-filtered flag alongside.
func (h *handlers) analyticsScope(ctx context.Context, engagementID string) (analytics.Scope, bool, error) {
	blind, err := h.stepBlindScope(ctx, engagementID)
	if err != nil {
		return analytics.Scope{}, false, fmt.Errorf("step blind scope: %w", err)
	}
	return analytics.Scope{
		EngagementID: engagementID,
		Blind:        blind,
	}, blind.Withholds(), nil
}

// GetAnalyticsCoverage returns technique and tactic coverage for an engagement.
func (h *handlers) GetAnalyticsCoverage(ctx context.Context,
	request gen.GetAnalyticsCoverageRequestObject) (gen.GetAnalyticsCoverageResponseObject, error) {

	scope, blindFiltered, err := h.analyticsScope(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	tc, err := h.analytics.TechniqueCoverage(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("technique coverage: %w", err)
	}
	tactic, err := h.analytics.TacticCoverage(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("tactic coverage: %w", err)
	}

	return gen.GetAnalyticsCoverage200JSONResponse(gen.AnalyticsCoverage{
		BlindFiltered: blindFiltered,
		Techniques: gen.TechniqueCoverage{
			Rows:         techniqueCoverageRowsToWire(tc.Rows),
			Attempted:    tc.AttemptedTechniques,
			NotAttempted: tc.NotAttemptedTechniques,
			Matrix:       tc.MatrixTechniques,
			Unmatched:    tc.UnmatchedTechniques,
		},
		Tactics: gen.TacticCoverage{
			Rows: tacticCoverageRowsToWire(tactic.Rows),
		},
	}), nil
}

// GetAnalyticsDistribution returns all four distributions in one payload.
func (h *handlers) GetAnalyticsDistribution(ctx context.Context,
	request gen.GetAnalyticsDistributionRequestObject) (gen.GetAnalyticsDistributionResponseObject, error) {

	scope, blindFiltered, err := h.analyticsScope(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	cat, err := h.analytics.CategoryDistribution(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("category distribution: %w", err)
	}
	prot, err := h.analytics.ProtectionRate(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("protection rate: %w", err)
	}
	out, err := h.analytics.OutcomeMix(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("outcome mix: %w", err)
	}
	mod, err := h.analytics.ModifierDistribution(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("modifier distribution: %w", err)
	}

	return gen.GetAnalyticsDistribution200JSONResponse(gen.AnalyticsDistribution{
		BlindFiltered: blindFiltered,
		Category:      distributionToWire(cat),
		Protection:    distributionToWire(prot),
		Outcome:       distributionToWire(out),
		Modifier:      distributionToWire(mod),
	}), nil
}

// GetAnalyticsMttd returns MTTD percentiles with mandatory denominator fields.
func (h *handlers) GetAnalyticsMttd(ctx context.Context,
	request gen.GetAnalyticsMttdRequestObject) (gen.GetAnalyticsMttdResponseObject, error) {

	scope, blindFiltered, err := h.analyticsScope(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	m, err := h.analytics.MTTD(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("mttd: %w", err)
	}

	return gen.GetAnalyticsMttd200JSONResponse(gen.AnalyticsMttd{
		BlindFiltered:     blindFiltered,
		P50:               nullablePtr(m.P50),
		P90:               nullablePtr(m.P90),
		Max:               nullablePtr(m.Max),
		DetectedCount:     m.DetectedCount,
		UndetectedCount:   m.UndetectedCount,
		UnscoredCount:     m.UnscoredCount,
		UnmeasurableCount: m.UnmeasurableCount,
		AttemptedCount:    m.AttemptedCount,
	}), nil
}

// GetAnalyticsBurndown returns the findings burndown series and severity snapshot.
func (h *handlers) GetAnalyticsBurndown(ctx context.Context,
	request gen.GetAnalyticsBurndownRequestObject) (gen.GetAnalyticsBurndownResponseObject, error) {

	scope, blindFiltered, err := h.analyticsScope(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	interval := analytics.BurndownInterval("")
	if request.Params.Interval != nil {
		interval = analytics.BurndownInterval(*request.Params.Interval)
	}

	result, err := h.analytics.FindingsBurndown(ctx, scope, interval)
	if err != nil {
		return nil, fmt.Errorf("findings burndown: %w", err)
	}

	severity, err := h.analytics.FindingsBySeverity(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("findings by severity: %w", err)
	}

	return gen.GetAnalyticsBurndown200JSONResponse(gen.AnalyticsBurndown{
		BlindFiltered: blindFiltered,
		Interval:      gen.BurndownInterval(result.Interval),
		Points:        burndownPointsToWire(result.Points),
		Severity:      severityToWire(severity),
	}), nil
}

// GetAnalyticsCompare returns a cross-engagement technique-by-technique comparison.
//
// This is the only handler that performs an explicit authorization check:
// the middleware authorizes report.read on the current (path) engagement;
// the handler must then check report.read on the baseline engagement through
// authz.Can — the same shape M4-001 used for per-topic authz, and for the
// same reason. A caller who may read the current engagement and not the
// baseline gets 403, never a partial compare.
func (h *handlers) GetAnalyticsCompare(ctx context.Context,
	request gen.GetAnalyticsCompareRequestObject) (gen.GetAnalyticsCompareResponseObject, error) {

	auth, ok := authorizationFrom(ctx)
	if !ok {
		return nil, apierr.Unauthenticated("no authorization context")
	}

	baselineID := request.Params.Baseline.String()

	// Explicit authz on baseline — cannot be done by middleware since the
	// middleware authorizes only the path engagement.
	baselineResource := authz.Resource{
		Type:         authz.ResourceReport,
		EngagementID: baselineID,
	}
	if !authz.Can(ctx, auth.Subject, authz.ActionReportRead, baselineResource).Allowed {
		return nil, apierr.Forbidden("report.read on baseline engagement")
	}

	currentScope, currentBlind, err := h.analyticsScope(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	baselineBlind, err := h.stepBlindScope(ctx, baselineID)
	if err != nil {
		return nil, fmt.Errorf("baseline blind scope: %w", err)
	}

	baselineScope := analytics.Scope{
		EngagementID: baselineID,
		Blind:        baselineBlind,
	}

	result, err := h.analytics.Compare(ctx, analytics.CompareScope{
		Baseline: baselineScope,
		Current:  currentScope,
	})
	if err != nil {
		return nil, fmt.Errorf("compare: %w", err)
	}

	resp := gen.AnalyticsCompare{
		BaselineBlindFiltered: baselineBlind.Withholds(),
		CurrentBlindFiltered:  currentBlind,
		Rows:                  compareRowsToWire(result.Rows),
		Improved:              result.Improved,
		Regressed:             result.Regressed,
		Unchanged:             result.Unchanged,
		NewlyAttempted:        result.NewlyAttempted,
		NoLongerAttempted:     result.NoLongerAttempted,
		Incomparable:          result.Incomparable,
	}
	if result.PinMismatch != nil {
		resp.PinMismatch = &gen.PinMismatch{
			Baseline: result.PinMismatch.Baseline,
			Current:  result.PinMismatch.Current,
		}
	}

	return gen.GetAnalyticsCompare200JSONResponse(resp), nil
}

// GetAnalyticsNavigatorLayer returns an ATT&CK Navigator layer document for the engagement.
func (h *handlers) GetAnalyticsNavigatorLayer(ctx context.Context,
	request gen.GetAnalyticsNavigatorLayerRequestObject) (gen.GetAnalyticsNavigatorLayerResponseObject, error) {

	scope, _, err := h.analyticsScope(ctx, request.EngagementId.String())
	if err != nil {
		return nil, err
	}

	result, err := h.analytics.NavigatorLayer(ctx, scope)
	if err != nil {
		return nil, fmt.Errorf("navigator layer: %w", err)
	}

	return gen.GetAnalyticsNavigatorLayer200JSONResponse(navigatorLayerToWire(result)), nil
}

// --- navigator conversion helpers ------------------------------------------------

func navigatorLayerToWire(r *analytics.NavigatorLayerResult) gen.NavigatorLayer {
	selectSub := false
	selectAcross := true
	showTactic := true
	tacticBg := "#dddddd"

	colours := make([]string, 5)
	//nolint:staticcheck // copy loop is readable
	for i, c := range analytics.NavigatorColourRamp {
		colours[i] = c
	}

	legend := make([]gen.NavigatorLegendItem, 5)
	for i, label := range analytics.NavigatorLegendLabels {
		legend[i] = gen.NavigatorLegendItem{
			Label: label,
			Color: analytics.NavigatorColourRamp[i],
		}
	}

	techniques := make([]gen.NavigatorTechnique, 0, len(r.Techniques))
	for _, t := range r.Techniques {
		entry := gen.NavigatorTechnique{
			TechniqueID: t.TechniqueID,
			Score:       t.Score,
			Color:       t.Color,
			Enabled:     t.Enabled,
		}
		if t.Comment != "" {
			entry.Comment = &t.Comment
		}
		if t.StepCount > 0 || t.Protection != "" {
			var meta []gen.NavigatorMetadata
			if t.Enabled {
				meta = append(meta, gen.NavigatorMetadata{
					Name:  "Steps",
					Value: fmt.Sprintf("%d", t.StepCount),
				})
			}
			if t.Protection != "" {
				meta = append(meta, gen.NavigatorMetadata{
					Name:  "Protection",
					Value: t.Protection,
				})
			}
			if len(meta) > 0 {
				entry.Metadata = &meta
			}
		}
		if t.IsSubtechnique {
			entry.ShowSubtechniques = &selectSub
		}
		techniques = append(techniques, entry)
	}

	return gen.NavigatorLayer{
		Name:        r.Name,
		Description: r.Description,
		Domain:      "enterprise-attack",
		Versions: struct {
			Attack    string `json:"attack"`
			Layer     string `json:"layer"`
			Navigator string `json:"navigator"`
		}{
			Attack:    r.AttackVersion,
			Layer:     analytics.NavigatorSchemaVersion,
			Navigator: analytics.NavigatorSchemaVersion,
		},
		Filters: struct {
			Platforms []string `json:"platforms"`
		}{
			Platforms: []string{},
		},
		Gradient: struct {
			Colors   []string `json:"colors"`
			MaxValue int      `json:"maxValue"`
			MinValue int      `json:"minValue"`
		}{
			Colors:   colours,
			MinValue: 0,
			MaxValue: 4,
		},
		LegendItems:                   legend,
		ShowTacticRowBackground:       &showTactic,
		TacticRowBackground:           &tacticBg,
		SelectTechniquesAcrossTactics: &selectAcross,
		SelectSubtechniquesWithParent: &selectSub,
		Techniques:                    techniques,
	}
}

// --- conversion helpers -------------------------------------------------------

func distributionToWire(d *analytics.DistributionResult) gen.DistributionResult {
	buckets := make([]gen.DistributionBucket, len(d.Buckets))
	for i, b := range d.Buckets {
		buckets[i] = gen.DistributionBucket{Label: b.Label, Count: b.Count}
	}
	return gen.DistributionResult{Attempted: d.Attempted, Buckets: buckets}
}

func techniqueCoverageRowsToWire(rows []analytics.TechniqueCoverageRow) []gen.TechniqueCoverageRow {
	out := make([]gen.TechniqueCoverageRow, len(rows))
	for i, r := range rows {
		out[i] = gen.TechniqueCoverageRow{
			TechniqueId:         r.TechniqueID,
			Name:                r.Name,
			IsSubtechnique:      r.IsSubtechnique,
			ParentTechniqueId:   r.ParentTechniqueID,
			Matched:             r.Matched,
			Attempted:           r.Attempted,
			BestCategory:        r.BestCategory,
			BestCategoryOrdinal: nullablePtr(r.BestCategoryOrdinal),
			BestProtection:      r.BestProtection,
			StepCount:           r.StepCount,
		}
	}
	return out
}

func tacticCoverageRowsToWire(rows []analytics.TacticCoverageRow) []gen.TacticCoverageRow {
	out := make([]gen.TacticCoverageRow, len(rows))
	for i, r := range rows {
		cats := make([]gen.CategoryBucket, 0, len(r.CategoryDistribution))
		for category, count := range r.CategoryDistribution {
			cats = append(cats, gen.CategoryBucket{Category: category, Count: count})
		}
		out[i] = gen.TacticCoverageRow{
			TacticId:            r.TacticID,
			TacticName:          r.TacticName,
			AttemptedTechniques: r.TechniquesAttempted,
			MatrixTechniques:    r.TechniquesInMatrix,
			Categories:          cats,
		}
	}
	return out
}

func burndownPointsToWire(points []analytics.BurndownPoint) []gen.BurndownPoint {
	out := make([]gen.BurndownPoint, len(points))
	for i, p := range points {
		out[i] = gen.BurndownPoint{
			Date:         p.Date,
			Open:         p.Open,
			InProgress:   p.InProgress,
			Resolved:     p.Resolved,
			AcceptedRisk: p.AcceptedRisk,
			TotalOpen:    p.TotalOpen,
		}
	}
	return out
}

func severityToWire(s *analytics.SeveritySnapshot) gen.SeveritySnapshot {
	buckets := make([]gen.SeverityBucket, len(s.Buckets))
	for i, b := range s.Buckets {
		buckets[i] = gen.SeverityBucket{
			Severity:     b.Severity,
			Open:         b.Open,
			InProgress:   b.InProgress,
			Resolved:     b.Resolved,
			AcceptedRisk: b.AcceptedRisk,
			TotalOpen:    b.TotalOpen,
		}
	}
	return gen.SeveritySnapshot{Buckets: buckets}
}

func compareRowsToWire(rows []analytics.CompareRow) []gen.CompareRow {
	out := make([]gen.CompareRow, len(rows))
	for i, r := range rows {
		out[i] = gen.CompareRow{
			TechniqueId:             r.TechniqueID,
			SubtechniqueId:          r.SubtechniqueID,
			Name:                    r.Name,
			BaselineCategory:        r.BaselineCategory,
			BaselineCategoryOrdinal: nullablePtr(r.BaselineCategoryOrdinal),
			BaselineProtection:      r.BaselineProtection,
			CurrentCategory:         r.CurrentCategory,
			CurrentCategoryOrdinal:  nullablePtr(r.CurrentCategoryOrdinal),
			CurrentProtection:       r.CurrentProtection,
			OrdinalDelta:            nullablePtr(r.OrdinalDelta),
			Classification:          r.Classification,
		}
	}
	return out
}

// nullablePtr converts a *int to a nullable.Nullable[int].
func nullablePtr(v *int) nullable.Nullable[int] {
	if v == nil {
		return nullable.NewNullNullable[int]()
	}
	return nullable.NewNullableWithValue(*v)
}
