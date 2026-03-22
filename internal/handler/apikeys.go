package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/db"
	"github.com/bryanster/purpleops/internal/models"
	"github.com/bryanster/purpleops/internal/render"
	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleAPIKeysPage renders the API key management page for the current user.
// GET /api-keys
func HandleAPIKeysPage(c *gin.Context) {
	ctx := c.Request.Context()
	user := auth.UserFromContext(ctx)
	if user == nil {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	keys, err := models.FindAPIKeysByUser(ctx, user.ID)
	if err != nil {
		keys = []models.APIKey{}
	}

	// Load all roles and assessments the user is allowed to grant.
	userRoleIDs := user.Roles
	var allowedRoles []models.Role
	for _, rid := range userRoleIDs {
		var role models.Role
		if err := db.Col(db.ColRole).FindOne(ctx, bson.M{"_id": rid}).Decode(&role); err == nil {
			allowedRoles = append(allowedRoles, role)
		}
	}

	// Admins can see all assessments.
	var allowedAssessments []models.Assessment
	for _, aid := range user.AssessmentList(ctx) {
		var a models.Assessment
		if err := db.Col(db.ColAssessment).FindOne(ctx, bson.M{"_id": aid}).Decode(&a); err == nil {
			allowedAssessments = append(allowedAssessments, a)
		}
	}

	// Build display-friendly key list.
	type keyDisplay struct {
		models.APIKey
		RoleNames       string
		AssessmentNames string
	}
	displayKeys := make([]keyDisplay, len(keys))
	for i, k := range keys {
		displayKeys[i] = keyDisplay{
			APIKey:          k,
			RoleNames:       strings.Join(k.GetRoleNames(ctx), ", "),
			AssessmentNames: strings.Join(k.GetAssessmentNames(ctx), ", "),
		}
	}

	render.Render(c.Writer, c.Request, "apikeys.html", pongo2.Context{
		"api_keys":    displayKeys,
		"roles":       allowedRoles,
		"assessments": allowedAssessments,
	})
}

// HandleCreateAPIKey creates a new API key for the authenticated user.
// POST /api-keys
func HandleCreateAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	user := auth.UserFromContext(ctx)
	if user == nil {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "Bad request")
		return
	}

	name := strings.TrimSpace(c.Request.FormValue("name"))
	if name == "" {
		c.String(http.StatusBadRequest, "Name is required")
		return
	}

	// Resolve requested role names to IDs, only allowing the user's own roles.
	userRoleSet := make(map[bson.ObjectID]struct{})
	for _, rid := range user.Roles {
		userRoleSet[rid] = struct{}{}
	}

	roleNames := c.Request.Form["roles"]
	var roleIDs []bson.ObjectID
	for _, rname := range roleNames {
		role, err := models.FindRole(ctx, rname)
		if err != nil {
			continue
		}
		if _, ok := userRoleSet[role.ID]; !ok {
			c.String(http.StatusForbidden, "Cannot grant role not assigned to your account")
			return
		}
		roleIDs = append(roleIDs, role.ID)
	}

	// Resolve requested assessment IDs, only allowing the user's own assessments.
	userAssessmentSet := make(map[bson.ObjectID]struct{})
	for _, aid := range user.AssessmentList(ctx) {
		userAssessmentSet[aid] = struct{}{}
	}

	assessmentIDs := c.Request.Form["assessments"]
	var allowedAssessmentIDs []bson.ObjectID
	for _, hexID := range assessmentIDs {
		oid, err := bson.ObjectIDFromHex(hexID)
		if err != nil {
			continue
		}
		if _, ok := userAssessmentSet[oid]; !ok {
			c.String(http.StatusForbidden, "Cannot grant access to assessment not assigned to your account")
			return
		}
		allowedAssessmentIDs = append(allowedAssessmentIDs, oid)
	}

	// Generate a cryptographically random key: prefix "pops_" + 32 random bytes hex-encoded.
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		c.String(http.StatusInternalServerError, "Failed to generate key")
		return
	}
	rawKey := "pops_" + hex.EncodeToString(rawBytes)

	// Hash the key for storage.
	sum := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(sum[:])

	now := time.Now()
	apiKey := models.APIKey{
		ID:          bson.NewObjectID(),
		UserID:      user.ID,
		Name:        name,
		KeyHash:     keyHash,
		Prefix:      rawKey[:13], // "pops_" + first 8 hex chars
		Roles:       roleIDs,
		Assessments: allowedAssessmentIDs,
		CreatedAt:   now,
		Active:      true,
	}

	col := db.Col(db.ColAPIKey)
	if col == nil {
		c.String(http.StatusInternalServerError, "Failed to create API key")
		return
	}
	if _, err := col.InsertOne(ctx, &apiKey); err != nil {
		c.String(http.StatusInternalServerError, "Failed to create API key")
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"id":     apiKey.ID.Hex(),
		"name":   apiKey.Name,
		"key":    rawKey,
		"prefix": apiKey.Prefix,
	})
}

// HandleDeleteAPIKey revokes an API key owned by the current user.
// DELETE /api-keys/:id
func HandleDeleteAPIKey(c *gin.Context) {
	ctx := c.Request.Context()
	user := auth.UserFromContext(ctx)
	if user == nil {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	id := c.Param("id")
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ID")
		return
	}

	// Only delete keys owned by the current user.
	result, err := db.Col(db.ColAPIKey).DeleteOne(ctx, bson.M{"_id": oid, "user_id": user.ID})
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to delete API key")
		return
	}
	if result.DeletedCount == 0 {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	c.Status(http.StatusOK)
}
