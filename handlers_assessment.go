package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/flosch/pongo2/v6"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleNewAssessment creates a new assessment. POST /assessment. Admin only.
func HandleNewAssessment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	now := nowPtr()
	assessment := Assessment{
		ID:          bson.NewObjectID(),
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Created:     now,
	}

	_, err := Col("assessment").InsertOne(r.Context(), &assessment)
	if err != nil {
		http.Error(w, "Failed to create assessment", http.StatusInternalServerError)
		return
	}

	// Create files directory for this assessment.
	filesDir := filepath.Join("files", assessment.ID.Hex())
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		http.Error(w, "Failed to create files directory", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(assessment.ToJSON(false))
}

// HandleEditAssessment updates an assessment's name and description. POST /assessment/{id}. Admin only.
func HandleEditAssessment(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	id := chi.URLParam(r, "id")
	assessment, err := FindAssessment(r.Context(), id)
	if err != nil {
		http.Error(w, "Assessment not found", http.StatusNotFound)
		return
	}

	assessment.Name = r.FormValue("name")
	assessment.Description = r.FormValue("description")

	_, err = Col("assessment").UpdateOne(r.Context(), bson.M{"_id": assessment.ID}, bson.M{
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

// HandleDeleteAssessment deletes an assessment, its testcases, and files. DELETE /assessment/{id}. Admin only.
func HandleDeleteAssessment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Delete all testcases belonging to this assessment.
	_, err := Col("test_case").DeleteMany(r.Context(), bson.M{"assessmentid": id})
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
	_, err = Col("assessment").DeleteOne(r.Context(), bson.M{"_id": oid})
	if err != nil {
		http.Error(w, "Failed to delete assessment", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleLoadAssessment renders the assessment page. GET /assessment/{id}.
func HandleLoadAssessment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	assessment, err := FindAssessment(r.Context(), id)
	if err != nil {
		http.Error(w, "Assessment not found", http.StatusNotFound)
		return
	}

	testcases, err := GetTestCases(r.Context(), id)
	if err != nil {
		http.Error(w, "Failed to load testcases", http.StatusInternalServerError)
		return
	}

	// Load all testcase templates.
	var templates []TestCaseTemplate
	cursor, err := Col("test_case_template").Find(r.Context(), bson.M{})
	if err == nil {
		cursor.All(r.Context(), &templates)
	}

	// Load techniques sorted by MitreID (templates access .MitreID and .Name).
	var techniques []Technique
	cursor, err = Col("technique").Find(r.Context(), bson.M{})
	if err == nil {
		cursor.All(r.Context(), &techniques)
	}
	sort.Slice(techniques, func(i, j int) bool {
		return techniques[i].MitreID < techniques[j].MitreID
	})

	// Load tactics (templates access .Name).
	var tactics []Tactic
	cursor, err = Col("tactic").Find(r.Context(), bson.M{})
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

	// Build multi map.
	multi := map[string]interface{}{
		"datasources":       assessment.MultiToJSON("datasources", false),
		"rules":             assessment.MultiToJSON("rules", false),
		"detectionsources":  assessment.MultiToJSON("detectionsources", false),
		"preventionsources": assessment.MultiToJSON("preventionsources", false),
	}

	Render(w, r, "assessment.html", pongo2.Context{
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
	user := UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if user.InitPwd {
		http.Redirect(w, r, "/password/change", http.StatusFound)
		return
	}

	var assessments []Assessment
	cursor, err := Col("assessment").Find(r.Context(), bson.M{})
	if err == nil {
		cursor.All(r.Context(), &assessments)
	}

	Render(w, r, "assessments.html", pongo2.Context{
		"assessments": assessments,
	})
}
