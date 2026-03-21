package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/db"
	"github.com/bryanster/purpleops/internal/models"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleToggleVisibility toggles the visible flag on a testcase.
// GET /testcase/{id}/toggle-visibility
func HandleToggleVisibility(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	tc, err := models.FindTestCase(ctx, id)
	if err != nil {
		http.Error(w, "Testcase not found", http.StatusNotFound)
		return
	}

	tc.Visible = !tc.Visible

	oid, _ := bson.ObjectIDFromHex(id)
	_, err = db.Col(db.ColTestCase).ReplaceOne(ctx, bson.M{"_id": oid}, tc)
	if err != nil {
		http.Error(w, "Failed to update testcase", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tc.ToJSON(false))
}

// HandleCloneTestCase clones a testcase with selected fields.
// GET /testcase/{id}/clone
func HandleCloneTestCase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	tc, err := models.FindTestCase(ctx, id)
	if err != nil {
		http.Error(w, "Testcase not found", http.StatusNotFound)
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
		http.Error(w, "Failed to clone testcase", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(clone.ToJSON(false))
}

// HandleDeleteTestCase deletes a testcase and its evidence directory.
// GET /testcase/{id}/delete
func HandleDeleteTestCase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	tc, err := models.FindTestCase(ctx, id)
	if err != nil {
		http.Error(w, "Testcase not found", http.StatusNotFound)
		return
	}

	oid, _ := bson.ObjectIDFromHex(id)
	_, err = db.Col(db.ColTestCase).DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		http.Error(w, "Failed to delete testcase", http.StatusInternalServerError)
		return
	}

	// Remove evidence directory
	dir := filepath.Join("files", tc.AssessmentID, tc.ID.Hex())
	os.RemoveAll(dir)

	w.WriteHeader(http.StatusOK)
}

// HandleDeleteEvidence deletes a single evidence file.
// DELETE /testcase/{id}/evidence/{colour}/{file}
func HandleDeleteEvidence(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	colour := chi.URLParam(r, "colour")
	filename := chi.URLParam(r, "file")
	ctx := r.Context()
	user := auth.UserFromContext(ctx)

	if colour != "red" && colour != "blue" {
		http.Error(w, "Invalid colour", http.StatusBadRequest)
		return
	}

	// Blue users cannot delete red files
	isBlue := user.HasRole(ctx, "Blue") && !user.HasRole(ctx, "Red") && !user.HasRole(ctx, "Admin")
	if isBlue && colour == "red" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tc, err := models.FindTestCase(ctx, id)
	if err != nil {
		http.Error(w, "Testcase not found", http.StatusNotFound)
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
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	*files = newFiles

	oid, _ := bson.ObjectIDFromHex(id)
	_, err = db.Col(db.ColTestCase).ReplaceOne(ctx, bson.M{"_id": oid}, tc)
	if err != nil {
		http.Error(w, "Failed to update testcase", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleFetchEvidence serves an evidence file.
// GET /testcase/{id}/evidence/{file}
func HandleFetchEvidence(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	filename := chi.URLParam(r, "file")

	tc, err := models.FindTestCase(r.Context(), id)
	if err != nil {
		http.Error(w, "Testcase not found", http.StatusNotFound)
		return
	}

	safeName := sanitizeFilenameSafe(filename)
	filePath := filepath.Join("files", tc.AssessmentID, tc.ID.Hex(), safeName)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// If download query param is present, set as attachment
	if r.URL.Query().Has("download") {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+safeName+"\"")
	}

	http.ServeFile(w, r, filePath)
}

// HandleToggleTimer starts or stops the timer on a testcase.
// GET /testcase/{id}/toggle-timer
func HandleToggleTimer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	user := auth.UserFromContext(ctx)

	// Blue-only users cannot control the timer
	isBlue := user.HasRole(ctx, "Blue") && !user.HasRole(ctx, "Red") && !user.HasRole(ctx, "Admin")
	if isBlue {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	tc, err := models.FindTestCase(ctx, id)
	if err != nil {
		http.Error(w, "Testcase not found", http.StatusNotFound)
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
	}

	tc.ModifyTime = now

	oid, _ := bson.ObjectIDFromHex(id)
	_, err = db.Col("test_case").ReplaceOne(ctx, bson.M{"_id": oid}, tc)
	if err != nil {
		http.Error(w, "Failed to update testcase", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tc.ToJSON(false))
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
