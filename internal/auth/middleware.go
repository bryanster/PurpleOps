package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bryanster/purpleops/internal/db"
	"github.com/bryanster/purpleops/internal/models"
	"go.mongodb.org/mongo-driver/v2/bson"
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

// APIKeyAuth is middleware that authenticates requests using an X-API-Key header.
// On success it sets a user in context whose roles and assessments are restricted
// to those granted by the API key (always a subset of the key owner's permissions).
func APIKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("X-API-Key")
		if raw == "" {
			// Also accept Authorization: Bearer <key>
			bearer := r.Header.Get("Authorization")
			if strings.HasPrefix(bearer, "Bearer ") {
				raw = strings.TrimPrefix(bearer, "Bearer ")
			}
		}
		if raw == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		sum := sha256.Sum256([]byte(raw))
		hash := hex.EncodeToString(sum[:])

		apiKey, err := models.FindAPIKeyByHash(r.Context(), hash)
		if err != nil || apiKey == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		owner, err := models.FindUser(r.Context(), apiKey.UserID.Hex())
		if err != nil || owner == nil || !owner.Active {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Build a restricted user: same as owner but with only the API key's
		// allowed roles and assessments.
		restricted := *owner
		restricted.Roles = intersectIDs(owner.Roles, apiKey.Roles)
		restricted.Assessments = intersectIDs(owner.Assessments, apiKey.Assessments)

		// Update last_used_at asynchronously.
		now := time.Now()
		go func() {
			if _, err := db.Col(db.ColAPIKey).UpdateOne(r.Context(), bson.M{"_id": apiKey.ID}, bson.M{"$set": bson.M{"last_used_at": now}}); err != nil {
				slog.Warn("api key: failed to update last_used_at", "key_id", apiKey.ID.Hex(), "err", err)
			}
		}()

		ctx := WithUser(r.Context(), &restricted)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// intersectIDs returns the intersection of two ObjectID slices.
func intersectIDs(a, b []bson.ObjectID) []bson.ObjectID {
	set := make(map[bson.ObjectID]struct{}, len(b))
	for _, id := range b {
		set[id] = struct{}{}
	}
	var result []bson.ObjectID
	for _, id := range a {
		if _, ok := set[id]; ok {
			result = append(result, id)
		}
	}
	return result
}
