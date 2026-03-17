package main

import (
	"context"
	"net/http"
	"strings"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

var store *sessions.CookieStore

func InitSessions(secretKey string) {
	store = sessions.NewCookieStore([]byte(secretKey))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 1 week
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}

// Session helpers

func GetSession(r *http.Request) *sessions.Session {
	sess, _ := store.Get(r, "purpleops")
	return sess
}

func SetSessionUser(w http.ResponseWriter, r *http.Request, userID string) {
	sess := GetSession(r)
	sess.Values["user_id"] = userID
	sess.Save(r, w)
}

func ClearSession(w http.ResponseWriter, r *http.Request) {
	sess := GetSession(r)
	sess.Values = make(map[interface{}]interface{})
	sess.Options.MaxAge = -1
	sess.Save(r, w)
}

func GetCurrentUser(r *http.Request) *User {
	sess := GetSession(r)
	userID, ok := sess.Values["user_id"].(string)
	if !ok || userID == "" {
		return nil
	}
	user, err := FindUser(r.Context(), userID)
	if err != nil {
		return nil
	}
	return user
}

// Password hashing

func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

func CheckPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// Context key for current user
type contextKey string

const userContextKey contextKey = "currentUser"

func UserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(userContextKey).(*User)
	return u
}

// --- Middleware ---

// AuthRequired redirects to /login if not authenticated
func AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetCurrentUser(r)
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RolesAccepted checks that the user has at least one of the specified roles
func RolesAccepted(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			userRoles := user.GetRoleNames(r.Context())
			for _, required := range roles {
				for _, has := range userRoles {
					if has == required {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			http.Error(w, "Forbidden", http.StatusForbidden)
		})
	}
}

// UserAssignedAssessment checks the user has access to the assessment
// The ID can be an assessment ID or a testcase ID
func UserAssignedAssessment(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := UserFromContext(r.Context())
		if user == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if user.HasRole(r.Context(), "Admin") {
			next.ServeHTTP(w, r)
			return
		}

		// Extract ID from URL - works for both /assessment/{id} and /testcase/{id}
		id := extractID(r)
		if id == "" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Check if ID is a testcase ID, if so get its assessment ID
		assessmentID := id
		tc, err := FindTestCase(r.Context(), id)
		if err == nil && tc != nil {
			assessmentID = tc.AssessmentID
		}

		// Check if user has access to this assessment
		for _, aid := range user.Assessments {
			if aid.Hex() == assessmentID {
				next.ServeHTTP(w, r)
				return
			}
		}

		http.Error(w, "Forbidden", http.StatusForbidden)
	})
}

func extractID(r *http.Request) string {
	// Extract the ID segment from the URL path
	parts := strings.Split(r.URL.Path, "/")
	for i, part := range parts {
		if (part == "assessment" || part == "testcase") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
