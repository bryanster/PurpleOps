package handler

import (
	"encoding/json"
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
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleNewAssessment creates a new assessment. POST /assessment.
func HandleNewAssessment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	now := models.NowPtr()
	assessment := models.Assessment{
		ID:          bson.NewObjectID(),
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Created:     now,
	}

	_, err := db.Col("assessment").InsertOne(r.Context(), &assessment)
	if err != nil {
		http.Error(w, "Failed to create assessment", http.StatusInternalServerError)
		return
	}

	// Create files directory for this assessment.
	filesDir := filepath.Join("files", assessment.ID.Hex())
	if err := os.MkdirAll(filesDir, 0o750); err != nil {
		http.Error(w, "Failed to create files directory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assessment.ToJSON(false))
}

// HandleEditAssessment updates an assessment's name and description. POST /assessment/{id}.
func HandleEditAssessment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	id := chi.URLParam(r, "id")
	assessment, err := models.FindAssessment(r.Context(), id)
	if err != nil {
		http.Error(w, "Assessment not found", http.StatusNotFound)
		return
	}

	assessment.Name = r.FormValue("name")
	assessment.Description = r.FormValue("description")

	_, err = db.Col("assessment").UpdateOne(r.Context(), bson.M{"_id": assessment.ID}, bson.M{
		"$set": bson.M{
			"name":        assessment.Name,
			"description": assessment.Description,
		},
	})
	if err != nil {
		http.Error(w, "Failed to update assessment", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assessment.ToJSON(false))
}

// HandleDeleteAssessment deletes an assessment, its testcases, and files. DELETE /assessment/{id}.
func HandleDeleteAssessment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Delete all testcases belonging to this assessment.
	_, err := db.Col("test_case").DeleteMany(r.Context(), bson.M{"assessmentid": id})
	if err != nil {
		http.Error(w, "Failed to delete testcases", http.StatusInternalServerError)
		return
	}

	// Remove the files directory.
	filesDir := filepath.Join("files", id)
	os.RemoveAll(filesDir)

	// Delete the assessment document.
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}
	_, err = db.Col("assessment").DeleteOne(r.Context(), bson.M{"_id": oid})
	if err != nil {
		http.Error(w, "Failed to delete assessment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleLoadAssessment renders the assessment page. GET /assessment/{id}.
func HandleLoadAssessment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	assessment, err := models.FindAssessment(r.Context(), id)
	if err != nil {
		http.Error(w, "Assessment not found", http.StatusNotFound)
		return
	}

	testcases, err := models.GetTestCases(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to load testcases", http.StatusInternalServerError)
		return
	}

	// Load all testcase templates.
	var templates []models.TestCaseTemplate
	cursor, err := db.Col("test_case_template").Find(r.Context(), bson.M{})
	if err == nil {
		cursor.All(r.Context(), &templates)
	}

	// Load techniques sorted by MitreID.
	var techniques []models.Technique
	cursor, err = db.Col("technique").Find(r.Context(), bson.M{})
	if err == nil {
		cursor.All(r.Context(), &techniques)
	}
	sort.Slice(techniques, func(i, j int) bool {
		return techniques[i].MitreID < techniques[j].MitreID
	})

	// Load tactics.
	var tactics []models.Tactic
	cursor, err = db.Col("tactic").Find(r.Context(), bson.M{})
	if err == nil {
		cursor.All(r.Context(), &tactics)
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

	render.Render(w, r, "assessment.html", pongo2.Context{
		"testcases":  testcases,
		"assessment": assessment,
		"templates":  templates,
		"mitres":     techniques,
		"tactics":    tactics,
		"hexagons":   hexagons,
		"reports":    reports,
		"multi":      multi,
	})
}

// HandleIndex renders the assessments index page. GET /.
func HandleIndex(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if user.InitPwd {
		http.Redirect(w, r, "/password/change", http.StatusFound)
		return
	}

	var assessments []models.Assessment
	cursor, err := db.Col("assessment").Find(r.Context(), bson.M{})
	if err == nil {
		cursor.All(r.Context(), &assessments)
	}

	render.Render(w, r, "assessments.html", pongo2.Context{
		"assessments": assessments,
	})
}
