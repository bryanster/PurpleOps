package httpapi

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/oapi-codegen/runtime/types"

	"github.com/bryanster/blacklight/internal/analytics"
	"github.com/bryanster/blacklight/internal/analytics/analyticstest"
	"github.com/bryanster/blacklight/internal/authz"
	engagement "github.com/bryanster/blacklight/internal/engagement"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	storengagement "github.com/bryanster/blacklight/internal/store/engagement"
	"github.com/bryanster/blacklight/internal/store/identity"
)

// TestAnalyticsCoverageShape verifies the coverage endpoint returns all required fields.
func TestAnalyticsCoverageShape(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	h := testHandlers(t, fx)

	resp, err := h.GetAnalyticsCoverage(authCtx(fx.BaselineID),
		gen.GetAnalyticsCoverageRequestObject{EngagementId: toUUID(fx.BaselineID)})
	if err != nil {
		t.Fatalf("GetAnalyticsCoverage: %v", err)
	}

	cov, ok := resp.(gen.GetAnalyticsCoverage200JSONResponse)
	if !ok {
		t.Fatalf("unexpected response type: %T", resp)
	}

	if cov.Techniques.Matrix == 0 {
		t.Error("matrix count is zero")
	}
}

func TestAnalyticsDistributionShape(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	h := testHandlers(t, fx)

	resp, err := h.GetAnalyticsDistribution(authCtx(fx.BaselineID),
		gen.GetAnalyticsDistributionRequestObject{EngagementId: toUUID(fx.BaselineID)})
	if err != nil {
		t.Fatalf("GetAnalyticsDistribution: %v", err)
	}

	dist, ok := resp.(gen.GetAnalyticsDistribution200JSONResponse)
	if !ok {
		t.Fatalf("unexpected response type: %T", resp)
	}

	if dist.Category.Attempted == 0 && dist.Protection.Attempted == 0 &&
		dist.Outcome.Attempted == 0 && dist.Modifier.Attempted == 0 {
		t.Log("all distribution attempted counts zero — fixture may have no executions")
	}
}

func TestAnalyticsMttdShape(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	h := testHandlers(t, fx)

	resp, err := h.GetAnalyticsMttd(authCtx(fx.BaselineID),
		gen.GetAnalyticsMttdRequestObject{EngagementId: toUUID(fx.BaselineID)})
	if err != nil {
		t.Fatalf("GetAnalyticsMttd: %v", err)
	}

	mttd, ok := resp.(gen.GetAnalyticsMttd200JSONResponse)
	if !ok {
		t.Fatalf("unexpected response type: %T", resp)
	}

	sum := mttd.DetectedCount + mttd.UndetectedCount + mttd.UnscoredCount + mttd.UnmeasurableCount
	if sum != mttd.AttemptedCount {
		t.Errorf("component counts sum %d != attemptedCount %d", sum, mttd.AttemptedCount)
	}
}

func TestAnalyticsBurndownShape(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	h := testHandlers(t, fx)

	interval := gen.BurndownIntervalDaily
	resp, err := h.GetAnalyticsBurndown(authCtx(fx.BaselineID),
		gen.GetAnalyticsBurndownRequestObject{
			EngagementId: toUUID(fx.BaselineID),
			Params:       gen.GetAnalyticsBurndownParams{Interval: &interval},
		})
	if err != nil {
		t.Fatalf("GetAnalyticsBurndown: %v", err)
	}

	bd, ok := resp.(gen.GetAnalyticsBurndown200JSONResponse)
	if !ok {
		t.Fatalf("unexpected response type: %T", resp)
	}

	if bd.Interval != gen.BurndownIntervalDaily {
		t.Errorf("interval = %v, want daily", bd.Interval)
	}
	if len(bd.Points) == 0 {
		t.Error("burndown has no points")
	}
	if len(bd.Severity.Buckets) == 0 {
		t.Error("severity snapshot has no buckets")
	}
}

func TestAnalyticsCompareShape(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	h := testHandlers(t, fx)

	resp, err := h.GetAnalyticsCompare(authCtxBoth(fx.RetestID, fx.BaselineID),
		gen.GetAnalyticsCompareRequestObject{
			EngagementId: toUUID(fx.RetestID),
			Params:       gen.GetAnalyticsCompareParams{Baseline: toUUID(fx.BaselineID)},
		})
	if err != nil {
		t.Fatalf("GetAnalyticsCompare: %v", err)
	}

	cmp, ok := resp.(gen.GetAnalyticsCompare200JSONResponse)
	if !ok {
		t.Fatalf("unexpected response type: %T", resp)
	}

	total := cmp.Improved + cmp.Regressed + cmp.Unchanged +
		cmp.NewlyAttempted + cmp.NoLongerAttempted + cmp.Incomparable
	if total != len(cmp.Rows) {
		t.Errorf("classification sum %d != rows %d", total, len(cmp.Rows))
	}
}

