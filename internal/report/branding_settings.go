package report

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	"github.com/bryanster/blacklight/internal/store/settings"
)

// Setting keys for install-wide report branding.
const (
	settingFirmName      = "report_branding.firm_name"
	settingPrimaryColor  = "report_branding.primary_color"
	settingSecondaryColor = "report_branding.secondary_color"
	settingLogoBlobRef   = "report_branding.logo_blob_ref"
)

// Built-in fallbacks when nothing has been configured.
const (
	defaultFirmName      = "Blacklight"
	defaultPrimaryColor  = "#1a1a2e"
	defaultSecondaryColor = "#16213e"
)

// Branding logo constraints.
const (
	maxLogoBytes = 2 << 20 // 2 MiB
)

// acceptedLogoMIMEs is the allowlist of image formats for branding logos.
var acceptedLogoMIMEs = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
}

// BrandingSettings holds the install-wide branding that reports fall back to.
type BrandingSettings struct {
	FirmName       string
	PrimaryColor   string
	SecondaryColor  string
	LogoBlobRef    string // empty = no logo
}

// BrandingSettingsService reads and writes install-wide branding defaults,
// and manages logo blob storage.
type BrandingSettingsService struct {
	store     *settings.Store
	logoDir   string // content-addressed logo storage root
}

// NewBrandingSettingsService returns a service over the settings store and
// logo directory. logoDir is created if absent. An empty logoDir disables
// logo storage (UploadLogo returns an error) but settings read/write still
// work.
func NewBrandingSettingsService(store *settings.Store, logoDir string) (*BrandingSettingsService, error) {
	if logoDir != "" {
		if err := os.MkdirAll(logoDir, 0o755); err != nil {
			return nil, fmt.Errorf("report: branding logo dir %s: %w", logoDir, err)
		}
	}
	return &BrandingSettingsService{
		store:   store,
		logoDir: logoDir,
	}, nil
}

// Get returns the current install-wide branding, applying built-in fallbacks
// for any unconfigured fields.
func (s *BrandingSettingsService) Get(ctx context.Context) (BrandingSettings, error) {
	all, err := s.store.All(ctx)
	if err != nil {
		return BrandingSettings{}, fmt.Errorf("report: branding settings: %w", err)
	}
	return brandingFromMap(all), nil
}

// Set replaces the install-wide branding defaults. Every field is required;
// a partial write is refused at the API layer. logoBlobRef may be empty to
// clear the logo.
func (s *BrandingSettingsService) Set(ctx context.Context, in BrandingSettings, actorID string) (BrandingSettings, error) {
	if err := validateBranding(in); err != nil {
		return BrandingSettings{}, err
	}
	values := map[string]string{
		settingFirmName:      in.FirmName,
		settingPrimaryColor:  in.PrimaryColor,
		settingSecondaryColor: in.SecondaryColor,
		settingLogoBlobRef:   in.LogoBlobRef,
	}
	if err := s.store.Put(ctx, values, actorID); err != nil {
		return BrandingSettings{}, fmt.Errorf("report: branding settings: %w", err)
	}
	// Return what was stored (same as input after validation).
	return in, nil
}

