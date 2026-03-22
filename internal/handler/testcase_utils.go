package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/db"
	"github.com/bryanster/purpleops/internal/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleToggleVisibility toggles the visible flag on a testcase.
// GET /testcase/:id/toggle-visibility
func HandleToggleVisibility(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	tc, err := models.FindTestCase(ctx, id)
	if err != nil {
		c.String(http.StatusNotFound, "Testcase not found")
		return
	}

	tc.Visible = !tc.Visible

	oid, _ := bson.ObjectIDFromHex(id)
	_, err = db.Col(db.ColTestCase).ReplaceOne(ctx, bson.M{"_id": oid}, tc)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to update testcase")
		return
	}

	c.JSON(http.StatusOK, tc.ToJSON(false))
}

// HandleCloneTestCase clones a testcase with selected fields.
// GET /testcase/:id/clone
func HandleCloneTestCase(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	tc, err := models.FindTestCase(ctx, id)
	if err != nil {
		c.String(http.StatusNotFound, "Testcase not found")
		return
	}

	clone := models.TestCase{
		ID:           bson.NewObjectID(),
		AssessmentID: tc.AssessmentID,
		Name:         tc.Name + " (Copy)",
		Objective:    tc.Objective,
		Actions:      tc.Actions,
		RedNotes:     tc.RedNotes,
		MitreID:      tc.MitreID,
		UUID:         tc.UUID,
		Tactic:       tc.Tactic,
		Tools:        tc.Tools,
		Tags:         tc.Tags,
		State:        "Pending",
		Visible:      true,
	}

	_, err = db.Col(db.ColTestCase).InsertOne(ctx, &clone)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to clone testcase")
		return
	}

	c.JSON(http.StatusOK, clone.ToJSON(false))
}

// HandleDeleteTestCase deletes a testcase and its evidence directory.
// GET /testcase/:id/delete
func HandleDeleteTestCase(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()

	tc, err := models.FindTestCase(ctx, id)
	if err != nil {
		c.String(http.StatusNotFound, "Testcase not found")
		return
	}

	oid, _ := bson.ObjectIDFromHex(id)
	_, err = db.Col(db.ColTestCase).DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to delete testcase")
		return
	}

	// Remove evidence directory
	dir := filepath.Join("files", tc.AssessmentID, tc.ID.Hex())
	os.RemoveAll(dir)

	c.Status(http.StatusOK)
}

// HandleDeleteEvidence deletes a single evidence file.
// DELETE /testcase/:id/evidence/:colour/:file
func HandleDeleteEvidence(c *gin.Context) {
	id := c.Param("id")
	colour := c.Param("colour")
	filename := c.Param("file")
	ctx := c.Request.Context()
	user := auth.UserFromContext(ctx)

	if colour != "red" && colour != "blue" {
		c.String(http.StatusBadRequest, "Invalid colour")
		return
	}

	// Blue users cannot delete red files
	isBlue := user.HasRole(ctx, "Blue") && !user.HasRole(ctx, "Red") && !user.HasRole(ctx, "Admin")
	if isBlue && colour == "red" {
		c.String(http.StatusForbidden, "Forbidden")
		return
	}

	tc, err := models.FindTestCase(ctx, id)
	if err != nil {
		c.String(http.StatusNotFound, "Testcase not found")
		return
	}

	safeName := sanitizeFilenameSafe(filename)

	// Remove from the appropriate file list and delete physical file
	var files *[]models.FileDoc
	if colour == "red" {
		files = &tc.RedFiles
	} else {
		files = &tc.BlueFiles
	}

	found := false
	newFiles := make([]models.FileDoc, 0, len(*files))
	for _, f := range *files {
		if f.Name == safeName {
			// Delete the physical file
			os.Remove(f.Path)
			found = true
		} else {
			newFiles = append(newFiles, f)
		}
	}

	if !found {
		c.String(http.StatusNotFound, "File not found")
		return
	}

	*files = newFiles

	oid, _ := bson.ObjectIDFromHex(id)
	_, err = db.Col(db.ColTestCase).ReplaceOne(ctx, bson.M{"_id": oid}, tc)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to update testcase")
		return
	}

	c.Status(http.StatusNoContent)
}

// HandleFetchEvidence serves an evidence file.
// GET /testcase/:id/evidence/:file
func HandleFetchEvidence(c *gin.Context) {
	id := c.Param("id")
	filename := c.Param("file")

	tc, err := models.FindTestCase(c.Request.Context(), id)
	if err != nil {
		c.String(http.StatusNotFound, "Testcase not found")
		return
	}

	safeName := sanitizeFilenameSafe(filename)
	filePath := filepath.Join("files", tc.AssessmentID, tc.ID.Hex(), safeName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.String(http.StatusNotFound, "File not found")
		return
	}

	// If download query param is present, set as attachment
	if c.Request.URL.Query().Has("download") {
		c.Header("Content-Disposition", "attachment; filename=\""+safeName+"\"")
	}

	http.ServeFile(c.Writer, c.Request, filePath)
}

// HandleToggleTimer starts or stops the timer on a testcase.
// GET /testcase/:id/toggle-timer
func HandleToggleTimer(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()
	user := auth.UserFromContext(ctx)

	// Blue-only users cannot control the timer
	isBlue := user.HasRole(ctx, "Blue") && !user.HasRole(ctx, "Red") && !user.HasRole(ctx, "Admin")
	if isBlue {
		c.String(http.StatusForbidden, "Forbidden")
		return
	}

	tc, err := models.FindTestCase(ctx, id)
	if err != nil {
		c.String(http.StatusNotFound, "Testcase not found")
		return
	}

	now := models.NowPtr()

	switch tc.State {
	case "Pending":
		// Start: set start time and state to Running
		tc.StartTime = now
		tc.EndTime = nil
		tc.State = "Running"
	case "Running":
		// Stop: set end time and state to Complete
		tc.EndTime = now
		tc.State = "Complete"
	case "Complete":
		// Restart: reset start time, clear end time, state to Running
		tc.StartTime = now
		tc.EndTime = nil
		tc.State = "Running"
	default:
		// Invalid/empty state: reset to Running (start fresh)
		tc.State = "Running"
		tc.StartTime = now
		tc.EndTime = nil
	}

	tc.ModifyTime = now

	oid, _ := bson.ObjectIDFromHex(id)
	_, err = db.Col(db.ColTestCase).ReplaceOne(ctx, bson.M{"_id": oid}, tc)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to update testcase")
		return
	}

	c.JSON(http.StatusOK, tc.ToJSON(false))
}

// sanitizeFilenameSafe provides simple filename sanitization for URL parameters.
func sanitizeFilenameSafe(name string) string {
	// Replace path separators
	name = strings.ReplaceAll(name, string(os.PathSeparator), "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	// Trim leading dots to prevent hidden files / traversal
	name = strings.TrimLeft(name, ".")
	if name == "" {
		name = "unnamed"
	}
	return name
}