func TestAnalyticsCompareAuthzBaselineRefused(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	h := testHandlers(t, fx)

	ctx := context.WithValue(context.Background(), authorizationKey{}, Authorization{
		OperationID: "getAnalyticsCompare",
		Subject: authz.Subject{
			UserID:       "test-user",
			PlatformRole: authz.PlatformRoleMember,
			Method:       authz.MethodCookie,
			Memberships: map[string]authz.EngagementRole{
				fx.RetestID: authz.EngagementRoleLead,
			},
		},
		Action:   authz.ActionReportRead,
		Resource: authz.Resource{Type: authz.ResourceReport, EngagementID: fx.RetestID},
		Allowed:  true,
	})

	_, err := h.GetAnalyticsCompare(ctx,
		gen.GetAnalyticsCompareRequestObject{
			EngagementId: toUUID(fx.RetestID),
			Params:       gen.GetAnalyticsCompareParams{Baseline: toUUID(fx.BaselineID)},
		})

	if err == nil {
		t.Fatal("expected error when baseline not authorized, got nil")
	}
	if !strings.Contains(err.Error(), "forbidden") && !strings.Contains(err.Error(), "permitted") {
		t.Errorf("expected forbidden error, got: %v", err)
	}
}

func TestAnalyticsBlindModeDifferentViews(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	h := testHandlers(t, fx)

	ctxBlue := context.WithValue(context.Background(), authorizationKey{}, Authorization{
		OperationID: "getAnalyticsCoverage",
		Subject: authz.Subject{
			UserID:       "blue-user",
			PlatformRole: authz.PlatformRoleMember,
			Method:       authz.MethodCookie,
			Memberships: map[string]authz.EngagementRole{
				fx.BaselineID: authz.EngagementRoleBlue,
			},
		},
		Action:   authz.ActionReportRead,
		Resource: authz.Resource{Type: authz.ResourceReport, EngagementID: fx.BaselineID},
		Allowed:  true,
	})

	blueResp, err := h.GetAnalyticsCoverage(ctxBlue,
		gen.GetAnalyticsCoverageRequestObject{EngagementId: toUUID(fx.BaselineID)})
	if err != nil {
		t.Fatalf("blue GetAnalyticsCoverage: %v", err)
	}
	blueCov := blueResp.(gen.GetAnalyticsCoverage200JSONResponse)

	ctxLead := context.WithValue(context.Background(), authorizationKey{}, Authorization{
		OperationID: "getAnalyticsCoverage",
		Subject: authz.Subject{
			UserID:       "lead-user",
			PlatformRole: authz.PlatformRoleMember,
			Method:       authz.MethodCookie,
			Memberships: map[string]authz.EngagementRole{
				fx.BaselineID: authz.EngagementRoleLead,
			},
		},
		Action:   authz.ActionReportRead,
		Resource: authz.Resource{Type: authz.ResourceReport, EngagementID: fx.BaselineID},
		Allowed:  true,
	})

	leadResp, err := h.GetAnalyticsCoverage(ctxLead,
		gen.GetAnalyticsCoverageRequestObject{EngagementId: toUUID(fx.BaselineID)})
	if err != nil {
		t.Fatalf("lead GetAnalyticsCoverage: %v", err)
	}
	leadCov := leadResp.(gen.GetAnalyticsCoverage200JSONResponse)

	if len(blueCov.Techniques.Rows) > len(leadCov.Techniques.Rows) {
		t.Errorf("blue sees %d technique rows, lead sees %d — blue should see <= lead",
			len(blueCov.Techniques.Rows), len(leadCov.Techniques.Rows))
	}
}

// --- Helpers ---------------------------------------------------------------

