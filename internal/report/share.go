package report

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/hkdf"

	"github.com/bryanster/blacklight/internal/authn/password"
	"github.com/bryanster/blacklight/internal/events"
	"github.com/bryanster/blacklight/internal/httpapi/apierr"
	storereport "github.com/bryanster/blacklight/internal/store/report"
)

// ---------------------------------------------------------------------------
// Share token hashing — follows servicetoken.Hasher pattern (M1-011).
// ---------------------------------------------------------------------------

const (
	shareTokenBytes     = 32 // 256 bits
	shareHashDomain     = "blacklight/report-share\x00"
	shareDerivationInfo = "blacklight/report-share/hmac-sha256/v1"
	shareHashKeyBytes   = 32
)

const SharePasswordCookieName = "bl_report_share"
const sharePasswordDomain = "blacklight/report-share-password\x00"

// ShareTokenHasher creates and verifies share token hashes.
type ShareTokenHasher struct {
	key []byte
}

func NewShareTokenHasher(encryptionKey []byte) (*ShareTokenHasher, error) {
	if len(encryptionKey) < 16 {
		return nil, fmt.Errorf("report share: encryption key too short (%d bytes; need at least 16)", len(encryptionKey))
	}
	key := make([]byte, shareHashKeyBytes)
	reader := hkdf.New(sha256.New, encryptionKey, nil, []byte(shareDerivationInfo))
	if _, err := reader.Read(key); err != nil {
		return nil, fmt.Errorf("report share: hkdf: %w", err)
	}
	return &ShareTokenHasher{key: key}, nil
}

