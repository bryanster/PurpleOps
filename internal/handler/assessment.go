package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/db"
	"github.com/bryanster/purpleops/internal/models"
	"github.com/bryanster/purpleops/internal/render"
	"github.com/flosch/pongo2/v6"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleNewAssessment creates a new assessment. POST /assessment.
func HandleNewAssessment(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "Bad request")
		return
	}

	now := models.NowPtr()
	assessment := models.Assessment{
		ID:          bson.NewObjectID(),
		Name:        c.Request.FormValue("name"),
		Description: c.Request.FormValue("description"),
		Created:     now,
	}

	_, err := db.Col(db.ColAssessment).InsertOne(c.Request.Context(), &assessment)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to create assessment")
		return
	}

	// Create files directory for this assessment.
	filesDir := filepath.Join("files", assessment.ID.Hex())
	if err := os.MkdirAll(filesDir, DirPerm); err != nil {
		c.String(http.StatusInternalServerError, "Failed to create files directory")
		return
	}

	c.JSON(http.StatusOK, assessment.ToJSON(false))
}

// HandleEditAssessment updates an assessment's name and description. POST /assessment/:id.
func HandleEditAssessment(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusBadRequest, "Bad request")
		return
	}

	id := c.Param("id")
	assessment, err := models.FindAssessment(c.Request.Context(), id)
	if err != nil {
		c.String(http.StatusNotFound, "Assessment not found")
		return
	}

	assessment.Name = c.Request.FormValue("name")
	assessment.Description = c.Request.FormValue("description")

	_, err = db.Col(db.ColAssessment).UpdateOne(c.Request.Context(), bson.M{"_id": assessment.ID}, bson.M{
		"$set": bson.M{
			"name":        assessment.Name,
			"description": assessment.Description,
		},
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to update assessment")
		return
	}

	c.JSON(http.StatusOK, assessment.ToJSON(false))
}

// HandleDeleteAssessment deletes an assessment, its testcases, and files. DELETE /assessment/:id.
func HandleDeleteAssessment(c *gin.Context) {
	id := c.Param("id")

	// Delete all testcases belonging to this assessment.
	_, err := db.Col(db.ColTestCase).DeleteMany(c.Request.Context(), bson.M{"assessmentid": id})
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to delete testcases")
		return
	}

	// Remove the files directory.
	filesDir := filepath.Join("files", id)
	os.RemoveAll(filesDir)

	// Delete the assessment document.
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ID")
		return
	}
	_, err = db.Col(db.ColAssessment).DeleteOne(c.Request.Context(), bson.M{"_id": oid})
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to delete assessment")
		return
	}

	c.Status(http.StatusOK)
}

// HandleLoadAssessment renders the assessment page. GET /assessment/:id.
func HandleLoadAssessment(c *gin.Context) {
	id := c.Param("id")

	assessment, err := models.FindAssessment(c.Request.Context(), id)
	if err != nil {
		c.String(http.StatusNotFound, "Assessment not found")
		return
	}

	testcases, err := models.GetTestCases(c.Request.Context(), id)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load testcases")
		return
	}

	// Load all testcase templates.
	var templates []models.TestCaseTemplate
	cursor, err := db.Col(db.ColTestCaseTemplate).Find(c.Request.Context(), bson.M{})
	if err == nil {
		cursor.All(c.Request.Context(), &templates)
	}

	// Load techniques sorted by MitreID.
	var techniques []models.Technique
	cursor, err = db.Col(db.ColTechnique).Find(c.Request.Context(), bson.M{})
	if err == nil {
		cursor.All(c.Request.Context(), &techniques)
	}
	sort.Slice(techniques, func(i, j int) bool {
		return techniques[i].MitreID < techniques[j].MitreID
	})

	// Load tactics.
	var tactics []models.Tactic
	cursor, err = db.Col(db.ColTactic).Find(c.Request.Context(), bson.M{})
	if err == nil {
		cursor.All(c.Request.Context(), &tactics)
	}

	// Render hexagons.
	hexagons := RenderHexagons(id)

	// List report files from custom/reports/.
	reports := []string{}
	reportsDir := filepath.Join("custom", "reports")
	entries, err := os.ReadDir(reportsDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".docx") {
				reports = append(reports, entry.Name())
			}
		}
	}

	// Build multi map — pass raw structs so templates can access
	// .ID.Hex, .Name, .Description directly via pongo2.
	multi := map[string]interface{}{
		"Datasources":       assessment.Datasources,
		"Rules":             assessment.Rules,
		"DetectionSources":  assessment.DetectionSources,
		"PreventionSources": assessment.PreventionSources,
	}

	// Build MitreID → first tactic mapping for JS auto-population.
	mitreTactics := buildMitreTacticsMap(techniques)

	// Build MitreID → technique name mapping for display.
	mitreNames := make(map[string]string, len(techniques))
	for _, t := range techniques {
		mitreNames[t.MitreID] = t.Name
	}

	render.Render(c.Writer, c.Request, "assessment.html", pongo2.Context{
		"testcases":     testcases,
		"assessment":    assessment,
		"templates":     templates,
		"mitres":        techniques,
		"tactics":       tactics,
		"hexagons":      hexagons,
		"reports":       reports,
		"multi":         multi,
		"mitre_tactics": mitreTactics,
		"mitre_names":   mitreNames,
	})
}

// HandleIndex renders the assessments index page. GET /.
func HandleIndex(c *gin.Context) {
	user := auth.UserFromContext(c.Request.Context())
	if user == nil {
		c.Redirect(http.StatusFound, "/login")
		return
	}

	if user.InitPwd {
		c.Redirect(http.StatusFound, "/password/change")
		return
	}

	var assessments []models.Assessment
	cursor, err := db.Col(db.ColAssessment).Find(c.Request.Context(), bson.M{})
	if err == nil {
		cursor.All(c.Request.Context(), &assessments)
	}

	render.Render(c.Writer, c.Request, "assessments.html", pongo2.Context{
		"assessments": assessments,
	})
}
