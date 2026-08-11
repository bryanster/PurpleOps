package httpapi

import (
	"context"
	"fmt"

	"github.com/oapi-codegen/nullable"

	"github.com/bryanster/blacklight/internal/httpapi/gen"
	"github.com/bryanster/blacklight/internal/report"
)

// live in internal/report.
//
// Who may change them is not decided here. api/openapi.yaml maps these
// operations to settings.read and settings.manage, and the authorization
// middleware refuses anybody who does not hold them before any function
// below is entered (M1-013).

// GetReportBranding returns the install-wide report branding defaults.
func (h *handlers) GetReportBranding(ctx context.Context, _ gen.GetReportBrandingRequestObject) (gen.GetReportBrandingResponseObject, error) {
	bs, err := h.brandingSettings.Get(ctx)
	if err != nil {
		return nil, err
	}
	return gen.GetReportBranding200JSONResponse(brandingToWire(bs)), nil
}

// SetReportBranding replaces the install-wide report branding defaults.
func (h *handlers) SetReportBranding(ctx context.Context, request gen.SetReportBrandingRequestObject) (gen.SetReportBrandingResponseObject, error) {
	subject, err := subjectFrom(ctx)
	if err != nil {
		return nil, err
	}

	stored, err := h.brandingSettings.Set(ctx, report.BrandingSettings{
		FirmName:       request.Body.FirmName,
		PrimaryColor:   request.Body.PrimaryColor,
		SecondaryColor: request.Body.SecondaryColor,
		LogoBlobRef:    nullableStringDeref(request.Body.LogoBlobRef),
	}, subject.UserID)
	if err != nil {
		return nil, err
	}
	return gen.SetReportBranding200JSONResponse(brandingToWire(stored)), nil
}
func (h *handlers) UploadReportBrandingLogo(ctx context.Context, request gen.UploadReportBrandingLogoRequestObject) (gen.UploadReportBrandingLogoResponseObject, error) {
	part, err := request.Body.NextPart()
	if err != nil {
		return nil, fmt.Errorf("httpapi: upload branding logo: %w", err)
	}

	contentType := part.Header.Get("Content-Type")

	blobRef, err := h.brandingSettings.UploadLogo(ctx, part, contentType, 0)
	if err != nil {
		return nil, err
	}

	return gen.UploadReportBrandingLogo201JSONResponse(gen.ReportBrandingLogo{
		BlobRef: blobRef,
	}), nil
}
func brandingToWire(bs report.BrandingSettings) gen.ReportBranding {
	w := gen.ReportBranding{
		FirmName:       bs.FirmName,
		PrimaryColor:   bs.PrimaryColor,
		SecondaryColor: bs.SecondaryColor,
	}
	if bs.LogoBlobRef != "" {
		w.LogoBlobRef.Set(bs.LogoBlobRef)
	}
	return w
}

// nullableStringDeref extracts a string from a nullable field, returning "" for null.
func nullableStringDeref(ns nullable.Nullable[string]) string {
	if ns.IsNull() || !ns.IsSpecified() {
		return ""
	}
	return ns.MustGet()
}
