package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/flosch/pongo2/v6"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleAssessmentMulti updates embedded document lists on an assessment.
// POST /assessment/{id}/multi/{field}
func HandleAssessmentMulti(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	field := chi.URLParam(r, "field")

	assessment, err := FindAssessment(r.Context(), id)
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

	switch field {
	case "sources":
		assessment.Sources = updateSources(assessment.Sources, body.Data)
	case "targets":
		assessment.Targets = updateTargets(assessment.Targets, body.Data)
	case "tools":
		assessment.Tools = updateTools(assessment.Tools, body.Data)
	case "controls":
		assessment.Controls = updateControls(assessment.Controls, body.Data)
	case "tags":
		assessment.Tags = updateTags(assessment.Tags, body.Data)
	case "datasources":
		assessment.Datasources = updateDatasources(assessment.Datasources, body.Data)
	case "rules":
		assessment.Rules = updateDetectionRules(assessment.Rules, body.Data)
	case "detectionsources":
		assessment.DetectionSources = updateDatasources(assessment.DetectionSources, body.Data)
	case "preventionsources":
		assessment.PreventionSources = updateDatasources(assessment.PreventionSources, body.Data)
	default:
		http.Error(w, "Invalid field", http.StatusBadRequest)
		return
	}

	_, err = Col("assessment").UpdateOne(r.Context(), bson.M{"_id": assessment.ID}, bson.M{
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

func getFieldValue(a *Assessment, field string) interface{} {
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

func updateSources(existing []Source, data []map[string]string) []Source {
	result := make([]Source, 0, len(data))
	for _, item := range data {
		id := item["id"]
		if strings.HasPrefix(id, "tmp-") {
			result = append(result, Source{
				ID:          bson.NewObjectID(),
				Name:        item["name"],
				Description: item["description"],
			})
		} else {
			// Find existing and update.
			for _, s := range existing {
				if s.ID.Hex() == id {
					s.Name = item["name"]
					s.Description = item["description"]
					result = append(result, s)
					break
				}
			}
		}
	}
	return result
}

func updateTargets(existing []Target, data []map[string]string) []Target {
	result := make([]Target, 0, len(data))
	for _, item := range data {
		id := item["id"]
		if strings.HasPrefix(id, "tmp-") {
			result = append(result, Target{
				ID:          bson.NewObjectID(),
				Name:        item["name"],
				Description: item["description"],
			})
		} else {
			for _, t := range existing {
				if t.ID.Hex() == id {
					t.Name = item["name"]
					t.Description = item["description"]
					result = append(result, t)
					break
				}
			}
		}
	}
	return result
}

func updateTools(existing []Tool, data []map[string]string) []Tool {
	result := make([]Tool, 0, len(data))
	for _, item := range data {
		id := item["id"]
		if strings.HasPrefix(id, "tmp-") {
			result = append(result, Tool{
				ID:          bson.NewObjectID(),
				Name:        item["name"],
				Description: item["description"],
			})
		} else {
			for _, t := range existing {
				if t.ID.Hex() == id {
					t.Name = item["name"]
					t.Description = item["description"]
					result = append(result, t)
					break
				}
			}
		}
	}
	return result
}

func updateControls(existing []Control, data []map[string]string) []Control {
	result := make([]Control, 0, len(data))
	for _, item := range data {
		id := item["id"]
		if strings.HasPrefix(id, "tmp-") {
			result = append(result, Control{
				ID:          bson.NewObjectID(),
				Name:        item["name"],
				Description: item["description"],
			})
		} else {
			for _, c := range existing {
				if c.ID.Hex() == id {
					c.Name = item["name"]
					c.Description = item["description"]
					result = append(result, c)
					break
				}
			}
		}
	}
	return result
}

func updateTags(existing []Tag, data []map[string]string) []Tag {
	result := make([]Tag, 0, len(data))
	for _, item := range data {
		id := item["id"]
		if strings.HasPrefix(id, "tmp-") {
			result = append(result, Tag{
				ID:     bson.NewObjectID(),
				Name:   item["name"],
				Colour: item["colour"],
			})
		} else {
			for _, t := range existing {
				if t.ID.Hex() == id {
					t.Name = item["name"]
					t.Colour = item["colour"]
					result = append(result, t)
					break
				}
			}
		}
	}
	return result
}

func updateDatasources(existing []Datasource, data []map[string]string) []Datasource {
	result := make([]Datasource, 0, len(data))
	for _, item := range data {
		id := item["id"]
		if strings.HasPrefix(id, "tmp-") {
			result = append(result, Datasource{
				ID:          bson.NewObjectID(),
				Name:        item["name"],
				Description: item["description"],
			})
		} else {
			for _, d := range existing {
				if d.ID.Hex() == id {
					d.Name = item["name"]
					d.Description = item["description"]
					result = append(result, d)
					break
				}
			}
		}
	}
	return result
}

func updateDetectionRules(existing []DetectionRule, data []map[string]string) []DetectionRule {
	result := make([]DetectionRule, 0, len(data))
	for _, item := range data {
		id := item["id"]
		if strings.HasPrefix(id, "tmp-") {
			result = append(result, DetectionRule{
				ID:          bson.NewObjectID(),
				Name:        item["name"],
				Description: item["description"],
			})
		} else {
			for _, r := range existing {
				if r.ID.Hex() == id {
					r.Name = item["name"]
					r.Description = item["description"]
					result = append(result, r)
					break
				}
			}
		}
	}
	return result
}

// HandleAssessmentNavigator renders the navigator export page.
// GET /assessment/{id}/navigator
func HandleAssessmentNavigator(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	assessment, err := FindAssessment(r.Context(), id)
	if err != nil {
		http.Error(w, "Assessment not found", http.StatusNotFound)
		return
	}

	// Generate a one-time secret.
	secretBytes := make([]byte, 16)
	rand.Read(secretBytes)
	secret := hex.EncodeToString(secretBytes)

	// Store as "timestamp|ip|secret" in assessment.NavigatorExport.
	timestamp := time.Now().UTC().Format(time.RFC3339)
	ip := r.RemoteAddr
	navigatorExport := fmt.Sprintf("%s|%s|%s", timestamp, ip, secret)

	_, err = Col("assessment").UpdateOne(r.Context(), bson.M{"_id": assessment.ID}, bson.M{
		"$set": bson.M{"navigatorexport": navigatorExport},
	})
	if err != nil {
		http.Error(w, "Failed to update assessment", http.StatusInternalServerError)
		return
	}

	ExportNavigatorFile(id, r)

	Render(w, r, "assessment_navigator.html", pongo2.Context{
		"assessment": assessment,
		"secret":     secret,
	})
}

// HandleNavigatorJSON serves the navigator JSON file. Unauthenticated.
// GET /assessment/{id}/navigator.json
func HandleNavigatorJSON(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	filePath := filepath.Join("files", id, "navigator.json")

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
	http.ServeFile(w, r, filePath)
}

// HandleAssessmentStats renders the assessment statistics page.
// GET /assessment/{id}/stats
func HandleAssessmentStats(w http.ResponseWriter, r *http.Request) {
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

	// Determine if the user is blue-only (not Admin, not Red).
	user := UserFromContext(r.Context())
	blueOnly := false
	if user != nil {
		isAdmin := user.HasRole(r.Context(), "Admin")
		isRed := user.HasRole(r.Context(), "Red")
		if !isAdmin && !isRed {
			blueOnly = true
		}
	}

	// Filter testcases for blue users.
	filtered := make([]TestCase, 0, len(testcases))
	for _, tc := range testcases {
		if blueOnly && !tc.Visible {
			continue
		}
		filtered = append(filtered, tc)
	}

	// Build stats per tactic + "All".
	stats := buildStats(filtered)

	hexagons := RenderHexagons(id)

	Render(w, r, "assessment_stats.html", pongo2.Context{
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

func buildStats(testcases []TestCase) map[string]*tacticStats {
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

			for _, c := range tc.Controls {
				s.Controls = append(s.Controls, c)
			}
		}
	}

	return stats
}

func ratingToScore(rating string) int {
	switch rating {
	case "Critical":
		return 5
	case "High":
		return 4
	case "Medium":
		return 3
	case "Low":
		return 2
	case "Informational":
		return 1
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
	testcases, _ := GetTestCases(bgCtx, id)

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
		color := "#FFC000" // yellow
		if score > 1 {
			color = "#B8DF43" // green
		} else if score < -1 {
			color = "#FB6B64" // red
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
	hexWidth := 120
	hexHeight := 104
	cols := len(hexes)
	if cols > 5 {
		cols = 5
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
			cx-50, cy,
			cx-25, cy-43,
			cx+25, cy-43,
			cx+50, cy,
			cx+25, cy+43,
			cx-25, cy+43,
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
