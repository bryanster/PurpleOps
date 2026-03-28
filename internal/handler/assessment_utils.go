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
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// HandleAssessmentMulti updates embedded document lists on an assessment.
// POST /assessment/:id/multi/:field
func HandleAssessmentMulti(c *gin.Context) {
	id := c.Param("id")
	field := c.Param("field")

	assessment, err := models.FindAssessment(c.Request.Context(), id)
	if err != nil {
		c.String(http.StatusNotFound, "Assessment not found")
		return
	}

	var body struct {
		Data []map[string]string `json:"data"`
	}
	if err := json.NewDecoder(c.Request.Body).Decode(&body); err != nil {
		c.String(http.StatusBadRequest, "Invalid JSON")
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
		c.String(http.StatusBadRequest, "Invalid field")
		return
	}

	_, err = db.Col(db.ColAssessment).UpdateOne(c.Request.Context(), bson.M{"_id": assessment.ID}, bson.M{
		"$set": bson.M{field: getFieldValue(assessment, field)},
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to update assessment")
		return
	}

	// Return the updated field items as JSON.
	result := assessment.MultiToJSON(field, false)

	c.JSON(http.StatusOK, result)
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
// GET /assessment/:id/navigator
func HandleAssessmentNavigator(c *gin.Context) {
	id := c.Param("id")

	assessment, err := models.FindAssessment(c.Request.Context(), id)
	if err != nil {
		c.String(http.StatusNotFound, "Assessment not found")
		return
	}

	// Generate a one-time secret.
	secretBytes := make([]byte, 16)
	if _, err := rand.Read(secretBytes); err != nil {
		slog.Error("navigator: failed to generate secret", "err", err)
		c.String(http.StatusInternalServerError, "Internal error")
		return
	}
	secret := hex.EncodeToString(secretBytes)

	// Store as "timestamp|ip|secret" in assessment.NavigatorExport.
	timestamp := time.Now().UTC().Format(time.RFC3339)
	ip := c.Request.RemoteAddr
	navigatorExport := fmt.Sprintf("%s|%s|%s", timestamp, ip, secret)

	_, err = db.Col(db.ColAssessment).UpdateOne(c.Request.Context(), bson.M{"_id": assessment.ID}, bson.M{
		"$set": bson.M{"navigatorexport": navigatorExport},
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to update assessment")
		return
	}

	if _, err := ExportNavigatorFile(id, c.Request); err != nil {
		c.String(http.StatusInternalServerError, "Failed to export navigator")
		return
	}

	render.Render(c.Writer, c.Request, "assessment_navigator.html", pongo2.Context{
		"assessment": assessment,
		"secret":     secret,
	})
}

// HandleNavigatorJSON serves the navigator JSON file. Unauthenticated but secret-gated.
// The caller must supply ?secret=<value> matching the token stored in assessment.NavigatorExport.
// GET /assessment/:id/navigator.json?secret=<token>
func HandleNavigatorJSON(c *gin.Context) {
	id := c.Param("id")
	secret := c.Request.URL.Query().Get("secret")
	if secret == "" {
		c.String(http.StatusForbidden, "Forbidden")
		return
	}

	assessment, err := models.FindAssessment(c.Request.Context(), id)
	if err != nil || assessment == nil {
		c.String(http.StatusNotFound, "Not found")
		return
	}

	// NavigatorExport is stored as "timestamp|ip|secret"
	parts := strings.SplitN(assessment.NavigatorExport, "|", 3)
	if len(parts) != 3 || parts[2] != secret {
		c.String(http.StatusForbidden, "Forbidden")
		return
	}

	filePath := filepath.Join("files", id, "navigator.json")
	origin := c.Request.Header.Get("Origin")
	allowed := config.Cfg.NavigatorCORSOrigin
	if origin != "" && allowed != "" && origin == allowed {
		c.Header("Access-Control-Allow-Origin", allowed)
	}
	c.Header("Content-Type", "application/json")
	http.ServeFile(c.Writer, c.Request, filePath)
}

// HandleAssessmentStats renders the assessment statistics page.
// GET /assessment/:id/stats
func HandleAssessmentStats(c *gin.Context) {
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

	// Determine if the user is blue-only (not Admin, not Red).
	user := auth.UserFromContext(c.Request.Context())
	blueOnly := false
	if user != nil {
		isAdmin := user.HasRole(c.Request.Context(), "Admin")
		isRed := user.HasRole(c.Request.Context(), "Red")
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

	render.Render(c.Writer, c.Request, "assessment_stats.html", pongo2.Context{
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
// GET /assessment/:id/assessment_hexagons.svg
func HandleAssessmentHexagons(c *gin.Context) {
	id := c.Param("id")
	svg := RenderHexagons(id)
	c.Header("Content-Type", "image/svg+xml")
	c.Writer.Write([]byte(svg))
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
	fmt.Fprintf(&sb, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, svgWidth, svgHeight, svgWidth, svgHeight)
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

		fmt.Fprintf(&sb, `  <polygon points="%s" fill="%s" stroke="#333" stroke-width="2"/>`, points, h.Color)
		sb.WriteString("\n")
		fmt.Fprintf(&sb, `  <text x="%d" y="%d" text-anchor="middle" font-size="10" font-family="Arial" fill="#333">%s</text>`, cx, cy-5, h.Name)
		sb.WriteString("\n")
		fmt.Fprintf(&sb, `  <text x="%d" y="%d" text-anchor="middle" font-size="12" font-weight="bold" font-family="Arial" fill="#333">%d</text>`, cx, cy+12, h.Count)
		sb.WriteString("\n")
	}

	sb.WriteString("</svg>")
	return sb.String()
}
