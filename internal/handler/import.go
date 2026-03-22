package handler

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bryanster/purpleops/internal/db"
	"github.com/bryanster/purpleops/internal/models"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleImportTemplate creates testcases from testcase templates.
// POST /assessment/:id/import/template
func HandleImportTemplate(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	_, err := models.FindAssessment(ctx, id)
	if err != nil {
		c.String(http.StatusNotFound, "Assessment not found")
		return
	}

	var body struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&body); err != nil {
		c.String(http.StatusBadRequest, "Invalid JSON body")
		return
	}

	var results []map[string]interface{}
	for _, templateID := range body.IDs {
		oid, err := bson.ObjectIDFromHex(templateID)
		if err != nil {
			slog.Warn("import template: invalid ObjectID", "id", templateID, "err", err)
			continue
		}

		var tmpl models.TestCaseTemplate
		if err := db.Col(db.ColTestCaseTemplate).FindOne(ctx, bson.M{"_id": oid}).Decode(&tmpl); err != nil {
			slog.Warn("import template: template not found", "id", templateID, "err", err)
			continue
		}

		// If the template has no tactic, look it up from the technique.
		tactic := tmpl.Tactic
		if tactic == "" && tmpl.MitreID != "" {
			tactic = lookupTacticForTechnique(ctx, tmpl.MitreID)
		}

		tc := models.TestCase{
			ID:           bson.NewObjectID(),
			AssessmentID: id,
			Name:         tmpl.Name,
			MitreID:      tmpl.MitreID,
			Tactic:       tactic,
			Objective:    tmpl.Objective,
			Actions:      tmpl.Actions,
			RedNotes:     tmpl.RedNotes,
			UUID:         tmpl.UUID,
			Visible:      true,
			ModifyTime:   models.NowPtr(),
		}

		if _, err := db.Col(db.ColTestCase).InsertOne(ctx, &tc); err != nil {
			slog.Error("import template: failed to insert testcase", "name", tmpl.Name, "err", err)
			continue
		}

		// Create evidence directory for this testcase.
		tcDir := filepath.Join("files", id, tc.ID.Hex())
		if err := os.MkdirAll(tcDir, DirPerm); err != nil {
			slog.Warn("import template: failed to create evidence dir", "path", tcDir, "err", err)
		}

		results = append(results, tc.ToJSON(false))
	}

	c.JSON(http.StatusOK, results)
}

// HandleImportNavigator imports testcases from a MITRE ATT&CK Navigator layer JSON file.
// POST /assessment/:id/import/navigator
func HandleImportNavigator(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	_, err := models.FindAssessment(ctx, id)
	if err != nil {
		c.String(http.StatusNotFound, "Assessment not found")
		return
	}

	// Parse the uploaded file.
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, "No file uploaded")
		return
	}
	defer file.Close()

	var layer struct {
		Techniques []struct {
			TechniqueID string `json:"techniqueID"`
			Tactic      string `json:"tactic"`
		} `json:"techniques"`
	}
	if err := json.NewDecoder(file).Decode(&layer); err != nil {
		c.String(http.StatusBadRequest, "Invalid navigator JSON")
		return
	}

	var results []map[string]interface{}
	for _, entry := range layer.Techniques {
		// Convert tactic from lowercase-hyphenated to Title Case.
		// e.g., "credential-access" -> "Credential Access"
		tacticTitle := toTitleCase(entry.Tactic)

		// Try to find a matching template.
		// Try to find a matching template (exact match on mitreid + tactic first,
		// then fall back to mitreid-only match).
		var tmpl models.TestCaseTemplate
		err := db.Col(db.ColTestCaseTemplate).FindOne(ctx, bson.M{
			"mitreid": entry.TechniqueID,
			"tactic":  tacticTitle,
		}).Decode(&tmpl)
		if err != nil {
			// Try mitreid-only match (e.g., ART templates have no tactic).
			_ = db.Col(db.ColTestCaseTemplate).FindOne(ctx, bson.M{
				"mitreid": entry.TechniqueID,
			}).Decode(&tmpl)
		}

		var tc models.TestCase
		if tmpl.Name != "" {
			// Create testcase from template, always using the navigator's tactic.
			tactic := tmpl.Tactic
			if tactic == "" {
				tactic = tacticTitle
			}
			tc = models.TestCase{
				ID:           bson.NewObjectID(),
				AssessmentID: id,
				Name:         tmpl.Name,
				MitreID:      tmpl.MitreID,
				Tactic:       tactic,
				Objective:    tmpl.Objective,
				Actions:      tmpl.Actions,
				RedNotes:     tmpl.RedNotes,
				UUID:         tmpl.UUID,
				Visible:      true,
				ModifyTime:   models.NowPtr(),
			}
		} else {
			// No template found. Look up the Technique name from the techniques collection.
			var tech models.Technique
			techErr := db.Col(db.ColTechnique).FindOne(ctx, bson.M{"mitreid": entry.TechniqueID}).Decode(&tech)
			name := entry.TechniqueID
			if techErr == nil {
				name = tech.Name
			}

			tc = models.TestCase{
				ID:           bson.NewObjectID(),
				AssessmentID: id,
				Name:         name,
				MitreID:      entry.TechniqueID,
				Tactic:       tacticTitle,
				Visible:      true,
				ModifyTime:   models.NowPtr(),
			}
		}

		if _, err := db.Col(db.ColTestCase).InsertOne(ctx, &tc); err != nil {
			slog.Error("import navigator: failed to insert testcase", "mitreid", entry.TechniqueID, "err", err)
			continue
		}

		tcDir := filepath.Join("files", id, tc.ID.Hex())
		if err := os.MkdirAll(tcDir, DirPerm); err != nil {
			slog.Warn("import navigator: failed to create evidence dir", "path", tcDir, "err", err)
		}

		results = append(results, tc.ToJSON(false))
	}

	c.JSON(http.StatusOK, results)
}

