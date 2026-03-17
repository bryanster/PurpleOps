package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/db"
	"github.com/bryanster/purpleops/internal/models"
	"github.com/bryanster/purpleops/internal/render"
	"github.com/flosch/pongo2/v6"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleNewTestCase creates a new testcase for the given assessment.
// POST /testcase/{id}/single
func HandleNewTestCase(w http.ResponseWriter, r *http.Request) {
	assessmentID := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	tc := models.TestCase{
		ID:           bson.NewObjectID(),
		AssessmentID: assessmentID,
		Name:         r.FormValue("name"),
		MitreID:      r.FormValue("mitreid"),
		Tactic:       r.FormValue("tactic"),
		State:        "Pending",
		Visible:      true,
	}

	_, err := db.Col("test_case").InsertOne(r.Context(), &tc)
	if err != nil {
		http.Error(w, "Failed to create testcase", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tc.ToJSON(false))
}

// HandleLoadTestCase loads and renders a single testcase page.
// GET /testcase/{id}
func HandleLoadTestCase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	user := auth.UserFromContext(ctx)

	tc, err := models.FindTestCase(ctx, id)
	if err != nil {
		http.Error(w, "Testcase not found", http.StatusNotFound)
		return
	}

	assessment, err := models.FindAssessment(ctx, tc.AssessmentID)
	if err != nil {
		http.Error(w, "Assessment not found", http.StatusNotFound)
		return
	}

	// Blue users cannot see non-visible testcases
	if !tc.Visible && user.HasRole(ctx, "Blue") && !user.HasRole(ctx, "Red") && !user.HasRole(ctx, "Admin") {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// All testcases for this assessment
	testcases, _ := models.GetTestCases(ctx, tc.AssessmentID)

	// All tactics
	var tactics []models.Tactic
	tacticCursor, err := db.Col("tactic").Find(ctx, bson.M{})
	if err == nil {
		tacticCursor.All(ctx, &tactics)
	}

	// KnowledgeBase matching mitreid
	var kb *models.KnowledgeBase
	if tc.MitreID != "" {
		var kbDoc models.KnowledgeBase
		if err := db.Col("knowledge_base").FindOne(ctx, bson.M{"mitreid": tc.MitreID}).Decode(&kbDoc); err == nil {
			kb = &kbDoc
		}
	}

	// TestCaseTemplates matching mitreid
	var templates []models.TestCaseTemplate
	if tc.MitreID != "" {
		tplCursor, err := db.Col("testcase_template").Find(ctx, bson.M{"mitreid": tc.MitreID})
		if err == nil {
			tplCursor.All(ctx, &templates)
		}
	}

	// All techniques
	var techniques []models.Technique
	techCursor, err := db.Col("technique").Find(ctx, bson.M{})
	if err == nil {
		techCursor.All(ctx, &techniques)
	}
	sort.Slice(techniques, func(i, j int) bool {
		return techniques[i].MitreID < techniques[j].MitreID
	})

	// Sigma docs matching mitreid
	var sigmas []models.Sigma
	if tc.MitreID != "" {
		sigmaCursor, err := db.Col("sigma").Find(ctx, bson.M{"mitreid": tc.MitreID})
		if err == nil {
			sigmaCursor.All(ctx, &sigmas)
		}
	}

	// Multi fields from assessment
	multi := map[string]interface{}{
		"sources":           assessment.MultiToJSON("sources", true),
		"targets":           assessment.MultiToJSON("targets", true),
		"tools":             assessment.MultiToJSON("tools", true),
		"controls":          assessment.MultiToJSON("controls", true),
		"tags":              assessment.MultiToJSON("tags", true),
		"datasources":       assessment.MultiToJSON("datasources", true),
		"rules":             assessment.MultiToJSON("rules", true),
		"detectionsources":  assessment.MultiToJSON("detectionsources", true),
		"preventionsources": assessment.MultiToJSON("preventionsources", true),
	}

	render.Render(w, r, "testcase.html", pongo2.Context{
		"testcase":   tc,
		"assessment": assessment,
		"testcases":  testcases,
		"tactics":    tactics,
		"kb":         kb,
		"templates":  templates,
		"mitres":     techniques,
		"sigmas":     sigmas,
		"multi":      multi,
	})
}

// HandleSaveTestCase saves changes to an existing testcase.
// POST /testcase/{id}
func HandleSaveTestCase(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	user := auth.UserFromContext(ctx)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	tc, err := models.FindTestCase(ctx, id)
	if err != nil {
		http.Error(w, "Testcase not found", http.StatusNotFound)
		return
	}

	assessment, err := models.FindAssessment(ctx, tc.AssessmentID)
	if err != nil {
		http.Error(w, "Assessment not found", http.StatusNotFound)
		return
	}

	isBlue := user.HasRole(ctx, "Blue") && !user.HasRole(ctx, "Red") && !user.HasRole(ctx, "Admin")

	// Direct fields
	if isBlue {
		// Blue users can only edit a subset of fields
		applyFormField(r, "bluenotes", &tc.BlueNotes)
		applyFormField(r, "prevented", &tc.Prevented)
		applyFormField(r, "alertseverity", &tc.AlertSeverity)
		applyFormField(r, "detectionsource", &tc.DetectionSource)
		applyFormField(r, "preventionsource", &tc.PreventionSource)

		// Bool fields for blue
		applyFormBool(r, "alerted", &tc.Alerted)
		applyFormBool(r, "logged", &tc.Logged)
	} else {
		// Red/Admin can edit all fields
		applyFormField(r, "name", &tc.Name)
		applyFormField(r, "objective", &tc.Objective)
		applyFormField(r, "actions", &tc.Actions)
		applyFormField(r, "rednotes", &tc.RedNotes)
		applyFormField(r, "bluenotes", &tc.BlueNotes)
		applyFormField(r, "uuid", &tc.UUID)
		applyFormField(r, "mitreid", &tc.MitreID)
		applyFormField(r, "tactic", &tc.Tactic)
		applyFormField(r, "state", &tc.State)
		applyFormField(r, "prevented", &tc.Prevented)
		applyFormField(r, "preventedrating", &tc.PreventedRating)
		applyFormField(r, "alertseverity", &tc.AlertSeverity)
		applyFormField(r, "detectionrating", &tc.DetectionRating)
		applyFormField(r, "priority", &tc.Priority)
		applyFormField(r, "priorityurgency", &tc.PriorityUrgency)
		applyFormField(r, "detectionsource", &tc.DetectionSource)
		applyFormField(r, "preventionsource", &tc.PreventionSource)

		// Bool fields
		applyFormBool(r, "alerted", &tc.Alerted)
		applyFormBool(r, "logged", &tc.Logged)

		// List fields
		tc.Sources = r.Form["sources"]
		tc.Targets = r.Form["targets"]
		tc.Tools = r.Form["tools"]
		tc.Controls = r.Form["controls"]
		tc.Tags = r.Form["tags"]
		tc.Datasources = r.Form["datasources"]
		tc.Rules = r.Form["rules"]

		// Ensure nil slices become empty slices for consistent behavior
		if tc.Sources == nil {
			tc.Sources = []string{}
		}
		if tc.Targets == nil {
			tc.Targets = []string{}
		}
		if tc.Tools == nil {
			tc.Tools = []string{}
		}
		if tc.Controls == nil {
			tc.Controls = []string{}
		}
		if tc.Tags == nil {
			tc.Tags = []string{}
		}
		if tc.Datasources == nil {
			tc.Datasources = []string{}
		}
		if tc.Rules == nil {
			tc.Rules = []string{}
		}

		// Time fields
		applyFormTime(r, "starttime", &tc.StartTime)
		applyFormTime(r, "endtime", &tc.EndTime)

		// Visible bool field (checkbox with hidden fallback)
		applyFormBoolVisible(r, "visible", &tc.Visible)

		// File fields
		processFiles(r, tc, assessment, "red")
		processFiles(r, tc, assessment, "blue")
	}

	// Set modify time
	tc.ModifyTime = models.NowPtr()

	// If logged is "Yes" and detecttime is nil, set detecttime
	loggedVal := r.FormValue("logged")
	if strings.EqualFold(loggedVal, "yes") && tc.DetectTime == nil {
		tc.DetectTime = models.NowPtr()
	}

	// Compute outcome
	tc.Outcome = computeOutcome(tc)

	// Sanity check: validate list field IDs against assessment's embedded docs
	if !isBlue {
		tc.Sources = sanitizeIDs(tc.Sources, extractIDs(assessment.Sources))
		tc.Targets = sanitizeIDs(tc.Targets, extractTargetIDs(assessment.Targets))
		tc.Tools = sanitizeIDs(tc.Tools, extractToolIDs(assessment.Tools))
		tc.Controls = sanitizeIDs(tc.Controls, extractControlIDs(assessment.Controls))
		tc.Tags = sanitizeIDs(tc.Tags, extractTagIDs(assessment.Tags))
		tc.Datasources = sanitizeIDs(tc.Datasources, extractDatasourceIDs(assessment.Datasources))
		tc.Rules = sanitizeIDs(tc.Rules, extractRuleIDs(assessment.Rules))
	}

	// Save
	oid, _ := bson.ObjectIDFromHex(id)
	_, err = db.Col("test_case").ReplaceOne(ctx, bson.M{"_id": oid}, tc)
	if err != nil {
		http.Error(w, "Failed to save testcase", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// --- Helper functions ---

func applyFormField(r *http.Request, field string, target *string) {
	if v := r.FormValue(field); v != "" || r.Form.Has(field) {
		*target = v
	}
}

func applyFormBool(r *http.Request, field string, target **bool) {
	if v := r.FormValue(field); v != "" {
		b := parseBool(v)
		*target = &b
	}
}

func applyFormBoolVisible(r *http.Request, field string, target *bool) {
	if v := r.FormValue(field); v != "" {
		*target = parseBool(v)
	}
}

func parseBool(v string) bool {
	v = strings.ToLower(v)
	return v == "true" || v == "yes" || v == "on"
}

func applyFormTime(r *http.Request, field string, target **time.Time) {
	v := r.FormValue(field)
	if v == "" {
		return
	}
	t, err := time.Parse("2006-01-02T15:04", v)
	if err != nil {
		return
	}
	// Apply timezone offset from form (in minutes)
	if tzStr := r.FormValue("timezone"); tzStr != "" {
		if offset, err := strconv.Atoi(tzStr); err == nil {
			t = t.Add(time.Duration(offset) * time.Minute)
		}
	}
	tUTC := t.UTC()
	*target = &tUTC
}

func computeOutcome(tc *models.TestCase) string {
	if tc.Prevented == "Yes" || tc.Prevented == "Partial" {
		return "Prevented"
	}
	if tc.Alerted != nil && *tc.Alerted {
		return "Alerted"
	}
	if tc.Logged != nil && *tc.Logged {
		return "Logged"
	}
	if (tc.Logged != nil && !*tc.Logged) && tc.Prevented != "" {
		return "Missed"
	}
	return ""
}

func processFiles(r *http.Request, tc *models.TestCase, assessment *models.Assessment, colour string) {
	formField := colour + "files"
	var existingFiles *[]models.FileDoc
	if colour == "red" {
		existingFiles = &tc.RedFiles
	} else {
		existingFiles = &tc.BlueFiles
	}

	// Update captions for existing image files
	for i, f := range *existingFiles {
		ext := strings.ToLower(filepath.Ext(f.Name))
		if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
			captionKey := strings.ToUpper(colour) + f.Name
			if caption := r.FormValue(captionKey); caption != "" || r.Form.Has(captionKey) {
				(*existingFiles)[i].Caption = caption
			}
		}
	}

	// Handle new uploaded files
	if r.MultipartForm == nil {
		return
	}
	files := r.MultipartForm.File[formField]
	if len(files) == 0 {
		return
	}

	dir := filepath.Join("files", assessment.ID.Hex(), tc.ID.Hex())
	os.MkdirAll(dir, 0755)

	for _, fh := range files {
		if fh.Filename == "" {
			continue
		}
		safeName := sanitizeUploadFilename(fh.Filename)
		destPath := filepath.Join(dir, safeName)

		src, err := fh.Open()
		if err != nil {
			continue
		}

		dst, err := os.Create(destPath)
		if err != nil {
			src.Close()
			continue
		}

		io.Copy(dst, src)
		src.Close()
		dst.Close()

		*existingFiles = append(*existingFiles, models.FileDoc{
			Name:    safeName,
			Path:    destPath,
			Caption: "",
		})
	}
}

func sanitizeUploadFilename(name string) string {
	// Use only the base name to prevent directory traversal
	name = filepath.Base(name)
	// Replace any remaining path separators
	name = strings.ReplaceAll(name, string(os.PathSeparator), "_")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	// Trim leading dots
	name = strings.TrimLeft(name, ".")
	if name == "" {
		name = "unnamed"
	}
	return name
}

// Sanity check helpers: extract valid IDs from assessment embedded docs

func sanitizeIDs(ids []string, validIDs map[string]bool) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if validIDs[id] {
			result = append(result, id)
		}
	}
	return result
}

func extractIDs(sources []models.Source) map[string]bool {
	m := make(map[string]bool, len(sources))
	for _, s := range sources {
		m[s.ID.Hex()] = true
	}
	return m
}

func extractTargetIDs(targets []models.Target) map[string]bool {
	m := make(map[string]bool, len(targets))
	for _, t := range targets {
		m[t.ID.Hex()] = true
	}
	return m
}

func extractToolIDs(tools []models.Tool) map[string]bool {
	m := make(map[string]bool, len(tools))
	for _, t := range tools {
		m[t.ID.Hex()] = true
	}
	return m
}

func extractControlIDs(controls []models.Control) map[string]bool {
	m := make(map[string]bool, len(controls))
	for _, c := range controls {
		m[c.ID.Hex()] = true
	}
	return m
}

func extractTagIDs(tags []models.Tag) map[string]bool {
	m := make(map[string]bool, len(tags))
	for _, t := range tags {
		m[t.ID.Hex()] = true
	}
	return m
}

func extractDatasourceIDs(datasources []models.Datasource) map[string]bool {
	m := make(map[string]bool, len(datasources))
	for _, d := range datasources {
		m[d.ID.Hex()] = true
	}
	return m
}

func extractRuleIDs(rules []models.DetectionRule) map[string]bool {
	m := make(map[string]bool, len(rules))
	for _, r := range rules {
		m[r.ID.Hex()] = true
	}
	return m
}
