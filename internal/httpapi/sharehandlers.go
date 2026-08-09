package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/oapi-codegen/nullable"

	"github.com/bryanster/blacklight/internal/authn"
	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/report"
	storereport "github.com/bryanster/blacklight/internal/store/report"
)

// ── Report share management handlers (M6-012) ───────────────────────────

// ListReportShares returns shares for a published version.
func (h *handlers) ListReportShares(ctx context.Context,
	request gen.ListReportSharesRequestObject) (gen.ListReportSharesResponseObject, error) {
	if h.shareSvc == nil {
		return nil, errors.New("share service not available")
	}

	result, err := h.shareSvc.ListShares(ctx, report.ListSharesInput{
		VersionID: request.VersionId.String(),
	})
	if err != nil {
		return nil, err
	}

	items := make([]gen.ReportShare, 0, len(result.Shares))
	for _, swg := range result.Shares {
		items = append(items, shareToWire(swg))
	}

	return gen.ListReportShares200JSONResponse(items), nil
}

// CreateReportShare creates a new share link for a published version.
func (h *handlers) CreateReportShare(ctx context.Context,
	request gen.CreateReportShareRequestObject) (gen.CreateReportShareResponseObject, error) {
	if h.shareSvc == nil {
		return nil, errors.New("share service not available")
	}

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	var expiresAt *time.Time
	var password *string
	var label *string
	var maxGrants *int

	if request.Body != nil {
		if request.Body.ExpiresAt != nil {
			expiresAt = request.Body.ExpiresAt
		}
		password = request.Body.Password
		label = request.Body.Label
		maxGrants = request.Body.MaxGrants
	}

	result, err := h.shareSvc.CreateShare(ctx, report.CreateShareInput{
		VersionID: request.VersionId.String(),
		Password:  password,
		ExpiresAt: expiresAt,
		CreatedBy: subject.UserID,
		Label:     label,
		MaxGrants: maxGrants,
	})
	if err != nil {
		return nil, err
	}

	wire := shareToWire(report.ShareWithGrants{
		Share:    result.Share,
		Grants:   nil,
		ClaimURL: result.ClaimURL,
	})

	return gen.CreateReportShare201JSONResponse(gen.CreateReportShareResult{
		Share:    wire,
		Token:    &result.Token,
		ClaimUrl: result.ClaimURL,
	}), nil
}

// RevokeReportShare revokes a share and all its grants.
func (h *handlers) RevokeReportShare(ctx context.Context,
	request gen.RevokeReportShareRequestObject) (gen.RevokeReportShareResponseObject, error) {
	if h.shareSvc == nil {
		return nil, errors.New("share service not available")
	}

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	if err := h.shareSvc.RevokeShare(ctx, report.RevokeShareInput{
		ShareID:   request.ShareId,
		RevokedBy: subject.UserID,
	}); err != nil {
		return nil, err
	}

	return gen.RevokeReportShare204Response{}, nil
}

// RevokeReportShareGrant revokes a single grant.
func (h *handlers) RevokeReportShareGrant(ctx context.Context,
	request gen.RevokeReportShareGrantRequestObject) (gen.RevokeReportShareGrantResponseObject, error) {
	if h.shareSvc == nil {
		return nil, errors.New("share service not available")
	}

	if err := h.shareSvc.RevokeGrant(ctx, report.RevokeGrantInput{
		GrantID: request.GrantId,
	}); err != nil {
		return nil, err
	}

	return gen.RevokeReportShareGrant204Response{}, nil
}

// ── Share view handlers (public-ish, authz by grant) ────────────────────

// GetReportShareInfo returns metadata about a share for the claim page.
func (h *handlers) GetReportShareInfo(ctx context.Context,
	request gen.GetReportShareInfoRequestObject) (gen.GetReportShareInfoResponseObject, error) {
	if h.shareSvc == nil {
		return nil, errors.New("share service not available")
	}

	_, share, err := h.shareSvc.GetShareVersion(ctx, request.Token)
	if err != nil {
		return nil, err
	}

	passwordRequired := share.PasswordHash != nil && *share.PasswordHash != ""

	alreadyClaimed := false
	subject, _ := authn.SubjectFrom(ctx)
	if subject.UserID != "" {
		grant, _ := h.shareSvc.FindGrant(ctx, share.ID, subject.UserID)
		alreadyClaimed = grant != nil
	}

	label := ""
	if share.Label != nil {
		label = *share.Label
	}

	return gen.GetReportShareInfo200JSONResponse(gen.ReportShareInfo{
		Exists:           true,
		PasswordRequired: &passwordRequired,
		AlreadyClaimed:   &alreadyClaimed,
		Label:            &label,
	}), nil
}

// ClaimReportShare binds a grant to the current user.
func (h *handlers) ClaimReportShare(ctx context.Context,
	request gen.ClaimReportShareRequestObject) (gen.ClaimReportShareResponseObject, error) {
	if h.shareSvc == nil {
		return nil, errors.New("share service not available")
	}

	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	var password *string
	if request.Body != nil {
		password = request.Body.Password
	}

	result, err := h.shareSvc.ClaimShare(ctx, report.ClaimShareInput{
		Token:    request.Token,
		Password: password,
		UserID:   subject.UserID,
	})
	if err != nil {
		return nil, err
	}

	return gen.ClaimReportShare200JSONResponse(gen.ClaimReportShareResult{
		VersionId: mustParseUUID(result.VersionID),
		ReportId:  strPtr(mustParseUUID(result.ReportID)),
	}), nil
}