// UploadLogo reads an image upload, validates MIME and size, and stores it
// content-addressed. Returns the SHA-256 blob reference.
func (s *BrandingSettingsService) UploadLogo(ctx context.Context, r io.Reader, contentType string, size int64) (string, error) {
	if s.logoDir == "" {
		return "", fmt.Errorf("report: branding logo store is not configured")
	}
	if size > maxLogoBytes {
		return "", apierr.Validation(apierr.FieldError{
			Field:   "file",
			Message: fmt.Sprintf("logo must be under 2 MiB; got %d bytes", size),
		})
	}

	// Normalise and validate MIME.
	contentType = strings.TrimSpace(contentType)
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = strings.TrimSpace(contentType[:i])
	}
	if !acceptedLogoMIMEs[contentType] {
		return "", apierr.Validation(apierr.FieldError{
			Field:   "file",
			Message: fmt.Sprintf("unsupported image type %q; accepted: PNG, JPEG, WebP", contentType),
		})
	}

	// Hash while writing to a temp file.
	hasher := sha256.New()
	tmp, err := os.CreateTemp(s.logoDir, ".logo-*")
	if err != nil {
		return "", fmt.Errorf("report: branding logo temp file: %w", err)
	}
	tmpName := tmp.Name()
	written := false
	defer func() {
		if !written {
			os.Remove(tmpName)
		}
	}()

	tr := io.TeeReader(io.LimitReader(r, maxLogoBytes+1), hasher)
	if _, err := io.Copy(tmp, tr); err != nil {
		tmp.Close()
		return "", fmt.Errorf("report: branding logo write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("report: branding logo close: %w", err)
	}

	digest := hex.EncodeToString(hasher.Sum(nil))

	// Verify the actual write size (catches oversize after the TeeReader limit).
	fi, err := os.Stat(tmpName)
	if err != nil {
		return "", fmt.Errorf("report: branding logo stat: %w", err)
	}
	if fi.Size() > maxLogoBytes {
		return "", apierr.Validation(apierr.FieldError{
			Field:   "file",
			Message: fmt.Sprintf("logo must be under 2 MiB; got %d bytes", fi.Size()),
		})
	}

	// Move to content-addressed location.
	dst := s.logoPath(digest)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", fmt.Errorf("report: branding logo mkdir: %w", err)
	}
	// Atomic rename; if the file already exists (duplicate content), the
	// rename fails and we remove the temp.
	if err := os.Rename(tmpName, dst); err != nil {
		// Check if the destination already exists (duplicate upload).
		if _, statErr := os.Stat(dst); statErr == nil {
			written = false // remove temp
			return digest, nil
		}
		return "", fmt.Errorf("report: branding logo rename: %w", err)
	}
	written = true
	return digest, nil
}

// LogoPath returns the disk path for a logo blob reference. It does not
// check whether the file exists.
func (s *BrandingSettingsService) LogoPath(blobRef string) string {
	return s.logoPath(blobRef)
}

// logoPath returns the content-addressed path under the logo directory.
// Layout: {logoDir}/{sha256[0:2]}/{sha256}
func (s *BrandingSettingsService) logoPath(digest string) string {
	if len(digest) < 2 {
		return filepath.Join(s.logoDir, digest)
	}
	return filepath.Join(s.logoDir, digest[:2], digest)
}

// brandingFromMap decodes the settings map into BrandingSettings with
// built-in fallbacks.
func brandingFromMap(all map[string]settings.Setting) BrandingSettings {
	b := BrandingSettings{
		FirmName:      defaultFirmName,
		PrimaryColor:  defaultPrimaryColor,
		SecondaryColor: defaultSecondaryColor,
	}
	if v, ok := all[settingFirmName]; ok && v.Value != "" {
		b.FirmName = v.Value
	}
	if v, ok := all[settingPrimaryColor]; ok && v.Value != "" {
		b.PrimaryColor = v.Value
	}
	if v, ok := all[settingSecondaryColor]; ok && v.Value != "" {
		b.SecondaryColor = v.Value
	}
	if v, ok := all[settingLogoBlobRef]; ok {
		b.LogoBlobRef = v.Value
	}
	return b
}

// validateBranding checks that both colours are valid hex triplets.
func validateBranding(b BrandingSettings) error {
	if !isHexColor(b.PrimaryColor) {
		return apierr.Validation(apierr.FieldError{
			Field:   "primaryColor",
			Message: fmt.Sprintf("%q is not a valid hex colour (expected #RRGGBB)", b.PrimaryColor),
		})
	}
	if !isHexColor(b.SecondaryColor) {
		return apierr.Validation(apierr.FieldError{
			Field:   "secondaryColor",
			Message: fmt.Sprintf("%q is not a valid hex colour (expected #RRGGBB)", b.SecondaryColor),
		})
	}
	return nil
}

// isHexColor reports whether s matches #RRGGBB.
func isHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}
