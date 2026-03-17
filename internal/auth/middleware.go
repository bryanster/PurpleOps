package auth

import (
	"net/http"
	"strings"

	"github.com/bryanster/purpleops/internal/models"
)

// AuthRequired redirects to /login if not authenticated.
func AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetCurrentUser(r)
		if user == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		ctx := WithUser(r.Context(), user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RolesAccepted checks that the user has at least one of the specified roles.
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

// UserAssignedAssessment checks the user has access to the assessment.
// The ID can be an assessment ID or a testcase ID.
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

		// Check if ID is a testcase ID; if so get its assessment ID.
		assessmentID := id
		tc, err := models.FindTestCase(r.Context(), id)
		if err == nil && tc != nil {
			assessmentID = tc.AssessmentID
		}

		// Check if user has access to this assessment.
		for _, aid := range user.Assessments {
			if aid.Hex() == assessmentID {
				next.ServeHTTP(w, r)
				return
			}
		}

		http.Error(w, "Forbidden", http.StatusForbidden)
	})
}

// extractID pulls the resource ID from the URL path for /assessment/{id} or /testcase/{id}.
func extractID(r *http.Request) string {
	parts := strings.Split(r.URL.Path, "/")
	for i, part := range parts {
		if (part == "assessment" || part == "testcase") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
