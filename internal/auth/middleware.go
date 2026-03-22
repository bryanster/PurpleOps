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
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// idleTimeout is the maximum allowed inactivity before a session is expired.
const idleTimeout = 30 * time.Minute

// AuthRequired redirects to /login if not authenticated.
// It also enforces an idle session timeout: if the user has been inactive for
// longer than idleTimeout the session is cleared.
func AuthRequired(c *gin.Context) {
	user := GetCurrentUser(c.Request)
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		c.Abort()
		return
	}

	// Idle timeout check.
	sess := GetSession(c.Request)
	if lastActive, ok := sess.Values["last_active"].(int64); ok {
		if time.Now().Unix()-lastActive > int64(idleTimeout.Seconds()) {
			ClearSession(c.Writer, c.Request)
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
	}
	sess.Values["last_active"] = time.Now().Unix()
	sess.Save(c.Request, c.Writer)

	ctx := WithUser(c.Request.Context(), user)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
}

// RolesAccepted checks that the user has at least one of the specified roles.
func RolesAccepted(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := UserFromContext(c.Request.Context())
		if user == nil {
			c.String(http.StatusUnauthorized, "Unauthorized")
			c.Abort()
			return
		}
		userRoles := user.GetRoleNames(c.Request.Context())
		for _, required := range roles {
			for _, has := range userRoles {
				if has == required {
					c.Next()
					return
				}
			}
		}
		c.String(http.StatusForbidden, "Forbidden")
		c.Abort()
	}
}

// UserAssignedAssessment checks the user has access to the assessment.
// The ID can be an assessment ID or a testcase ID.
func UserAssignedAssessment(c *gin.Context) {
	user := UserFromContext(c.Request.Context())
	if user == nil {
		c.String(http.StatusUnauthorized, "Unauthorized")
		c.Abort()
		return
	}
	if user.HasRole(c.Request.Context(), "Admin") {
		c.Next()
		return
	}

	// Extract ID from URL - works for both /assessment/:id and /testcase/:id
	id := extractID(c.Request)
	if id == "" {
		c.String(http.StatusForbidden, "Forbidden")
		c.Abort()
		return
	}

	// Check if ID is a testcase ID; if so get its assessment ID.
	assessmentID := id
	tc, err := models.FindTestCase(c.Request.Context(), id)
	if err == nil && tc != nil {
		assessmentID = tc.AssessmentID
	}

	// Check if user has access to this assessment.
	for _, aid := range user.Assessments {
		if aid.Hex() == assessmentID {
			c.Next()
			return
		}
	}

	c.String(http.StatusForbidden, "Forbidden")
	c.Abort()
}

// extractID pulls the resource ID from the URL path for /assessment/:id or /testcase/:id.
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
func APIKeyAuth(c *gin.Context) {
	raw := c.Request.Header.Get("X-API-Key")
	if raw == "" {
		// Also accept Authorization: Bearer <key>
		bearer := c.Request.Header.Get("Authorization")
		if strings.HasPrefix(bearer, "Bearer ") {
			raw = strings.TrimPrefix(bearer, "Bearer ")
		}
	}
	if raw == "" {
		c.String(http.StatusUnauthorized, "Unauthorized")
		c.Abort()
		return
	}

	sum := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(sum[:])

	apiKey, err := models.FindAPIKeyByHash(c.Request.Context(), hash)
	if err != nil || apiKey == nil {
		c.String(http.StatusUnauthorized, "Unauthorized")
		c.Abort()
		return
	}

	owner, err := models.FindUser(c.Request.Context(), apiKey.UserID.Hex())
	if err != nil || owner == nil || !owner.Active {
		c.String(http.StatusUnauthorized, "Unauthorized")
		c.Abort()
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
		if _, err := db.Col(db.ColAPIKey).UpdateOne(c.Request.Context(), bson.M{"_id": apiKey.ID}, bson.M{"$set": bson.M{"last_used_at": now}}); err != nil {
			slog.Warn("api key: failed to update last_used_at", "key_id", apiKey.ID.Hex(), "err", err)
		}
	}()

	ctx := WithUser(c.Request.Context(), &restricted)
	c.Request = c.Request.WithContext(ctx)
	c.Next()
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