// toTitleCase converts a hyphen-separated lowercase string to Title Case.
// e.g., "credential-access" -> "Credential Access"
func toTitleCase(s string) string {
	parts := strings.Split(s, "-")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

// HandleImportCampaign imports testcases from a campaign JSON file.
// POST /assessment/:id/import/campaign
func HandleImportCampaign(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	assessment, err := models.FindAssessment(ctx, id)
	if err != nil {
		c.String(http.StatusNotFound, "Assessment not found")
		return
	}

	// Parse the uploaded file.
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, "No file uploaded")
		return
	}
	defer file.Close()

	var entries []map[string]interface{}
	if err := json.NewDecoder(file).Decode(&entries); err != nil {
		c.String(http.StatusBadRequest, "Invalid campaign JSON")
		return
	}

	var results []map[string]interface{}
	for _, entry := range entries {
		tc := models.TestCase{
			ID:           bson.NewObjectID(),
			AssessmentID: id,
			Visible:      true,
			ModifyTime:   models.NowPtr(),
		}

		if v, ok := entry["name"].(string); ok {
			tc.Name = v
		}
		if v, ok := entry["mitreid"].(string); ok {
			tc.MitreID = v
		}
		if v, ok := entry["tactic"].(string); ok {
			tc.Tactic = v
		}
		if v, ok := entry["objective"].(string); ok {
			tc.Objective = v
		}
		if v, ok := entry["actions"].(string); ok {
			tc.Actions = v
		}
		if v, ok := entry["uuid"].(string); ok {
			tc.UUID = v
		}

		// Handle "tools" - match by name against assessment's existing tools or create new ones.
		if toolsRaw, ok := entry["tools"]; ok {
			tc.Tools = resolveMultiField(ctx, assessment, "tools", toolsRaw)
		}

		// Handle "tags" - match by name against assessment's existing tags or create new ones.
		if tagsRaw, ok := entry["tags"]; ok {
			tc.Tags = resolveMultiField(ctx, assessment, "tags", tagsRaw)
		}

		if _, err := db.Col(db.ColTestCase).InsertOne(ctx, &tc); err != nil {
			slog.Error("import campaign: failed to insert testcase", "name", tc.Name, "err", err)
			continue
		}

		tcDir := filepath.Join("files", id, tc.ID.Hex())
		if err := os.MkdirAll(tcDir, DirPerm); err != nil {
			slog.Warn("import campaign: failed to create evidence dir", "path", tcDir, "err", err)
		}

		results = append(results, tc.ToJSON(false))
	}

	c.JSON(http.StatusOK, results)
}

