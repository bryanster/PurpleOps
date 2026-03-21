package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/bryanster/purpleops/internal/auth"
	"github.com/bryanster/purpleops/internal/config"
	"github.com/bryanster/purpleops/internal/db"
	"github.com/bryanster/purpleops/internal/models"
	"github.com/bryanster/purpleops/internal/render"
	"github.com/flosch/pongo2/v6"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleAssessmentMulti updates embedded document lists on an assessment.
// POST /assessment/{id}/multi/{field}
func HandleAssessmentMulti(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	field := chi.URLParam(r, "field")

	assessment, err := models.FindAssessment(r.Context(), id)
	if err != nil {
		http.Error(w, "Assessment not found", http.StatusNotFound)
		return
	}

	var body struct {
		Data []map[string]string `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	newSource := func() *models.Source { return &models.Source{ID: bson.NewObjectID()} }
	newTarget := func() *models.Target { return &models.Target{ID: bson.NewObjectID()} }
	newTool := func() *models.Tool { return &models.Tool{ID: bson.NewObjectID()} }
	newControl := func() *models.Control { return &models.Control{ID: bson.NewObjectID()} }
	newTag := func() *models.Tag { return &models.Tag{ID: bson.NewObjectID()} }
	newDatasource := func() *models.Datasource { return &models.Datasource{ID: bson.NewObjectID()} }
	newRule := func() *models.DetectionRule { return &models.DetectionRule{ID: bson.NewObjectID()} }

	switch field {
	case "sources":
		assessment.Sources = updateItems(assessment.Sources, body.Data, newSource)
	case "targets":
		assessment.Targets = updateItems(assessment.Targets, body.Data, newTarget)
	case "tools":
		assessment.Tools = updateItems(assessment.Tools, body.Data, newTool)
	case "controls":
		assessment.Controls = updateItems(assessment.Controls, body.Data, newControl)
	case "tags":
		assessment.Tags = updateItems(assessment.Tags, body.Data, newTag)
	case "datasources":
		assessment.Datasources = updateItems(assessment.Datasources, body.Data, newDatasource)
	case "rules":
		assessment.Rules = updateItems(assessment.Rules, body.Data, newRule)
	case "detectionsources":
		assessment.DetectionSources = updateItems(assessment.DetectionSources, body.Data, newDatasource)
	case "preventionsources":
		assessment.PreventionSources = updateItems(assessment.PreventionSources, body.Data, newDatasource)
	default:
		http.Error(w, "Invalid field", http.StatusBadRequest)
		return
	}

	_, err = db.Col(db.ColAssessment).UpdateOne(r.Context(), bson.M{"_id": assessment.ID}, bson.M{
		"$set": bson.M{field: getFieldValue(assessment, field)},
	})
	if err != nil {
		http.Error(w, "Failed to update assessment", http.StatusInternalServerError)
		return
	}

	// Return the updated field items as JSON.
	result := assessment.MultiToJSON(field, false)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func getFieldValue(a *models.Assessment, field string) interface{} {
	switch field {
	case "sources":
		return a.Sources
	case "targets":
		return a.Targets
	case "tools":
		return a.Tools
	case "controls":
		return a.Controls
	case "tags":
		return a.Tags
	case "datasources":
		return a.Datasources
	case "rules":
		return a.Rules
	case "detectionsources":
		return a.DetectionSources
	case "preventionsources":
		return a.PreventionSources
	}
	return nil
}

// updateItems is a generic function that processes embedded document updates.
// Items with a "tmp-" ID prefix are created as new; others are matched against
// existing items and updated in place. PT must be a pointer to T that satisfies NamedItem.
func updateItems[T any, PT interface {
	*T
	models.NamedItem
}](existing []T, data []map[string]string, newFn func() PT) []T {
	result := make([]T, 0, len(data))
	for _, item := range data {
		id := item["id"]
		if strings.HasPrefix(id, "tmp-") {
			n := newFn()
			n.SetFields(item)
			result = append(result, *n)
		} else {
			for i := range existing {
				ep := PT(&existing[i])
				if ep.GetID().Hex() == id {
					ep.SetFields(item)
					result = append(result, existing[i])
					break
				}
			}
		}
	}
	return result
}

// Hexagon visualization constants.
const (
	hexColorYellow = "#FFC000"
	hexColorGreen  = "#B8DF43"
	hexColorRed    = "#FB6B64"

	hexWidth    = 120
	hexHeight   = 104
	hexMaxCols  = 5
	hexHalfW    = 50
	hexQuarterW = 25
	hexSideH    = 43

	// Score thresholds for hexagon coloring.
	hexScoreGreen = 1  // score > this → green
	hexScoreRed   = -1 // score < this → red
)

// Rating-to-score mapping used for prevention/detection scoring.
var ratingScores = map[string]int{
	"Critical":      5,
	"High":          4,
	"Medium":        3,
	"Low":           2,
	"Informational": 1,
}

// HandleAssessmentNavigator renders the navigator export page.
// GET /assessment/{id}/navigator
func HandleAssessmentNavigator(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	assessment, err := models.FindAssessment(r.Context(), id)
	if err != nil {
		http.Error(w, "Assessment not found", http.StatusNotFound)
		return
	}

	// Generate a one-time secret.
	secretBytes := make([]byte, 16)
	if _, err := rand.Read(secretBytes); err != nil {
		slog.Error("navigator: failed to generate secret", "err", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	secret := hex.EncodeToString(secretBytes)

	// Store as "timestamp|ip|secret" in assessment.NavigatorExport.
	timestamp := time.Now().UTC().Format(time.RFC3339)
	ip := r.RemoteAddr
	navigatorExport := fmt.Sprintf("%s|%s|%s", timestamp, ip, secret)

	_, err = db.Col(db.ColAssessment).UpdateOne(r.Context(), bson.M{"_id": assessment.ID}, bson.M{
		"$set": bson.M{"navigatorexport": navigatorExport},
	})
	if err != nil {
		http.Error(w, "Failed to update assessment", http.StatusInternalServerError)
		return
	}

	if _, err := ExportNavigatorFile(id, r); err != nil {
		http.Error(w, "Failed to export navigator", http.StatusInternalServerError)
		return
	}

	render.Render(w, r, "assessment_navigator.html", pongo2.Context{
		"assessment": assessment,
		"secret":     secret,
	})
}

// HandleNavigatorJSON serves the navigator JSON file. Unauthenticated but secret-gated.
// The caller must supply ?secret=<value> matching the token stored in assessment.NavigatorExport.
// GET /assessment/{id}/navigator.json?secret=<token>
func HandleNavigatorJSON(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	secret := r.URL.Query().Get("secret")
	if secret == "" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	assessment, err := models.FindAssessment(r.Context(), id)
	if err != nil || assessment == nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// NavigatorExport is stored as "timestamp|ip|secret"
	parts := strings.SplitN(assessment.NavigatorExport, "|", 3)
	if len(parts) != 3 || parts[2] != secret {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	filePath := filepath.Join("files", id, "navigator.json")
	origin := r.Header.Get("Origin")
	allowed := config.Cfg.NavigatorCORSOrigin
	if origin != "" && allowed != "" && origin == allowed {
		w.Header().Set("Access-Control-Allow-Origin", allowed)
	}
	w.Header().Set("Content-Type", "application/json")
	http.ServeFile(w, r, filePath)
}

// HandleAssessmentStats renders the assessment statistics page.
// GET /assessment/{id}/stats
func HandleAssessmentStats(w http.ResponseWriter, r *http.Request) {
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

	// Determine if the user is blue-only (not Admin, not Red).
	user := auth.UserFromContext(r.Context())
	blueOnly := false
	if user != nil {
		isAdmin := user.HasRole(r.Context(), "Admin")
		isRed := user.HasRole(r.Context(), "Red")
		if !isAdmin && !isRed {
			blueOnly = true
		}
	}

	// Filter testcases for blue users.
	filtered := make([]models.TestCase, 0, len(testcases))
	for _, tc := range testcases {
		if blueOnly && !tc.Visible {
			continue
		}
		filtered = append(filtered, tc)
	}

	// Build stats per tactic + "All".
	stats := buildStats(filtered)

	hexagons := RenderHexagons(id)

	render.Render(w, r, "assessment_stats.html", pongo2.Context{
		"assessment": assessment,
		"stats":      stats,
		"hexagons":   hexagons,
	})
}

type tacticStats struct {
	Prevented     int
	Alerted       int
	Logged        int
	Missed        int
	Critical      int
	High          int
	Medium        int
	Low           int
	Informational int
	ScoresPrevent []int
	ScoresDetect  []int
	PriorityType  []string
	PriorityUrg   []string
	Controls      []string
}

func buildStats(testcases []models.TestCase) map[string]*tacticStats {
	stats := map[string]*tacticStats{
		"All": {},
	}

	for _, tc := range testcases {
		tactic := tc.Tactic
		if _, ok := stats[tactic]; !ok {
			stats[tactic] = &tacticStats{}
		}

		for _, key := range []string{tactic, "All"} {
			s := stats[key]

			switch tc.Outcome {
			case "Prevented":
				s.Prevented++
			case "Alerted":
				s.Alerted++
			case "Logged":
				s.Logged++
			case "Missed":
				s.Missed++
			}

			switch tc.AlertSeverity {
			case "Critical":
				s.Critical++
			case "High":
				s.High++
			case "Medium":
				s.Medium++
			case "Low":
				s.Low++
			case "Informational":
				s.Informational++
			}

			if tc.PreventedRating != "" {
				s.ScoresPrevent = append(s.ScoresPrevent, ratingToScore(tc.PreventedRating))
			}
			if tc.DetectionRating != "" {
				s.ScoresDetect = append(s.ScoresDetect, ratingToScore(tc.DetectionRating))
			}

			if tc.Priority != "" {
				s.PriorityType = append(s.PriorityType, tc.Priority)
			}
			if tc.PriorityUrgency != "" {
				s.PriorityUrg = append(s.PriorityUrg, tc.PriorityUrgency)
			}

			s.Controls = append(s.Controls, tc.Controls...)
		}
	}

	return stats
}

func ratingToScore(rating string) int {
	if s, ok := ratingScores[rating]; ok {
		return s
	}
	return 0
}

// HandleAssessmentHexagons serves the hexagon SVG visualization.
// GET /assessment/{id}/assessment_hexagons.svg
func HandleAssessmentHexagons(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	svg := RenderHexagons(id)
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Write([]byte(svg))
}

// RenderHexagons generates an SVG hexagon visualization for the assessment.
func RenderHexagons(id string) string {
	tactics := []string{
		"Execution",
		"Command and Control",
		"Discovery",
		"Persistence",
		"Privilege Escalation",
		"Credential Access",
		"Lateral Movement",
		"Exfiltration",
		"Impact",
	}

	bgCtx := context.Background()
	testcases, _ := models.GetTestCases(bgCtx, id)

	type hexData struct {
		Name  string
		Count int
		Score int
		Color string
	}

	var hexes []hexData

	for _, tactic := range tactics {
		complete := 0
		prevented := 0
		alerted := 0
		missed := 0

		for _, tc := range testcases {
			if tc.Tactic == tactic && tc.State == "Complete" {
				complete++
				switch tc.Outcome {
				case "Prevented":
					prevented++
				case "Alerted":
					alerted++
				case "Missed":
					missed++
				}
			}
		}

		if complete == 0 {
			continue
		}

		score := prevented + alerted - missed
		color := hexColorYellow
		if score > hexScoreGreen {
			color = hexColorGreen
		} else if score < hexScoreRed {
			color = hexColorRed
		}

		hexes = append(hexes, hexData{
			Name:  tactic,
			Count: complete,
			Score: score,
			Color: color,
		})
	}

	if len(hexes) == 0 {
		return ""
	}

	// Calculate dimensions based on number of hexagons.
	cols := len(hexes)
	if cols > hexMaxCols {
		cols = hexMaxCols
	}
	rows := (len(hexes) + cols - 1) / cols
	svgWidth := cols*hexWidth + 40
	svgHeight := rows*hexHeight + 60

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, svgWidth, svgHeight, svgWidth, svgHeight))
	sb.WriteString("\n")

	for i, h := range hexes {
		col := i % cols
		row := i / cols
		cx := col*hexWidth + hexWidth/2 + 20
		cy := row*hexHeight + hexHeight/2 + 30

		// Hexagon points (flat-top).
		points := fmt.Sprintf("%d,%d %d,%d %d,%d %d,%d %d,%d %d,%d",
			cx-hexHalfW, cy,
			cx-hexQuarterW, cy-hexSideH,
			cx+hexQuarterW, cy-hexSideH,
			cx+hexHalfW, cy,
			cx+hexQuarterW, cy+hexSideH,
			cx-hexQuarterW, cy+hexSideH,
		)

		sb.WriteString(fmt.Sprintf(`  <polygon points="%s" fill="%s" stroke="#333" stroke-width="2"/>`, points, h.Color))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" text-anchor="middle" font-size="10" font-family="Arial" fill="#333">%s</text>`, cx, cy-5, h.Name))
		sb.WriteString("\n")
		sb.WriteString(fmt.Sprintf(`  <text x="%d" y="%d" text-anchor="middle" font-size="12" font-weight="bold" font-family="Arial" fill="#333">%d</text>`, cx, cy+12, h.Count))
		sb.WriteString("\n")
	}

	sb.WriteString("</svg>")
	return sb.String()
}
