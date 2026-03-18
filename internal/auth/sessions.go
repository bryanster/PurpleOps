package auth

import (
	"net/http"

	"github.com/bryanster/purpleops/internal/models"
	"github.com/gorilla/sessions"
)

// store is the package-level session store.
var store *sessions.CookieStore

// InitSessions initialises the cookie-based session store.
func InitSessions(secretKey string) {
	store = sessions.NewCookieStore([]byte(secretKey))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 1 week
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}

// GetSession returns the named session from the request.
func GetSession(r *http.Request) *sessions.Session {
	sess, _ := store.Get(r, "purpleops")
	return sess
}

// SetSessionUser stores the user ID in a new session, invalidating the old one
// to prevent session fixation attacks.
func SetSessionUser(w http.ResponseWriter, r *http.Request, userID string) {
	// Invalidate the pre-login session so a previously known session ID
	// cannot be used by an attacker after the user authenticates.
	old := GetSession(r)
	old.Values = make(map[interface{}]interface{})
	old.Options.MaxAge = -1
	old.Save(r, w)

	// Create a fresh session with a new ID.
	newSess, _ := store.New(r, "purpleops")
	newSess.Values["user_id"] = userID
	newSess.Save(r, w)
}

// ClearSession destroys the session.
func ClearSession(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	sess.Values = make(map[interface{}]interface{})
	sess.Options.MaxAge = -1
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