// resolveMultiField resolves tool/tag names to IDs, creating new entries on the assessment if needed.
func resolveMultiField(ctx context.Context, assessment *models.Assessment, field string, raw interface{}) []string {
	var names []string
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				// Handle "name|description" format from exports.
				parts := strings.SplitN(s, "|", 2)
				names = append(names, parts[0])
			}
		}
	case []string:
		for _, s := range v {
			parts := strings.SplitN(s, "|", 2)
			names = append(names, parts[0])
		}
	}

	var ids []string
	for _, name := range names {
		existingID := findExistingMultiEntry(assessment, field, name)
		if existingID != "" {
			ids = append(ids, existingID)
			continue
		}
		newID := bson.NewObjectID()
		pushField(ctx, assessment, field, newID, name, "")
		ids = append(ids, newID.Hex())
	}
	return ids
}

// HandleImportEntire imports a full assessment from a ZIP archive.
// POST /assessment/import/entire
func HandleImportEntire(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse the uploaded ZIP file.
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, "No file uploaded")
		return
	}
	defer file.Close()

	// Save uploaded file to a temporary location.
	tmpFile, err := os.CreateTemp("", "purpleops-import-*.zip")
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to create temp file")
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, file); err != nil {
		c.String(http.StatusInternalServerError, "Failed to save uploaded file")
		return
	}

	// Open the ZIP.
	zipReader, err := zip.OpenReader(tmpFile.Name())
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ZIP file")
		return
	}
	defer zipReader.Close()

	// Create a new assessment.
	newAssessmentID := bson.NewObjectID()
	filesDir := filepath.Join("files", newAssessmentID.Hex())
	tmpExtractDir := filepath.Join(filesDir, "tmp")
	if err := os.MkdirAll(tmpExtractDir, DirPerm); err != nil {
		c.String(http.StatusInternalServerError, "Failed to create directory")
		return
	}

	// Extract ZIP contents safely.
	for _, f := range zipReader.File {
		// Sanitize filename to prevent path traversal.
		safeName := sanitizeFilename(f.Name)
		destPath := filepath.Join(tmpExtractDir, safeName)

		if f.FileInfo().IsDir() {
			os.MkdirAll(destPath, DirPerm)
			continue
		}

		// Ensure parent directory exists.
		os.MkdirAll(filepath.Dir(destPath), DirPerm)

		rc, err := f.Open()
		if err != nil {
			continue
		}

		outFile, err := os.Create(destPath)
		if err != nil {
			rc.Close()
			continue
		}
		io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
	}

	// Read meta.json for assessment name/description.
	metaPath := filepath.Join(tmpExtractDir, "meta.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		c.String(http.StatusBadRequest, "meta.json not found in archive")
		return
	}

	var meta map[string]interface{}
	if err := json.Unmarshal(metaData, &meta); err != nil {
		c.String(http.StatusBadRequest, "Invalid meta.json")
		return
	}

	assessmentName, _ := meta["name"].(string)
	assessmentDesc, _ := meta["description"].(string)
	if assessmentName == "" {
		assessmentName = "Imported Assessment"
	}

	assessment := models.Assessment{
		ID:          newAssessmentID,
		Name:        assessmentName,
		Description: assessmentDesc,
		Created:     models.NowPtr(),
	}

	if _, err := db.Col(db.ColAssessment).InsertOne(ctx, &assessment); err != nil {
		c.String(http.StatusInternalServerError, "Failed to create assessment")
		return
	}

	// Read export.json for testcases.
	exportPath := filepath.Join(tmpExtractDir, "export.json")
	exportData, err := os.ReadFile(exportPath)
	if err != nil {
		// No testcases to import, return the assessment.
		c.JSON(http.StatusOK, assessment.ToJSON(false))
		return
	}

	var testcaseEntries []map[string]interface{}
	if err := json.Unmarshal(exportData, &testcaseEntries); err != nil {
		c.String(http.StatusBadRequest, "Invalid export.json")
		return
	}

	var results []map[string]interface{}
	for _, entry := range testcaseEntries {
		oldID, _ := entry["id"].(string)

		tc := models.TestCase{
			ID:           bson.NewObjectID(),
			AssessmentID: newAssessmentID.Hex(),
			State:        "Pending",
			Visible:      true,
			ModifyTime:   models.NowPtr(),
		}

		// Set string fields.
		if v, ok := entry["name"].(string); ok {
			tc.Name = v
		}
		if v, ok := entry["mitreid"].(string); ok {
			tc.MitreID = v
		}
		if v, ok := entry["tactic"].(string); ok {
			tc.Tactic = v
		}
		if v, ok := entry["objective"].(string); ok {
			tc.Objective = v
		}
		if v, ok := entry["actions"].(string); ok {
			tc.Actions = v
		}
		if v, ok := entry["rednotes"].(string); ok {
			tc.RedNotes = v
		}
		if v, ok := entry["bluenotes"].(string); ok {
			tc.BlueNotes = v
		}
		if v, ok := entry["uuid"].(string); ok {
			tc.UUID = v
		}
		if v, ok := entry["state"].(string); ok {
			tc.State = v
		}
		if v, ok := entry["prevented"].(string); ok {
			tc.Prevented = v
		}
		if v, ok := entry["preventedrating"].(string); ok {
			tc.PreventedRating = v
		}
		if v, ok := entry["alertseverity"].(string); ok {
			tc.AlertSeverity = v
		}
		if v, ok := entry["detectionrating"].(string); ok {
			tc.DetectionRating = v
		}
		if v, ok := entry["priority"].(string); ok {
			tc.Priority = v
		}
		if v, ok := entry["priorityurgency"].(string); ok {
			tc.PriorityUrgency = v
		}
		if v, ok := entry["outcome"].(string); ok {
			tc.Outcome = v
		}
		if v, ok := entry["detectionsource"].(string); ok {
			tc.DetectionSource = v
		}
		if v, ok := entry["preventionsource"].(string); ok {
			tc.PreventionSource = v
		}
		if v, ok := entry["alerted"].(bool); ok {
			tc.Alerted = &v
		}
		if v, ok := entry["logged"].(bool); ok {
			tc.Logged = &v
		}
		if v, ok := entry["visible"].(bool); ok {
			tc.Visible = v
		}

		// Rebuild embedded docs (sources, targets, tools, controls, tags).
		for _, field := range []string{"sources", "targets", "tools", "controls", "tags", "datasources", "rules"} {
			if rawVal, ok := entry[field]; ok {
				ids := rebuildMultiField(ctx, &assessment, field, rawVal)
				switch field {
				case "sources":
					tc.Sources = ids
				case "targets":
					tc.Targets = ids
				case "tools":
					tc.Tools = ids
				case "controls":
					tc.Controls = ids
				case "tags":
					tc.Tags = ids
				case "datasources":
					tc.Datasources = ids
				case "rules":
					tc.Rules = ids
				}
			}
		}

		if _, err := db.Col(db.ColTestCase).InsertOne(ctx, &tc); err != nil {
			slog.Error("import entire: failed to insert testcase", "name", tc.Name, "err", err)
			continue
		}

		// Copy evidence files from old testcase directory to new one.
		newTCDir := filepath.Join(filesDir, tc.ID.Hex())
		if err := os.MkdirAll(newTCDir, DirPerm); err != nil {
			slog.Warn("import entire: failed to create evidence dir", "path", newTCDir, "err", err)
		}

		if oldID != "" {
			oldTCDir := filepath.Join(tmpExtractDir, oldID)
			if info, err := os.Stat(oldTCDir); err == nil && info.IsDir() {
				if err := copyDir(oldTCDir, newTCDir); err != nil {
					log.Printf("warning: failed to copy evidence directory %s: %v", oldTCDir, err)
				}
			}
		}

		results = append(results, tc.ToJSON(false))
	}

	// Clean up the tmp extraction directory.
	os.RemoveAll(tmpExtractDir)

	c.JSON(http.StatusOK, results)
}