func testHandlers(t *testing.T, fx analyticstest.Fixture) *handlers {
	t.Helper()

	queries := analytics.NewQueries(fx.DB)

	engRepo := storengagement.NewEngagements(fx.DB)
	scenarios := storengagement.NewScenarios(fx.DB)
	steps := storengagement.NewSteps(fx.DB)
	executions := storengagement.NewExecutions(fx.DB)
	comments := storengagement.NewComments(fx.DB)
	findings := storengagement.NewFindings(fx.DB)
	memberships := identity.NewMemberships(fx.DB)
	users := identity.NewUsers(fx.DB)

	engSvc, err := engagement.New(engagement.Deps{
		Engagements: engRepo,
		Scenarios:   scenarios,
		Steps:       steps,
		Executions:  executions,
		Comments:    comments,
		Findings:    findings,
		Memberships: memberships,
		Users:       users,
	})
	if err != nil {
		t.Fatalf("creating engagement service: %v", err)
	}

	return &handlers{
		store:       fx.DB,
		analytics:   queries,
		engagements: engSvc,
		log:         testLogger(t),
	}
}

func authCtx(engID string) context.Context {
	return context.WithValue(context.Background(), authorizationKey{}, Authorization{
		OperationID: "getAnalytics",
		Subject: authz.Subject{
			UserID:       "test-user",
			PlatformRole: authz.PlatformRoleMember,
			Method:       authz.MethodCookie,
			Memberships: map[string]authz.EngagementRole{
				engID: authz.EngagementRoleLead,
			},
		},
		Action:   authz.ActionReportRead,
		Resource: authz.Resource{Type: authz.ResourceReport, EngagementID: engID},
		Allowed:  true,
	})
}

func authCtxBoth(currentID, baselineID string) context.Context {
	return context.WithValue(context.Background(), authorizationKey{}, Authorization{
		OperationID: "getAnalyticsCompare",
		Subject: authz.Subject{
			UserID:       "test-user",
			PlatformRole: authz.PlatformRoleMember,
			Method:       authz.MethodCookie,
			Memberships: map[string]authz.EngagementRole{
				currentID:  authz.EngagementRoleLead,
				baselineID: authz.EngagementRoleLead,
			},
		},
		Action:   authz.ActionReportRead,
		Resource: authz.Resource{Type: authz.ResourceReport, EngagementID: currentID},
		Allowed:  true,
	})
}

func toUUID(s string) types.UUID {
	u, err := uuid.Parse(strings.ToLower(s))
	if err != nil {
		panic(fmt.Sprintf("invalid UUID in test: %s: %v", s, err))
	}
	return u
}

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestAnalyticsNavigatorLayerShape(t *testing.T) {
	t.Parallel()

	fx := analyticstest.Seed(t)
	h := testHandlers(t, fx)

	resp, err := h.GetAnalyticsNavigatorLayer(authCtx(fx.BaselineID),
		gen.GetAnalyticsNavigatorLayerRequestObject{EngagementId: toUUID(fx.BaselineID)})
	if err != nil {
		t.Fatalf("GetAnalyticsNavigatorLayer: %v", err)
	}

	nl, ok := resp.(gen.GetAnalyticsNavigatorLayer200JSONResponse)
	if !ok {
		t.Fatalf("unexpected response type: %T", resp)
	}

	if nl.Name == "" {
		t.Error("layer name is empty")
	}
	if nl.Versions.Attack == "" {
		t.Error("versions.attack is empty")
	}
	if nl.Versions.Navigator != analytics.NavigatorSchemaVersion {
		t.Errorf("versions.navigator = %q, want %q", nl.Versions.Navigator, analytics.NavigatorSchemaVersion)
	}
	if nl.Versions.Layer != analytics.NavigatorSchemaVersion {
		t.Errorf("versions.layer = %q, want %q", nl.Versions.Layer, analytics.NavigatorSchemaVersion)
	}
	if nl.Domain != "enterprise-attack" {
		t.Errorf("domain = %q, want enterprise-attack", nl.Domain)
	}
	if len(nl.Techniques) == 0 {
		t.Error("techniques is empty")
	}
	if len(nl.LegendItems) != 5 {
		t.Errorf("legendItems = %d, want 5", len(nl.LegendItems))
	}
	if len(nl.Gradient.Colors) != 5 {
		t.Errorf("gradient.colors = %d, want 5", len(nl.Gradient.Colors))
	}
	if nl.Gradient.MinValue != 0 {
		t.Errorf("gradient.minValue = %d, want 0", nl.Gradient.MinValue)
	}
	if nl.Gradient.MaxValue != 4 {
		t.Errorf("gradient.maxValue = %d, want 4", nl.Gradient.MaxValue)
	}
}
