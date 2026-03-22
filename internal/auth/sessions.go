package auth

import (
	"net/http"
	"strings"

	"github.com/bryanster/purpleops/internal/models"
	"github.com/gorilla/sessions"
)

// store is the package-level session store.
var store *sessions.CookieStore

// isSecureRequest determines if a request is over HTTPS.
func isSecureRequest(r *http.Request) bool {
	// Check if request is HTTPS
	if r.TLS != nil {
		return true
	}
	// Check for X-Forwarded-Proto header (common in proxied environments)
	if proto := r.Header.Get("X-Forwarded-Proto"); strings.EqualFold(proto, "https") {
		return true
	}
	return false
}

// InitSessions initialises the cookie-based session store.
// When ssoEnabled is true, SameSite is set to Lax (required for OAuth/SAML
// redirects that return from an external IdP); otherwise Strict is used.
func InitSessions(secretKey string, ssoEnabled, debug bool) {
	store = sessions.NewCookieStore([]byte(secretKey))
	sameSite := http.SameSiteStrictMode
	if ssoEnabled {
		sameSite = http.SameSiteLaxMode
	}
	// Note: Secure flag is set at save time in SetSessionUser to handle mixed
	// HTTP/HTTPS environments. Using !debug here breaks authentication over HTTP.
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 1 week
		HttpOnly: true,
		Secure:   false, // Set per-request instead of globally
		SameSite: sameSite,
	}
}

// GetSession returns the named session from the request.
func GetSession(r *http.Request) *sessions.Session {
	sess, _ := store.Get(r, "purpleops")
	return sess
}

// SaveSession saves the session, properly setting the Secure flag based on the request.
func SaveSession(w http.ResponseWriter, r *http.Request, sess *sessions.Session) error {
	sess.Options.Secure = isSecureRequest(r)
	return sess.Save(r, w)
}

// SetSessionUser stores the user ID in a new session, invalidating the old one
// to prevent session fixation attacks.
func SetSessionUser(w http.ResponseWriter, r *http.Request, userID string) {
	// Invalidate the pre-login session so a previously known session ID
	// cannot be used by an attacker after the user authenticates.
	old := GetSession(r)
	old.Values = make(map[interface{}]interface{})
	old.Options.MaxAge = -1
	old.Options.Secure = isSecureRequest(r)
	old.Save(r, w)

	// Create a fresh session with a new ID.
	newSess, _ := store.New(r, "purpleops")
	newSess.Values["user_id"] = userID
	newSess.Options.Secure = isSecureRequest(r)
	newSess.Save(r, w)
}

// ClearSession destroys the session.
func ClearSession(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	sess.Values = make(map[interface{}]interface{})
	sess.Options.MaxAge = -1
	sess.Options.Secure = isSecureRequest(r)
	sess.Save(r, w)
}

// GetCurrentUser looks up the authenticated user from the session.
func GetCurrentUser(r *http.Request) *models.User {
	sess := GetSession(r)
	userID, ok := sess.Values["user_id"].(string)
	if !ok || userID == "" {
		return nil
	}
	user, err := models.FindUser(r.Context(), userID)
	if err != nil {
		return nil
	}
	return user
}