// rebuildMultiField recreates embedded document references on the assessment for imported testcases.
func rebuildMultiField(ctx context.Context, assessment *models.Assessment, field string, rawVal interface{}) []string {
	var nameDescPairs []string
	switch v := rawVal.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				nameDescPairs = append(nameDescPairs, s)
			}
		}
	case []string:
		nameDescPairs = v
	}

	var ids []string
	for _, pair := range nameDescPairs {
		parts := strings.SplitN(pair, "|", 2)
		name := parts[0]
		desc := ""
		if len(parts) > 1 {
			desc = parts[1]
		}

		// Check if this name already exists on the assessment.
		existingID := findExistingMultiEntry(assessment, field, name)
		if existingID != "" {
			ids = append(ids, existingID)
			continue
		}

		// Create a new entry on the assessment and persist it.
		newID := bson.NewObjectID()
		pushField(ctx, assessment, field, newID, name, desc)
		ids = append(ids, newID.Hex())
	}
	return ids
}

// pushField creates a new embedded document entry on the assessment and persists it to the DB.
// For tags, desc is treated as the colour value.
func pushField(ctx context.Context, assessment *models.Assessment, field string, newID bson.ObjectID, name, desc string) {
	var entry interface{}
	switch field {
	case "sources":
		e := models.Source{ID: newID, Name: name, Description: desc}
		assessment.Sources = append(assessment.Sources, e)
		entry = e
	case "targets":
		e := models.Target{ID: newID, Name: name, Description: desc}
		assessment.Targets = append(assessment.Targets, e)
		entry = e
	case "tools":
		e := models.Tool{ID: newID, Name: name, Description: desc}
		assessment.Tools = append(assessment.Tools, e)
		entry = e
	case "controls":
		e := models.Control{ID: newID, Name: name, Description: desc}
		assessment.Controls = append(assessment.Controls, e)
		entry = e
	case "tags":
		e := models.Tag{ID: newID, Name: name, Colour: desc}
		assessment.Tags = append(assessment.Tags, e)
		entry = e
	case "datasources":
		e := models.Datasource{ID: newID, Name: name, Description: desc}
		assessment.Datasources = append(assessment.Datasources, e)
		entry = e
	case "rules":
		e := models.DetectionRule{ID: newID, Name: name, Description: desc}
		assessment.Rules = append(assessment.Rules, e)
		entry = e
	default:
		return
	}
	db.Col(db.ColAssessment).UpdateOne(ctx, bson.M{"_id": assessment.ID}, bson.M{
		"$push": bson.M{field: entry},
	})
}

