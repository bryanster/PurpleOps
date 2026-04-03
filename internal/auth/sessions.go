package auth

import (
	"net/http"
	"strings"

	"github.com/bryanster/purpleops/internal/models"
	ginsessions "github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// store is the package-level session store.
var store ginsessions.Store

// isSecureRequest determines if a request is over HTTPS.
func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); strings.EqualFold(proto, "https") {
		return true
	}
	return false
}

// InitSessions initialises the cookie-based session store and returns the Gin
// middleware that must be registered on the router.
// When ssoEnabled is true, SameSite is set to Lax (required for OAuth/SAML
// redirects that return from an external IdP); otherwise Strict is used.
func InitSessions(secretKey string, ssoEnabled, debug bool) gin.HandlerFunc {
	store = cookie.NewStore([]byte(secretKey))
	sameSite := http.SameSiteStrictMode
	if ssoEnabled {
		sameSite = http.SameSiteLaxMode
	}
	// Note: Secure flag is set at save time in SaveSession to handle mixed
	// HTTP/HTTPS environments. Using !debug here breaks authentication over HTTP.
	store.Options(ginsessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 1 week
		HttpOnly: true,
		Secure:   false, // Set per-request instead of globally
		SameSite: sameSite,
	})
	return ginsessions.Sessions("purpleops", store)
}

// GetSession returns the session from the Gin context.
func GetSession(c *gin.Context) ginsessions.Session {
	return ginsessions.Default(c)
}

// SaveSession saves the session, properly setting the Secure flag based on the request.
func SaveSession(c *gin.Context, sess ginsessions.Session) error {
	sess.Options(ginsessions.Options{Secure: isSecureRequest(c.Request)})
	return sess.Save()
}

// SetSessionUser stores the user ID in a new session, invalidating the old one
// to prevent session fixation attacks.
func SetSessionUser(c *gin.Context, userID string) {
	sess := GetSession(c)
	// Expire the pre-login session so a previously known session ID cannot be
	// used by an attacker after the user authenticates.
	sess.Options(ginsessions.Options{MaxAge: -1, Secure: isSecureRequest(c.Request)})
	sess.Save()

	// Clear values and write a fresh session with a new ID.
	sess.Clear()
	sess.Options(ginsessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		Secure:   isSecureRequest(c.Request),
	})
	sess.Set("user_id", userID)
	sess.Save()
}

// ClearSession destroys the session.
func ClearSession(c *gin.Context) {
	sess := GetSession(c)
	sess.Clear()
	sess.Options(ginsessions.Options{MaxAge: -1, Secure: isSecureRequest(c.Request)})
	sess.Save()
}

// GetCurrentUser looks up the authenticated user from the session.
func GetCurrentUser(c *gin.Context) *models.User {
	sess := GetSession(c)
	userID, ok := sess.Get("user_id").(string)
	if !ok || userID == "" {
		return nil
	}
	user, err := models.FindUser(c.Request.Context(), userID)
	if err != nil {
		return nil
	}
	return user
}
