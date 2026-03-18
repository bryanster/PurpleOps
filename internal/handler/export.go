package handler

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
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

// HandleExportAssessment exports testcases as JSON or CSV.
// GET /assessment/{id}/export/{filetype}
func HandleExportAssessment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.UserFromContext(ctx)
	id := chi.URLParam(r, "id")
	filetype := chi.URLParam(r, "filetype")

	_, err := models.FindAssessment(ctx, id)
	if err != nil {
		http.Error(w, "Assessment not found", http.StatusNotFound)
		return
	}

	testcases, err := models.GetTestCases(ctx, id)
	if err != nil {
		http.Error(w, "Failed to load testcases", http.StatusInternalServerError)
		return
	}

	// Blue users only see visible testcases.
	isBlue := !user.HasRole(ctx, "Admin") && !user.HasRole(ctx, "Red")
	var records []map[string]interface{}
	for i := range testcases {
		if isBlue && !testcases[i].Visible {
			continue
		}
		records = append(records, testcases[i].ToJSON(true))
	}

	filesDir := filepath.Join("files", id)
	if err := os.MkdirAll(filesDir, 0o750); err != nil {
		http.Error(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}

	switch filetype {
	case "json":
		outPath := filepath.Join(filesDir, "export.json")
		data, err := json.MarshalIndent(records, "", "  ")
		if err != nil {
			http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(outPath, data, 0o600); err != nil {
			http.Error(w, "Failed to write file", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="export.json"`)
		w.Header().Set("Content-Type", "application/json")
		http.ServeFile(w, r, outPath)

	case "csv":
		outPath := filepath.Join(filesDir, "export.csv")
		if err := writeCSVExport(outPath, records); err != nil {
			http.Error(w, "Failed to write CSV", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="export.csv"`)
		w.Header().Set("Content-Type", "text/csv")
		http.ServeFile(w, r, outPath)

	default:
		http.Error(w, "Invalid filetype, must be json or csv", http.StatusBadRequest)
	}
}

// writeCSVExport flattens testcase JSON records into a CSV file.
func writeCSVExport(outPath string, records []map[string]interface{}) error {
	if len(records) == 0 {
		return os.WriteFile(outPath, []byte{}, 0o600)
	}

	// Collect all keys for headers.
	keySet := map[string]bool{}
	for _, rec := range records {
		for k := range rec {
			keySet[k] = true
		}
	}
	headers := make([]string, 0, len(keySet))
	for k := range keySet {
		headers = append(headers, k)
	}

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	if err := writer.Write(headers); err != nil {
		return err
	}

	for _, rec := range records {
		row := make([]string, len(headers))
		for i, h := range headers {
			row[i] = flattenValue(rec[h])
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// flattenValue converts a value to a string, joining arrays with commas.
func flattenValue(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case []string:
		return strings.Join(val, ", ")
	case []interface{}:
		strs := make([]string, 0, len(val))
		for _, item := range val {
			strs = append(strs, fmt.Sprintf("%v", item))
		}
		return strings.Join(strs, ", ")
	case bool:
		if val {
			return "true"
		}
		return "false"
	case *bool:
		if val == nil {
			return ""
		}
		if *val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// HandleExportCampaign exports campaign-relevant fields only.
// GET /assessment/{id}/export/campaign
func HandleExportCampaign(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.UserFromContext(ctx)
	id := chi.URLParam(r, "id")

	data, err := buildCampaignExport(ctx, id, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filesDir := filepath.Join("files", id)
	if err := os.MkdirAll(filesDir, 0o750); err != nil {
		http.Error(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}

	outPath := filepath.Join(filesDir, "campaign.json")
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(outPath, jsonData, 0o600); err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="campaign.json"`)
	w.Header().Set("Content-Type", "application/json")
	http.ServeFile(w, r, outPath)
}

// buildCampaignExport builds the campaign data for an assessment.
func buildCampaignExport(ctx context.Context, id string, user *models.User) ([]map[string]interface{}, error) {
	testcases, err := models.GetTestCases(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load testcases")
	}

	isBlue := !user.HasRole(ctx, "Admin") && !user.HasRole(ctx, "Red")
	campaignFields := []string{"mitreid", "tactic", "name", "objective", "actions", "tools", "uuid", "tags"}

	var records []map[string]interface{}
	for i := range testcases {
		if isBlue && !testcases[i].Visible {
			continue
		}
		full := testcases[i].ToJSON(true)
		entry := map[string]interface{}{}
		for _, field := range campaignFields {
			entry[field] = full[field]
		}
		records = append(records, entry)
	}
	return records, nil
}

// HandleExportTestcases exports testcase templates with provider field.
// GET /assessment/{id}/export/templates
func HandleExportTestcases(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.UserFromContext(ctx)
	id := chi.URLParam(r, "id")

	data, err := buildTestcaseTemplatesExport(ctx, id, user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	filesDir := filepath.Join("files", id)
	if err := os.MkdirAll(filesDir, 0o750); err != nil {
		http.Error(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}

	outPath := filepath.Join(filesDir, "testcases.json")
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
		return
	}
	if err := os.WriteFile(outPath, jsonData, 0o600); err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="testcases.json"`)
	w.Header().Set("Content-Type", "application/json")
	http.ServeFile(w, r, outPath)
}

// buildTestcaseTemplatesExport builds campaign export data with provider field added.
func buildTestcaseTemplatesExport(ctx context.Context, id string, user *models.User) ([]map[string]interface{}, error) {
	testcases, err := models.GetTestCases(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to load testcases")
	}

	isBlue := !user.HasRole(ctx, "Admin") && !user.HasRole(ctx, "Red")
	campaignFields := []string{"mitreid", "tactic", "name", "objective", "actions", "tools", "uuid", "tags"}

	var records []map[string]interface{}
	for i := range testcases {
		if isBlue && !testcases[i].Visible {
			continue
		}
		full := testcases[i].ToJSON(true)
		entry := map[string]interface{}{}
		for _, field := range campaignFields {
			entry[field] = full[field]
		}
		entry["provider"] = "???"
		records = append(records, entry)
	}
	return records, nil
}

// HandleExportReport handles report export via docx template.
// POST /assessment/{id}/export/report
// TODO: Implement docxtpl equivalent in Go. The Python version uses python-docx-template
// to render .docx reports with assessment data. No mature Go equivalent exists yet.
func HandleExportReport(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Report export not implemented", http.StatusNotImplemented)
}

// HandleExportNavigator exports and serves the MITRE ATT&CK Navigator layer.
// GET /assessment/{id}/export/navigator
func HandleExportNavigator(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	outPath, err := ExportNavigatorFile(id, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Disposition", `attachment; filename="navigator.json"`)
	w.Header().Set("Content-Type", "application/json")
	http.ServeFile(w, r, outPath)
}

// ExportNavigatorFile generates the MITRE ATT&CK Navigator layer JSON file.
func ExportNavigatorFile(id string, r *http.Request) (string, error) {
	ctx := r.Context()
	user := auth.UserFromContext(ctx)

	assessment, err := models.FindAssessment(ctx, id)
	if err != nil {
		return "", fmt.Errorf("assessment not found")
	}

	testcases, err := models.GetTestCases(ctx, id)
	if err != nil {
		return "", fmt.Errorf("failed to load testcases")
	}

	isBlue := user != nil && !user.HasRole(ctx, "Admin") && !user.HasRole(ctx, "Red")

	// Load all techniques from the DB.
	var techniques []models.Technique
	cursor, err := db.Col("technique").Find(ctx, bson.M{})
	if err != nil {
		return "", fmt.Errorf("failed to load techniques")
	}
	if err := cursor.All(ctx, &techniques); err != nil {
		return "", fmt.Errorf("failed to decode techniques")
	}

	// Build technique entries for the navigator layer.
	var navTechniques []map[string]interface{}
	for _, tech := range techniques {
		// Find testcases matching this technique's mitreid.
		var matching []models.TestCase
		for i := range testcases {
			if testcases[i].MitreID == tech.MitreID {
				if isBlue && !testcases[i].Visible {
					continue
				}
				matching = append(matching, testcases[i])
			}
		}

		if len(matching) == 0 {
			continue
		}

		// Calculate score: (Prevented*3 + Alerted*2 + Logged*1) / (count*3) * 100
		totalScore := 0
		for _, tc := range matching {
			if tc.Prevented == "Yes" {
				totalScore += 3
			}
			if tc.Alerted != nil && *tc.Alerted {
				totalScore += 2
			}
			if tc.Logged != nil && *tc.Logged {
				totalScore += 1
			}
		}
		score := float64(totalScore) / float64(len(matching)*3) * 100

		// Add an entry for each tactic this technique belongs to.
		for _, tactic := range tech.Tactics {
			entry := map[string]interface{}{
				"techniqueID": tech.MitreID,
				"tactic":      tactic,
				"score":       score,
				"enabled":     true,
			}
			navTechniques = append(navTechniques, entry)
		}
	}

	layer := map[string]interface{}{
		"name":     assessment.Name,
		"versions": map[string]string{"layer": "4.5"},
		"domain":   "enterprise-attack",
		"sorting":  3,
		"layout": map[string]interface{}{
			"layout":              "flat",
			"aggregateFunction":   "average",
			"showID":              true,
			"showName":            true,
			"showAggregateScores": true,
			"countUnscored":       false,
		},
		"hideDisabled": false,
		"techniques":   navTechniques,
		"gradient": map[string]interface{}{
			"colors":   []string{"#ff6666ff", "#ffe766ff", "#8ec843ff"},
			"minValue": 0,
			"maxValue": 100,
		},
		"showTacticRowBackground":       true,
		"tacticRowBackground":           "#593196",
		"selectTechniquesAcrossTactics": true,
		"selectSubtechniquesWithParent": false,
	}

	filesDir := filepath.Join("files", id)
	if err := os.MkdirAll(filesDir, 0o750); err != nil {
		return "", fmt.Errorf("failed to create directory")
	}

	outPath := filepath.Join(filesDir, "navigator.json")
	data, err := json.MarshalIndent(layer, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to encode navigator JSON")
	}
	if err := os.WriteFile(outPath, data, 0o600); err != nil {
		return "", fmt.Errorf("failed to write navigator file")
	}

	return outPath, nil
}

// HandleExportEntire exports the full assessment as a ZIP archive.
// GET /assessment/{id}/export/entire
func HandleExportEntire(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.UserFromContext(ctx)
	id := chi.URLParam(r, "id")

	assessment, err := models.FindAssessment(ctx, id)
	if err != nil {
		http.Error(w, "Assessment not found", http.StatusNotFound)
		return
	}

	testcases, err := models.GetTestCases(ctx, id)
	if err != nil {
		http.Error(w, "Failed to load testcases", http.StatusInternalServerError)
		return
	}

	isBlue := !user.HasRole(ctx, "Admin") && !user.HasRole(ctx, "Red")

	filesDir := filepath.Join("files", id)
	if err := os.MkdirAll(filesDir, 0o750); err != nil {
		http.Error(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}

	// Run CSV export.
	var csvRecords []map[string]interface{}
	for i := range testcases {
		if isBlue && !testcases[i].Visible {
			continue
		}
		csvRecords = append(csvRecords, testcases[i].ToJSON(true))
	}
	csvPath := filepath.Join(filesDir, "export.csv")
	if err := writeCSVExport(csvPath, csvRecords); err != nil {
		http.Error(w, "Failed to write CSV", http.StatusInternalServerError)
		return
	}

	// Run testcase templates export.
	templateRecords, err := buildTestcaseTemplatesExport(ctx, id, user)
	if err != nil {
		http.Error(w, "Failed to build testcase templates", http.StatusInternalServerError)
		return
	}
	testcasesPath := filepath.Join(filesDir, "testcases.json")
	testcasesData, _ := json.MarshalIndent(templateRecords, "", "  ")
	if err := os.WriteFile(testcasesPath, testcasesData, 0o600); err != nil {
		http.Error(w, "Failed to write testcases export", http.StatusInternalServerError)
		return
	}

	// Run navigator export.
	if _, err := ExportNavigatorFile(id, r); err != nil {
		http.Error(w, "Failed to export navigator", http.StatusInternalServerError)
		return
	}

	// Write meta.json with assessment data.
	metaPath := filepath.Join(filesDir, "meta.json")
	metaData, _ := json.MarshalIndent(assessment.ToJSON(true), "", "  ")
	if err := os.WriteFile(metaPath, metaData, 0o600); err != nil {
		http.Error(w, "Failed to write meta export", http.StatusInternalServerError)
		return
	}

	// Create ZIP.
	var zipDir string
	if isBlue {
		// Copy to tmp dir and remove non-visible testcase evidence directories.
		tmpDir, err := os.MkdirTemp("", "purpleops-export-*")
		if err != nil {
			http.Error(w, "Failed to create temp directory", http.StatusInternalServerError)
			return
		}
		defer os.RemoveAll(tmpDir)

		if err := copyDir(filesDir, tmpDir); err != nil {
			http.Error(w, "Failed to copy files", http.StatusInternalServerError)
			return
		}

		// Remove evidence directories for non-visible testcases.
		for i := range testcases {
			if !testcases[i].Visible {
				tcDir := filepath.Join(tmpDir, testcases[i].ID.Hex())
				os.RemoveAll(tcDir)
			}
		}
		zipDir = tmpDir
	} else {
		zipDir = filesDir
	}

	zipPath := filepath.Join(os.TempDir(), assessment.Name+".zip")
	if err := createZip(zipPath, zipDir); err != nil {
		http.Error(w, "Failed to create ZIP", http.StatusInternalServerError)
		return
	}
	defer os.Remove(zipPath)

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, assessment.Name))
	w.Header().Set("Content-Type", "application/zip")
	http.ServeFile(w, r, zipPath)
}

// createZip creates a ZIP archive of the given directory.
func createZip(zipPath, sourceDir string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		writer, err := zw.Create(relPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(writer, file)
		return err
	})
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0o750)
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return err
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		return err
	})
}