// findByName searches a slice of named items for one matching the given name and returns its hex ID.
func findByName[T interface {
	GetID() bson.ObjectID
	GetName() string
}](items []T, name string) string {
	for _, item := range items {
		if item.GetName() == name {
			return item.GetID().Hex()
		}
	}
	return ""
}

// findExistingMultiEntry checks if a name already exists in the assessment's embedded docs.
func findExistingMultiEntry(assessment *models.Assessment, field, name string) string {
	switch field {
	case "sources":
		return findByName(assessment.Sources, name)
	case "targets":
		return findByName(assessment.Targets, name)
	case "tools":
		return findByName(assessment.Tools, name)
	case "controls":
		return findByName(assessment.Controls, name)
	case "tags":
		return findByName(assessment.Tags, name)
	case "datasources":
		return findByName(assessment.Datasources, name)
	case "rules":
		return findByName(assessment.Rules, name)
	}
	return ""
}

// lookupTacticForTechnique finds the first tactic name for a given MitreID
// by looking up the technique in the database.
func lookupTacticForTechnique(ctx context.Context, mitreID string) string {
	var tech models.Technique
	if err := db.Col(db.ColTechnique).FindOne(ctx, bson.M{"mitreid": mitreID}).Decode(&tech); err != nil {
		return ""
	}
	if len(tech.Tactics) > 0 {
		return tech.Tactics[0]
	}
	return ""
}

// sanitizeFilename replaces path separators with underscores to prevent path traversal.
func sanitizeFilename(name string) string {
	// Clean the path first.
	name = filepath.Clean(name)
	// Remove any leading path separators or ".." components.
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimPrefix(name, "\\")
	// Replace any remaining ".." with underscores.
	for strings.Contains(name, "..") {
		name = strings.ReplaceAll(name, "..", "_")
	}
	return name
}