// VerifyReportSharePassword verifies the share password and sets the cookie.
func (h *handlers) VerifyReportSharePassword(ctx context.Context,
	request gen.VerifyReportSharePasswordRequestObject) (gen.VerifyReportSharePasswordResponseObject, error) {
	if h.shareSvc == nil {
		return nil, errors.New("share service not available")
	}

	_, err := h.shareSvc.VerifySharePassword(ctx, request.Token, request.Body.Password)
	if err != nil {
		return nil, err
	}

	cookieValue := h.shareSvc.SharePasswordCookieValue(request.Token)
	cookieStr := (&http.Cookie{
		Name:     report.SharePasswordCookieName,
		Value:    cookieValue,
		Path:     "/api/v1/report-views/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3600,
	}).String()

	return gen.VerifyReportSharePassword204Response{
		Headers: gen.VerifyReportSharePassword204ResponseHeaders{
			SetCookie: &cookieStr,
		},
	}, nil
}

// GetReportShareHtml returns the frozen HTML of a shared version.
func (h *handlers) GetReportShareHtml(ctx context.Context,
	request gen.GetReportShareHtmlRequestObject) (gen.GetReportShareHtmlResponseObject, error) {
	if h.shareSvc == nil {
		return nil, errors.New("share service not available")
	}

	ver, share, err := h.shareSvc.GetShareVersion(ctx, request.Token)
	if err != nil {
		return nil, err
	}

	if !h.canAccessSharedVersion(ctx, share, ver) {
		return nil, errors.New("access denied")
	}

	return gen.GetReportShareHtml200TexthtmlResponse{
		Body: bytes.NewReader([]byte(ver.HTML)),
	}, nil
}

// GetReportSharePdf returns the PDF of a shared version.
func (h *handlers) GetReportSharePdf(ctx context.Context,
	request gen.GetReportSharePdfRequestObject) (gen.GetReportSharePdfResponseObject, error) {
	if h.shareSvc == nil {
		return nil, errors.New("share service not available")
	}
	if h.pdfPrinter == nil {
		return nil, errors.New("PDF rendering is not configured")
	}

	ver, share, err := h.shareSvc.GetShareVersion(ctx, request.Token)
	if err != nil {
		return nil, err
	}

	if !h.canAccessSharedVersion(ctx, share, ver) {
		return nil, errors.New("access denied")
	}

	pdf, err := h.pdfPrinter.RenderPDF(ctx, []byte(ver.HTML))
	if err != nil {
		return nil, err
	}

	return gen.GetReportSharePdf200ApplicationpdfResponse{
		Body: bytes.NewReader(pdf),
	}, nil
}

// canAccessSharedVersion returns true if the caller has a valid grant.
func (h *handlers) canAccessSharedVersion(ctx context.Context, share *storereport.ReportShare, _ *storereport.ReportVersion) bool {
	subject, ok := authn.SubjectFrom(ctx)
	if !ok || subject.UserID == "" {
		return false
	}

	grant, err := h.shareSvc.FindGrant(ctx, share.ID, subject.UserID)
	return err == nil && grant != nil && grant.RevokedAt == nil
}

// ── Guest registration handler (M6-012) ─────────────────────────────────

// GuestRegister creates a minimal local account for share invite recipients.
func (h *handlers) GuestRegister(ctx context.Context,
	request gen.GuestRegisterRequestObject) (gen.GuestRegisterResponseObject, error) {
	return nil, errors.New("guest registration not yet implemented")
}

// ── Wire helpers ────────────────────────────────────────────────────────

func shareToWire(swg report.ShareWithGrants) gen.ReportShare {
	sh := swg.Share
	passwordProtected := sh.PasswordHash != nil && *sh.PasswordHash != ""
	wire := gen.ReportShare{
		Id:                mustParseUUID(sh.ID),
		VersionId:         mustParseUUID(sh.VersionID),
		PasswordProtected: &passwordProtected,
		CreatedBy:         sh.CreatedBy,
		CreatedAt:         sh.CreatedAt,
	}

	if sh.ExpiresAt != nil {
		wire.ExpiresAt = nullable.NewNullableWithValue(*sh.ExpiresAt)
	}
	if sh.RevokedAt != nil {
		wire.RevokedAt = nullable.NewNullableWithValue(*sh.RevokedAt)
	}
	if sh.Label != nil {
		wire.Label = nullable.NewNullableWithValue(*sh.Label)
	}

	if sh.MaxGrants != nil {
		wire.MaxGrants = nullable.NewNullableWithValue(*sh.MaxGrants)
	}

	grantCount := 0
	for _, g := range swg.Grants {
		if g.RevokedAt == nil {
			grantCount++
		}
	}
	wire.GrantCount = &grantCount

	grants := make([]gen.ReportShareGrant, 0, len(swg.Grants))
	for _, g := range swg.Grants {
		gw := gen.ReportShareGrant{
			Id:        mustParseUUID(g.ID),
			ShareId:   mustParseUUID(g.ShareID),
			CreatedAt: g.CreatedAt,
		}
		if g.UserID != nil {
			gw.UserId = nullable.NewNullableWithValue(mustParseUUID(*g.UserID))
		}
		if g.ClaimedAt != nil {
			gw.ClaimedAt = nullable.NewNullableWithValue(*g.ClaimedAt)
		}
		if g.RevokedAt != nil {
			gw.RevokedAt = nullable.NewNullableWithValue(*g.RevokedAt)
		}
		grants = append(grants, gw)
	}
	wire.Grants = &grants

	return wire
}

func strPtr[T any](v T) *T { return &v }