func (h *ShareTokenHasher) Hash(token string) string {
	mac := hmac.New(sha256.New, h.key)
	mac.Write([]byte(shareHashDomain))
	mac.Write([]byte(token))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (h *ShareTokenHasher) Generate() (token string, hash string, err error) {
	raw := make([]byte, shareTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("report share: token generation: %w", err)
	}
	token = hex.EncodeToString(raw)
	hash = h.Hash(token)
	return token, hash, nil
}

func SharePasswordCookie(shareToken string, sessionSecret []byte) string {
	mac := hmac.New(sha256.New, sessionSecret)
	mac.Write([]byte(sharePasswordDomain))
	mac.Write([]byte(shareToken))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func VerifySharePasswordCookie(cookieValue string, shareToken string, sessionSecret []byte) bool {
	expected := SharePasswordCookie(shareToken, sessionSecret)
	return subtle.ConstantTimeCompare([]byte(cookieValue), []byte(expected)) == 1
}

// ---------------------------------------------------------------------------
// Share service
// ---------------------------------------------------------------------------

type ShareServiceDeps struct {
	Shares      *storereport.Shares
	Grants      *storereport.Grants
	Versions    *storereport.Versions
	TokenHasher *ShareTokenHasher
	Activity    *events.Log
	BaseURL     string
	SessionSec  []byte
}

type ShareService struct {
	shares      *storereport.Shares
	grants      *storereport.Grants
	versions    *storereport.Versions
	tokenHasher *ShareTokenHasher
	activity    *events.Log
	baseURL     string
	sessionSec  []byte
}

func NewShareService(deps ShareServiceDeps) (*ShareService, error) {
	switch {
	case deps.Shares == nil:
		return nil, errors.New("report share: Shares is nil")
	case deps.Grants == nil:
		return nil, errors.New("report share: Grants is nil")
	case deps.Versions == nil:
		return nil, errors.New("report share: Versions is nil")
	case deps.TokenHasher == nil:
		return nil, errors.New("report share: TokenHasher is nil")
	case deps.Activity == nil:
		return nil, errors.New("report share: Activity is nil")
	case deps.BaseURL == "":
		return nil, errors.New("report share: BaseURL is empty")
	case len(deps.SessionSec) == 0:
		return nil, errors.New("report share: SessionSec is empty")
	}
	return &ShareService{
		shares:      deps.Shares,
		grants:      deps.Grants,
		versions:    deps.Versions,
		tokenHasher: deps.TokenHasher,
		activity:    deps.Activity,
		baseURL:     deps.BaseURL,
		sessionSec:  deps.SessionSec,
	}, nil
}

// CreateShareInput is what the caller provides to create a share.
type CreateShareInput struct {
	VersionID string
	Password  *string
	ExpiresAt *time.Time
	CreatedBy string
	Label     *string
	MaxGrants *int
}

type CreateShareResult struct {
	Share    storereport.ReportShare
	Token    string
	ClaimURL string
}

func (s *ShareService) CreateShare(ctx context.Context, in CreateShareInput) (*CreateShareResult, error) {
	ver, err := s.versions.ByID(ctx, in.VersionID)
	if err != nil {
		return nil, fmt.Errorf("report share: version lookup: %w", err)
	}

	token, tokenHash, err := s.tokenHasher.Generate()
	if err != nil {
		return nil, err
	}

	var passwordHash *string
	if in.Password != nil && *in.Password != "" {
		plain := password.Plaintext(*in.Password)
		hash, err := password.Hash(plain)
		if err != nil {
			return nil, fmt.Errorf("report share: password hash: %w", err)
		}
		passwordHash = &hash
	}

	share, err := s.shares.Insert(ctx, storereport.NewShare{
		VersionID:    in.VersionID,
		TokenHash:    tokenHash,
		PasswordHash: passwordHash,
		ExpiresAt:    in.ExpiresAt,
		CreatedBy:    in.CreatedBy,
		Label:        in.Label,
		MaxGrants:    in.MaxGrants,
	})
	if err != nil {
		return nil, fmt.Errorf("report share: insert: %w", err)
	}

	if s.activity != nil {
		if err := s.activity.RecordAlone(ctx, events.Entry{
			ActorID:    in.CreatedBy,
			Verb:       events.VerbReportShareCreated,
			ObjectType: "report_share",
			ObjectID:   share.ID,
			Delta: map[string]any{
				"version_id": in.VersionID,
				"report_id":  ver.ReportID,
				"label":      in.Label,
			},
		}); err != nil {
			return nil, fmt.Errorf("report share: activity: %w", err)
		}
	}

	claimURL := fmt.Sprintf("%s/report-views/%s", s.baseURL, token)

	return &CreateShareResult{
		Share:    share,
		Token:    token,
		ClaimURL: claimURL,
	}, nil
}

type ClaimShareInput struct {
	Token    string
	Password *string
	UserID   string
}

type ClaimShareResult struct {
	Grant     storereport.ReportShareGrant
	VersionID string
	ReportID  string
}

func (s *ShareService) ClaimShare(ctx context.Context, in ClaimShareInput) (*ClaimShareResult, error) {
	tokenHash := s.tokenHasher.Hash(in.Token)
	share, err := s.shares.ByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if share == nil {
		return nil, apierr.NotFound("report_share", "token")
	}
	if share.RevokedAt != nil {
		return nil, apierr.NotFound("report_share", "token")
	}
	if share.ExpiresAt != nil && time.Now().UTC().After(*share.ExpiresAt) {
		return nil, apierr.NotFound("report_share", "token")
	}

	// Password gate.
	if share.PasswordHash != nil && *share.PasswordHash != "" {
		if in.Password == nil || *in.Password == "" {
			return nil, apierr.Unauthenticated("this share requires a password")
		}
		ok, _, err := password.Verify(password.Plaintext(*in.Password), *share.PasswordHash)
		if err != nil {
			return nil, fmt.Errorf("report share: password verify: %w", err)
		}
		if !ok {
			return nil, apierr.Unauthenticated("the share password is incorrect")
		}
	}

	// Grant limit.
	if share.MaxGrants != nil && *share.MaxGrants > 0 {
		count, err := s.grants.GrantCount(ctx, share.ID)
		if err != nil {
			return nil, err
		}
		if count >= *share.MaxGrants {
			return nil, apierr.Forbidden("share grant limit reached")
		}
	}

	// Check if already claimed.
	existing, gErr := s.grants.ByShareAndUser(ctx, share.ID, in.UserID)
	if gErr != nil {
		return nil, gErr
	}
	if existing != nil {
		return nil, apierr.Conflict("share already claimed")
	}

	grant, err := s.grants.Insert(ctx, storereport.NewGrant{
		ShareID: share.ID,
		UserID:  &in.UserID,
	})
	if err != nil {
		return nil, fmt.Errorf("report share: grant insert: %w", err)
	}

	if s.activity != nil {
		ver, aErr := s.versions.ByID(ctx, share.VersionID)
		if aErr != nil {
			return nil, fmt.Errorf("report share: version lookup for activity: %w", aErr)
		}
		if aErr2 := s.activity.RecordAlone(ctx, events.Entry{
			ActorID:    in.UserID,
			Verb:       events.VerbReportShareClaimed,
			ObjectType: "report_share_grant",
			ObjectID:   grant.ID,
			Delta: map[string]any{
				"share_id":   share.ID,
				"version_id": share.VersionID,
				"report_id":  ver.ReportID,
			},
		}); aErr2 != nil {
			return nil, fmt.Errorf("report share: activity: %w", aErr2)
		}
	}

	ver, err := s.versions.ByID(ctx, share.VersionID)
	if err != nil {
		return nil, err
	}

	return &ClaimShareResult{
		Grant:     grant,
		VersionID: share.VersionID,
		ReportID:  ver.ReportID,
	}, nil
}

type RevokeShareInput struct {
	ShareID   string
	RevokedBy string
}

func (s *ShareService) RevokeShare(ctx context.Context, in RevokeShareInput) error {
	share, err := s.shares.ByID(ctx, in.ShareID)
	if err != nil {
		return err
	}
	if share == nil {
		return apierr.NotFound("report_share", in.ShareID)
	}
	if share.RevokedAt != nil {
		return apierr.NotFound("report_share", in.ShareID)
	}

	if err := s.shares.Revoke(ctx, in.ShareID); err != nil {
		return err
	}

	grants, err := s.grants.ListByShare(ctx, in.ShareID)
	if err != nil {
		return err
	}
	for _, g := range grants {
		if g.RevokedAt == nil {
			if err := s.grants.Revoke(ctx, g.ID); err != nil {
				return err
			}
		}
	}

	if s.activity != nil {
		ver, verErr := s.versions.ByID(ctx, share.VersionID)
		delta := map[string]any{"version_id": share.VersionID}
		if verErr == nil {
			delta["report_id"] = ver.ReportID
		}
		_ = s.activity.RecordAlone(ctx, events.Entry{ //nolint:errcheck
			ActorID:    in.RevokedBy,
			Verb:       events.VerbReportShareRevoked,
			ObjectType: "report_share",
			ObjectID:   in.ShareID,
			Delta:      delta,
		})
	}
	return nil
}

type RevokeGrantInput struct {
	GrantID   string
	RevokedBy string
}

func (s *ShareService) RevokeGrant(ctx context.Context, in RevokeGrantInput) error {
	return s.grants.Revoke(ctx, in.GrantID)
}

type ListSharesInput struct {
	VersionID string
}

type ListSharesResult struct {
	Shares []ShareWithGrants
}

type ShareWithGrants struct {
	Share    storereport.ReportShare
	Grants   []storereport.ReportShareGrant
	ClaimURL string
}

func (s *ShareService) ListShares(ctx context.Context, in ListSharesInput) (*ListSharesResult, error) {
	shares, err := s.shares.ListByVersion(ctx, in.VersionID)
	if err != nil {
		return nil, err
	}
	result := make([]ShareWithGrants, 0, len(shares))
	for _, share := range shares {
		grants, err := s.grants.ListByShare(ctx, share.ID)
		if err != nil {
			return nil, err
		}
		if grants == nil {
			grants = []storereport.ReportShareGrant{}
		}
		result = append(result, ShareWithGrants{
			Share:    share,
			Grants:   grants,
			ClaimURL: fmt.Sprintf("%s/report-views/…", s.baseURL),
		})
	}
	return &ListSharesResult{Shares: result}, nil
}

func (s *ShareService) FindGrant(ctx context.Context, shareID, userID string) (*storereport.ReportShareGrant, error) {
	return s.grants.ByShareAndUser(ctx, shareID, userID)
}

func (s *ShareService) VerifySharePassword(ctx context.Context, token, passwordStr string) (*storereport.ReportShare, error) {
	tokenHash := s.tokenHasher.Hash(token)
	share, err := s.shares.ByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, err
	}
	if share == nil || share.RevokedAt != nil {
		return nil, apierr.NotFound("report_share", "token")
	}
	if share.ExpiresAt != nil && time.Now().UTC().After(*share.ExpiresAt) {
		return nil, apierr.NotFound("report_share", "token")
	}
	if share.PasswordHash == nil || *share.PasswordHash == "" {
		return share, nil
	}

	ok, _, err := password.Verify(password.Plaintext(passwordStr), *share.PasswordHash)
	if err != nil {
		return nil, fmt.Errorf("report share: password verify: %w", err)
	}
	if !ok {
		return nil, apierr.Unauthenticated("the share password is incorrect")
	}
	return share, nil
}

func (s *ShareService) GetShareVersion(ctx context.Context, token string) (*storereport.ReportVersion, *storereport.ReportShare, error) {
	tokenHash := s.tokenHasher.Hash(token)
	share, err := s.shares.ByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil, nil, err
	}
	if share == nil || share.RevokedAt != nil {
		return nil, nil, apierr.NotFound("report_share", "token")
	}
	if share.ExpiresAt != nil && time.Now().UTC().After(*share.ExpiresAt) {
		return nil, nil, apierr.NotFound("report_share", "token")
	}
	ver, err := s.versions.ByID(ctx, share.VersionID)
	if err != nil {
		return nil, nil, err
	}
	return &ver, share, nil
}

func (s *ShareService) SharePasswordCookieValue(token string) string {
	return SharePasswordCookie(token, s.sessionSec)
}

func (s *ShareService) VerifySharePasswordCookieValue(cookieValue, token string) bool {
	return VerifySharePasswordCookie(cookieValue, token, s.sessionSec)
}
