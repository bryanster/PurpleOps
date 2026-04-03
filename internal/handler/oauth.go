package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/config"
	"github.com/bryanster/purpleops/internal/db"
	"github.com/bryanster/purpleops/internal/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"golang.org/x/oauth2"
)

// oauthConfig is the package-level OAuth2 config, initialised at startup.
var oauthConfig *oauth2.Config

// InitOAuth sets up the OAuth2 configuration from the application config.
// Call this at startup if OAuth is enabled.
func InitOAuth(cfg *config.Config) {
	scopes := strings.Split(cfg.OAuthScopes, ",")
	for i := range scopes {
		scopes[i] = strings.TrimSpace(scopes[i])
	}

	oauthConfig = &oauth2.Config{
		ClientID:     cfg.OAuthClientID,
		ClientSecret: cfg.OAuthClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  cfg.OAuthAuthURL,
			TokenURL: cfg.OAuthTokenURL,
		},
		RedirectURL: cfg.OAuthRedirectURL,
		Scopes:      scopes,
	}
}

// HandleOAuthLogin redirects the user to the OAuth2 provider's authorization page.
func HandleOAuthLogin(c *gin.Context) {
	if oauthConfig == nil {
		c.String(http.StatusNotFound, "OAuth not configured")
		return
	}

	// Generate a random state parameter to prevent CSRF.
	state, err := randomState()
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal error")
		return
	}

	sess := auth.GetSession(c)
	sess.Set("oauth_state", state)
	if err := auth.SaveSession(c, sess); err != nil {
		c.String(http.StatusInternalServerError, "Internal error")
		return
	}

	url := oauthConfig.AuthCodeURL(state)
	c.Redirect(http.StatusFound, url)
}

// HandleOAuthCallback processes the OAuth2 callback from the provider.
func HandleOAuthCallback(c *gin.Context) {
	if oauthConfig == nil {
		c.String(http.StatusNotFound, "OAuth not configured")
		return
	}

	sess := auth.GetSession(c)

	setFlash := func(msg string) {
		sess.Set("flash", msg)
		sess.Set("flash_category", "danger")
		if err := auth.SaveSession(c, sess); err != nil {
			c.String(http.StatusInternalServerError, "Internal error")
			return
		}
		c.Redirect(http.StatusFound, "/login")
	}

	// Verify state parameter (validate before deleting to avoid race conditions).
	expectedState, _ := sess.Get("oauth_state").(string)
	if expectedState == "" || c.Request.URL.Query().Get("state") != expectedState {
		setFlash("OAuth login failed: invalid state parameter.")
		return
	}
	sess.Delete("oauth_state")
	if err := auth.SaveSession(c, sess); err != nil {
		c.String(http.StatusInternalServerError, "Internal error")
		return
	}

	// Check for error from provider.
	if errParam := c.Request.URL.Query().Get("error"); errParam != "" {
		desc := c.Request.URL.Query().Get("error_description")
		if desc == "" {
			desc = errParam
		}
		setFlash("OAuth login failed: " + desc)
		return
	}

	// Exchange authorization code for token.
	code := c.Request.URL.Query().Get("code")
	if code == "" {
		setFlash("OAuth login failed: no authorization code received.")
		return
	}

	token, err := oauthConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		log.Printf("OAuth token exchange failed: %v", err)
		setFlash("OAuth login failed: could not exchange authorization code.")
		return
	}

	// Fetch user info from the provider.
	email, username, err := fetchOAuthUserInfo(c.Request, token)
	if err != nil {
		log.Printf("OAuth user info fetch failed: %v", err)
		setFlash("OAuth login failed: could not retrieve user information.")
		return
	}

	if email == "" {
		setFlash("OAuth login failed: no email address returned by provider.")
		return
	}

	// Validate email format.
	if _, err := mail.ParseAddress(email); err != nil {
		setFlash("OAuth login failed: invalid email format from provider.")
		return
	}

	// Find or create the user.
	cfg := config.Cfg
	user, err := models.FindOrCreateSSOUser(c.Request.Context(), email, username, "oauth", cfg.SSODefaultRole, cfg.SSOAutoProvision)
	if err != nil {
		log.Printf("OAuth user provisioning failed: %v", err)
		setFlash("OAuth login failed: internal error.")
		return
	}
	if user == nil {
		setFlash("No account found for " + email + ". Contact an administrator.")
		return
	}

	if !user.Active {
		setFlash("Account is disabled.")
		return
	}

	// Update login tracking.
	now := time.Now().UTC()
	db.Col(db.ColUser).UpdateOne(c.Request.Context(), bson.M{"_id": user.ID}, bson.M{
		"$set": bson.M{
			"last_login_at":    user.CurrentLoginAt,
			"last_login_ip":    user.CurrentLoginIP,
			"current_login_at": &now,
			"current_login_ip": c.Request.RemoteAddr,
		},
		"$inc": bson.M{"login_count": 1},
	})

	auth.SetSessionUser(c, user.ID.Hex())
	c.Redirect(http.StatusFound, "/")
}

// fetchOAuthUserInfo calls the UserInfo endpoint and extracts email and name.
func fetchOAuthUserInfo(r *http.Request, token *oauth2.Token) (email, username string, err error) {
	client := oauthConfig.Client(r.Context(), token)
	resp, err := client.Get(config.Cfg.OAuthUserInfoURL)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("userinfo endpoint returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var info map[string]interface{}
	if err := json.Unmarshal(body, &info); err != nil {
		return "", "", err
	}

	// Try common field names for email.
	for _, key := range []string{"email", "mail", "upn"} {
		if v, ok := info[key].(string); ok && v != "" {
			email = v
			break
		}
	}

	// Try common field names for display name.
	for _, key := range []string{"name", "displayName", "preferred_username", "login"} {
		if v, ok := info[key].(string); ok && v != "" {
			username = v
			break
		}
	}

	return email, username, nil
}

// randomState generates a cryptographically random hex string for OAuth state.
func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
